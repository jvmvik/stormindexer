package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/victor/stormindexer/internal/config"
	"github.com/victor/stormindexer/internal/database"
)

var cfg *config.Config
var db *database.DB

var rootCmd = &cobra.Command{
	Use:   "stormindexer",
	Short: "A powerful file indexing and syncing tool",
	Long: `StormIndexer is a tool for indexing files across multiple disks,
machines, and external drives. It tracks file metadata, calculates checksums,
and enables synchronization between different locations.`,
}

func init() {
	cobra.OnInitialize(initConfig, initDB)
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
}

func initDB() {
	var err error
	db, err = database.NewDB(cfg.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func Cleanup() {
	if db != nil {
		db.Close()
	}
}

