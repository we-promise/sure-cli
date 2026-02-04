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

	// Device info required by Sure AuthController
	viper.SetDefault("auth.device.device_id", "sure-cli")
	viper.SetDefault("auth.device.device_name", "sure-cli")
	viper.SetDefault("auth.device.device_type", "android")
	viper.SetDefault("auth.device.os_version", "unknown")
	viper.SetDefault("auth.device.app_version", "sure-cli")

	// Heuristics config (insights)
	viper.SetDefault("heuristics.fees.keywords", []string{}) // empty = use defaults
	viper.SetDefault("heuristics.subscriptions.period_min_days", 20)
	viper.SetDefault("heuristics.subscriptions.period_max_days", 40)
	viper.SetDefault("heuristics.subscriptions.weekly_min_days", 6)
	viper.SetDefault("heuristics.subscriptions.weekly_max_days", 9)
	viper.SetDefault("heuristics.subscriptions.stddev_max_days", 3.0)
	viper.SetDefault("heuristics.subscriptions.amount_stddev_ratio", 0.1)
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

	if err := os.MkdirAll(filepath.Dir(cfgFile), 0o755); err != nil {
		return err
	}
	return viper.WriteConfigAs(cfgFile)
}

func APIURL() string { return viper.GetString("api_url") }

func AuthMode() string { return viper.GetString("auth.mode") }
func Token() string    { return viper.GetString("auth.token") }
func APIKey() string   { return viper.GetString("auth.api_key") }
