package fsutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
