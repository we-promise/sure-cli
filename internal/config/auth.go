package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	errs "github.com/we-promise/sure-cli/internal/errors"
	"github.com/we-promise/sure-cli/internal/models"
)

func RefreshToken() string { return viper.GetString("auth.refresh_token") }

func TokenExpiresAt() (time.Time, bool) {
	s := viper.GetString("auth.token_expires_at")
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func SetAuthMode(mode string) {
	Set("auth.mode", mode)
}

func SetToken(token string) {
	Set("auth.token", token)
}

func SetRefreshToken(token string) {
	Set("auth.refresh_token", token)
}

func SetTokenExpiresAt(t time.Time) {
	Set("auth.token_expires_at", t.UTC().Format(time.RFC3339))
}

// RescueTokens reports a Save that failed after Sure issued new tokens. Sure
// has already revoked the stored refresh token, so the new pair is written to
// a new owner-only file under the user cache directory and only its path is
// reported: the error envelope lands in logs and agent transcripts.
func RescueTokens(saveErr error) *errs.CLIError {
	file, err := writeRescueFile()
	if err != nil {
		return errs.Wrap(errs.CodeConfigSaveFailed, fmt.Sprintf(
			"new tokens could not be saved to the config file (%v) nor to a rescue file (%v); log in again", saveErr, err), saveErr)
	}
	return errs.Wrap(errs.CodeConfigSaveFailed, fmt.Sprintf(
		"new tokens could not be saved to the config file (%v); copy auth.token and auth.refresh_token from details.rescue_file into it with `sure-cli config set`, then delete that file, or log in again", saveErr), saveErr).
		WithDetails(map[string]any{"rescue_file": file})
}

func writeRescueFile() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, DefaultAppDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// CreateTemp opens a new 0600 file exclusively.
	f, err := os.CreateTemp(dir, "rescued-tokens-*."+DefaultConfigType)
	if err != nil {
		return "", err
	}
	pair := viper.New()
	pair.SetConfigType(DefaultConfigType)
	for _, k := range []string{"auth.token", "auth.refresh_token", "auth.token_expires_at"} {
		pair.Set(k, viper.GetString(k))
	}
	err = pair.WriteConfigTo(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// LegacyDeviceID is the device_id every install shared before per-install
// ids. Sure's MobileDevice#issue_token! revokes all tokens of a device, so a
// shared id made each login log out every other machine.
const LegacyDeviceID = "sure-cli"

const deviceIDKey = "auth.device.device_id"

// EnsureDeviceID returns this install's device_id, generating a random UUID
// when none is configured or only the legacy shared id is. A custom
// configured value is kept. The id is set in memory; callers persist it with
// Save (login does, together with the tokens).
func EnsureDeviceID() string {
	id := strings.TrimSpace(viper.GetString(deviceIDKey))
	if id != "" && id != LegacyDeviceID {
		return id
	}
	id = newUUID()
	Set(deviceIDKey, id)
	return id
}

// newUUID returns a random RFC 4122 version 4 UUID.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err)) // never fails on supported platforms
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func Device() models.DeviceInfo {
	dt := strings.ToLower(strings.TrimSpace(viper.GetString("auth.device.device_type")))
	switch dt {
	case "ios", "android", "web":
		// Accepted by MobileDevice's device_type validation.
	default:
		// Keep CLI resilient if config is wrong (e.g. legacy "browser").
		dt = "android"
	}

	id := strings.TrimSpace(viper.GetString(deviceIDKey))
	name := strings.TrimSpace(viper.GetString("auth.device.device_name"))
	osv := strings.TrimSpace(viper.GetString("auth.device.os_version"))
	appv := strings.TrimSpace(viper.GetString("auth.device.app_version"))

	if id == "" {
		// Sessions from before EnsureDeviceID keep refreshing under the old id.
		id = LegacyDeviceID
	}
	if name == "" {
		name = "sure-cli"
	}
	if osv == "" {
		osv = "unknown"
	}
	if appv == "" {
		appv = "sure-cli"
	}

	return models.DeviceInfo{
		DeviceID:   id,
		DeviceName: name,
		DeviceType: dt,
		OSVersion:  osv,
		AppVersion: appv,
	}
}
