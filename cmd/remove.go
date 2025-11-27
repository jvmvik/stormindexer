package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/victor/stormindexer/internal/models"
)

var removeCmd = &cobra.Command{
	Use:   "remove [index-id|name]...",
	Short: "Remove one or more indexes from the database",
	Long: `Remove indexes and all their associated file entries from the database.
This does not delete the actual files on disk, only removes the index tracking.

You can specify indexes by:
  - Full index ID
  - Partial ID (at least 8 characters, e.g., 'f0bd0c0e')
  - Exact index name

You can remove multiple indexes at once by providing multiple names/IDs.

To find indexes, use 'stormindexer list'.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifiers := args
		force, _ := cmd.Flags().GetBool("force")

		// Find all indexes first
		type indexInfo struct {
			index      *models.Index
			identifier string
		}
		var indexesToRemove []indexInfo
		var totalFiles int64

		for _, identifier := range identifiers {
			index, err := db.FindIndexByNameOrID(identifier)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Index not found: %s\n", identifier)
				fmt.Fprintf(os.Stderr, "You can use:\n")
				fmt.Fprintf(os.Stderr, "  - Full index ID\n")
				fmt.Fprintf(os.Stderr, "  - Partial ID (at least 8 characters, e.g., 'f0bd0c0e')\n")
				fmt.Fprintf(os.Stderr, "  - Exact index name\n")
				fmt.Fprintf(os.Stderr, "\nUse 'stormindexer list' to see available indexes.\n")
				os.Exit(1)
			}
			indexesToRemove = append(indexesToRemove, indexInfo{index: index, identifier: identifier})
			totalFiles += index.TotalFiles
		}

		// Show what will be removed
		if len(indexesToRemove) == 1 {
			idx := indexesToRemove[0].index
			fmt.Printf("Index to remove:\n")
			fmt.Printf("  ID:   %s\n", idx.ID[:12])
			fmt.Printf("  Name: %s\n", idx.Name)
			fmt.Printf("  Path: %s\n", idx.RootPath)
			fmt.Printf("  Files: %d\n", idx.TotalFiles)
			fmt.Printf("\nThis will remove the index and all %d file entries from the database.\n", idx.TotalFiles)
		} else {
			fmt.Printf("Indexes to remove (%d):\n", len(indexesToRemove))
			for i, info := range indexesToRemove {
				idx := info.index
				fmt.Printf("\n  %d. %s\n", i+1, idx.Name)
				fmt.Printf("     ID:   %s\n", idx.ID[:12])
				fmt.Printf("     Path: %s\n", idx.RootPath)
				fmt.Printf("     Files: %d\n", idx.TotalFiles)
			}
			fmt.Printf("\nThis will remove %d indexes and all %d file entries from the database.\n", len(indexesToRemove), totalFiles)
		}
		fmt.Printf("The actual files on disk will NOT be deleted.\n")

		// Confirm deletion
		if !force {
			fmt.Printf("\n⚠️  Warning: This action cannot be undone!\n")
			if len(indexesToRemove) == 1 {
				fmt.Printf("Use --force flag to confirm removal: stormindexer remove %s --force\n", indexesToRemove[0].identifier)
			} else {
				fmt.Printf("Use --force flag to confirm removal: stormindexer remove --force %s\n", identifiers[0])
				for i := 1; i < len(identifiers); i++ {
					fmt.Printf("  %s\n", identifiers[i])
				}
			}
			os.Exit(0)
		}

		// Remove all indexes
		var errors []string
		var successCount int
		for _, info := range indexesToRemove {
			idx := info.index
			if err := db.DeleteIndex(idx.ID); err != nil {
				errors = append(errors, fmt.Sprintf("Error removing index %s: %v", idx.Name, err))
				fmt.Fprintf(os.Stderr, "✗ Failed to remove index: %s (%s)\n", idx.Name, idx.RootPath)
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			} else {
				fmt.Printf("✓ Successfully removed index: %s (%s)\n", idx.Name, idx.RootPath)
				fmt.Printf("  Removed %d file entries from database.\n", idx.TotalFiles)
				successCount++
			}
		}

		// Summary
		if len(indexesToRemove) > 1 {
			fmt.Printf("\nSummary: %d of %d indexes removed successfully.\n", successCount, len(indexesToRemove))
		}

		if len(errors) > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(removeCmd)
}

