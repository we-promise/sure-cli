package models

// DeviceInfo is required by Sure's Api::V1::AuthController for both login and refresh.
// device_type must be one of: ios|android|web.
type DeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"` // ios|android|web
	OSVersion  string `json:"os_version"`
	AppVersion string `json:"app_version"`
}
