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

func TestFindDuplicates_AcrossMultipleDrives(t *testing.T) {
	syncer, db, _, _ := setupTestSync(t)
	defer db.Close()

	// Simulate multiple drives with different mount points
	drive1ID := "drive1-index"
	drive2ID := "drive2-index"
	drive3ID := "drive3-index"

	// Create indexes for different drives (simulating /Volumes/drive1, /Volumes/drive2, etc.)
	createTestIndex(t, db, drive1ID, "Drive 1", "/Volumes/drive1")
	createTestIndex(t, db, drive2ID, "Drive 2", "/Volumes/drive2")
	createTestIndex(t, db, drive3ID, "Drive 3", "/mnt/external")

	// Add files with same checksum across different drives
	duplicateChecksum1 := "abc123def456"
	duplicateChecksum2 := "xyz789uvw012"

	// First duplicate set: same file exists on drive1 and drive2
	addTestFile(t, db, drive1ID, "/Volumes/drive1/documents/file1.pdf", "documents/file1.pdf", 1024, duplicateChecksum1)
	addTestFile(t, db, drive2ID, "/Volumes/drive2/backup/file1.pdf", "backup/file1.pdf", 1024, duplicateChecksum1)

	// Second duplicate set: same file exists on all three drives
	addTestFile(t, db, drive1ID, "/Volumes/drive1/photos/image.jpg", "photos/image.jpg", 2048, duplicateChecksum2)
	addTestFile(t, db, drive2ID, "/Volumes/drive2/images/image.jpg", "images/image.jpg", 2048, duplicateChecksum2)
	addTestFile(t, db, drive3ID, "/mnt/external/pics/image.jpg", "pics/image.jpg", 2048, duplicateChecksum2)

	// Add unique files to each drive (should not appear in duplicates)
	addTestFile(t, db, drive1ID, "/Volumes/drive1/unique1.txt", "unique1.txt", 512, "unique-checksum-1")
	addTestFile(t, db, drive2ID, "/Volumes/drive2/unique2.txt", "unique2.txt", 512, "unique-checksum-2")
	addTestFile(t, db, drive3ID, "/mnt/external/unique3.txt", "unique3.txt", 512, "unique-checksum-3")

	duplicates, err := syncer.FindDuplicates()
	if err != nil {
		t.Fatalf("FindDuplicates failed: %v", err)
	}

	// Should find 2 duplicate sets
	if len(duplicates) != 2 {
		t.Errorf("Expected 2 duplicate sets, got %d", len(duplicates))
	}

	// Verify first duplicate set (2 files on drive1 and drive2)
	dupSet1, exists := duplicates[duplicateChecksum1]
	if !exists {
		t.Fatal("Expected duplicate set for checksum1")
	}
	if len(dupSet1) != 2 {
		t.Errorf("Expected 2 duplicate files for checksum1, got %d", len(dupSet1))
	}
	// Verify files are from different drives
	indexIDs := make(map[string]bool)
	for _, file := range dupSet1 {
		indexIDs[file.IndexID] = true
	}
	if len(indexIDs) != 2 {
		t.Errorf("Expected duplicates from 2 different drives, got %d", len(indexIDs))
	}

	// Verify second duplicate set (3 files across all drives)
	dupSet2, exists := duplicates[duplicateChecksum2]
	if !exists {
		t.Fatal("Expected duplicate set for checksum2")
	}
	if len(dupSet2) != 3 {
		t.Errorf("Expected 3 duplicate files for checksum2, got %d", len(dupSet2))
	}
	// Verify files are from all three drives
	indexIDs2 := make(map[string]bool)
	for _, file := range dupSet2 {
		indexIDs2[file.IndexID] = true
	}
	if len(indexIDs2) != 3 {
		t.Errorf("Expected duplicates from 3 different drives, got %d", len(indexIDs2))
	}

	// Verify unique files are not in duplicates
	for checksum, files := range duplicates {
		if checksum == "unique-checksum-1" || checksum == "unique-checksum-2" || checksum == "unique-checksum-3" {
			t.Errorf("Unique file checksum %s should not appear in duplicates", checksum)
		}
		// Verify all files in duplicate sets have the same checksum
		for _, file := range files {
			if file.Checksum != checksum {
				t.Errorf("File in duplicate set has mismatched checksum: expected %s, got %s", checksum, file.Checksum)
			}
		}
	}
}

