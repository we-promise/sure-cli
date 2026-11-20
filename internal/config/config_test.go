package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	errs "github.com/we-promise/sure-cli/internal/errors"
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

func TestSave_NewFileIsOwnerOnlyAndLeavesOnlyItsLockFile(t *testing.T) {
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
	var names []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, " ") != ".config.yaml.lock config.yaml" {
		t.Fatalf("dir holds %v, want only config.yaml and its lock file", names)
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
	for v, want := range map[any]time.Duration{"90s": 90 * time.Second, "30": 30 * time.Second, 45: 45 * time.Second, "0": 0} {
		viper.Set("timeout", v)
		if got := Timeout(); got != want {
			t.Fatalf("timeout %#v = %v, want %v", v, got, want)
		}
	}
}

func TestParseTimeout(t *testing.T) {
	for in, want := range map[string]time.Duration{"30": 30 * time.Second, " 2m ": 2 * time.Minute, "1.5s": 1500 * time.Millisecond, "0": 0, "0s": 0} {
		if got, err := ParseTimeout(in); err != nil || got != want {
			t.Errorf("ParseTimeout(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "-5s", "-1", "30x", "9300000000", "-9300000000"} {
		if _, err := ParseTimeout(in); err == nil {
			t.Errorf("ParseTimeout(%q) accepted", in)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		apiURL, timeout string
		allowHTTP       bool
		wantErr         string
	}{
		{"https://sure.example.com", "30s", false, ""},
		{"http://localhost:3000", "30s", false, ""},
		{"http://127.0.0.1:8080/", "30s", false, ""},
		{"http://[::1]:3000", "30s", false, ""},
		{"http://app.localhost", "30s", false, ""},
		{"http://sure.example.com", "30s", false, "allow-insecure-http"},
		{"http://localhost.evil.com", "30s", false, "allow-insecure-http"},
		{"http://192.168.1.10:3000", "30s", false, "allow-insecure-http"},
		{"http://sure.example.com", "30s", true, ""},
		{"ftp://sure.example.com", "30s", true, "api_url"},
		{"sure.example.com", "30s", false, "api_url"},
		{"https://sure.example.com", "abc", false, "timeout"},
		{"https://sure.example.com", "-5s", false, "timeout"},
	}
	for _, c := range cases {
		viper.Reset()
		if err := Init(filepath.Join(t.TempDir(), "none.yaml")); err != nil {
			t.Fatal(err)
		}
		viper.Set("api_url", c.apiURL)
		viper.Set("timeout", c.timeout)
		viper.Set("allow_insecure_http", c.allowHTTP)
		err := Validate()
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%+v: unexpected error %v", c, err)
		case c.wantErr != "" && (err == nil || err.Code != errs.CodeValidation || !strings.Contains(err.Message, c.wantErr)):
			t.Errorf("%+v: error = %v, want validation_failed mentioning %q", c, err, c.wantErr)
		}
	}
}

// TestHelperSaver is the child process of TestSave_ConcurrentProcessesLoseNoUpdates.
func TestHelperSaver(t *testing.T) {
	cfg := os.Getenv("SURE_CLI_TEST_SAVER_CFG")
	if cfg == "" {
		t.Skip("helper process")
	}
	who := os.Getenv("SURE_CLI_TEST_SAVER_WHO")
	for i := 0; i < saverKeys; i++ {
		viper.Reset()
		if err := Init(cfg); err != nil {
			t.Fatal(err)
		}
		Set(fmt.Sprintf("k.%s%d", who, i), "v")
		if err := Save(); err != nil {
			t.Fatal(err)
		}
	}
}

const saverKeys = 40

func TestSave_ConcurrentProcessesLoseNoUpdates(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfg, "auth:\n  token: t\n")
	writers := []string{"a", "b", "c"}
	var wg sync.WaitGroup
	for _, who := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperSaver$", "-test.count=1")
			cmd.Env = append(os.Environ(), "SURE_CLI_TEST_SAVER_CFG="+cfg, "SURE_CLI_TEST_SAVER_WHO="+who)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("writer %s: %v\n%s", who, err, out)
			}
		}()
	}
	wg.Wait()
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	for _, who := range writers {
		for i := 0; i < saverKeys; i++ {
			if viper.GetString(fmt.Sprintf("k.%s%d", who, i)) != "v" {
				t.Fatalf("key k.%s%d lost by a concurrent Save", who, i)
			}
		}
	}
	if Token() != "t" {
		t.Fatal("auth.token lost")
	}
}

func TestSave_DanglingSymlinkCreatesTheTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dotfiles"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.yaml")
	// Relative, and not created yet: a dotfiles manager links before first use.
	if err := os.Symlink(filepath.Join("dotfiles", "sure.yaml"), link); err != nil {
		t.Fatal(err)
	}
	viper.Reset()
	if err := Init(link); err != nil {
		t.Fatal(err)
	}
	Set("api_url", "https://sure.test")
	if err := Save(); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced by a regular file (%v)", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "dotfiles", "sure.yaml")); !strings.Contains(string(b), "https://sure.test") {
		t.Fatalf("target not written: %q", b)
	}
}

func TestSave_NeverLoosensFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	for before, want := range map[os.FileMode]os.FileMode{0o644: 0o600, 0o640: 0o600, 0o600: 0o600, 0o400: 0o400} {
		cfg := filepath.Join(t.TempDir(), "config.yaml")
		writeFile(t, cfg, "api_url: https://a\n")
		if err := os.Chmod(cfg, before); err != nil {
			t.Fatal(err)
		}
		viper.Reset()
		if err := Init(cfg); err != nil {
			t.Fatal(err)
		}
		Set("api_url", "https://b")
		if err := Save(); err != nil {
			t.Fatal(err)
		}
		if st, _ := os.Stat(cfg); st.Mode().Perm() != want {
			t.Errorf("mode %v after Save = %v, want %v", before, st.Mode().Perm(), want)
		}
	}
}

func TestCheckOwner_RefusesAFileOfAnotherUser(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file owned by another POSIX user")
	}
	root, err := os.Stat("/")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkOwner(root); err == nil {
		t.Fatal("a root-owned file passed the owner check")
	}
	mine, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkOwner(mine); err != nil {
		t.Fatalf("own file refused: %v", err)
	}
}

func TestInit_RemovesTempFilesACrashedSaveLeft(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	writeFile(t, cfg, "api_url: https://a\n")
	stale := filepath.Join(dir, ".config.yaml.tmp-123456")
	writeFile(t, stale, "auth:\n  refresh_token: leaked\n")
	other := filepath.Join(dir, "notes.txt")
	writeFile(t, other, "keep")
	viper.Reset()
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp file still there (%v)", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("unrelated file removed: %v", err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInit_ReadsAConfigFileOfAnyName(t *testing.T) {
	// The config is YAML whatever the file is called (~/.sure-cli, a file
	// descriptor from a process substitution, ...).
	for _, name := range []string{"sure-cli", ".sure-cli", "config.conf"} {
		viper.Reset()
		cfg := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(cfg, []byte("api_url: https://sure.example\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Init(cfg); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := APIURL(); got != "https://sure.example" {
			t.Fatalf("%s: api_url = %q", name, got)
		}
		Set("timeout", "45s")
		if err := Save(); err != nil {
			t.Fatalf("%s: save: %v", name, err)
		}
		if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "https://sure.example") || !strings.Contains(string(b), "45s") {
			t.Fatalf("%s: saved config = %s", name, b)
		}
	}
}
