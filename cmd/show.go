package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show [index-id|name]",
	Short: "Show detailed information about an index",
	Long:  `Display detailed information about a specific index including statistics. You can use full ID, partial ID (8+ chars), or exact name.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]

		index, err := db.FindIndexByNameOrID(identifier)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Index not found: %s\n", identifier)
			fmt.Fprintf(os.Stderr, "You can use full ID, partial ID (8+ chars), or exact name.\n")
			fmt.Fprintf(os.Stderr, "Use 'stormindexer list' to see available indexes.\n")
			os.Exit(1)
		}

		files, err := db.ListFiles(index.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing files: %v\n", err)
			os.Exit(1)
		}

		var totalSize int64
		var fileCount, dirCount int64
		for _, file := range files {
			if file.IsDirectory {
				dirCount++
			} else {
				fileCount++
				totalSize += file.Size
			}
		}

		fmt.Printf("Index Details\n")
		fmt.Printf("=============\n\n")
		fmt.Printf("ID:          %s\n", index.ID)
		fmt.Printf("Name:        %s\n", index.Name)
		fmt.Printf("Root Path:   %s\n", index.RootPath)
		fmt.Printf("Machine ID:  %s\n", index.MachineID)
		fmt.Printf("Created:     %s\n", index.CreatedAt.Format(time.RFC3339))
		if !index.LastSync.IsZero() {
			fmt.Printf("Last Sync:   %s\n", index.LastSync.Format(time.RFC3339))
		}
		fmt.Printf("\nStatistics\n")
		fmt.Printf("----------\n")
		fmt.Printf("Total Files:      %d\n", fileCount)
		fmt.Printf("Total Directories: %d\n", dirCount)
		fmt.Printf("Total Size:       %s\n", formatBytes(totalSize))
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

