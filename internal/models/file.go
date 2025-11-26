package models

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"
)

// FileEntry represents a file in the index
type FileEntry struct {
	ID           int64     `json:"id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Checksum     string    `json:"checksum"`
	IndexID      string    `json:"index_id"`      // Identifier for the index (e.g., machine name + drive)
	LastScanned  time.Time `json:"last_scanned"`
	IsDirectory  bool      `json:"is_directory"`
	RelativePath string    `json:"relative_path"` // Path relative to the indexed root
}

// FileInfo wraps os.FileInfo with additional metadata
type FileInfo struct {
	os.FileInfo
	Path string
}

// CalculateChecksum computes SHA256 hash of file contents
func CalculateChecksum(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// Index represents a collection of files from a specific location
type Index struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RootPath    string    `json:"root_path"`
	CreatedAt   time.Time `json:"created_at"`
	LastSync    time.Time `json:"last_sync"`
	MachineID   string    `json:"machine_id"`
	TotalFiles  int64     `json:"total_files"`
	TotalSize   int64     `json:"total_size"`
}

