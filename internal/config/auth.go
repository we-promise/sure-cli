package config

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
	"github.com/spf13/viper"
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
	viper.Set("auth.mode", mode)
}

func SetToken(token string) {
	viper.Set("auth.token", token)
}

func SetRefreshToken(token string) {
	viper.Set("auth.refresh_token", token)
}

func SetTokenExpiresAt(t time.Time) {
	viper.Set("auth.token_expires_at", t.UTC().Format(time.RFC3339))
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
	viper.Set(deviceIDKey, id)
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
