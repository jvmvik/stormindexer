package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/victor/stormindexer/internal/database"
	"github.com/victor/stormindexer/internal/models"
)

type Indexer struct {
	db      *database.DB
	indexID string
	rootPath string
}

// NewIndexer creates a new indexer instance
func NewIndexer(db *database.DB, indexID, rootPath string) *Indexer {
	return &Indexer{
		db:       db,
		indexID:  indexID,
		rootPath: rootPath,
	}
}

// Index scans the root path and indexes all files
func (idx *Indexer) Index(calculateChecksums bool) error {
	startTime := time.Now()
	fmt.Printf("Starting index of: %s\n", idx.rootPath)

	// First, count total files for progress bar (with 1 minute timeout)
	totalFiles := int64(0)
	countingTimedOut := false
	countDone := make(chan bool, 1)
	
	go func() {
		filepath.Walk(idx.rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if filepath.Base(path)[0] == '.' {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !info.IsDir() {
				totalFiles++
			}
			return nil
		})
		countDone <- true
	}()
	
	// Wait for counting to complete or timeout after 1 minute
	select {
	case <-countDone:
		// Counting completed successfully
	case <-time.After(1 * time.Minute):
		// Timeout - continue without knowing total file count
		countingTimedOut = true
		fmt.Fprintf(os.Stderr, "Warning: File counting timed out after 1 minute. Continuing with indeterminate progress...\n")
	}

	stats := struct {
		files       int64
		directories int64
		size        int64
	}{}

	// Create progress bar
	var bar *progressbar.ProgressBar
	if countingTimedOut {
		// Use indeterminate progress bar when we don't know the total
		bar = progressbar.NewOptions64(
			-1, // -1 means indeterminate
			progressbar.OptionSetDescription("Indexing files"),
			progressbar.OptionSetWidth(60),
			progressbar.OptionShowBytes(false),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(100*time.Millisecond),
		)
		defer bar.Close()
	} else if totalFiles > 0 {
		// Use determinate progress bar when we know the total
		bar = progressbar.NewOptions64(
			totalFiles,
			progressbar.OptionSetDescription("Indexing files"),
			progressbar.OptionSetWidth(60),
			progressbar.OptionShowBytes(false),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(100*time.Millisecond),
		)
		defer bar.Close()
	} else {
		fmt.Fprintf(os.Stderr, "No files found to index.\n")
	}

	var currentFile string
	err := filepath.Walk(idx.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue despite errors
		}

		// Skip hidden files and directories
		if filepath.Base(path)[0] == '.' {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relativePath, err := filepath.Rel(idx.rootPath, path)
		if err != nil {
			relativePath = path
		}

		fileEntry := &models.FileEntry{
			Path:         path,
			RelativePath: relativePath,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			IndexID:      idx.indexID,
			LastScanned:  time.Now(),
			IsDirectory:  info.IsDir(),
		}

		// Calculate checksum for files (not directories)
		if !info.IsDir() && calculateChecksums {
			checksum, err := models.CalculateChecksum(path)
			if err != nil {
				// Don't print warning during progress bar, just continue
			} else {
				fileEntry.Checksum = checksum
			}
		}

		if err := idx.db.UpsertFile(fileEntry); err != nil {
			if bar != nil {
				bar.Close()
			}
			return fmt.Errorf("failed to upsert file %s: %w", path, err)
		}

		if info.IsDir() {
			stats.directories++
		} else {
			stats.files++
			stats.size += info.Size()
			
			// Update progress bar with current file and stats
			if bar != nil {
				currentFile = relativePath
				if len(currentFile) > 40 {
					currentFile = "..." + currentFile[len(currentFile)-37:]
				}
				bar.Describe(fmt.Sprintf("Indexing: %s | %d files | %s", 
					currentFile, stats.files, formatBytes(stats.size)))
				_ = bar.Add64(1) // Ignore error, just update progress
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk error: %w", err)
	}

	// Update index statistics
	if err := idx.db.UpdateIndexStats(idx.indexID); err != nil {
		return fmt.Errorf("failed to update index stats: %w", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("✓ Indexing complete: %d files, %d directories, %s total size (completed in %s)\n",
		stats.files, stats.directories, formatBytes(stats.size), formatDuration(elapsed))

	return nil
}

// Reindex updates the index by scanning for changes
func (idx *Indexer) Reindex(calculateChecksums bool) error {
	startTime := time.Now()
	fmt.Printf("Reindexing: %s\n", idx.rootPath)

	// Get existing files from database
	existingFiles, err := idx.db.ListFiles(idx.indexID)
	if err != nil {
		return fmt.Errorf("failed to list existing files: %w", err)
	}

	existingMap := make(map[string]*models.FileEntry)
	for _, file := range existingFiles {
		existingMap[file.Path] = file
	}

	// Count total files for progress bar (with 1 minute timeout)
	totalFiles := int64(0)
	countingTimedOut := false
	countDone := make(chan bool, 1)
	
	go func() {
		filepath.Walk(idx.rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if filepath.Base(path)[0] == '.' {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !info.IsDir() {
				totalFiles++
			}
			return nil
		})
		countDone <- true
	}()
	
	// Wait for counting to complete or timeout after 1 minute
	select {
	case <-countDone:
		// Counting completed successfully
	case <-time.After(1 * time.Minute):
		// Timeout - continue without knowing total file count
		countingTimedOut = true
		fmt.Fprintf(os.Stderr, "Warning: File counting timed out after 1 minute. Continuing with indeterminate progress...\n")
	}

	stats := struct {
		added      int64
		updated    int64
		removed    int64
		size       int64
		processed  int64
	}{}

	// Track files found during scan
	foundPaths := make(map[string]bool)

	// Create progress bar
	var bar *progressbar.ProgressBar
	if countingTimedOut {
		// Use indeterminate progress bar when we don't know the total
		bar = progressbar.NewOptions64(
			-1, // -1 means indeterminate
			progressbar.OptionSetDescription("Reindexing files"),
			progressbar.OptionSetWidth(60),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(100*time.Millisecond),
		)
		defer bar.Close()
	} else if totalFiles > 0 {
		// Use determinate progress bar when we know the total
		bar = progressbar.NewOptions64(
			totalFiles,
			progressbar.OptionSetDescription("Reindexing files"),
			progressbar.OptionSetWidth(60),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(100*time.Millisecond),
		)
		defer bar.Close()
	}

	var currentFile string
	err = filepath.Walk(idx.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if filepath.Base(path)[0] == '.' {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		foundPaths[path] = true

		relativePath, err := filepath.Rel(idx.rootPath, path)
		if err != nil {
			relativePath = path
		}

		existing, exists := existingMap[path]
		needsUpdate := !exists ||
			existing.Size != info.Size() ||
			existing.ModTime.Unix() != info.ModTime().Unix()

		if needsUpdate {
			fileEntry := &models.FileEntry{
				Path:         path,
				RelativePath: relativePath,
				Size:         info.Size(),
				ModTime:      info.ModTime(),
				IndexID:      idx.indexID,
				LastScanned:  time.Now(),
				IsDirectory:  info.IsDir(),
			}

			// Calculate checksum if needed
			if !info.IsDir() && (calculateChecksums || !exists || existing.Checksum == "") {
				checksum, err := models.CalculateChecksum(path)
				if err != nil {
					// Don't print warning during progress bar
				} else {
					fileEntry.Checksum = checksum
				}
			} else if exists {
				fileEntry.Checksum = existing.Checksum
			}

			if err := idx.db.UpsertFile(fileEntry); err != nil {
				if bar != nil {
					bar.Close()
				}
				return fmt.Errorf("failed to upsert file %s: %w", path, err)
			}

			if exists {
				stats.updated++
			} else {
				stats.added++
			}
		}

		if !info.IsDir() {
			stats.size += info.Size()
			stats.processed++
			
			// Update progress bar
			if bar != nil {
				currentFile = relativePath
				if len(currentFile) > 40 {
					currentFile = "..." + currentFile[len(currentFile)-37:]
				}
				bar.Describe(fmt.Sprintf("Reindexing: %s | +%d ~%d", 
					currentFile, stats.added, stats.updated))
				_ = bar.Add64(1) // Ignore error, just update progress
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk error: %w", err)
	}

	// Remove files that no longer exist
	for path := range existingMap {
		if !foundPaths[path] {
			if err := idx.db.DeleteFile(path, idx.indexID); err != nil {
				// Don't print warning, just continue
			} else {
				stats.removed++
			}
		}
	}

	// Update index statistics
	if err := idx.db.UpdateIndexStats(idx.indexID); err != nil {
		return fmt.Errorf("failed to update index stats: %w", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("✓ Reindexing complete: %d added, %d updated, %d removed (completed in %s)\n",
		stats.added, stats.updated, stats.removed, formatDuration(elapsed))

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}

