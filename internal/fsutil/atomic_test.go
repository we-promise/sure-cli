package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeString(s string) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := io.WriteString(w, s)
		return err
	}
}

// entries lists dir, so a test can tell no temp file was left behind.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, de := range des {
		names = append(names, de.Name())
	}
	return names
}

func TestWriteFileAtomic_WritesAPrivateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	if err := WriteFileAtomic(path, writeString("data")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "data" {
		t.Fatalf("content = %q, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(path)
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Fatalf("mode = %v, want 0600: the file holds financial data", perm)
		}
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Fatalf("dir holds %v, want only out.csv", names)
	}
}

func TestWriteFileAtomic_ReplacesASymlinkInsteadOfFollowingIt(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("IMPORTANT"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "out.csv")
	if err := os.Symlink(victim, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := WriteFileAtomic(link, writeString("data")); err != nil {
		t.Fatal(err)
	}
	if v, _ := os.ReadFile(victim); string(v) != "IMPORTANT" {
		t.Fatalf("symlink target overwritten: %q", v)
	}
	fi, err := os.Lstat(link)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("out.csv is still a symlink (%v, %v)", fi.Mode(), err)
	}
}

func TestWriteFileAtomic_FailureKeepsTheOldFileAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := WriteFileAtomic(path, func(w io.Writer) error {
		_, _ = io.WriteString(w, "half")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the writer's error", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "previous" {
		t.Fatalf("content = %q, want the previous file untouched", got)
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Fatalf("dir holds %v, want only out.csv", names)
	}
}

func TestWriteFileAtomic_MissingDirectory(t *testing.T) {
	if err := WriteFileAtomic(filepath.Join(t.TempDir(), "missing", "out"), writeString("")); err == nil {
		t.Fatal("expected an error creating a file in a missing directory")
	}
}

func TestWriteFileAtomic_WritesToADeviceDirectly(t *testing.T) {
	// A temp file cannot be created next to /dev/null (NUL on Windows), nor
	// renamed over it.
	if err := WriteFileAtomic(os.DevNull, writeString("data")); err != nil {
		t.Fatal(err)
	}
	if !IsStream(os.DevNull) {
		t.Fatalf("IsStream(%s) = false", os.DevNull)
	}
	if IsStream(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("IsStream(missing file) = true")
	}
}

func TestWriteFileAtomic_ReplacingAFileKeepsItsModeButNeverLooserThanOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	for before, want := range map[os.FileMode]os.FileMode{0o644: 0o600, 0o666: 0o600, 0o600: 0o600, 0o400: 0o400, 0o440: 0o400} {
		path := filepath.Join(t.TempDir(), "out.csv")
		if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, before); err != nil {
			t.Fatal(err)
		}
		if err := WriteFileAtomic(path, writeString("data")); err != nil {
			t.Fatal(err)
		}
		fi, _ := os.Stat(path)
		if got := fi.Mode().Perm(); got != want {
			t.Errorf("replacing a %v file: mode = %v, want %v", before, got, want)
		}
	}
}

func TestPrivatePerm(t *testing.T) {
	for in, want := range map[os.FileMode]os.FileMode{0o777: 0o600, 0o644: 0o600, 0o600: 0o600, 0o400: 0o400, 0o200: 0o600, 0: 0o400} {
		if got := PrivatePerm(in); got != want {
			t.Errorf("PrivatePerm(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestReplace_CheckVetsTheFileBeingReplaced(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("not yours")
	err := Replace(path, ReplaceOptions{TempSuffix: ".tmp-", Check: func(fs.FileInfo) error { return refused }}, writeString("new"))
	if !errors.Is(err, refused) {
		t.Fatalf("err = %v, want the check's error", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "old" {
		t.Fatalf("file = %q, want it untouched", b)
	}
	if got := entries(t, dir); len(got) != 1 {
		t.Fatalf("dir = %v, want only the original file", got)
	}
}

func TestReplace_NilWriteOnlyProbesTheDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := Replace(path, ReplaceOptions{TempSuffix: ".tmp-"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := entries(t, dir); len(got) != 0 {
		t.Fatalf("dir = %v, want nothing created", got)
	}
	if err := Replace(filepath.Join(dir, "missing", "config.yaml"), ReplaceOptions{}, nil); err == nil {
		t.Fatal("probe of a missing directory passed")
	}
}

func TestReplace_TempFileNamedAfterTheTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	var seen []string
	err := Replace(path, ReplaceOptions{TempSuffix: ".tmp-"}, func(w io.Writer) error {
		seen = entries(t, dir)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || !strings.HasPrefix(seen[0], ".config.yaml.tmp-") {
		t.Fatalf("temp file = %v, want .config.yaml.tmp-*", seen)
	}
}
