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
	DefaultAppDir      = "sure-cli"
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
	viper.SetDefault("auth.api_key", "")

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
