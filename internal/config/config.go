package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultConfigName = "config"
	DefaultConfigType = "yaml"
	DefaultAppDir     = "sure-cli"
)

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", DefaultAppDir, DefaultConfigName+"."+DefaultConfigType), nil
}

func Init(cfgFile string) error {
	// Defaults
	viper.SetDefault("api_url", "http://localhost:3000")
	viper.SetDefault("auth.mode", "bearer") // bearer|api_key
	viper.SetDefault("auth.token", "")
	viper.SetDefault("auth.refresh_token", "")
	viper.SetDefault("auth.token_expires_at", "") // RFC3339
	viper.SetDefault("auth.api_key", "")

	// Device info required by Sure AuthController. device_id has no default:
	// login generates a per-install id (see EnsureDeviceID).
	viper.SetDefault("auth.device.device_name", "sure-cli")
	viper.SetDefault("auth.device.device_type", "android")
	viper.SetDefault("auth.device.os_version", "unknown")
	viper.SetDefault("auth.device.app_version", "sure-cli")

	// Heuristics config (insights)
	viper.SetDefault("heuristics.leaks.min_count", 3)
	viper.SetDefault("heuristics.leaks.min_total", 15.0)
	viper.SetDefault("heuristics.leaks.max_avg", 10.0)
	viper.SetDefault("heuristics.rules.min_consistency", 0.7)
	viper.SetDefault("heuristics.rules.min_occurrences", 2)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		path, err := defaultConfigPath()
		if err != nil {
			return err
		}
		viper.SetConfigFile(path)
	}

	if err := viper.ReadInConfig(); err != nil {
		// If config doesn't exist, that's OK.
		// Viper may return an *os.PathError when SetConfigFile points to a non-existent file.
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		}
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

func Save() error {
	cfgFile := viper.ConfigFileUsed()
	if cfgFile == "" {
		path, err := defaultConfigPath()
		if err != nil {
			return err
		}
		cfgFile = path
		viper.SetConfigFile(cfgFile)
	}

	if err := os.MkdirAll(filepath.Dir(cfgFile), 0o700); err != nil {
		return err
	}
	if err := viper.WriteConfigAs(cfgFile); err != nil {
		return err
	}
	// Config contains sensitive credentials (tokens, API keys) — restrict to owner-only.
	return os.Chmod(cfgFile, 0o600)
}

func APIURL() string { return viper.GetString("api_url") }

func AuthMode() string { return viper.GetString("auth.mode") }
func Token() string    { return viper.GetString("auth.token") }
func APIKey() string   { return viper.GetString("auth.api_key") }
