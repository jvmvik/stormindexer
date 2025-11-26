package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all indexes",
	Long:  `List all indexes stored in the database.`,
	Run: func(cmd *cobra.Command, args []string) {
		indexes, err := db.ListIndexes()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing indexes: %v\n", err)
			os.Exit(1)
		}

		if len(indexes) == 0 {
			fmt.Println("No indexes found.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tPATH\tFILES\tSIZE\tLAST SYNC")
		fmt.Fprintln(w, "---\t----\t----\t-----\t----\t---------")

		for _, index := range indexes {
			sizeStr := formatBytes(index.TotalSize)
			lastSync := "Never"
			if !index.LastSync.IsZero() {
				lastSync = index.LastSync.Format("2006-01-02 15:04:05")
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
				index.ID[:12], // Truncate ID for display (12 chars)
				index.Name,
				index.RootPath,
				index.TotalFiles,
				sizeStr,
				lastSync,
			)
		}

		w.Flush()
	},
}

var listFilesCmd = &cobra.Command{
	Use:   "files [index-id|name]",
	Short: "List files in an index",
	Long:  `List all files in the specified index. You can use full ID, partial ID (8+ chars), or exact name.`,
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

		fmt.Printf("Index: %s (%s)\n", index.Name, index.RootPath)
		fmt.Printf("Total files: %d\n\n", len(files))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "PATH\tSIZE\tMODIFIED\tCHECKSUM")
		fmt.Fprintln(w, "----\t----\t--------\t--------")

		for _, file := range files {
			if file.IsDirectory {
				continue
			}

			checksum := file.Checksum
			if checksum == "" {
				checksum = "-"
			} else if len(checksum) > 12 {
				checksum = checksum[:12] + "..."
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				file.RelativePath,
				formatBytes(file.Size),
				file.ModTime.Format(time.RFC3339),
				checksum,
			)
		}

		w.Flush()
	},
}

func init() {
	listCmd.AddCommand(listFilesCmd)
	rootCmd.AddCommand(listCmd)
}

