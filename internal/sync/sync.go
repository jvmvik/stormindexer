package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/victor/stormindexer/internal/database"
	"github.com/victor/stormindexer/internal/models"
)

type SyncResult struct {
	SourceIndexID string
	TargetIndexID string
	NewFiles      []*models.FileEntry
	UpdatedFiles  []*models.FileEntry
	DeletedFiles  []*models.FileEntry
	DuplicateFiles map[string][]*models.FileEntry
}

type Syncer struct {
	db *database.DB
}

func NewSyncer(db *database.DB) *Syncer {
	return &Syncer{db: db}
}

// CompareIndexes compares two indexes and returns differences
func (s *Syncer) CompareIndexes(sourceIndexID, targetIndexID string) (*SyncResult, error) {
	sourceFiles, err := s.db.ListFiles(sourceIndexID)
	if err != nil {
		return nil, fmt.Errorf("failed to list source files: %w", err)
	}

	targetFiles, err := s.db.ListFiles(targetIndexID)
	if err != nil {
		return nil, fmt.Errorf("failed to list target files: %w", err)
	}

	// Build maps for quick lookup
	targetMap := make(map[string]*models.FileEntry)
	targetChecksumMap := make(map[string][]*models.FileEntry)

	for _, file := range targetFiles {
		// Index by relative path
		targetMap[file.RelativePath] = file
		// Index by checksum for duplicate detection
		if file.Checksum != "" {
			targetChecksumMap[file.Checksum] = append(targetChecksumMap[file.Checksum], file)
		}
	}

	result := &SyncResult{
		SourceIndexID:  sourceIndexID,
		TargetIndexID:  targetIndexID,
		NewFiles:       []*models.FileEntry{},
		UpdatedFiles:   []*models.FileEntry{},
		DeletedFiles:   []*models.FileEntry{},
		DuplicateFiles: make(map[string][]*models.FileEntry),
	}

	// Find new and updated files
	for _, sourceFile := range sourceFiles {
		if sourceFile.IsDirectory {
			continue
		}

		targetFile, exists := targetMap[sourceFile.RelativePath]

		if !exists {
			// Check if file exists with same checksum (duplicate detection)
			if sourceFile.Checksum != "" {
				if duplicates, found := targetChecksumMap[sourceFile.Checksum]; found {
					result.DuplicateFiles[sourceFile.RelativePath] = duplicates
				}
			}
			result.NewFiles = append(result.NewFiles, sourceFile)
		} else {
			// Check if file was updated
			if sourceFile.Size != targetFile.Size ||
				sourceFile.ModTime.Unix() != targetFile.ModTime.Unix() ||
				(sourceFile.Checksum != "" && targetFile.Checksum != "" && sourceFile.Checksum != targetFile.Checksum) {
				result.UpdatedFiles = append(result.UpdatedFiles, sourceFile)
			}
		}
	}

	// Find deleted files (in target but not in source)
	sourceMap := make(map[string]bool)
	for _, file := range sourceFiles {
		sourceMap[file.RelativePath] = true
	}

	for _, targetFile := range targetFiles {
		if !targetFile.IsDirectory && !sourceMap[targetFile.RelativePath] {
			result.DeletedFiles = append(result.DeletedFiles, targetFile)
		}
	}

	return result, nil
}

// SyncToIndex syncs files from source index to target index using rsync
// This performs actual file copying and updates the database
func (s *Syncer) SyncToIndex(sourceIndexID, targetIndexID, targetRootPath string, dryRun bool, deleteExtra bool) error {
	// Get source index to get the source root path
	sourceIndex, err := s.db.GetIndex(sourceIndexID)
	if err != nil {
		return fmt.Errorf("failed to get source index: %w", err)
	}

	sourceRootPath := sourceIndex.RootPath

	result, err := s.CompareIndexes(sourceIndexID, targetIndexID)
	if err != nil {
		return err
	}

	fmt.Printf("\n=== Sync Report ===\n")
	fmt.Printf("Source: %s (%s)\n", sourceIndex.Name, sourceRootPath)
	fmt.Printf("Target: %s (%s)\n", targetIndexID, targetRootPath)
	fmt.Printf("New files: %d\n", len(result.NewFiles))
	fmt.Printf("Updated files: %d\n", len(result.UpdatedFiles))
	fmt.Printf("Deleted files: %d\n", len(result.DeletedFiles))
	fmt.Printf("Duplicate files found: %d\n", len(result.DuplicateFiles))

	if dryRun {
		fmt.Printf("\n[DRY RUN] No changes will be made.\n")
		return nil
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetRootPath, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Build rsync command
	// rsync options:
	// -a: archive mode (preserves permissions, timestamps, etc.)
	// -v: verbose
	// -h: human-readable sizes
	// --progress: show progress
	// --delete: delete files in destination that don't exist in source (if requested)
	rsyncArgs := []string{
		"-avh",
		"--progress",
	}

	if deleteExtra {
		rsyncArgs = append(rsyncArgs, "--delete")
	}

	// Add source path (with trailing slash to sync contents)
	sourcePath := sourceRootPath
	if sourcePath[len(sourcePath)-1] != '/' {
		sourcePath += "/"
	}
	rsyncArgs = append(rsyncArgs, sourcePath)

	// Add target path
	rsyncArgs = append(rsyncArgs, targetRootPath)

	// Check if rsync is available
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found in PATH. Please install rsync to use file synchronization")
	}

	fmt.Printf("\nRunning rsync...\n")
	fmt.Printf("Command: rsync %v\n", rsyncArgs)

	// Execute rsync
	cmd := exec.Command("rsync", rsyncArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	// After rsync completes, update the database with synced files
	fmt.Printf("\nUpdating index database...\n")

	// Get source files
	sourceFiles, err := s.db.ListFiles(sourceIndexID)
	if err != nil {
		return fmt.Errorf("failed to list source files: %w", err)
	}

	// Create file entries for target index
	for _, sourceFile := range sourceFiles {
		targetPath := filepath.Join(targetRootPath, sourceFile.RelativePath)
		targetFile := &models.FileEntry{
			Path:         targetPath,
			RelativePath: sourceFile.RelativePath,
			Size:         sourceFile.Size,
			ModTime:      sourceFile.ModTime,
			Checksum:     sourceFile.Checksum,
			IndexID:      targetIndexID,
			LastScanned:  time.Now(),
			IsDirectory:  sourceFile.IsDirectory,
		}

		if err := s.db.UpsertFile(targetFile); err != nil {
			return fmt.Errorf("failed to sync file %s: %w", targetPath, err)
		}
	}

	// Update target index stats
	if err := s.db.UpdateIndexStats(targetIndexID); err != nil {
		return fmt.Errorf("failed to update target index stats: %w", err)
	}

	fmt.Printf("\nSync completed successfully!\n")
	return nil
}

// FindDuplicates finds duplicate files across all indexes
func (s *Syncer) FindDuplicates() (map[string][]*models.FileEntry, error) {
	indexes, err := s.db.ListIndexes()
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	checksumMap := make(map[string][]*models.FileEntry)

	for _, index := range indexes {
		files, err := s.db.ListFiles(index.ID)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.Checksum != "" && !file.IsDirectory {
				checksumMap[file.Checksum] = append(checksumMap[file.Checksum], file)
			}
		}
	}

	// Filter to only duplicates (more than one file)
	duplicates := make(map[string][]*models.FileEntry)
	for checksum, files := range checksumMap {
		if len(files) > 1 {
			duplicates[checksum] = files
		}
	}

	return duplicates, nil
}

