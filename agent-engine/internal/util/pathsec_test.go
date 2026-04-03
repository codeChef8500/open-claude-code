package util

import (
	"path/filepath"
	"testing"
)

func TestIsUNCPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`\\server\share`, true},
		{`\\?\C:\path`, true},
		{`//server/share`, true},
		{`C:\Users\test`, false},
		{`/home/user`, false},
		{`./relative`, false},
		{``, false},
	}
	for _, tt := range tests {
		if got := IsUNCPath(tt.path); got != tt.want {
			t.Errorf("IsUNCPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsBlockedDevicePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/dev/zero", true},
		{"/dev/random", true},
		{"/dev/urandom", true},
		{"/dev/stdin", true},
		{"/dev/tty", true},
		{"/dev/console", true},
		{"/dev/stdout", true},
		{"/dev/stderr", true},
		{"/dev/fd/0", true},
		{"/dev/fd/1", true},
		{"/dev/fd/2", true},
		{"/proc/self/fd/0", true},
		{"/proc/self/fd/1", true},
		{"/proc/1234/fd/2", true},
		{"/dev/sda", false},
		{"/dev/null", false},
		{"/home/user/file.txt", false},
		{"/proc/self/status", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsBlockedDevicePath(tt.path); got != tt.want {
			t.Errorf("IsBlockedDevicePath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"path\x00with\x00nulls", "pathwithnulls"},
		{"/normal/path", filepath.Clean("/normal/path")},
		{"", "."},
	}
	for _, tt := range tests {
		got := SanitizePath(tt.input)
		if got != tt.want {
			t.Errorf("SanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMaxFileSizeConstants(t *testing.T) {
	if MaxEditFileSize != 1<<30 {
		t.Errorf("MaxEditFileSize = %d, want %d", MaxEditFileSize, 1<<30)
	}
	if MaxReadFileSize != 100*1024*1024 {
		t.Errorf("MaxReadFileSize = %d, want %d", MaxReadFileSize, 100*1024*1024)
	}
}
