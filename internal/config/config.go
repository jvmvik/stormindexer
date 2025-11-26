package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	DatabasePath string `mapstructure:"database_path"`
	MachineID    string `mapstructure:"machine_id"`
}

var defaultConfig = Config{
	DatabasePath: ".stormindexer.db",
	MachineID:    getDefaultMachineID(),
}

func getDefaultMachineID() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// Load loads configuration from file or uses defaults
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.stormindexer")

	// Set defaults
	viper.SetDefault("database_path", defaultConfig.DatabasePath)
	viper.SetDefault("machine_id", defaultConfig.MachineID)

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found; use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Expand database path to absolute
	if !filepath.IsAbs(config.DatabasePath) {
		cwd, _ := os.Getwd()
		config.DatabasePath = filepath.Join(cwd, config.DatabasePath)
	}

	return config, nil
}

// Save saves the current configuration to file
func Save(config *Config) error {
	viper.Set("database_path", config.DatabasePath)
	viper.Set("machine_id", config.MachineID)

	configDir := "$HOME/.stormindexer"
	configPath := filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return viper.WriteConfigAs(configPath)
}

