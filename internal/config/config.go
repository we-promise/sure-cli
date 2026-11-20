package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	if err := Reload(); err != nil {
		return err
	}
	removeStaleTemps()
	return nil
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
// disk now and replaces the file atomically. The config's lock file
// serializes concurrent Saves, so no process merges into a file another is
// about to replace.
func Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	err = withLock(path, func() error {
		disk, err := readDisk(path)
		if err != nil {
			return err
		}
		for k, v := range dirty {
			disk.Set(k, v)
		}
		return writeAtomic(path, disk.WriteConfigTo)
	})
	if err != nil {
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
	return withLock(path, func() error {
		if _, err := readDisk(path); err != nil {
			return err
		}
		return writeAtomic(path, nil)
	})
}

// configPath is the file Save writes: the one in use, resolved through
// symlinks so a dotfiles-managed link keeps pointing at its target.
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
	return resolveLinks(path)
}

// maxLinks bounds symlink chains, as the kernel does (SYMLOOP_MAX).
const maxLinks = 40

// resolveLinks follows path through symlinks, also when the final target
// does not exist yet, which filepath.EvalSymlinks refuses.
func resolveLinks(path string) (string, error) {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real, nil
	}
	for range maxLinks {
		fi, err := os.Lstat(path)
		if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
			return path, nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	return "", fmt.Errorf("config path %s: too many levels of symbolic links", path)
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

// withLock runs fn holding the exclusive lock on path's lock file, creating
// path's directory if needed. Every writer holds the lock while its temp
// file exists, so the temp files found under it were left by a crash.
func withLock(path string, fn func() error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "."+filepath.Base(path)+".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() // closing releases the lock
	if err := lockFile(f); err != nil {
		return fmt.Errorf("lock config: %w", err)
	}
	for _, stale := range tempFiles(path) {
		_ = os.Remove(stale)
	}
	return fn()
}

// tempFiles lists the temp files writeAtomic creates for path.
func tempFiles(path string) []string {
	dir, prefix := filepath.Dir(path), "."+filepath.Base(path)+".tmp-"
	entries, _ := os.ReadDir(dir)
	var found []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	return found
}

// removeStaleTemps deletes the temp files, holding a copy of the tokens, that
// a Save killed before its rename left next to the config. It is best
// effort: a read-only directory only keeps them.
func removeStaleTemps() {
	path, err := configPath()
	if err != nil || len(tempFiles(path)) == 0 {
		return
	}
	_ = withLock(path, func() error { return nil })
}

// writeAtomic writes path through a temp file in the same directory, synced
// and renamed into place, so a crash never leaves a truncated config. The
// file keeps its mode when that is stricter than owner-only 0600, and is
// never loosened; a file owned by another user (e.g. under sudo) is refused.
// A nil write only proves the directory accepts the temp file.
func writeAtomic(path string, write func(io.Writer) error) error {
	perm := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		if err := checkOwner(fi); err != nil {
			return err
		}
		perm = fi.Mode().Perm()&0o600 | 0o400
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
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
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmp.Name(), path)
}

func APIURL() string { return viper.GetString("api_url") }

func AuthMode() string { return viper.GetString("auth.mode") }
func Token() string    { return viper.GetString("auth.token") }
func APIKey() string   { return viper.GetString("auth.api_key") }

// Timeout bounds one API call, retries included; for a download it bounds the
// wait for headers and any pause between body bytes. Zero disables it. Set it
// with --timeout or the `timeout` config key (a Go duration such as "2m").
func Timeout() time.Duration { return viper.GetDuration("timeout") }
