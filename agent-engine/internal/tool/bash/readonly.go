package bash

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// readOnlyCommands is the set of command names that are unconditionally
// safe to execute in any permission mode — they only read state, never mutate it.
var readOnlyCommands = map[string]bool{
	// File inspection
	"cat": true, "less": true, "more": true, "head": true, "tail": true,
	"file": true, "stat": true, "ls": true, "ll": true, "dir": true,
	"find": true, "locate": true, "which": true, "whereis": true, "type": true,
	// Content search
	"grep": true, "rg": true, "ag": true, "ack": true, "fgrep": true, "egrep": true,
	// Text processing (read-only)
	"wc": true, "sort": true, "uniq": true, "cut": true, "awk": true, "sed": true,
	"tr": true, "column": true, "diff": true, "comm": true, "join": true,
	// Archive inspection (not extraction)
	"zipinfo": true, "unzip": false, // unzip extracts — not read-only
	// System info
	"echo": true, "printf": true, "date": true, "pwd": true, "whoami": true,
	"id": true, "uname": true, "hostname": true, "uptime": true, "env": true,
	"printenv": true, "set": false, // set modifies shell vars
	// Process / network info
	"ps": true, "top": false, // top is interactive
	"pgrep": true, "netstat": true, "ss": true, "lsof": true, "ifconfig": true,
	"ip": true, "ping": true, "traceroute": true, "nslookup": true, "dig": true,
	"host": true, "curl": true, "wget": false, // wget downloads files
	// Git read ops
	"git": false, // git is handled separately below
	// Misc read
	"man": true, "help": true, "true": true, "false": true,
	"test": true, "[": true, "[[": true,
	"jq": true, "yq": true, "xmllint": true,
	"md5sum": true, "sha256sum": true, "sha1sum": true, "xxd": true, "od": true,
}

// readOnlyGitSubcmds are the git subcommands that only read repository state.
var readOnlyGitSubcmds = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "branch": true,
	"tag": true, "remote": true, "fetch": false, // fetch changes remote refs
	"ls-files": true, "ls-remote": true, "rev-parse": true, "describe": true,
	"blame": true, "shortlog": true, "stash": false, // stash modifies
	"config": true, // reading config is ok; writing requires --global etc.
}

// IsReadOnlyCommand reports whether the given shell command string appears to
// be safe to run in read-only mode — i.e. it only reads state and makes no
// persistent changes.  It parses the command with mvdan.cc/sh when possible and
// falls back to a simple word-split heuristic.
//
// Returns (true, "") when the command is whitelisted, or (false, reason) when
// it is not.  A non-whitelisted command is not necessarily dangerous; callers
// should use this as an optimistic fast-path, not as a security gate.
func IsReadOnlyCommand(command string) (ok bool, reason string) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	f, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		// Parse failed — use fallback heuristic.
		return isReadOnlyFallback(command)
	}

	allReadOnly := true
	var firstReason string

	syntax.Walk(f, func(node syntax.Node) bool {
		if !allReadOnly {
			return false
		}
		switch n := node.(type) {
		case *syntax.Redirect:
			// Any output redirection is considered mutating.
			switch n.Op {
			case syntax.RdrOut, syntax.AppOut, syntax.RdrInOut,
				syntax.RdrAll, syntax.AppAll:
				allReadOnly = false
				firstReason = "contains output redirection"
			}

		case *syntax.CallExpr:
			if len(n.Args) == 0 {
				return true
			}
			name := wordLiteral(n.Args[0])
			if name == "" {
				// Complex expansion — can't determine, treat as unsafe.
				allReadOnly = false
				firstReason = "command name is a dynamic expansion"
				return false
			}
			if name == "git" && len(n.Args) > 1 {
				sub := wordLiteral(n.Args[1])
				if !readOnlyGitSubcmds[sub] {
					allReadOnly = false
					firstReason = "git subcommand " + sub + " is not read-only"
					return false
				}
				return true
			}
			ro, listed := readOnlyCommands[name]
			if !listed || !ro {
				allReadOnly = false
				firstReason = "command " + name + " is not in read-only whitelist"
				return false
			}
		}
		return true
	})

	if allReadOnly {
		return true, ""
	}
	return false, firstReason
}

// isReadOnlyFallback is a best-effort heuristic when AST parse fails.
func isReadOnlyFallback(command string) (bool, string) {
	// Reject any output redirections.
	for _, op := range []string{">", ">>", "&>"} {
		if strings.Contains(command, op) {
			return false, "contains output redirection operator " + op
		}
	}
	// Check the first word against the whitelist.
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return true, ""
	}
	name := fields[0]
	ro, listed := readOnlyCommands[name]
	if listed && ro {
		return true, ""
	}
	return false, "command " + name + " not in read-only whitelist"
}
