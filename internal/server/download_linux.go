//go:build linux

package server

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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

// downloadAndExtractOnnxRuntime downloads the ONNX Runtime tgz and extracts
// the lib/ directory contents into destDir.
func downloadAndExtractOnnxRuntime(destDir string, progress ProgressFunc) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil { //nolint:gosec // standard data directory permissions
		return fmt.Errorf("create onnx dir: %w", err)
	}

	url := onnxRuntimeURL()
	resp, err := downloadClient.Get(url)
	if err != nil {
		return fmt.Errorf("download onnxruntime: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
		dest := filepath.Clean(filepath.Join(destDir, filename))
		// Validate dest stays within destDir to prevent path traversal
		if !strings.HasPrefix(dest, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("tar entry %q escapes destination directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeSymlink:
			// Use only the base name of the link target to prevent path traversal.
			// ONNX Runtime symlinks are same-directory (e.g. libonnxruntime.so -> libonnxruntime.so.1.24.2).
			linkTarget := filepath.Base(hdr.Linkname)
			// Recreate symlinks
			_ = os.Remove(dest)
			if err := os.Symlink(linkTarget, dest); err != nil {
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
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)) //nolint:gosec // dest validated above, mode from trusted archive
			if err != nil {
				return fmt.Errorf("create %s: %w", filename, err)
			}
			if _, err := io.Copy(out, io.LimitReader(tr, limit+1)); err != nil {
				_ = out.Close()
				return fmt.Errorf("extract %s: %w", filename, err)
			}
			_ = out.Close()
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
