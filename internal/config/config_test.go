package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestInit_NoConfigFile_DoesNotError(t *testing.T) {
	viper.Reset()

	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "does-not-exist.yaml")

	if err := Init(cfg); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestSave_CreatesParentDirAndWritesFile(t *testing.T) {
	viper.Reset()

	tmp := t.TempDir()
	cfgDir := filepath.Join(tmp, "nested", "sure-cli")
	cfg := filepath.Join(cfgDir, "config.yaml")
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}

	Set("api_url", "http://example.test")
	Set("auth.mode", "api_key")
	Set("auth.api_key", "secret")

	if err := Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	// ensure readable via Init
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if got := viper.GetString("api_url"); got != "http://example.test" {
		t.Fatalf("api_url mismatch: got %q", got)
	}
}

func TestSave_MergesOnlyKeysSetInThisProcess(t *testing.T) {
	// Another process rotated the tokens after this one loaded the file; a
	// `config set` here must not write the revoked pair back.
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfg, "auth:\n  token: at-old\n  refresh_token: rt-old\n")
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	writeFile(t, cfg, "auth:\n  token: at-new\n  refresh_token: rt-new\n")

	Set("api_url", "http://sure.test")
	if err := Save(); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	if Token() != "at-new" || RefreshToken() != "rt-new" || APIURL() != "http://sure.test" {
		t.Fatalf("token=%q refresh=%q api_url=%q", Token(), RefreshToken(), APIURL())
	}
}

func TestSave_NewFileIsOwnerOnlyAndLeavesNoTempFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	Set("auth.token", "secret")
	if err := Save(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", st.Mode().Perm())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("dir holds %d entries, want only config.yaml", len(entries))
	}
}

func TestSave_UnreadableFileOnDiskIsNotClobbered(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	broken := "api_url: http://x\nauth:\n  token: \"abc"
	writeFile(t, cfg, broken)

	Set("auth.token", "new")
	if err := Save(); err == nil {
		t.Fatal("expected an error saving over an unparseable config")
	}
	if b, _ := os.ReadFile(cfg); string(b) != broken {
		t.Fatalf("config rewritten: %q", b)
	}
}

func TestSave_WritesThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles-config.yaml")
	writeFile(t, target, "api_url: http://old\n")
	link := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	if err := Init(link); err != nil {
		t.Fatal(err)
	}
	Set("api_url", "http://new")
	if err := Save(); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced by a regular file (%v)", err)
	}
	if b, _ := os.ReadFile(target); !strings.Contains(string(b), "http://new") {
		t.Fatalf("target not updated: %q", b)
	}
}

func TestCheckWritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced")
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	if err := CheckWritable(); err != nil {
		t.Fatalf("writable dir reported unwritable: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := CheckWritable(); err == nil {
		t.Fatal("read-only dir reported writable")
	}
}

func TestReloadAuth_PicksUpRotationFromDisk(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfg, "auth:\n  token: at-old\n  refresh_token: rt-old\n")
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	writeFile(t, cfg, "auth:\n  token: at-new\n  refresh_token: rt-new\n  token_expires_at: \"2099-01-01T00:00:00Z\"\n")
	if err := Reload(); err != nil {
		t.Fatal(err)
	}
	if Token() != "at-new" || RefreshToken() != "rt-new" {
		t.Fatalf("token=%q refresh=%q", Token(), RefreshToken())
	}
}

func TestTimeout(t *testing.T) {
	viper.Reset()
	if err := Init(filepath.Join(t.TempDir(), "none.yaml")); err != nil {
		t.Fatal(err)
	}
	if got := Timeout(); got != 30*time.Second {
		t.Fatalf("default timeout = %v, want 30s", got)
	}
	viper.Set("timeout", "90s")
	if got := Timeout(); got != 90*time.Second {
		t.Fatalf("timeout = %v, want 90s", got)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
