// Package fsutil holds file-system helpers shared by commands that write
// files the user asked for.
package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic creates or replaces the file at path with what write
// writes, through Replace. A path that names a stream (see IsStream), such as
// /dev/stdout, is written to directly: there is no directory to put a temp
// file in, nor a file to keep from being half written.
func WriteFileAtomic(path string, write func(io.Writer) error) error {
	if IsStream(path) {
		return writeStream(path, write)
	}
	return Replace(path, ReplaceOptions{TempSuffix: ".part-"}, write)
}

// ReplaceOptions tunes Replace for a caller.
type ReplaceOptions struct {
	// TempSuffix names the temp file "." + base(path) + TempSuffix + random,
	// so a caller can find the ones a crash left behind.
	TempSuffix string
	// Check, when set, vets the regular file about to be replaced (e.g. its
	// owner) before anything is written.
	Check func(fs.FileInfo) error
}

// Replace creates or replaces the file at path with what write writes. The
// bytes land in a temp file in path's directory, which is renamed over path
// once write, sync and close succeed. So a failure leaves any previous file
// untouched and no temp file behind, and a symlink at path is replaced rather
// than followed. The file is readable only by the user: a new one is 0600,
// and one replaced keeps its mode as tightened by PrivatePerm. A nil write
// only proves the directory accepts the temp file.
func Replace(path string, opts ReplaceOptions, write func(io.Writer) error) (err error) {
	perm := os.FileMode(0o600)
	if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() {
		if opts.Check != nil {
			if err := opts.Check(fi); err != nil {
				return err
			}
		}
		perm = PrivatePerm(fi.Mode())
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+opts.TempSuffix+"*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil || write == nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if write == nil {
		return nil
	}
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
	return rename(tmp.Name(), path)
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

// IsStdout reports whether path is this process's own stdout (/dev/stdout,
// /dev/fd/1, or the file stdout is redirected to).
func IsStdout(path string) bool {
	out, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	var fi os.FileInfo
	if IsStream(path) {
		// A pipe or device path can stat differently from the descriptor
		// it names (macOS): compare what opening it gives.
		fi, err = streamInfo(path)
	} else {
		fi, err = os.Stat(path)
	}
	return err == nil && os.SameFile(fi, out)
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
