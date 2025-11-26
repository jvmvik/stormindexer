package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [index-id|name]",
	Short: "Remove an index from the database",
	Long: `Remove an index and all its associated file entries from the database.
This does not delete the actual files on disk, only removes the index tracking.

You can specify the index by:
  - Full index ID
  - Partial ID (at least 8 characters, e.g., 'f0bd0c0e')
  - Exact index name

To find indexes, use 'stormindexer list'.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]

		// Try to find index by ID, name, or partial ID
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

		// Show what will be removed
		fmt.Printf("Index to remove:\n")
		fmt.Printf("  ID:   %s\n", index.ID[:12])
		fmt.Printf("  Name: %s\n", index.Name)
		fmt.Printf("  Path: %s\n", index.RootPath)
		fmt.Printf("  Files: %d\n", index.TotalFiles)
		fmt.Printf("\nThis will remove the index and all %d file entries from the database.\n", index.TotalFiles)
		fmt.Printf("The actual files on disk will NOT be deleted.\n")

		// Confirm deletion
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("\n⚠️  Warning: This action cannot be undone!\n")
			fmt.Printf("Use --force flag to confirm removal: stormindexer remove %s --force\n", identifier)
			os.Exit(0)
		}

		// Delete the index using the actual ID
		if err := db.DeleteIndex(index.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing index: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Successfully removed index: %s (%s)\n", index.Name, index.RootPath)
		fmt.Printf("  Removed %d file entries from database.\n", index.TotalFiles)
	},
}

func init() {
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(removeCmd)
}

