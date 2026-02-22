//go:build darwin

package server

import (
	"fmt"
	"io"
	"os"
)

func libExtension() string {
	return ".dylib"
}

func libraryPathEnvVar() string {
	return "DYLD_LIBRARY_PATH"
}

func parakeetAvailable() bool {
	return false
}

// verifyBinary checks that a file starts with valid Mach-O magic bytes.
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

	// Check for Mach-O magic bytes:
	// 0xFEEDFACF = 64-bit Mach-O (little-endian, ARM64 or x86_64)
	// 0xFEEDFACE = 32-bit Mach-O (little-endian)
	// 0xCAFEBABE = Universal binary (fat binary)
	// 0xBEBAFECA = Universal binary (little-endian representation)
	if (magic[0] == 0xCF && magic[1] == 0xFA && magic[2] == 0xED && magic[3] == 0xFE) || // 64-bit LE
		(magic[0] == 0xCE && magic[1] == 0xFA && magic[2] == 0xED && magic[3] == 0xFE) || // 32-bit LE
		(magic[0] == 0xCA && magic[1] == 0xFE && magic[2] == 0xBA && magic[3] == 0xBE) || // Universal BE
		(magic[0] == 0xBE && magic[1] == 0xBA && magic[2] == 0xFE && magic[3] == 0xCA) { // Universal LE
		return nil
	}

	return fmt.Errorf("not a valid Mach-O binary (got %x)", magic)
}

// systemOnnxRuntimeAvailable returns false on macOS (no ldconfig).
func systemOnnxRuntimeAvailable() bool {
	return false
}
