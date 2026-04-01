package agent

import "sync/atomic"

// ANSI colour codes assigned to agents for log differentiation.
var agentColors = []string{
	"\033[36m",  // cyan
	"\033[33m",  // yellow
	"\033[35m",  // magenta
	"\033[32m",  // green
	"\033[34m",  // blue
	"\033[91m",  // bright red
	"\033[96m",  // bright cyan
	"\033[93m",  // bright yellow
	"\033[95m",  // bright magenta
	"\033[92m",  // bright green
}

const colorReset = "\033[0m"

var colorCounter atomic.Uint64

// AssignColor returns the next ANSI colour code in a round-robin fashion.
func AssignColor() string {
	idx := colorCounter.Add(1) - 1
	return agentColors[idx%uint64(len(agentColors))]
}

// Colorize wraps text with the given ANSI colour and a reset suffix.
func Colorize(color, text string) string {
	if color == "" {
		return text
	}
	return color + text + colorReset
}
