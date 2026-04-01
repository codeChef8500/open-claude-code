package bash

import (
	"fmt"
	"strings"
)

// dangerousPatterns lists shell command patterns that are unconditionally
// refused regardless of mode.
var dangerousPatterns = []string{
	"rm -rf /",
	"rm -rf /*",
	"dd if=/dev/zero",
	"dd if=/dev/random",
	"mkfs",
	":(){ :|:& };:", // fork bomb
	"> /dev/sda",
	"mv /* /dev/null",
}

// checkShellAST inspects command for dangerous patterns using string-level analysis.
// A full AST-level check (mvdan.cc/sh/v3) can be added once the package is available.
func checkShellAST(command string) error {
	lower := strings.ToLower(command)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return fmt.Errorf("command contains dangerous pattern %q", pattern)
		}
	}
	return nil
}
