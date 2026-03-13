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
	switch runtime.GOARCH {
	case "arm64":
		platform = "linux-aarch64"
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

		// Sanitize: only use the base filename, reject any path separators or traversal
		filename := filepath.Base(relPath)
		if filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeSymlink:
			// ONNX Runtime symlinks are same-directory (e.g. libonnxruntime.so -> libonnxruntime.so.1.24.2).
			// Verify both destination and resolved target stay within destDir.
			safeDest := filepath.Clean(filepath.Join(destDir, filename))
			if !strings.HasPrefix(safeDest, filepath.Clean(destDir)+string(os.PathSeparator)) {
				return fmt.Errorf("symlink %s destination escapes target directory", filename)
			}
			// Resolve the link target relative to the symlink's directory and verify containment
			resolvedTarget := filepath.Clean(filepath.Join(filepath.Dir(safeDest), hdr.Linkname))
			if !strings.HasPrefix(resolvedTarget, filepath.Clean(destDir)+string(os.PathSeparator)) {
				return fmt.Errorf("symlink %s target %q escapes target directory", filename, hdr.Linkname)
			}
			// Compute a safe relative target from the verified absolute path
			safeLink, err := filepath.Rel(filepath.Dir(safeDest), resolvedTarget)
			if err != nil {
				return fmt.Errorf("symlink %s: compute relative target: %w", filename, err)
			}
			_ = os.Remove(safeDest)
			if err := os.Symlink(safeLink, safeDest); err != nil {
				return fmt.Errorf("symlink %s: %w", filename, err)
			}
		default:
			// Limit extraction size and detect oversized entries.
			const maxFileSize = 500 * 1024 * 1024 // 500 MB safety cap
			limit := hdr.Size
			if limit <= 0 || limit > maxFileSize {
				limit = maxFileSize
			}
			safeDest := filepath.Clean(filepath.Join(destDir, filename))
			if !strings.HasPrefix(safeDest, filepath.Clean(destDir)+string(os.PathSeparator)) {
				return fmt.Errorf("file %s escapes target directory", filename)
			}
			out, err := os.OpenFile(safeDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)) //nolint:gosec // mode from trusted ONNX Runtime archive
			if err != nil {
				return fmt.Errorf("create %s: %w", filename, err)
			}
			n, err := io.Copy(out, io.LimitReader(tr, limit+1))
			if err != nil {
				_ = out.Close()
				return fmt.Errorf("extract %s: %w", filename, err)
			}
			if n > limit {
				_ = out.Close()
				_ = os.Remove(safeDest)
				return fmt.Errorf("extract %s: file exceeds size limit (%d bytes)", filename, limit)
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
