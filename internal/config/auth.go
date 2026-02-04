package config

import (
	"strings"
	"time"

	"github.com/dgilperez/sure-cli/internal/models"
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

func Device() models.DeviceInfo {
	dt := viper.GetString("auth.device.device_type")
	dt = strings.ToLower(strings.TrimSpace(dt))
	if dt == "browser" {
		dt = "android"
	}
	if dt == "web" {
		// Some Sure deployments currently only accept ios|android.
		dt = "android"
	}
	switch dt {
	case "ios", "android":
		// ok
	default:
		// Keep CLI resilient if config is wrong.
		dt = "android"
	}

	id := strings.TrimSpace(viper.GetString("auth.device.device_id"))
	name := strings.TrimSpace(viper.GetString("auth.device.device_name"))
	osv := strings.TrimSpace(viper.GetString("auth.device.os_version"))
	appv := strings.TrimSpace(viper.GetString("auth.device.app_version"))

	if id == "" {
		id = "sure-cli"
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
