package bash

import (
	"testing"
)

func TestIsReadOnlyCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		// Read-only commands
		{"cat /etc/hosts", true},
		{"ls -la /tmp", true},
		{"grep -r pattern .", true},
		{"rg pattern", true},
		{"wc -l file.txt", true},
		{"head -n 10 file.txt", true},
		{"tail -f log.txt", true},
		{"echo hello", true},
		{"pwd", true},
		{"whoami", true},
		{"ps aux", true},
		{"jq .name package.json", true},

		// Newly added read-only commands
		{"base64 file.bin", true},
		{"tree /tmp", true},
		{"readlink -f symlink", true},
		{"realpath ./file", true},
		{"basename /tmp/file.go", true},
		{"dirname /tmp/file.go", true},
		{"du -sh /tmp", true},
		{"df -h", true},
		{"free -m", true},
		{"nl file.txt", true},
		{"tac file.txt", true},
		{"rev file.txt", true},
		{"hexdump -C file.bin", true},
		{"info coreutils", true},

		// Git read-only
		{"git status", true},
		{"git log --oneline -5", true},
		{"git diff HEAD", true},
		{"git branch -a", true},
		{"git blame file.go", true},
		{"git rev-list HEAD..main", true},
		{"git ls-tree HEAD", true},
		{"git cat-file -p HEAD", true},
		{"git grep pattern", true},
		{"git reflog", true},
		{"git format-patch HEAD~3", true},
		{"git diff-tree --no-commit-id -r HEAD", true},
		{"git describe --tags", true},
		{"git rev-parse HEAD", true},
		{"git verify-commit HEAD", true},
		{"git count-objects -v", true},
		{"git help log", true},
		{"git version", true},

		// Git write operations
		{"git push origin main", false},
		{"git commit -m 'test'", false},
		{"git merge feature", false},
		{"git checkout -b new-branch", false},
		{"git rebase main", false},
		{"git stash", false},
		{"git notes add -m 'note'", false},
		{"git submodule update", false},
		{"git worktree add /tmp/wt main", false},

		// Non-read-only commands
		{"rm -rf /tmp/test", false},
		{"mv file1 file2", false},
		{"cp file1 file2", false},
		{"chmod 755 file", false},
		{"mkdir -p /tmp/test", false},
		{"npm install", false},

		// Output redirections
		{"echo hello > file.txt", false},
		{"cat file >> output.txt", false},

		// Dynamic expansions
		{"$(command)", false},

		// Empty
		{"", true},
	}

	for _, tt := range tests {
		ok, reason := IsReadOnlyCommand(tt.cmd)
		if ok != tt.want {
			t.Errorf("IsReadOnlyCommand(%q) = (%v, %q), want %v", tt.cmd, ok, reason, tt.want)
		}
	}
}

func TestCheckShellAST(t *testing.T) {
	tests := []struct {
		cmd     string
		wantErr bool
		desc    string
	}{
		// Safe commands
		{"ls -la", false, "simple ls"},
		{"echo hello", false, "simple echo"},
		{"git status", false, "git status"},

		// Dangerous commands
		{"mkfs /dev/sda", true, "mkfs"},
		{"shutdown -h now", true, "shutdown"},
		{"reboot", true, "reboot"},
		{"halt", true, "halt"},
		{"poweroff", true, "poweroff"},
		{"fdisk /dev/sda", true, "fdisk"},
		{"iptables -F", true, "iptables"},

		// Fork bomb
		{":(){ :|:& };:", true, "fork bomb"},

		// Destructive rm
		{"rm -rf /", true, "rm root"},
		{"rm -rf /*", true, "rm root glob"},

		// dd to device
		{"dd if=/dev/zero of=/dev/sda", true, "dd to device"},

		// Redirect to sensitive path
		{"echo x > /dev/sda", true, "redirect to block device"},
		{"echo x > /etc/passwd", true, "redirect to passwd"},
		{"echo x > /proc/self", true, "redirect to proc"},
		{"echo x > /boot/grub", true, "redirect to boot"},
	}

	for _, tt := range tests {
		err := checkShellAST(tt.cmd)
		if (err != nil) != tt.wantErr {
			t.Errorf("checkShellAST(%q) [%s] error=%v, wantErr=%v", tt.cmd, tt.desc, err, tt.wantErr)
		}
	}
}

func TestWordLiteral(t *testing.T) {
	// wordLiteral is tested indirectly through the above tests,
	// but we can test nil handling.
	if got := wordLiteral(nil); got != "" {
		t.Errorf("wordLiteral(nil) = %q, want empty", got)
	}
}
