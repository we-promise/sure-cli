package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	errs "github.com/we-promise/sure-cli/internal/errors"
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

// tempCacheDir points os.UserCacheDir at a fresh directory.
func tempCacheDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("LocalAppData", filepath.Join(home, "AppData"))
	dir, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRescueTokens_WritesThePairToAPrivateFileAndReportsOnlyItsPath(t *testing.T) {
	cache := tempCacheDir(t)
	initTemp(t)
	SetToken("at-new")
	SetRefreshToken("rt-new")
	SetTokenExpiresAt(time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))

	ce := RescueTokens(errors.New("disk full"))
	if ce.Code != errs.CodeConfigSaveFailed || ce.Cause == nil || ce.Cause.Error() != "disk full" {
		t.Fatalf("error = %#v", ce)
	}
	if b, _ := json.Marshal(ce); strings.Contains(string(b), "rt-new") || strings.Contains(string(b), "at-new") {
		t.Fatalf("token leaked into the error envelope: %s", b)
	}
	file, _ := ce.Details["rescue_file"].(string)
	if !strings.HasPrefix(file, cache) {
		t.Fatalf("rescue_file = %q, want a file under %s", file, cache)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"refresh_token: rt-new", "token: at-new", "2030-01-02T03:04:05Z"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("rescue file lacks %q:\n%s", want, b)
		}
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(file); st.Mode().Perm() != 0o600 {
			t.Fatalf("rescue file mode = %v, want 0600", st.Mode().Perm())
		}
	}
	if again := RescueTokens(errors.New("disk full")); again.Details["rescue_file"] == file {
		t.Fatal("second rescue reused the first file")
	}
}

func TestRescueTokens_UnwritableCacheStillHidesTheToken(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced")
	}
	cache := tempCacheDir(t)
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cache, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cache, 0o700) })
	initTemp(t)
	SetRefreshToken("rt-new")

	ce := RescueTokens(errors.New("disk full"))
	if b, _ := json.Marshal(ce); ce.Code != errs.CodeConfigSaveFailed || strings.Contains(string(b), "rt-new") {
		t.Fatalf("error = %s", b)
	}
	if _, ok := ce.Details["rescue_file"]; ok || !strings.Contains(ce.Message, "log in again") {
		t.Fatalf("error = %#v", ce)
	}
}
