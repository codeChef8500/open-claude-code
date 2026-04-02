//go:build windows

package agent

import "os"

// openNoFollow on Windows falls back to a regular open (O_NOFOLLOW is
// unavailable; symlink attack surface is significantly reduced by NTFS
// junction semantics and the file is created inside a session-scoped
// directory that the agent controls).
func openNoFollow(path string, flags int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flags, perm)
}
