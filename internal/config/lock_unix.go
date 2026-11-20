//go:build unix

package config

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func lockFile(f *os.File) error {
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			return err
		}
	}
}

// checkOwner refuses a config file another user owns, e.g. the user's own
// file when sure-cli runs under sudo: replacing it would hand it to root.
func checkOwner(fi fs.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if uid := os.Geteuid(); int(st.Uid) != uid {
		return fmt.Errorf("%s belongs to uid %d, not the current user (uid %d); run sure-cli as its owner", fi.Name(), st.Uid, uid)
	}
	return nil
}

func replaceFile(from, to string) error { return os.Rename(from, to) }
