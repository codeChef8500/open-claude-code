//go:build !windows

package agent

import (
	"os"
	"syscall"
)

// openNoFollow opens a file with O_NOFOLLOW to prevent symlink attacks.
func openNoFollow(path string, flags int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flags|syscall.O_NOFOLLOW, perm)
}
