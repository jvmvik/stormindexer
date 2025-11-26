package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/victor/stormindexer/internal/models"
)

func setupTestDB(t *testing.T) (*DB, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return db, dbPath
}

func TestNewDB(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	if db == nil {
		t.Fatal("Expected non-nil database")
	}
}

func TestCreateIndex(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	index := &models.Index{
		ID:        "test-index-1",
		Name:      "Test Index",
		RootPath:  "/test/path",
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}

	err := db.CreateIndex(index)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Verify index was created
	retrieved, err := db.GetIndex("test-index-1")
	if err != nil {
		t.Fatalf("Failed to retrieve index: %v", err)
	}

	if retrieved.Name != index.Name {
		t.Errorf("Expected name %s, got %s", index.Name, retrieved.Name)
	}

	if retrieved.RootPath != index.RootPath {
		t.Errorf("Expected root path %s, got %s", index.RootPath, retrieved.RootPath)
	}
}

func TestGetIndex_NotFound(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	_, err := db.GetIndex("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent index")
	}
}

func TestListIndexes(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create multiple indexes
	indexes := []*models.Index{
		{ID: "index-1", Name: "Index 1", RootPath: "/path1", CreatedAt: time.Now(), MachineID: "machine1"},
		{ID: "index-2", Name: "Index 2", RootPath: "/path2", CreatedAt: time.Now(), MachineID: "machine1"},
		{ID: "index-3", Name: "Index 3", RootPath: "/path3", CreatedAt: time.Now(), MachineID: "machine1"},
	}

	for _, idx := range indexes {
		if err := db.CreateIndex(idx); err != nil {
			t.Fatalf("Failed to create index: %v", err)
		}
	}

	list, err := db.ListIndexes()
	if err != nil {
		t.Fatalf("Failed to list indexes: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 indexes, got %d", len(list))
	}
}

func TestUpsertFile(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create an index first
	index := &models.Index{
		ID:        "test-index",
		Name:      "Test",
		RootPath:  "/test",
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}
	db.CreateIndex(index)

	file := &models.FileEntry{
		Path:         "/test/file.txt",
		RelativePath: "file.txt",
		Size:         1024,
		ModTime:      time.Now(),
		Checksum:     "abc123",
		IndexID:      "test-index",
		LastScanned:  time.Now(),
		IsDirectory:  false,
	}

	err := db.UpsertFile(file)
	if err != nil {
		t.Fatalf("Failed to upsert file: %v", err)
	}

	// Retrieve the file
	retrieved, err := db.GetFile("/test/file.txt", "test-index")
	if err != nil {
		t.Fatalf("Failed to retrieve file: %v", err)
	}

	if retrieved.Size != file.Size {
		t.Errorf("Expected size %d, got %d", file.Size, retrieved.Size)
	}

	if retrieved.Checksum != file.Checksum {
		t.Errorf("Expected checksum %s, got %s", file.Checksum, retrieved.Checksum)
	}
}

func TestUpsertFile_Update(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create an index first
	index := &models.Index{
		ID:        "test-index",
		Name:      "Test",
		RootPath:  "/test",
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}
	db.CreateIndex(index)

	file := &models.FileEntry{
		Path:         "/test/file.txt",
		RelativePath: "file.txt",
		Size:         1024,
		ModTime:      time.Now(),
		Checksum:     "abc123",
		IndexID:      "test-index",
		LastScanned:  time.Now(),
		IsDirectory:  false,
	}

	db.UpsertFile(file)

	// Update the file
	file.Size = 2048
	file.Checksum = "def456"
	db.UpsertFile(file)

	// Verify update
	retrieved, err := db.GetFile("/test/file.txt", "test-index")
	if err != nil {
		t.Fatalf("Failed to retrieve file: %v", err)
	}

	if retrieved.Size != 2048 {
		t.Errorf("Expected updated size 2048, got %d", retrieved.Size)
	}

	if retrieved.Checksum != "def456" {
		t.Errorf("Expected updated checksum def456, got %s", retrieved.Checksum)
	}
}

func TestListFiles(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create an index
	index := &models.Index{
		ID:        "test-index",
		Name:      "Test",
		RootPath:  "/test",
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}
	db.CreateIndex(index)

	// Create multiple files
	files := []*models.FileEntry{
		{Path: "/test/file1.txt", RelativePath: "file1.txt", Size: 100, ModTime: time.Now(), IndexID: "test-index", LastScanned: time.Now(), IsDirectory: false},
		{Path: "/test/file2.txt", RelativePath: "file2.txt", Size: 200, ModTime: time.Now(), IndexID: "test-index", LastScanned: time.Now(), IsDirectory: false},
		{Path: "/test/dir", RelativePath: "dir", Size: 0, ModTime: time.Now(), IndexID: "test-index", LastScanned: time.Now(), IsDirectory: true},
	}

	for _, file := range files {
		db.UpsertFile(file)
	}

	list, err := db.ListFiles("test-index")
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 files, got %d", len(list))
	}
}

func TestDeleteFile(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create an index
	index := &models.Index{
		ID:        "test-index",
		Name:      "Test",
		RootPath:  "/test",
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}
	db.CreateIndex(index)

	file := &models.FileEntry{
		Path:         "/test/file.txt",
		RelativePath: "file.txt",
		Size:         1024,
		ModTime:      time.Now(),
		IndexID:      "test-index",
		LastScanned:  time.Now(),
		IsDirectory:  false,
	}

	db.UpsertFile(file)

	// Delete the file
	err := db.DeleteFile("/test/file.txt", "test-index")
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Verify deletion
	_, err = db.GetFile("/test/file.txt", "test-index")
	if err == nil {
		t.Error("Expected error when retrieving deleted file")
	}
}

func TestUpdateIndexStats(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create an index
	index := &models.Index{
		ID:        "test-index",
		Name:      "Test",
		RootPath:  "/test",
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}
	db.CreateIndex(index)

	// Add files
	files := []*models.FileEntry{
		{Path: "/test/file1.txt", RelativePath: "file1.txt", Size: 100, ModTime: time.Now(), IndexID: "test-index", LastScanned: time.Now(), IsDirectory: false},
		{Path: "/test/file2.txt", RelativePath: "file2.txt", Size: 200, ModTime: time.Now(), IndexID: "test-index", LastScanned: time.Now(), IsDirectory: false},
		{Path: "/test/dir", RelativePath: "dir", Size: 0, ModTime: time.Now(), IndexID: "test-index", LastScanned: time.Now(), IsDirectory: true},
	}

	for _, file := range files {
		db.UpsertFile(file)
	}

	// Update stats
	err := db.UpdateIndexStats("test-index")
	if err != nil {
		t.Fatalf("Failed to update stats: %v", err)
	}

	// Verify stats
	retrieved, err := db.GetIndex("test-index")
	if err != nil {
		t.Fatalf("Failed to retrieve index: %v", err)
	}

	// TotalFiles counts all entries (files + directories)
	// We added 2 files + 1 directory = 3 entries
	if retrieved.TotalFiles != 3 {
		t.Errorf("Expected 3 total entries (2 files + 1 directory), got %d", retrieved.TotalFiles)
	}

	// TotalSize should only count non-directory files (100 + 200)
	if retrieved.TotalSize != 300 {
		t.Errorf("Expected total size 300, got %d", retrieved.TotalSize)
	}
}

func TestFindFilesByChecksum(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create indexes
	index1 := &models.Index{ID: "index-1", Name: "Index 1", RootPath: "/path1", CreatedAt: time.Now(), MachineID: "machine1"}
	index2 := &models.Index{ID: "index-2", Name: "Index 2", RootPath: "/path2", CreatedAt: time.Now(), MachineID: "machine1"}
	db.CreateIndex(index1)
	db.CreateIndex(index2)

	// Create files with same checksum
	checksum := "abc123"
	file1 := &models.FileEntry{
		Path:         "/path1/file.txt",
		RelativePath: "file.txt",
		Size:         100,
		ModTime:      time.Now(),
		Checksum:     checksum,
		IndexID:      "index-1",
		LastScanned:  time.Now(),
		IsDirectory:  false,
	}
	file2 := &models.FileEntry{
		Path:         "/path2/file.txt",
		RelativePath: "file.txt",
		Size:         100,
		ModTime:      time.Now(),
		Checksum:     checksum,
		IndexID:      "index-2",
		LastScanned:  time.Now(),
		IsDirectory:  false,
	}

	db.UpsertFile(file1)
	db.UpsertFile(file2)

	// Find duplicates
	duplicates, err := db.FindFilesByChecksum(checksum)
	if err != nil {
		t.Fatalf("Failed to find files by checksum: %v", err)
	}

	if len(duplicates) != 2 {
		t.Errorf("Expected 2 files with same checksum, got %d", len(duplicates))
	}
}

