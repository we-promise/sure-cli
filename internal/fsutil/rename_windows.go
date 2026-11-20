//go:build windows

package fsutil

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// rename retries a rename that fails while another process (a reader, an
// antivirus scan) briefly holds the target open.
func rename(from, to string) error {
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
