package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInit_NoConfigFile_DoesNotError(t *testing.T) {
	viper.Reset()

	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "does-not-exist.yaml")

	if err := Init(cfg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestSave_CreatesParentDirAndWritesFile(t *testing.T) {
	viper.Reset()

	tmp := t.TempDir()
	cfgDir := filepath.Join(tmp, "nested", "sure-cli")
	cfg := filepath.Join(cfgDir, "config.yaml")
	viper.SetConfigFile(cfg)

	viper.Set("api_url", "http://example.test")
	viper.Set("auth.mode", "api_key")
	viper.Set("auth.api_key", "secret")

	if err := Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	// ensure readable via Init
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if got := viper.GetString("api_url"); got != "http://example.test" {
		t.Fatalf("api_url mismatch: got %q", got)
	}
}
