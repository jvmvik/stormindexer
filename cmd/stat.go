package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var statCmd = &cobra.Command{
	Use:   "stat",
	Short: "Show database statistics and information",
	Long:  `Display database file location, size, and statistics about indexed data.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get database path
		dbPath := cfg.DatabasePath

		// Get file info
		fileInfo, err := os.Stat(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not access database file: %v\n", err)
			os.Exit(1)
		}

		// Get absolute path
		absPath, err := filepath.Abs(dbPath)
		if err != nil {
			absPath = dbPath
		}

		// Get database statistics
		indexes, err := db.ListIndexes()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not list indexes: %v\n", err)
			os.Exit(1)
		}

		var totalIndexes int64
		var totalFiles int64
		var totalSize int64

		for _, index := range indexes {
			totalIndexes++
			totalFiles += index.TotalFiles
			totalSize += index.TotalSize
		}

		// Display statistics
		fmt.Println("Database Statistics")
		fmt.Println("===================")
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Database Path:\t%s\n", absPath)
		fmt.Fprintf(w, "Database Size:\t%s\n", formatBytes(fileInfo.Size()))
		fmt.Fprintf(w, "Last Modified:\t%s\n", fileInfo.ModTime().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "Total Indexes:\t%d\n", totalIndexes)
		fmt.Fprintf(w, "Total Files Indexed:\t%d\n", totalFiles)
		fmt.Fprintf(w, "Total Size Indexed:\t%s\n", formatBytes(totalSize))
		w.Flush()

		// Show per-index breakdown if there are indexes
		if len(indexes) > 0 {
			fmt.Println()
			fmt.Println("Index Breakdown")
			fmt.Println("--------------")
			fmt.Println()

			w2 := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w2, "NAME\tFILES\tSIZE\tLAST SYNC")
			fmt.Fprintln(w2, "----\t-----\t----\t---------")

			for _, index := range indexes {
				sizeStr := formatBytes(index.TotalSize)
				lastSync := "Never"
				if !index.LastSync.IsZero() {
					lastSync = index.LastSync.Format("2006-01-02 15:04:05")
				}

				fmt.Fprintf(w2, "%s\t%d\t%s\t%s\n",
					index.Name,
					index.TotalFiles,
					sizeStr,
					lastSync,
				)
			}
			w2.Flush()
		}
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
}

