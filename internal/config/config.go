package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

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

// DefaultTimeout bounds one API call, retries included (see Timeout).
const DefaultTimeout = 30 * time.Second

// dirty holds the keys Set in this process. Save writes only these over the
// file as it is on disk, so a token rotation another process persisted after
// this one loaded the config is not overwritten with stale values.
var dirty = map[string]any{}

func Init(cfgFile string) error {
	dirty = map[string]any{}

	// Defaults
	viper.SetDefault("api_url", "http://localhost:3000")
	viper.SetDefault("timeout", DefaultTimeout.String())
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

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		path, err := defaultConfigPath()
		if err != nil {
			return err
		}
		viper.SetConfigFile(path)
	}
	return Reload()
}

// Reload re-reads the config file into memory, e.g. to pick up tokens another
// process rotated. A missing file is not an error.
func Reload() error {
	if err := viper.ReadInConfig(); err != nil && !isNotFound(err) {
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

// isNotFound reports a missing config file. Viper returns an *os.PathError
// when SetConfigFile points to a file that does not exist.
func isNotFound(err error) bool {
	var nf viper.ConfigFileNotFoundError
	return errors.As(err, &nf) || errors.Is(err, fs.ErrNotExist)
}

// Set changes key in memory and marks it for the next Save.
func Set(key string, value any) {
	viper.Set(key, value)
	dirty[key] = value
}

// Save merges the keys Set in this process into the config file as it is on
// disk now and replaces the file atomically with owner-only permissions.
func Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	disk, err := readDisk(path)
	if err != nil {
		return err
	}
	for k, v := range dirty {
		disk.Set(k, v)
	}
	if err := writeAtomic(path, disk.WriteConfigTo); err != nil {
		return err
	}
	dirty = map[string]any{}
	return nil
}

// CheckWritable reports whether Save can succeed, without changing anything.
// Callers about to rotate credentials check first, since a token Sure has
// rotated but the CLI cannot persist logs the user out.
func CheckWritable() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if _, err := readDisk(path); err != nil {
		return err
	}
	return writeAtomic(path, nil)
}

// configPath is the file Save writes: the one in use, resolved through a
// symlink so a dotfiles-managed link keeps pointing at its target.
func configPath() (string, error) {
	path := viper.ConfigFileUsed()
	if path == "" {
		p, err := defaultConfigPath()
		if err != nil {
			return "", err
		}
		path = p
		viper.SetConfigFile(path)
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path, nil
}

// readDisk loads the file at path into a fresh Viper without defaults, so
// Save writes back only what the file already holds plus the changed keys.
func readDisk(path string) (*viper.Viper, error) {
	disk := viper.New()
	disk.SetConfigFile(path)
	if err := disk.ReadInConfig(); err != nil && !isNotFound(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return disk, nil
}

// writeAtomic writes path through a 0600 temp file in the same directory,
// synced and renamed into place, so a crash never leaves a truncated config.
// A nil write only proves the directory accepts the temp file.
func writeAtomic(path string, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	defer tmp.Close()
	if write == nil {
		return tmp.Close()
	}
	if err := write(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func APIURL() string { return viper.GetString("api_url") }

func AuthMode() string { return viper.GetString("auth.mode") }
func Token() string    { return viper.GetString("auth.token") }
func APIKey() string   { return viper.GetString("auth.api_key") }

// Timeout bounds one API call, retries included; for a download it bounds the
// wait for headers and any pause between body bytes. Zero disables it. Set it
// with --timeout or the `timeout` config key (a Go duration such as "2m").
func Timeout() time.Duration { return viper.GetDuration("timeout") }
