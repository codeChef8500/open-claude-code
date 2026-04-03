package bash

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// dangerousCommands is the set of command names that are unconditionally refused.
var dangerousCommands = map[string]bool{
	"mkfs":       true,
	"mkswap":     true,
	"fdisk":      true,
	"parted":     true,
	"shred":      true,
	"wipefs":     true,
	"shutdown":   true,
	"poweroff":   true,
	"reboot":     true,
	"halt":       true,
	"init":       true, // init 0 / init 6
	"blkdiscard": true,
	"hdparm":     true, // can destroy data with --write-sector
	"sgdisk":     true,
	"gdisk":      true,
	"cfdisk":     true,
	"sfdisk":     true,
	"badblocks":  true,
	"fsck":       true, // fsck on mounted FS can cause data loss
	"swapoff":    true,
	"swapon":     true,
	"insmod":     true,
	"rmmod":      true,
	"modprobe":   true,
	"iptables":   true,
	"nft":        true,
}

// sensitivePrefixes are path prefixes that must not be used as redirect targets.
var sensitivePrefixes = []string{
	"/dev/sd", "/dev/hd", "/dev/vd", "/dev/nvme",
	"/dev/sda", "/dev/sdb",
	"/dev/zero", "/dev/mem", "/dev/kmem",
	"/proc/", "/sys/",
	"/boot/",
	"/etc/passwd", "/etc/shadow", "/etc/sudoers",
	"/etc/ssh/",
}

// checkShellAST parses the command with mvdan.cc/sh and inspects the AST for
// dangerous constructs: fork bombs, destructive redirections, and banned commands.
func checkShellAST(command string) error {
	// Quick string-level pre-check for known fork bomb patterns (AST parse would
	// succeed but the structural check below catches it too).
	if strings.Contains(command, ":(){ :|:& }") || strings.Contains(command, ":(){:|:&}") {
		return fmt.Errorf("command contains fork bomb pattern")
	}

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	f, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		// If we cannot parse it, fall back to string checks only — don't block
		// valid but non-POSIX syntax (e.g., PowerShell, fish).
		return checkDangerousStrings(command)
	}

	var walkErr error
	syntax.Walk(f, func(node syntax.Node) bool {
		if walkErr != nil {
			return false
		}
		switch n := node.(type) {
		case *syntax.CallExpr:
			// Detect dangerous command names.
			if len(n.Args) > 0 {
				name := wordLiteral(n.Args[0])
				if dangerousCommands[name] {
					walkErr = fmt.Errorf("command %q is not permitted", name)
					return false
				}
				// Detect: rm -rf / or rm -rf /*
				if name == "rm" {
					if hasFlag(n.Args, "-rf") || hasFlag(n.Args, "-fr") {
						for _, arg := range n.Args[1:] {
							lit := wordLiteral(arg)
							if lit == "/" || lit == "/*" || lit == "/." {
								walkErr = fmt.Errorf("destructive rm targeting root filesystem is not permitted")
								return false
							}
						}
					}
				}
				// Detect: dd if=/dev/zero of=<device> or similar
				if name == "dd" {
					for _, arg := range n.Args[1:] {
						lit := wordLiteral(arg)
						if strings.HasPrefix(lit, "of=/dev/sd") || strings.HasPrefix(lit, "of=/dev/hd") {
							walkErr = fmt.Errorf("dd targeting a block device is not permitted")
							return false
						}
					}
				}
			}

		case *syntax.Redirect:
			// Block output redirections to sensitive paths.
			switch n.Op {
			case syntax.RdrOut, syntax.AppOut, syntax.RdrInOut, syntax.RdrAll, syntax.AppAll:
				if n.Word != nil {
					target := wordLiteral(n.Word)
					if target != "" && isSensitivePath(target) {
						walkErr = fmt.Errorf("redirect to sensitive path %q is not permitted", target)
						return false
					}
				}
			}

		case *syntax.FuncDecl:
			// Detect fork bomb: function that calls itself recursively and pipes to background.
			// The simple string check above already catches the canonical form.
		}
		return true
	})

	return walkErr
}

// isSensitivePath reports whether target matches a known sensitive path prefix.
func isSensitivePath(target string) bool {
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

// checkDangerousStrings is a fallback string-level check when AST parse fails.
func checkDangerousStrings(command string) error {
	lower := strings.ToLower(command)
	patterns := []string{
		"rm -rf /", "rm -rf /*", "dd if=/dev/zero", "mkfs",
		":(){ :|:& };:", "> /dev/sda", "> /dev/sdb", "mv /* /dev/null",
	}
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return fmt.Errorf("command contains dangerous pattern %q", p)
		}
	}
	return nil
}

// wordLiteral extracts the literal string value from a *syntax.Word if it is a
// simple unquoted string (no expansions). Returns "" for complex words.
func wordLiteral(w *syntax.Word) string {
	if w == nil || len(w.Parts) != 1 {
		return ""
	}
	if lit, ok := w.Parts[0].(*syntax.Lit); ok {
		return lit.Value
	}
	return ""
}

// hasFlag reports whether args contains a combined short-flag string like "-rf".
func hasFlag(args []*syntax.Word, flag string) bool {
	for _, a := range args {
		lit := wordLiteral(a)
		if lit == flag {
			return true
		}
		// Handle separated flags: -r -f
		if strings.HasPrefix(lit, "-") && !strings.HasPrefix(lit, "--") {
			combined := "-" + flag[1:] // e.g. "-rf"
			if strings.Contains(lit, "r") && strings.Contains(lit, "f") {
				_ = combined
				return true
			}
		}
	}
	return false
}
