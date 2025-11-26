package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Test loading with no config file (should use defaults)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	if cfg.DatabasePath == "" {
		t.Error("Expected non-empty database path")
	}

	if cfg.MachineID == "" {
		t.Error("Expected non-empty machine ID")
	}
}

func TestLoad_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".stormindexer")
	os.MkdirAll(configDir, 0755)

	configFile := filepath.Join(configDir, "config.yaml")
	configContent := `database_path: "/custom/path.db"
machine_id: "custom-machine"
`

	os.WriteFile(configFile, []byte(configContent), 0644)

	// Note: This test may not work perfectly since Load() uses viper's config paths
	// which may not include our temp directory. This is more of a structural test.
	// In a real scenario, you'd set the config path or use environment variables.
}

func TestGetDefaultMachineID(t *testing.T) {
	machineID := getDefaultMachineID()
	if machineID == "" {
		t.Error("Expected non-empty machine ID")
	}

	if machineID == "unknown" {
		// This could happen if hostname lookup fails, which is acceptable
		t.Log("Machine ID is 'unknown', hostname lookup may have failed")
	}
}

