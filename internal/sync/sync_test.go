package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/victor/stormindexer/internal/database"
	"github.com/victor/stormindexer/internal/models"
)

func setupTestSync(t *testing.T) (*Syncer, *database.DB, string, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	sourceRoot := filepath.Join(tmpDir, "source")
	targetRoot := filepath.Join(tmpDir, "target")

	os.MkdirAll(sourceRoot, 0755)
	os.MkdirAll(targetRoot, 0755)

	syncer := NewSyncer(db)

	return syncer, db, sourceRoot, targetRoot
}

func createTestIndex(t *testing.T, db *database.DB, indexID, name, rootPath string) {
	index := &models.Index{
		ID:        indexID,
		Name:      name,
		RootPath:  rootPath,
		CreatedAt: time.Now(),
		MachineID: "test-machine",
	}

	if err := db.CreateIndex(index); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
}

func addTestFile(t *testing.T, db *database.DB, indexID, filePath, relativePath string, size int64, checksum string) {
	file := &models.FileEntry{
		Path:         filePath,
		RelativePath: relativePath,
		Size:         size,
		ModTime:      time.Now(),
		Checksum:     checksum,
		IndexID:      indexID,
		LastScanned:  time.Now(),
		IsDirectory:  false,
	}

	if err := db.UpsertFile(file); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
}

func TestCompareIndexes_NewFiles(t *testing.T) {
	syncer, db, sourceRoot, targetRoot := setupTestSync(t)
	defer db.Close()

	sourceID := "source-index"
	targetID := "target-index"

	createTestIndex(t, db, sourceID, "Source", sourceRoot)
	createTestIndex(t, db, targetID, "Target", targetRoot)

	// Add files to source
	addTestFile(t, db, sourceID, filepath.Join(sourceRoot, "file1.txt"), "file1.txt", 100, "checksum1")
	addTestFile(t, db, sourceID, filepath.Join(sourceRoot, "file2.txt"), "file2.txt", 200, "checksum2")

	// Add one file to target (different from source)
	addTestFile(t, db, targetID, filepath.Join(targetRoot, "file3.txt"), "file3.txt", 300, "checksum3")

	result, err := syncer.CompareIndexes(sourceID, targetID)
	if err != nil {
		t.Fatalf("CompareIndexes failed: %v", err)
	}

	if len(result.NewFiles) != 2 {
		t.Errorf("Expected 2 new files, got %d", len(result.NewFiles))
	}

	if len(result.DeletedFiles) != 1 {
		t.Errorf("Expected 1 deleted file, got %d", len(result.DeletedFiles))
	}
}

func TestCompareIndexes_UpdatedFiles(t *testing.T) {
	syncer, db, sourceRoot, targetRoot := setupTestSync(t)
	defer db.Close()

	sourceID := "source-index"
	targetID := "target-index"

	createTestIndex(t, db, sourceID, "Source", sourceRoot)
	createTestIndex(t, db, targetID, "Target", targetRoot)

	// Add same file with different size (updated)
	addTestFile(t, db, sourceID, filepath.Join(sourceRoot, "file.txt"), "file.txt", 200, "checksum1")
	addTestFile(t, db, targetID, filepath.Join(targetRoot, "file.txt"), "file.txt", 100, "checksum1")

	result, err := syncer.CompareIndexes(sourceID, targetID)
	if err != nil {
		t.Fatalf("CompareIndexes failed: %v", err)
	}

	if len(result.UpdatedFiles) != 1 {
		t.Errorf("Expected 1 updated file, got %d", len(result.UpdatedFiles))
	}
}

func TestCompareIndexes_DuplicateDetection(t *testing.T) {
	syncer, db, sourceRoot, targetRoot := setupTestSync(t)
	defer db.Close()

	sourceID := "source-index"
	targetID := "target-index"

	createTestIndex(t, db, sourceID, "Source", sourceRoot)
	createTestIndex(t, db, targetID, "Target", targetRoot)

	// Add file to source
	addTestFile(t, db, sourceID, filepath.Join(sourceRoot, "file.txt"), "file.txt", 100, "checksum123")

	// Add different file with same checksum to target
	addTestFile(t, db, targetID, filepath.Join(targetRoot, "different.txt"), "different.txt", 100, "checksum123")

	result, err := syncer.CompareIndexes(sourceID, targetID)
	if err != nil {
		t.Fatalf("CompareIndexes failed: %v", err)
	}

	if len(result.DuplicateFiles) == 0 {
		t.Error("Expected duplicate files to be detected")
	}
}

func TestFindDuplicates(t *testing.T) {
	syncer, db, _, _ := setupTestSync(t)
	defer db.Close()

	index1ID := "index-1"
	index2ID := "index-2"
	index3ID := "index-3"

	createTestIndex(t, db, index1ID, "Index 1", "/path1")
	createTestIndex(t, db, index2ID, "Index 2", "/path2")
	createTestIndex(t, db, index3ID, "Index 3", "/path3")

	// Add files with same checksum
	checksum := "duplicate-checksum"
	addTestFile(t, db, index1ID, "/path1/file1.txt", "file1.txt", 100, checksum)
	addTestFile(t, db, index2ID, "/path2/file2.txt", "file2.txt", 100, checksum)
	addTestFile(t, db, index3ID, "/path3/file3.txt", "file3.txt", 100, checksum)

	// Add file with different checksum
	addTestFile(t, db, index1ID, "/path1/unique.txt", "unique.txt", 100, "unique-checksum")

	duplicates, err := syncer.FindDuplicates()
	if err != nil {
		t.Fatalf("FindDuplicates failed: %v", err)
	}

	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate set, got %d", len(duplicates))
	}

	// Verify the duplicate set has 3 files
	dupFiles, exists := duplicates[checksum]
	if !exists {
		t.Fatal("Expected duplicate set for checksum")
	}

	if len(dupFiles) != 3 {
		t.Errorf("Expected 3 duplicate files, got %d", len(dupFiles))
	}
}

func TestFindDuplicates_NoDuplicates(t *testing.T) {
	syncer, db, _, _ := setupTestSync(t)
	defer db.Close()

	index1ID := "index-1"
	createTestIndex(t, db, index1ID, "Index 1", "/path1")

	// Add files with different checksums
	addTestFile(t, db, index1ID, "/path1/file1.txt", "file1.txt", 100, "checksum1")
	addTestFile(t, db, index1ID, "/path1/file2.txt", "file2.txt", 100, "checksum2")

	duplicates, err := syncer.FindDuplicates()
	if err != nil {
		t.Fatalf("FindDuplicates failed: %v", err)
	}

	if len(duplicates) != 0 {
		t.Errorf("Expected no duplicates, got %d", len(duplicates))
	}
}

func TestCompareIndexes_IdenticalIndexes(t *testing.T) {
	syncer, db, sourceRoot, targetRoot := setupTestSync(t)
	defer db.Close()

	sourceID := "source-index"
	targetID := "target-index"

	createTestIndex(t, db, sourceID, "Source", sourceRoot)
	createTestIndex(t, db, targetID, "Target", targetRoot)

	// Add identical files
	addTestFile(t, db, sourceID, filepath.Join(sourceRoot, "file.txt"), "file.txt", 100, "checksum1")
	addTestFile(t, db, targetID, filepath.Join(targetRoot, "file.txt"), "file.txt", 100, "checksum1")

	result, err := syncer.CompareIndexes(sourceID, targetID)
	if err != nil {
		t.Fatalf("CompareIndexes failed: %v", err)
	}

	if len(result.NewFiles) != 0 {
		t.Errorf("Expected 0 new files, got %d", len(result.NewFiles))
	}

	if len(result.UpdatedFiles) != 0 {
		t.Errorf("Expected 0 updated files, got %d", len(result.UpdatedFiles))
	}

	if len(result.DeletedFiles) != 0 {
		t.Errorf("Expected 0 deleted files, got %d", len(result.DeletedFiles))
	}
}

