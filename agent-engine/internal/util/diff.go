package util

import (
	"fmt"
	"strings"
)

// DiffLine is a single line in a unified diff output.
type DiffLine struct {
	Op   DiffOp
	Text string
}

// DiffOp is the operation type for a diff line.
type DiffOp int

const (
	DiffOpContext DiffOp = iota // unchanged context line
	DiffOpAdd                   // inserted line ('+')
	DiffOpDelete                // deleted line ('-')
)

// UnifiedDiff produces a unified-diff-style string between oldLines and
// newLines, with contextLines surrounding each change hunk.
func UnifiedDiff(oldLines, newLines []string, contextLines int) string {
	if contextLines < 0 {
		contextLines = 3
	}

	// Build an LCS-based edit sequence via Myers diff (O(ND) simplified).
	edits := myersDiff(oldLines, newLines)

	if len(edits) == 0 {
		return ""
	}

	var sb strings.Builder
	i := 0
	for i < len(edits) {
		// Find next change.
		for i < len(edits) && edits[i].Op == DiffOpContext {
			i++
		}
		if i >= len(edits) {
			break
		}
		// Collect hunk boundaries.
		start := i - contextLines
		if start < 0 {
			start = 0
		}
		end := i
		for end < len(edits) && (edits[end].Op != DiffOpContext || end-i < contextLines) {
			end++
		}
		end += contextLines
		if end > len(edits) {
			end = len(edits)
		}

		oldStart, newStart := 1, 1
		oldCount, newCount := 0, 0
		for _, e := range edits[start:end] {
			if e.Op != DiffOpAdd {
				oldCount++
			}
			if e.Op != DiffOpDelete {
				newCount++
			}
		}

		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount))
		for _, e := range edits[start:end] {
			switch e.Op {
			case DiffOpContext:
				sb.WriteString(" " + e.Text + "\n")
			case DiffOpAdd:
				sb.WriteString("+" + e.Text + "\n")
			case DiffOpDelete:
				sb.WriteString("-" + e.Text + "\n")
			}
		}
		i = end
	}
	return sb.String()
}

// DiffStats returns counts of added and deleted lines.
func DiffStats(diff string) (added, deleted int) {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deleted++
		}
	}
	return
}

// myersDiff is a simplified O(ND) Myers diff producing a flat edit sequence.
func myersDiff(a, b []string) []DiffLine {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		out := make([]DiffLine, m)
		for i, s := range b {
			out[i] = DiffLine{Op: DiffOpAdd, Text: s}
		}
		return out
	}
	if m == 0 {
		out := make([]DiffLine, n)
		for i, s := range a {
			out[i] = DiffLine{Op: DiffOpDelete, Text: s}
		}
		return out
	}

	// LCS via DP table (simpler than full Myers for moderate sizes).
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] > dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var out []DiffLine
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, DiffLine{Op: DiffOpContext, Text: a[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, DiffLine{Op: DiffOpDelete, Text: a[i]})
			i++
		} else {
			out = append(out, DiffLine{Op: DiffOpAdd, Text: b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, DiffLine{Op: DiffOpDelete, Text: a[i]})
	}
	for ; j < m; j++ {
		out = append(out, DiffLine{Op: DiffOpAdd, Text: b[j]})
	}
	return out
}

// SplitLines splits s into lines, preserving the original line endings.
func SplitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
