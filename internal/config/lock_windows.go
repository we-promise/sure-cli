//go:build windows

package config

import (
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, new(windows.Overlapped))
}

// checkOwner is a no-op: Windows controls access to the file with ACLs, not
// a uid owner to compare.
func checkOwner(fs.FileInfo) error { return nil }
