//go:build linux

package server

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func libExtension() string {
	return ".so"
}

func libraryPathEnvVar() string {
	return "LD_LIBRARY_PATH"
}

func parakeetAvailable() bool {
	return true
}

// verifyBinary checks that a file starts with the ELF magic bytes, providing
// a basic integrity check that the downloaded binary is a valid executable.
func verifyBinary(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("read magic bytes: %w", err)
	}
	// ELF magic: 0x7f 'E' 'L' 'F'
	if magic[0] != 0x7f || magic[1] != 'E' || magic[2] != 'L' || magic[3] != 'F' {
		return fmt.Errorf("not a valid ELF binary (got %x)", magic)
	}
	return nil
}

// systemOnnxRuntimeAvailable checks if ONNX Runtime is available system-wide via ldconfig.
func systemOnnxRuntimeAvailable() bool {
	out, err := exec.Command("ldconfig", "-p").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "libonnxruntime.so") {
				return true
			}
		}
	}
	return false
}
