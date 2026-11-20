package root

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	errs "github.com/we-promise/sure-cli/internal/errors"
)

// rotatingServer answers every refresh with the pair at-new/rt-new and runs
// onRefresh first.
func rotatingServer(t *testing.T, refreshes *atomic.Int32, onRefresh func()) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes.Add(1)
		onRefresh()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-new","refresh_token":"rt-new","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func bearerConfig(t *testing.T, dir, srvURL string) string {
	t.Helper()
	cfg := filepath.Join(dir, "config.yaml")
	body := "api_url: " + srvURL + "\nauth:\n  mode: bearer\n  token: at-old\n  refresh_token: rt-old\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func skipWithoutPOSIXPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced")
	}
}

func TestRefresh_NotSentWhenConfigIsNotWritable(t *testing.T) {
	skipWithoutPOSIXPermissions(t)
	var refreshes atomic.Int32
	srv := rotatingServer(t, &refreshes, func() {})
	dir := t.TempDir()
	cfg := bearerConfig(t, dir, srv.URL)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if ce := cliError(t, "--config", cfg, "refresh"); ce == nil || ce.Code != errs.CodeConfigSaveFailed {
		t.Fatalf("error = %v, want config_save_failed", ce)
	}
	if refreshes.Load() != 0 {
		t.Fatal("refresh sent although the rotated tokens could not be saved")
	}
}

func TestRefresh_SaveFailureRescuesTheRotatedPairToAPrivateFile(t *testing.T) {
	skipWithoutPOSIXPermissions(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	dir := t.TempDir()
	var refreshes atomic.Int32
	// The directory turns read-only after the writability check: Sure has
	// already revoked rt-old when the save fails.
	srv := rotatingServer(t, &refreshes, func() { _ = os.Chmod(dir, 0o500) })
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	cfg := bearerConfig(t, dir, srv.URL)

	ce, stdout := cliRun(t, "--config", cfg, "refresh")
	if strings.Contains(stdout, "rt-new") {
		t.Fatalf("rotated refresh token printed: %s", stdout)
	}
	if ce == nil || ce.Code != errs.CodeConfigSaveFailed {
		t.Fatalf("error = %v, want config_save_failed", ce)
	}
	file, _ := ce.Details["rescue_file"].(string)
	if b, err := os.ReadFile(file); err != nil || !strings.Contains(string(b), "rt-new") {
		t.Fatalf("rescue_file %q: %v %q", file, err, b)
	}
}
