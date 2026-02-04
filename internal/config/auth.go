package config

import (
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
	return models.DeviceInfo{
		DeviceID:   viper.GetString("auth.device.device_id"),
		DeviceName: viper.GetString("auth.device.device_name"),
		DeviceType: viper.GetString("auth.device.device_type"),
		OSVersion:  viper.GetString("auth.device.os_version"),
		AppVersion: viper.GetString("auth.device.app_version"),
	}
}
