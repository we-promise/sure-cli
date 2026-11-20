//go:build windows

package config

import (
	"errors"
	"io/fs"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func lockFile(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, new(windows.Overlapped))
}

// checkOwner is a no-op: Windows controls access to the file with ACLs, not
// a uid owner to compare.
func checkOwner(fs.FileInfo) error { return nil }

// replaceFile retries a rename that fails while another process (a reader,
// an antivirus scan) briefly holds the config open.
func replaceFile(from, to string) error {
	var err error
	for wait := 10 * time.Millisecond; wait <= 640*time.Millisecond; wait *= 2 {
		err = os.Rename(from, to)
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		time.Sleep(wait)
	}
	return err
}
