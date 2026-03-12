package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// downloadClient is a shared HTTP client with a timeout for all download operations.
// The 10-minute timeout accommodates large model files on slower connections.
var downloadClient = &http.Client{
	Timeout: 10 * time.Minute,
}

// ProgressFunc is called during downloads with the stage name and bytes downloaded/total.
type ProgressFunc func(stage string, downloaded, total int64)

// downloadFile downloads a URL to a local path, calling progress on each chunk.
// It writes to a temporary file first and renames on completion (atomic).
// Returns the SHA256 hex digest of the downloaded file.
func downloadFile(url, dest string, progress ProgressFunc, stage string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { //nolint:gosec // standard data directory permissions
		return "", fmt.Errorf("create dir: %w", err)
	}

	resp, err := downloadClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp) //nolint:gosec // temp file path constructed internally
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmp) // clean up on error; no-op if renamed
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
