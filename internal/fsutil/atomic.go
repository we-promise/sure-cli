// Package fsutil holds file-system helpers shared by commands that write
// files the user asked for.
package fsutil

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic creates or replaces the file at path with what write
// writes. The bytes land in a temp file in path's directory, which is renamed
// over path once write, sync and close succeed. So a failure leaves any
// previous file untouched and no temp file behind, and a symlink at path is
// replaced rather than followed. The file is readable only by the user: a new
// one is 0600, and one replaced keeps its mode as tightened by PrivatePerm.
//
// A path that names a stream (see IsStream), such as /dev/stdout, is written
// to directly: there is no directory to put a temp file in, nor a file to
// keep from being half written.
func WriteFileAtomic(path string, write func(io.Writer) error) (err error) {
	if IsStream(path) {
		return writeStream(path, write)
	}
	perm := os.FileMode(0o600)
	if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() {
		perm = PrivatePerm(fi.Mode())
	}
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
	if err = tmp.Chmod(perm); err != nil {
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

// PrivatePerm is the mode a file that held mode gets when it is rewritten
// with private data: the owner's bits, readable by the owner, and nothing
// for group or others. 0644 becomes 0600; 0400 stays 0400.
func PrivatePerm(mode fs.FileMode) fs.FileMode {
	return mode.Perm()&0o600 | 0o400
}

// IsStream reports whether path, followed through symlinks, exists and is a
// pipe, a character device (a terminal, /dev/null) or a socket: something
// written to, not replaced.
func IsStream(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&(fs.ModeNamedPipe|fs.ModeCharDevice|fs.ModeSocket) != 0
}

func writeStream(path string, write func(io.Writer) error) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
