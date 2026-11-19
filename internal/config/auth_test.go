package config

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/spf13/viper"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func initTemp(t *testing.T) string {
	t.Helper()
	viper.Reset()
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestDevice_DeviceTypeAcceptsSureValues(t *testing.T) {
	// MobileDevice validates device_type in %w[ios android web].
	cases := map[string]string{
		"":        "android",
		"ios":     "ios",
		"android": "android",
		"web":     "web",
		" WEB ":   "web",
		"browser": "android",
		"desktop": "android",
	}
	for in, want := range cases {
		initTemp(t)
		if in != "" {
			viper.Set("auth.device.device_type", in)
		}
		if got := Device().DeviceType; got != want {
			t.Errorf("device_type %q -> %q, want %q", in, got, want)
		}
	}
}

func TestDevice_FallsBackToLegacyIDWhenUnset(t *testing.T) {
	// Sessions created before per-install ids keep refreshing as "sure-cli".
	initTemp(t)
	if got := Device().DeviceID; got != LegacyDeviceID {
		t.Fatalf("device_id = %q, want %q", got, LegacyDeviceID)
	}
}

func TestEnsureDeviceID_GeneratesStablePerInstallID(t *testing.T) {
	// Sure's issue_token! revokes every token of the device, so a shared
	// constant id logs out every other machine on each login.
	for _, existing := range []string{"", LegacyDeviceID} {
		cfg := initTemp(t)
		if existing != "" {
			viper.Set("auth.device.device_id", existing)
		}
		id := EnsureDeviceID()
		if !uuidV4.MatchString(id) {
			t.Fatalf("existing %q: generated id %q is not a v4 UUID", existing, id)
		}
		if again := EnsureDeviceID(); again != id {
			t.Fatalf("id not stable: %q then %q", id, again)
		}
		if Device().DeviceID != id {
			t.Fatalf("Device().DeviceID = %q, want %q", Device().DeviceID, id)
		}
		if err := Save(); err != nil {
			t.Fatal(err)
		}
		viper.Reset()
		if err := Init(cfg); err != nil {
			t.Fatal(err)
		}
		if got := Device().DeviceID; got != id {
			t.Fatalf("persisted id = %q, want %q", got, id)
		}
	}

	initTemp(t)
	if a, b := EnsureDeviceID(), func() string { initTemp(t); return EnsureDeviceID() }(); a == b {
		t.Fatalf("two installs got the same id %q", a)
	}
}

func TestEnsureDeviceID_KeepsCustomID(t *testing.T) {
	initTemp(t)
	viper.Set("auth.device.device_id", "my-laptop")
	if got := EnsureDeviceID(); got != "my-laptop" {
		t.Fatalf("got %q, want configured my-laptop", got)
	}
}
