//go:build !windows

package fsutil

import "os"

func rename(from, to string) error { return os.Rename(from, to) }
