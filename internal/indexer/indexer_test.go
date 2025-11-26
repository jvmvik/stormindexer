package indexer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/victor/stormindexer/internal/database"
	"github.com/victor/stormindexer/internal/models"
)

func setupTestIndexer(t *testing.T) (*Indexer, *database.DB, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	testRoot := filepath.Join(tmpDir, "testroot")
	if err := os.MkdirAll(testRoot, 0755); err != nil {
		t.Fatalf("Failed to create test root: %v", err)
	}

	indexID := "test-index"
	index := &models.Index{
		ID:        indexID,
		Name:      "Test Index",
		RootPath:  testRoot,
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}

	if err := db.CreateIndex(index); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	idxr := NewIndexer(db, indexID, testRoot)

	return idxr, db, testRoot
}

func TestNewIndexer(t *testing.T) {
	idxr, db, _ := setupTestIndexer(t)
	defer db.Close()

	if idxr == nil {
		t.Fatal("Expected non-nil indexer")
	}

	if idxr.indexID != "test-index" {
		t.Errorf("Expected indexID 'test-index', got '%s'", idxr.indexID)
	}
}

func TestIndex_Basic(t *testing.T) {
	idxr, db, testRoot := setupTestIndexer(t)
	defer db.Close()

	// Create test files
	testFile1 := filepath.Join(testRoot, "file1.txt")
	testFile2 := filepath.Join(testRoot, "file2.txt")
	subDir := filepath.Join(testRoot, "subdir")
	testFile3 := filepath.Join(subDir, "file3.txt")

	os.WriteFile(testFile1, []byte("content1"), 0644)
	os.WriteFile(testFile2, []byte("content2"), 0644)
	os.MkdirAll(subDir, 0755)
	os.WriteFile(testFile3, []byte("content3"), 0644)

	// Index without checksums
	err := idxr.Index(false)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Verify files were indexed
	files, err := db.ListFiles("test-index")
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	// Should have 3 files + 1 directory = 4 entries
	if len(files) < 3 {
		t.Errorf("Expected at least 3 files, got %d", len(files))
	}

	// Verify specific files exist
	file1, err := db.GetFile(testFile1, "test-index")
	if err != nil {
		t.Errorf("File1 not found in index: %v", err)
	} else {
		if file1.IsDirectory {
			t.Error("File1 should not be marked as directory")
		}
		if file1.Size == 0 {
			t.Error("File1 should have non-zero size")
		}
	}
}

func TestIndex_WithChecksums(t *testing.T) {
	idxr, db, testRoot := setupTestIndexer(t)
	defer db.Close()

	// Create test file
	testFile := filepath.Join(testRoot, "test.txt")
	content := []byte("test content")
	os.WriteFile(testFile, content, 0644)

	// Index with checksums
	err := idxr.Index(true)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Verify checksum was calculated
	file, err := db.GetFile(testFile, "test-index")
	if err != nil {
		t.Fatalf("File not found: %v", err)
	}

	if file.Checksum == "" {
		t.Error("Expected checksum to be calculated")
	}

	// Verify checksum is correct
	expectedChecksum, _ := models.CalculateChecksum(testFile)
	if file.Checksum != expectedChecksum {
		t.Errorf("Expected checksum %s, got %s", expectedChecksum, file.Checksum)
	}
}

func TestIndex_SkipsHiddenFiles(t *testing.T) {
	idxr, db, testRoot := setupTestIndexer(t)
	defer db.Close()

	// Create hidden file
	hiddenFile := filepath.Join(testRoot, ".hidden")
	os.WriteFile(hiddenFile, []byte("hidden"), 0644)

	// Create normal file
	normalFile := filepath.Join(testRoot, "normal.txt")
	os.WriteFile(normalFile, []byte("normal"), 0644)

	err := idxr.Index(false)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Verify hidden file is not indexed
	_, err = db.GetFile(hiddenFile, "test-index")
	if err == nil {
		t.Error("Hidden file should not be indexed")
	}

	// Verify normal file is indexed
	_, err = db.GetFile(normalFile, "test-index")
	if err != nil {
		t.Error("Normal file should be indexed")
	}
}

func TestReindex_AddNewFile(t *testing.T) {
	idxr, db, testRoot := setupTestIndexer(t)
	defer db.Close()

	// Initial index
	file1 := filepath.Join(testRoot, "file1.txt")
	os.WriteFile(file1, []byte("content1"), 0644)

	err := idxr.Index(false)
	if err != nil {
		t.Fatalf("Initial index failed: %v", err)
	}

	// Add new file
	file2 := filepath.Join(testRoot, "file2.txt")
	os.WriteFile(file2, []byte("content2"), 0644)

	// Reindex
	err = idxr.Reindex(false)
	if err != nil {
		t.Fatalf("Reindex failed: %v", err)
	}

	// Verify new file is indexed
	_, err = db.GetFile(file2, "test-index")
	if err != nil {
		t.Error("New file should be indexed after reindex")
	}
}

func TestReindex_UpdateFile(t *testing.T) {
	idxr, db, testRoot := setupTestIndexer(t)
	defer db.Close()

	// Create and index file
	testFile := filepath.Join(testRoot, "test.txt")
	os.WriteFile(testFile, []byte("original"), 0644)

	err := idxr.Index(false)
	if err != nil {
		t.Fatalf("Initial index failed: %v", err)
	}

	originalFile, _ := db.GetFile(testFile, "test-index")
	originalSize := originalFile.Size

	// Update file
	os.WriteFile(testFile, []byte("updated content"), 0644)

	// Reindex
	err = idxr.Reindex(false)
	if err != nil {
		t.Fatalf("Reindex failed: %v", err)
	}

	// Verify file was updated
	updatedFile, err := db.GetFile(testFile, "test-index")
	if err != nil {
		t.Fatalf("File not found: %v", err)
	}

	if updatedFile.Size == originalSize {
		t.Error("File size should be updated after reindex")
	}
}

func TestReindex_DeleteFile(t *testing.T) {
	idxr, db, testRoot := setupTestIndexer(t)
	defer db.Close()

	// Create and index file
	testFile := filepath.Join(testRoot, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	err := idxr.Index(false)
	if err != nil {
		t.Fatalf("Initial index failed: %v", err)
	}

	// Verify file is indexed
	_, err = db.GetFile(testFile, "test-index")
	if err != nil {
		t.Fatal("File should be indexed")
	}

	// Delete file
	os.Remove(testFile)

	// Reindex
	err = idxr.Reindex(false)
	if err != nil {
		t.Fatalf("Reindex failed: %v", err)
	}

	// Verify file is removed from index
	_, err = db.GetFile(testFile, "test-index")
	if err == nil {
		t.Error("Deleted file should be removed from index")
	}
}

