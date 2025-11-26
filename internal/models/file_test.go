package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateChecksum(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate checksum
	checksum, err := CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("CalculateChecksum failed: %v", err)
	}

	// Verify checksum is not empty
	if checksum == "" {
		t.Error("Expected non-empty checksum")
	}

	// Verify checksum is consistent
	checksum2, err := CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("CalculateChecksum failed on second call: %v", err)
	}

	if checksum != checksum2 {
		t.Error("Checksum should be consistent")
	}

	// Verify checksum length (SHA256 produces 64 hex characters)
	if len(checksum) != 64 {
		t.Errorf("Expected checksum length 64, got %d", len(checksum))
	}
}

func TestCalculateChecksum_NonExistentFile(t *testing.T) {
	_, err := CalculateChecksum("/nonexistent/file/path")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestCalculateChecksum_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	checksum1, err := CalculateChecksum(file1)
	if err != nil {
		t.Fatalf("Failed to calculate checksum1: %v", err)
	}

	checksum2, err := CalculateChecksum(file2)
	if err != nil {
		t.Fatalf("Failed to calculate checksum2: %v", err)
	}

	if checksum1 == checksum2 {
		t.Error("Different content should produce different checksums")
	}
}

