package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/victor/stormindexer/internal/sync"
)

var syncCmd = &cobra.Command{
	Use:   "sync [source-index-id] [target-index-id]",
	Short: "Sync files between indexes",
	Long: `Compare and sync files between two indexes. Shows differences
and optionally syncs files from source to target.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceIndexID := args[0]
		targetIndexID := args[1]

		// Verify indexes exist
		sourceIndex, err := db.GetIndex(sourceIndexID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Source index not found: %s\n", sourceIndexID)
			os.Exit(1)
		}

		targetIndex, err := db.GetIndex(targetIndexID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Target index not found: %s\n", targetIndexID)
			os.Exit(1)
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		deleteExtra, _ := cmd.Flags().GetBool("delete")

		syncer := sync.NewSyncer(db)
		result, err := syncer.CompareIndexes(sourceIndexID, targetIndexID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error comparing indexes: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n=== Sync Comparison ===\n")
		fmt.Printf("Source: %s (%s)\n", sourceIndex.Name, sourceIndex.RootPath)
		fmt.Printf("Target: %s (%s)\n", targetIndex.Name, targetIndex.RootPath)
		fmt.Printf("\nNew files: %d\n", len(result.NewFiles))
		fmt.Printf("Updated files: %d\n", len(result.UpdatedFiles))
		fmt.Printf("Deleted files: %d\n", len(result.DeletedFiles))
		fmt.Printf("Duplicate files: %d\n", len(result.DuplicateFiles))

		if len(result.NewFiles) > 0 {
			fmt.Printf("\nNew files:\n")
			for _, file := range result.NewFiles[:min(10, len(result.NewFiles))] {
				fmt.Printf("  + %s (%s)\n", file.RelativePath, formatBytes(file.Size))
			}
			if len(result.NewFiles) > 10 {
				fmt.Printf("  ... and %d more\n", len(result.NewFiles)-10)
			}
		}

		if len(result.UpdatedFiles) > 0 {
			fmt.Printf("\nUpdated files:\n")
			for _, file := range result.UpdatedFiles[:min(10, len(result.UpdatedFiles))] {
				fmt.Printf("  ~ %s\n", file.RelativePath)
			}
			if len(result.UpdatedFiles) > 10 {
				fmt.Printf("  ... and %d more\n", len(result.UpdatedFiles)-10)
			}
		}

		if len(result.DeletedFiles) > 0 {
			fmt.Printf("\nDeleted files:\n")
			for _, file := range result.DeletedFiles[:min(10, len(result.DeletedFiles))] {
				fmt.Printf("  - %s\n", file.RelativePath)
			}
			if len(result.DeletedFiles) > 10 {
				fmt.Printf("  ... and %d more\n", len(result.DeletedFiles)-10)
			}
		}

		if !dryRun {
			// Perform actual sync using rsync
			if err := syncer.SyncToIndex(sourceIndexID, targetIndexID, targetIndex.RootPath, false, deleteExtra); err != nil {
				fmt.Fprintf(os.Stderr, "Error syncing: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("\n[DRY RUN] No changes made. Remove --dry-run to sync.\n")
		}
	},
}

var compareCmd = &cobra.Command{
	Use:   "compare [index-id-1] [index-id-2]",
	Short: "Compare two indexes",
	Long:  `Compare two indexes and show differences without syncing.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		indexID1 := args[0]
		indexID2 := args[1]

		syncer := sync.NewSyncer(db)
		result, err := syncer.CompareIndexes(indexID1, indexID2)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error comparing indexes: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n=== Comparison Results ===\n")
		fmt.Printf("Files in index 1 but not in index 2: %d\n", len(result.NewFiles))
		fmt.Printf("Files in index 2 but not in index 1: %d\n", len(result.DeletedFiles))
		fmt.Printf("Files that differ: %d\n", len(result.UpdatedFiles))
	},
}

var duplicatesCmd = &cobra.Command{
	Use:   "duplicates",
	Short: "Find duplicate files across all indexes",
	Long:  `Find files with identical checksums across all indexed locations.`,
	Run: func(cmd *cobra.Command, args []string) {
		syncer := sync.NewSyncer(db)
		duplicates, err := syncer.FindDuplicates()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding duplicates: %v\n", err)
			os.Exit(1)
		}

		if len(duplicates) == 0 {
			fmt.Println("No duplicate files found.")
			return
		}

		fmt.Printf("Found %d sets of duplicate files:\n\n", len(duplicates))

		count := 0
		for checksum, files := range duplicates {
			if count >= 20 {
				fmt.Printf("... and %d more duplicate sets\n", len(duplicates)-count)
				break
			}

			fmt.Printf("Checksum: %s... (%d copies)\n", checksum[:12], len(files))
			for _, file := range files {
				fmt.Printf("  - %s [%s]\n", file.Path, file.IndexID[:12])
			}
			fmt.Println()
			count++
		}
	},
}

func init() {
	syncCmd.Flags().BoolP("dry-run", "d", false, "Show what would be synced without making changes")
	syncCmd.Flags().Bool("delete", false, "Delete files in target that don't exist in source (use with caution)")

	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(compareCmd)
	rootCmd.AddCommand(duplicatesCmd)
}

