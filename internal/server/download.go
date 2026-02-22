package server

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// downloadClient is a shared HTTP client with a timeout for all download operations.
// The 10-minute timeout accommodates large model files on slower connections.
var downloadClient = &http.Client{
	Timeout: 10 * time.Minute,
}

// ProgressFunc is called during downloads with the stage name and bytes downloaded/total.
type ProgressFunc func(stage string, downloaded, total int64)

const onnxRuntimeVersion = "1.24.2"

// onnxRuntimeURL returns the GitHub release URL for the ONNX Runtime C library.
func onnxRuntimeURL() string {
	var platform string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			platform = "osx-arm64"
		} else {
			platform = "osx-x86_64"
		}
	default:
		platform = "linux-x64"
	}
	return fmt.Sprintf(
		"https://github.com/microsoft/onnxruntime/releases/download/v%s/onnxruntime-%s-%s.tgz",
		onnxRuntimeVersion, platform, onnxRuntimeVersion,
	)
}

// downloadFile downloads a URL to a local path, calling progress on each chunk.
// It writes to a temporary file first and renames on completion (atomic).
// Returns the SHA256 hex digest of the downloaded file.
func downloadFile(url, dest string, progress ProgressFunc, stage string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	resp, err := downloadClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmp) // clean up on error; no-op if renamed
	}()

	total := resp.ContentLength
	var downloaded int64
	hash := sha256.New()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return "", fmt.Errorf("write: %w", writeErr)
			}
			hash.Write(buf[:n])
			downloaded += int64(n)
			if progress != nil {
				progress(stage, downloaded, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read: %w", readErr)
		}
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close: %w", err)
	}

	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("rename: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// downloadAndExtractOnnxRuntime downloads the ONNX Runtime tgz and extracts
// the lib/ directory contents into destDir.
func downloadAndExtractOnnxRuntime(destDir string, progress ProgressFunc) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create onnx dir: %w", err)
	}

	url := onnxRuntimeURL()
	resp, err := downloadClient.Get(url)
	if err != nil {
		return fmt.Errorf("download onnxruntime: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download onnxruntime: HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	var downloaded int64

	// Wrap body in a counting reader for progress
	cr := &countingReader{
		r:          resp.Body,
		total:      total,
		progress:   progress,
		stage:      "onnxruntime",
		downloaded: &downloaded,
	}

	gz, err := gzip.NewReader(cr)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		// We only want files from the lib/ subdirectory
		// Path looks like: onnxruntime-linux-x64-X.Y.Z/lib/libonnxruntime.so.X.Y.Z
		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		relPath := parts[1]
		if !strings.HasPrefix(relPath, "lib/") {
			continue
		}

		filename := filepath.Base(relPath)
		dest := filepath.Join(destDir, filename)

		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeSymlink:
			// Validate symlink target stays within destDir to prevent path traversal
			target := filepath.Join(destDir, hdr.Linkname)
			if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) &&
				filepath.Clean(target) != filepath.Clean(destDir) {
				return fmt.Errorf("symlink %s target %q escapes destination directory", filename, hdr.Linkname)
			}
			// Recreate symlinks (e.g. libonnxruntime.so -> libonnxruntime.so.1.24.2)
			os.Remove(dest)
			if err := os.Symlink(hdr.Linkname, dest); err != nil {
				return fmt.Errorf("symlink %s: %w", filename, err)
			}
		default:
			// Limit extraction size to declared header size + 1 byte to detect overflow.
			// This prevents zip-bomb style attacks with deceptive headers.
			const maxFileSize = 500 * 1024 * 1024 // 500 MB safety cap
			limit := hdr.Size
			if limit <= 0 || limit > maxFileSize {
				limit = maxFileSize
			}
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("create %s: %w", filename, err)
			}
			if _, err := io.Copy(out, io.LimitReader(tr, limit+1)); err != nil {
				out.Close()
				return fmt.Errorf("extract %s: %w", filename, err)
			}
			out.Close()
		}
	}

	return nil
}

// countingReader wraps an io.Reader and reports progress.
type countingReader struct {
	r          io.Reader
	total      int64
	downloaded *int64
	progress   ProgressFunc
	stage      string
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	*cr.downloaded += int64(n)
	if cr.progress != nil {
		cr.progress(cr.stage, *cr.downloaded, cr.total)
	}
	return n, err
}
