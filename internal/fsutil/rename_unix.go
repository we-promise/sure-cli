//go:build !windows

package fsutil

import (
	"os"
	"syscall"
)

func rename(from, to string) error { return os.Rename(from, to) }

// streamInfo opens path without blocking (a FIFO with no reader fails
// instead of waiting) and returns what its descriptor stats as.
func streamInfo(path string) (os.FileInfo, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}
