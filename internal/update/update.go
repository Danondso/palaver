package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRepo    = "Danondso/palaver"
	defaultAPIURL  = "https://api.github.com"
	maxAPIBytes    = 1 << 20 // 1 MB
	maxArchiveSize = 50 << 20
	maxBinarySize  = 50 << 20
	httpTimeout    = 5 * time.Minute
)

// Options configures a self-update.
type Options struct {
	CurrentVersion string
	InstallPath    string
	Repo           string
	APIURL         string
	Client         *http.Client
	Stdout         io.Writer
	GOOS           string
	GOARCH         string
}

// Result reports the outcome of an update attempt.
type Result struct {
	Updated bool
	Version string
	Path    string
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

// Run checks GitHub Releases for a newer palaver build, replaces the installed
// binary, and leaves config and model data in place.
func Run(opts Options) (Result, error) {
	opts = applyDefaults(opts)
	out := opts.Stdout

	writeln(out, "=== Palaver Update ===")
	writeln(out)

	rel, err := fetchLatest(opts)
	if err != nil {
		return Result{}, err
	}

	current := displayVersion(opts.CurrentVersion)
	writef(out, "Current version: %s\n", current)
	writef(out, "Latest version:  %s\n", rel.TagName)

	dest, err := installPath(opts)
	if err != nil {
		return Result{}, err
	}

	if !needsUpdate(opts.CurrentVersion, rel.TagName) {
		writeln(out)
		writef(out, "Already up to date (%s).\n", rel.TagName)
		return Result{Updated: false, Version: rel.TagName, Path: dest}, nil
	}

	assetName := archiveName(rel.TagName, opts.GOOS, opts.GOARCH)
	asset, err := findAsset(rel.Assets, assetName)
	if err != nil {
		return Result{}, err
	}

	writeln(out)
	writef(out, "Downloading %s...\n", asset.Name)

	tmpDir, err := os.MkdirTemp("", "palaver-update-")
	if err != nil {
		return Result{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, filepath.Base(asset.Name))
	if err := downloadVerified(opts, asset, archivePath); err != nil {
		return Result{}, err
	}

	extracted := filepath.Join(tmpDir, "palaver")
	if err := extractBinary(archivePath, extracted); err != nil {
		return Result{}, err
	}
	if err := verifyBinary(extracted, opts.GOOS); err != nil {
		return Result{}, err
	}

	writef(out, "Replacing %s...\n", dest)
	if err := replaceBinary(extracted, dest); err != nil {
		return Result{}, err
	}

	writeln(out)
	writef(out, "Installed %s to %s\n", rel.TagName, dest)
	writeln(out, "Config and model files were kept.")
	return Result{Updated: true, Version: rel.TagName, Path: dest}, nil
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

func applyDefaults(opts Options) Options {
	if opts.Repo == "" {
		opts.Repo = defaultRepo
	}
	if opts.APIURL == "" {
		opts.APIURL = defaultAPIURL
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: httpTimeout}
	}
	return opts
}

func displayVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" || v == "(devel)" {
		return "dev (source build)"
	}
	return v
}

func needsUpdate(current, latest string) bool {
	c := normalizeVersion(current)
	if c == "" || c == "dev" || c == "(devel)" {
		return true
	}
	return c != normalizeVersion(latest)
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func archiveName(tag, goos, goarch string) string {
	return fmt.Sprintf("palaver_%s_%s_%s.tar.gz", tag, goos, goarch)
}

func installPath(opts Options) (string, error) {
	if opts.InstallPath != "" {
		return opts.InstallPath, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate current binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if isEphemeral(exe) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".local", "bin", "palaver"), nil
	}
	return exe, nil
}

func isEphemeral(path string) bool {
	if strings.HasSuffix(path, ".test") {
		return true
	}
	sep := string(filepath.Separator)
	if strings.Contains(path, sep+"go-build") {
		return true
	}
	tmp := os.TempDir()
	rel, err := filepath.Rel(tmp, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != "" && !strings.HasPrefix(rel, ".."))
}

func fetchLatest(opts Options) (*githubRelease, error) {
	apiURL := strings.TrimRight(opts.APIURL, "/") + "/repos/" + opts.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "palaver/"+strings.TrimSpace(opts.CurrentVersion))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := opts.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIBytes))
	if err != nil {
		return nil, fmt.Errorf("read latest release: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch latest release: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("latest release is missing a tag name")
	}
	return &rel, nil
}

func findAsset(assets []githubAsset, name string) (githubAsset, error) {
	for _, a := range assets {
		if a.Name == name {
			return a, nil
		}
	}
	return githubAsset{}, fmt.Errorf("latest release has no asset %q for this platform", name)
}

func downloadVerified(opts Options, asset githubAsset, dest string) error {
	u, err := url.Parse(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("parse download URL: %w", err)
	}
	if !allowedDownloadURL(u, opts.APIURL) {
		return fmt.Errorf("refusing download from untrusted URL %s", asset.BrowserDownloadURL)
	}

	wantDigest, err := parseSHA256Digest(asset.Digest)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", "palaver/"+strings.TrimSpace(opts.CurrentVersion))

	resp, err := opts.Client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", asset.Name, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { //nolint:gosec // temp directory created by MkdirTemp
		return fmt.Errorf("create download dir: %w", err)
	}
	f, err := os.Create(dest) //nolint:gosec // dest is a temp path constructed internally
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	defer func() { _ = f.Close() }()

	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, maxArchiveSize+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", asset.Name, err)
	}
	if n > maxArchiveSize {
		return fmt.Errorf("download %s: archive exceeds %d byte limit", asset.Name, maxArchiveSize)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close download file: %w", err)
	}

	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, wantDigest) {
		_ = os.Remove(dest)
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", asset.Name, got, wantDigest)
	}
	writef(opts.Stdout, "  SHA256: %s\n", got)
	return nil
}

func parseSHA256Digest(digest string) (string, error) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return "", fmt.Errorf("release asset is missing a SHA256 digest")
	}
	algo, hexDigest, ok := strings.Cut(digest, ":")
	if !ok || !strings.EqualFold(algo, "sha256") || hexDigest == "" {
		return "", fmt.Errorf("unsupported asset digest %q (want sha256:...)", digest)
	}
	if _, err := hex.DecodeString(hexDigest); err != nil {
		return "", fmt.Errorf("invalid SHA256 digest %q: %w", digest, err)
	}
	return strings.ToLower(hexDigest), nil
}

func allowedDownloadURL(u *url.URL, apiURL string) bool {
	if u == nil || u.Host == "" {
		return false
	}
	host := u.Hostname()
	if apiURL != "" {
		if au, err := url.Parse(apiURL); err == nil && au.Hostname() == host {
			return allowedScheme(u)
		}
	}
	switch {
	case host == "github.com":
		return u.Scheme == "https"
	case host == "objects.githubusercontent.com", host == "release-assets.githubusercontent.com":
		return u.Scheme == "https"
	case strings.HasSuffix(host, ".githubusercontent.com"):
		return u.Scheme == "https"
	default:
		return false
	}
}

func allowedScheme(u *url.URL) bool {
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}

func extractBinary(archivePath, dest string) error {
	f, err := os.Open(archivePath) //nolint:gosec // archivePath is a temp file we just downloaded
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		name := filepath.ToSlash(hdr.Name)
		name = strings.TrimPrefix(name, "./")
		if name != "palaver" {
			continue
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return fmt.Errorf("refusing to extract linked archive entry %q", hdr.Name)
		}
		// Typeflag 0 is the historic NUL regular-file marker (formerly tar.TypeRegA).
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != 0 {
			return fmt.Errorf("archive entry %q is not a regular file", hdr.Name)
		}

		limit := hdr.Size
		if limit <= 0 || limit > maxBinarySize {
			limit = maxBinarySize
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // dest is a temp path constructed internally
		if err != nil {
			return fmt.Errorf("create extracted binary: %w", err)
		}
		n, copyErr := io.Copy(out, io.LimitReader(tr, limit+1))
		closeErr := out.Close()
		if copyErr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("extract palaver: %w", copyErr)
		}
		if n > limit {
			_ = os.Remove(dest)
			return fmt.Errorf("extract palaver: file exceeds size limit (%d bytes)", limit)
		}
		if closeErr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("close extracted binary: %w", closeErr)
		}
		return nil
	}
	return fmt.Errorf("archive does not contain a palaver binary")
}

func verifyBinary(path, goos string) error {
	f, err := os.Open(path) //nolint:gosec // path is a temp file we just extracted
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("read binary magic: %w", err)
	}
	switch goos {
	case "linux":
		if magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
			return nil
		}
		return fmt.Errorf("downloaded file is not an ELF binary (got %x)", magic)
	case "darwin":
		if isMachO(magic) {
			return nil
		}
		return fmt.Errorf("downloaded file is not a Mach-O binary (got %x)", magic)
	default:
		return fmt.Errorf("unsupported platform %s", goos)
	}
}

func isMachO(magic []byte) bool {
	if len(magic) < 4 {
		return false
	}
	switch {
	case magic[0] == 0xfe && magic[1] == 0xed && magic[2] == 0xfa && (magic[3] == 0xce || magic[3] == 0xcf):
		return true
	case (magic[0] == 0xce || magic[0] == 0xcf) && magic[1] == 0xfa && magic[2] == 0xed && magic[3] == 0xfe:
		return true
	case magic[0] == 0xca && magic[1] == 0xfe && magic[2] == 0xba && magic[3] == 0xbe:
		return true
	case magic[0] == 0xbe && magic[1] == 0xba && magic[2] == 0xfe && magic[3] == 0xca:
		return true
	default:
		return false
	}
}

func replaceBinary(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { //nolint:gosec // user-local install directory
		return fmt.Errorf("create install dir: %w", err)
	}

	in, err := os.Open(src) //nolint:gosec // src is a temp file we just extracted
	if err != nil {
		return fmt.Errorf("open new binary: %w", err)
	}
	defer func() { _ = in.Close() }()

	tmp := dest + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // tmp is dest+".new" in the install directory
	if err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync new binary: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close new binary: %w", err)
	}

	old := dest + ".old"
	_ = os.Remove(old)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("move old binary aside: %w", err)
		}
	} else if !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return fmt.Errorf("stat existing binary: %w", err)
	}

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Rename(old, dest)
		_ = os.Remove(tmp)
		return fmt.Errorf("install new binary: %w", err)
	}
	_ = os.Remove(old)
	return nil
}
