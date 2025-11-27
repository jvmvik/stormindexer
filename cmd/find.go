package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/victor/stormindexer/internal/database"
)

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find files across multiple drives/indexes",
	Long: `Find files across all indexed locations with support for various filters.
You can search by filename pattern, directory name, checksum, size, modification date, and more.
Duplicate files can be grouped by drive for easy review.`,
	Run: func(cmd *cobra.Command, args []string) {
		opts := database.FindOptions{}

		// Parse flags
		namePattern, _ := cmd.Flags().GetString("name")
		dirPattern, _ := cmd.Flags().GetString("dir")
		checksum, _ := cmd.Flags().GetString("checksum")
		sizeFilter, _ := cmd.Flags().GetString("size")
		indexIDs, _ := cmd.Flags().GetStringArray("index")
		duplicates, _ := cmd.Flags().GetBool("duplicates")
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		fileType, _ := cmd.Flags().GetString("type")

		opts.NamePattern = namePattern
		opts.DirectoryPattern = dirPattern
		opts.Checksum = checksum
		opts.IndexIDs = indexIDs
		opts.OnlyDuplicates = duplicates

		// Parse file type
		if fileType == "" {
			fileType = "all"
		}
		if fileType != "file" && fileType != "dir" && fileType != "directory" && fileType != "all" {
			fmt.Fprintf(os.Stderr, "Error: Invalid file type: %s. Must be 'file', 'dir', 'directory', or 'all'\n", fileType)
			os.Exit(1)
		}
		opts.FileType = fileType

		// Parse size filter
		if sizeFilter != "" {
			minSize, maxSize, err := parseSizeFilter(sizeFilter)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing size filter: %v\n", err)
				os.Exit(1)
			}
			opts.MinSize = minSize
			opts.MaxSize = maxSize
		}

		// Parse date filters
		if sinceStr != "" {
			sinceTime, err := parseDate(sinceStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing --since date: %v\n", err)
				os.Exit(1)
			}
			opts.ModifiedSince = &sinceTime
		}

		if untilStr != "" {
			untilTime, err := parseDate(untilStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing --until date: %v\n", err)
				os.Exit(1)
			}
			opts.ModifiedUntil = &untilTime
		}

		// Validate date range
		if opts.ModifiedSince != nil && opts.ModifiedUntil != nil {
			if opts.ModifiedSince.After(*opts.ModifiedUntil) {
				fmt.Fprintf(os.Stderr, "Error: --since date must be before --until date\n")
				os.Exit(1)
			}
		}

		// Execute search
		results, err := db.FindFiles(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding files: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No files found matching the criteria.")
			return
		}

		// Format and display results
		if duplicates {
			displayDuplicatesGrouped(results)
		} else {
			displayResultsTable(results, fileType)
		}
	},
}

func init() {
	findCmd.Flags().StringP("name", "n", "", "Search by filename pattern (supports wildcards: *, ?)")
	findCmd.Flags().StringP("dir", "D", "", "Search by directory name pattern (supports wildcards: *, ?)")
	findCmd.Flags().StringP("checksum", "c", "", "Search by checksum (exact match)")
	findCmd.Flags().StringP("size", "s", "", "Filter by size (e.g., >100M, <1G, =500K)")
	findCmd.Flags().StringArrayP("index", "i", []string{}, "Limit search to specific index(es) (can specify multiple)")
	findCmd.Flags().BoolP("duplicates", "d", false, "Show only duplicate files (grouped by checksum)")
	findCmd.Flags().String("since", "", "Show files modified since the given date/time (e.g., \"2 weeks ago\", \"2024-01-15\")")
	findCmd.Flags().String("until", "", "Show files modified until the given date/time (e.g., \"yesterday\", \"2024-01-20\")")
	findCmd.Flags().StringP("type", "t", "all", "Filter by type: file (only files), dir or directory (only directories), all (default: both)")

	rootCmd.AddCommand(findCmd)
}

// parseDate parses various date formats including relative dates
func parseDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	now := time.Now()

	// Handle relative dates
	switch strings.ToLower(dateStr) {
	case "yesterday":
		return now.AddDate(0, 0, -1), nil
	case "today":
		return now, nil
	}

	// Parse "N days/weeks/months/years ago"
	relativePattern := regexp.MustCompile(`^(\d+)\s+(day|days|week|weeks|month|months|year|years)\s+ago$`)
	if matches := relativePattern.FindStringSubmatch(strings.ToLower(dateStr)); matches != nil {
		num, _ := strconv.Atoi(matches[1])
		unit := matches[2]

		switch unit {
		case "day", "days":
			return now.AddDate(0, 0, -num), nil
		case "week", "weeks":
			return now.AddDate(0, 0, -num*7), nil
		case "month", "months":
			return now.AddDate(0, -num, 0), nil
		case "year", "years":
			return now.AddDate(-num, 0, 0), nil
		}
	}

	// Try parsing as ISO date
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t, nil
	}

	// Try parsing as ISO datetime
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}

	// Try parsing as common datetime format
	if t, err := time.Parse("2006-01-02 15:04:05", dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// parseSizeFilter parses size filter strings like ">100M", "<1G", "=500K"
func parseSizeFilter(sizeStr string) (int64, int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)
	var minSize, maxSize int64

	// Parse operators: >, <, =, >=, <=
	pattern := regexp.MustCompile(`^([><=]+)\s*(\d+(?:\.\d+)?)\s*([KMGT]?B?)$`)
	matches := pattern.FindStringSubmatch(strings.ToUpper(sizeStr))
	if len(matches) != 4 {
		return 0, 0, fmt.Errorf("invalid size format: %s (expected format: >100M, <1G, =500K)", sizeStr)
	}

	operator := matches[1]
	value, _ := strconv.ParseFloat(matches[2], 64)
	unit := matches[3]

	// Convert to bytes
	multiplier := int64(1)
	switch unit {
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	sizeBytes := int64(value * float64(multiplier))

	switch operator {
	case ">":
		minSize = sizeBytes + 1
	case ">=":
		minSize = sizeBytes
	case "<":
		maxSize = sizeBytes - 1
	case "<=":
		maxSize = sizeBytes
	case "=":
		minSize = sizeBytes
		maxSize = sizeBytes
	default:
		return 0, 0, fmt.Errorf("invalid operator: %s", operator)
	}

	return minSize, maxSize, nil
}

// displayResultsTable displays search results in a table format
func displayResultsTable(results []*database.FileWithIndex, fileType string) {
	var typeLabel string
	switch fileType {
	case "file":
		typeLabel = "files"
	case "dir", "directory":
		typeLabel = "directories"
	default:
		typeLabel = "items"
	}

	fmt.Printf("Found %d %s\n\n", len(results), typeLabel)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PATH\tSIZE\tMODIFIED\tCHECKSUM\tDRIVE")
	fmt.Fprintln(w, "----\t----\t--------\t--------\t-----")

	for _, result := range results {
		sizeStr := "-"
		if !result.IsDirectory {
			sizeStr = formatBytes(result.Size)
		}

		checksum := result.Checksum
		if checksum == "" {
			checksum = "-"
		} else if len(checksum) > 12 {
			checksum = checksum[:12] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			result.RelativePath,
			sizeStr,
			result.ModTime.Format("2006-01-02 15:04:05"),
			checksum,
			result.IndexName,
		)
	}

	w.Flush()
}

// displayDuplicatesGrouped displays duplicate files grouped by checksum and drive
func displayDuplicatesGrouped(results []*database.FileWithIndex) {
	// Group by checksum
	checksumGroups := make(map[string][]*database.FileWithIndex)
	for _, result := range results {
		if result.Checksum != "" {
			checksumGroups[result.Checksum] = append(checksumGroups[result.Checksum], result)
		}
	}

	// Count total files
	totalFiles := len(results)
	totalSets := len(checksumGroups)

	fmt.Printf("Found %d duplicate set(s) (%d files total)\n\n", totalSets, totalFiles)

	// Display each duplicate set
	setNum := 0
	for checksum, files := range checksumGroups {
		setNum++
		fmt.Println("========================================")
		checksumDisplay := checksum
		if len(checksumDisplay) > 12 {
			checksumDisplay = checksumDisplay[:12] + "..."
		}
		fmt.Printf("Checksum: %s (%d copies)\n", checksumDisplay, len(files))
		fmt.Println("========================================")

		// Group by drive/index
		driveGroups := make(map[string][]*database.FileWithIndex)
		for _, file := range files {
			driveKey := file.IndexName
			driveGroups[driveKey] = append(driveGroups[driveKey], file)
		}

		// Display files grouped by drive
		for driveName, driveFiles := range driveGroups {
			// Get index path from first file
			indexPath := driveFiles[0].IndexPath
			fmt.Printf("\n📁 Drive: %s (%s)\n", driveName, indexPath)

			for _, file := range driveFiles {
				sizeStr := formatBytes(file.Size)
				fmt.Printf("  • %s (%s, %s)\n",
					file.RelativePath,
					sizeStr,
					file.ModTime.Format("2006-01-02 15:04:05"),
				)
			}
		}

		if setNum < totalSets {
			fmt.Println()
		}
	}
}

