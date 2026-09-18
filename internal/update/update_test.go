package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNeedsUpdate(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{current: "v1.1.14", latest: "v1.1.14", want: false},
		{current: "1.1.14", latest: "v1.1.14", want: false},
		{current: "v1.1.13", latest: "v1.1.14", want: true},
		{current: "dev", latest: "v1.1.14", want: true},
		{current: "(devel)", latest: "v1.1.14", want: true},
		{current: "", latest: "v1.1.14", want: true},
	}
	for _, tt := range tests {
		if got := needsUpdate(tt.current, tt.latest); got != tt.want {
			t.Errorf("needsUpdate(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestArchiveName(t *testing.T) {
	got := archiveName("v1.1.14", "darwin", "arm64")
	want := "palaver_v1.1.14_darwin_arm64.tar.gz"
	if got != want {
		t.Errorf("archiveName = %q, want %q", got, want)
	}
}

func TestParseSHA256Digest(t *testing.T) {
	got, err := parseSHA256Digest("sha256:e8ec7018ecbe82513e678a99674c960f84e35f9a193fd37b1ea1e15d4455d71e")
	if err != nil {
		t.Fatalf("parseSHA256Digest: %v", err)
	}
	if got != "e8ec7018ecbe82513e678a99674c960f84e35f9a193fd37b1ea1e15d4455d71e" {
		t.Errorf("digest = %q", got)
	}

	if _, err := parseSHA256Digest(""); err == nil {
		t.Fatal("expected error for empty digest")
	}
	if _, err := parseSHA256Digest("md5:abc"); err == nil {
		t.Fatal("expected error for non-sha256 digest")
	}
}

func TestAllowedDownloadURL(t *testing.T) {
	api := "http://127.0.0.1:1234"
	tests := []struct {
		raw  string
		want bool
	}{
		{"https://github.com/Danondso/palaver/releases/download/v1.1.14/palaver_v1.1.14_linux_amd64.tar.gz", true},
		{"https://objects.githubusercontent.com/github-production-release-asset-2e65be/foo", true},
		{"https://evil.example/palaver.tar.gz", false},
		{"http://github.com/Danondso/palaver/releases/download/v1/x.tar.gz", false},
		{"http://127.0.0.1:1234/palaver.tar.gz", true},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.raw, err)
		}
		if got := allowedDownloadURL(u, api); got != tt.want {
			t.Errorf("allowedDownloadURL(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestExtractBinaryRejectsTraversalAndSymlinks(t *testing.T) {
	dir := t.TempDir()

	traversal := filepath.Join(dir, "traversal.tar.gz")
	writeTarGz(t, traversal, tarEntry{name: "../evil", body: []byte("nope"), mode: 0o644, typeflag: tar.TypeReg})
	if err := extractBinary(traversal, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected error for traversal archive without palaver entry")
	}

	symlink := filepath.Join(dir, "symlink.tar.gz")
	writeTarGz(t, symlink, tarEntry{name: "palaver", body: nil, mode: 0o755, typeflag: tar.TypeSymlink, linkname: "/etc/passwd"})
	if err := extractBinary(symlink, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected error for symlink palaver entry")
	}
}

func TestExtractBinaryWritesPalaver(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "release.tar.gz")
	body := fakeBinary(runtime.GOOS)
	writeTarGz(t, archive,
		tarEntry{name: "README.md", body: []byte("docs"), mode: 0o644, typeflag: tar.TypeReg},
		tarEntry{name: "./palaver", body: body, mode: 0o755, typeflag: tar.TypeReg},
	)

	dest := filepath.Join(dir, "extracted")
	if err := extractBinary(archive, dest); err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	got, err := os.ReadFile(dest) //nolint:gosec // test-controlled temp path
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("extracted bytes = %q, want %q", got, body)
	}
}

func TestReplaceBinaryKeepsSiblingFiles(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "palaver")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(dir, "keep-me")
	if err := os.WriteFile(sibling, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(dir, "new")
	if err := os.WriteFile(src, []byte("new-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceBinary(src, dest); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}

	got, err := os.ReadFile(dest) //nolint:gosec // test-controlled temp path
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("dest = %q, want new-binary", got)
	}
	kept, err := os.ReadFile(sibling) //nolint:gosec // test-controlled temp path
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "config" {
		t.Fatalf("sibling overwritten: %q", kept)
	}
	if _, err := os.Stat(dest + ".old"); !os.IsNotExist(err) {
		t.Fatalf("old backup should be removed, err=%v", err)
	}
}

func TestRunAlreadyUpToDate(t *testing.T) {
	payload, _, cleanup := startReleaseServer(t, "v1.1.14", fakeBinary(runtime.GOOS))
	defer cleanup()

	var buf bytes.Buffer
	result, err := Run(Options{
		CurrentVersion: "v1.1.14",
		InstallPath:    filepath.Join(t.TempDir(), "palaver"),
		APIURL:         payload.URL,
		Client:         payload.Client,
		Stdout:         &buf,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Updated {
		t.Fatal("expected Updated=false")
	}
	if result.Version != "v1.1.14" {
		t.Fatalf("Version = %q", result.Version)
	}
	if !strings.Contains(buf.String(), "Already up to date") {
		t.Fatalf("output = %q", buf.String())
	}
}

func TestRunReplacesBinary(t *testing.T) {
	body := fakeBinary(runtime.GOOS)
	payload, assetName, cleanup := startReleaseServer(t, "v1.1.15", body)
	defer cleanup()

	dir := t.TempDir()
	dest := filepath.Join(dir, "palaver")
	if err := os.WriteFile(dest, []byte("old"), 0o700); err != nil { //nolint:gosec // test binary
		t.Fatal(err)
	}

	result, err := Run(Options{
		CurrentVersion: "v1.1.14",
		InstallPath:    dest,
		APIURL:         payload.URL,
		Client:         payload.Client,
		Stdout:         io.Discard,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Updated {
		t.Fatal("expected Updated=true")
	}
	if result.Version != "v1.1.15" {
		t.Fatalf("Version = %q", result.Version)
	}

	got, err := os.ReadFile(dest) //nolint:gosec // test-controlled temp path
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("installed bytes do not match release payload for %s", assetName)
	}
}

func TestRunChecksumMismatch(t *testing.T) {
	body := fakeBinary(runtime.GOOS)
	archive := buildReleaseArchive(t, body)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	assetName := archiveName("v1.1.15", runtime.GOOS, runtime.GOARCH)
	rel := githubRelease{
		TagName: "v1.1.15",
		Assets: []githubAsset{{
			Name:               assetName,
			BrowserDownloadURL: server.URL + "/download/" + assetName,
			Digest:             "sha256:" + strings.Repeat("ab", 32),
		}},
	}
	mux.HandleFunc("/repos/Danondso/palaver/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/download/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})

	_, err := Run(Options{
		CurrentVersion: "v1.1.14",
		InstallPath:    filepath.Join(t.TempDir(), "palaver"),
		APIURL:         server.URL,
		Client:         server.Client(),
		Stdout:         io.Discard,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
	})
	server.Close()
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestRunMissingAsset(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	rel := githubRelease{
		TagName: "v1.1.15",
		Assets: []githubAsset{{
			Name:               "palaver_v1.1.15_plan9_arm.tar.gz",
			BrowserDownloadURL: server.URL + "/download/wrong.tar.gz",
			Digest:             "sha256:" + strings.Repeat("ab", 32),
		}},
	}
	mux.HandleFunc("/repos/Danondso/palaver/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(rel)
	})

	_, err := Run(Options{
		CurrentVersion: "v1.1.14",
		InstallPath:    filepath.Join(t.TempDir(), "palaver"),
		APIURL:         server.URL,
		Client:         server.Client(),
		Stdout:         io.Discard,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
	})
	if err == nil || !strings.Contains(err.Error(), "no asset") {
		t.Fatalf("expected missing asset error, got %v", err)
	}
}

func TestIsEphemeral(t *testing.T) {
	if !isEphemeral(filepath.Join(os.TempDir(), "go-build123", "palaver")) {
		t.Fatal("temp go-build path should be ephemeral")
	}
	if !isEphemeral(filepath.Join(t.TempDir(), "update.test")) {
		t.Fatal(".test binary should be ephemeral")
	}
	homeLike := "/usr/local/bin/palaver"
	if isEphemeral(homeLike) {
		t.Fatalf("%q should not be ephemeral", homeLike)
	}
}

type tarEntry struct {
	name     string
	body     []byte
	mode     int64
	typeflag byte
	linkname string
}

func writeTarGz(t *testing.T, path string, entries ...tarEntry) {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // test-controlled temp path
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: e.typeflag,
			Linkname: e.linkname,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.typeflag == tar.TypeReg || e.typeflag == 0 {
			if _, err := tw.Write(e.body); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func fakeBinary(goos string) []byte {
	switch goos {
	case "darwin":
		return []byte{0xcf, 0xfa, 0xed, 0xfe, 0x01, 0x02, 0x03, 0x04}
	default:
		return []byte{0x7f, 'E', 'L', 'F', 0x01, 0x02, 0x03, 0x04}
	}
}

type releaseServer struct {
	URL    string
	Client *http.Client
}

func startReleaseServer(t *testing.T, tag string, binary []byte) (releaseServer, string, func()) {
	t.Helper()
	archive := buildReleaseArchive(t, binary)
	sum := sha256.Sum256(archive)
	assetName := archiveName(tag, runtime.GOOS, runtime.GOARCH)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	rel := githubRelease{
		TagName: tag,
		Assets: []githubAsset{{
			Name:               assetName,
			BrowserDownloadURL: server.URL + "/download/" + assetName,
			Digest:             "sha256:" + hex.EncodeToString(sum[:]),
		}},
	}
	mux.HandleFunc("/repos/Danondso/palaver/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/download/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})

	return releaseServer{URL: server.URL, Client: server.Client()}, assetName, server.Close
}

func buildReleaseArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "palaver",
		Mode:     0o755,
		Size:     int64(len(binary)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
