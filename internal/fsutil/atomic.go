// Package fsutil holds file-system helpers shared by commands that write
// files the user asked for.
package fsutil

import (
	"io"
	"os"
	"path/filepath"
)

// WriteFileAtomic creates or replaces the file at path with what write
// writes. The bytes land in a temp file in path's directory, readable only by
// the user (0600), which is renamed over path once write, sync and close
// succeed. So a failure leaves any previous file untouched and no temp file
// behind, and a symlink at path is replaced rather than followed.
func WriteFileAtomic(path string, write func(io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".part-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if err = write(tmp); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
