//go:build unix

package fsutil

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWriteFileAtomic_WritesToAFIFODirectly(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	got := make(chan string)
	go func() {
		f, err := os.Open(fifo)
		if err != nil {
			got <- err.Error()
			return
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		got <- string(b)
	}()
	if err := WriteFileAtomic(fifo, writeString("data")); err != nil {
		t.Fatal(err)
	}
	if s := <-got; s != "data" {
		t.Fatalf("reader got %q", s)
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Fatalf("dir holds %v, want only the pipe", names)
	}
}
