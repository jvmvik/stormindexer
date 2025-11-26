package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/victor/stormindexer/internal/indexer"
	"github.com/victor/stormindexer/internal/models"
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index files in a directory",
	Long: `Index all files in the specified directory. Creates a new index
or updates an existing one if the path was previously indexed.`,
	Args: cobra.MinimumNArgs(1), // Allow at least 1 arg, handle -name manually
	Run: func(cmd *cobra.Command, args []string) {
		var path string
		var name string
		
		// First, try to get name from cobra's flag parser (works for --name and -n)
		name, _ = cmd.Flags().GetString("name")
		
		// Check if cobra parsed -name as -n with value "ame" (cobra treats -name as -n + "ame")
		// In this case, the actual name value is likely the second positional arg
		if name == "ame" && len(args) >= 2 {
			// User probably meant: index /path -name actualname
			// Cobra parsed it as: -n ame, with args = [/path, actualname]
			name = args[1] // Use the second arg as the name
			path = args[0] // First arg is the path
			args = args[:1] // Keep only path in args for validation
		} else if name == "ame" {
			// Just -name without value
			fmt.Fprintf(os.Stderr, "Error: Invalid flag usage. Did you mean --name or -n?\n")
			fmt.Fprintf(os.Stderr, "The flag -name is not recognized. Use --name or -n instead.\n")
			fmt.Fprintf(os.Stderr, "Example: stormindexer index /path --name myindex\n")
			os.Exit(1)
		} else {
			// Normal parsing - get path from args
			if len(args) == 0 {
				fmt.Fprintf(os.Stderr, "Error: Path argument required\n")
				os.Exit(1)
			}
			path = args[0]
			
			// If there are more args after path, it's an error (unless we handled -name above)
			if len(args) > 1 {
				fmt.Fprintf(os.Stderr, "Error: Unexpected argument: %s\n", args[1])
				fmt.Fprintf(os.Stderr, "Use --name or -n flag to specify index name\n")
				fmt.Fprintf(os.Stderr, "Example: stormindexer index %s --name %s\n", path, args[1])
				os.Exit(1)
			}
		}
		
		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid path: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Path does not exist: %s\n", absPath)
			os.Exit(1)
		}

		// Get name from flag if not already set from args
		if name == "" {
			name, _ = cmd.Flags().GetString("name")
		}
		if name == "" {
			name = filepath.Base(absPath)
		}

		calculateChecksums, _ := cmd.Flags().GetBool("checksums")
		force, _ := cmd.Flags().GetBool("force")

		// Generate index ID from path and machine ID
		indexID := generateIndexID(absPath)

		// Check if index exists
		existingIndex, err := db.GetIndex(indexID)
		if err == nil && !force {
			fmt.Printf("Index already exists: %s\n", existingIndex.Name)
			fmt.Printf("Use --force to reindex or use 'reindex' command\n")
			os.Exit(0)
		}

		// Create or update index entry
		index := &models.Index{
			ID:        indexID,
			Name:      name,
			RootPath:  absPath,
			CreatedAt: time.Now(),
			MachineID: cfg.MachineID,
		}

		if existingIndex == nil {
			if err := db.CreateIndex(index); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating index: %v\n", err)
				os.Exit(1)
			}
		}

		// Perform indexing
		idxr := indexer.NewIndexer(db, indexID, absPath)
		if err := idxr.Index(calculateChecksums); err != nil {
			fmt.Fprintf(os.Stderr, "Error indexing: %v\n", err)
			os.Exit(1)
		}

		// Update index stats
		if err := db.UpdateIndexStats(indexID); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating stats: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nIndexing completed successfully!\n")
	},
}

var reindexCmd = &cobra.Command{
	Use:   "reindex [index-id]",
	Short: "Reindex an existing index",
	Long: `Updates an existing index by scanning for changes, additions, and deletions.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		indexID := args[0]

		index, err := db.GetIndex(indexID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Index not found: %s\n", indexID)
			os.Exit(1)
		}

		calculateChecksums, _ := cmd.Flags().GetBool("checksums")

		idxr := indexer.NewIndexer(db, indexID, index.RootPath)
		if err := idxr.Reindex(calculateChecksums); err != nil {
			fmt.Fprintf(os.Stderr, "Error reindexing: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nReindexing completed successfully!\n")
	},
}

func generateIndexID(path string) string {
	// Generate a unique ID based on machine ID and path
	data := fmt.Sprintf("%s:%s", cfg.MachineID, path)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}

func init() {
	// Add name flag with both short (-n) and long (--name) forms
	nameFlag := indexCmd.Flags().StringP("name", "n", "", "Name for the index")
	_ = nameFlag // Suppress unused variable warning
	
	// Also support -name as an alias by adding it as a separate flag
	// Note: Cobra doesn't natively support single-dash multi-character flags,
	// so users should use --name or -n. But we'll handle -name in the Run function.
	indexCmd.Flags().BoolP("checksums", "c", false, "Calculate file checksums (slower but enables duplicate detection)")
	indexCmd.Flags().BoolP("force", "f", false, "Force reindex even if index exists")

	reindexCmd.Flags().BoolP("checksums", "c", false, "Calculate file checksums")

	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(reindexCmd)
}

