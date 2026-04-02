package diff

import (
	"fmt"
	"strings"
)

// Op represents a diff operation type.
type Op int

const (
	OpEqual  Op = iota
	OpInsert
	OpDelete
)

// Line represents a single line in a diff.
type Line struct {
	Op      Op
	Content string
	OldNum  int // 0 if insert
	NewNum  int // 0 if delete
}

// Hunk is a contiguous group of diff lines with context.
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []Line
}

// Result holds the complete diff output.
type Result struct {
	OldPath string
	NewPath string
	Hunks   []Hunk
	Added   int
	Removed int
}

// Compute produces a unified diff between old and new content.
func Compute(oldContent, newContent, path string) *Result {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	// Simple LCS-based diff.
	ops := computeOps(oldLines, newLines)

	result := &Result{
		OldPath: path,
		NewPath: path,
	}

	// Count changes.
	for _, op := range ops {
		switch op.Op {
		case OpInsert:
			result.Added++
		case OpDelete:
			result.Removed++
		}
	}

	// Group into hunks with 3 lines of context.
	result.Hunks = groupHunks(ops, 3)
	return result
}

// Format renders the diff in unified diff format.
func (r *Result) Format() string {
	if len(r.Hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s\n", r.OldPath))
	sb.WriteString(fmt.Sprintf("+++ %s\n", r.NewPath))

	for _, h := range r.Hunks {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			h.OldStart, h.OldCount, h.NewStart, h.NewCount))
		for _, l := range h.Lines {
			switch l.Op {
			case OpEqual:
				sb.WriteString(" " + l.Content + "\n")
			case OpInsert:
				sb.WriteString("+" + l.Content + "\n")
			case OpDelete:
				sb.WriteString("-" + l.Content + "\n")
			}
		}
	}
	return sb.String()
}

// Summary returns a one-line summary like "+5 -3 path/to/file".
func (r *Result) Summary() string {
	return fmt.Sprintf("+%d -%d %s", r.Added, r.Removed, r.NewPath)
}

// HasChanges reports whether there are any differences.
func (r *Result) HasChanges() bool {
	return r.Added > 0 || r.Removed > 0
}

// ── Internal ────────────────────────────────────────────────────────────────

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Remove trailing empty line from trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// computeOps uses a simple O(NM) LCS algorithm to produce diff operations.
func computeOps(old, new []string) []Line {
	n := len(old)
	m := len(new)

	// LCS table.
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if old[i] == new[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	// Walk LCS to produce ops.
	var ops []Line
	i, j := 0, 0
	oldNum, newNum := 1, 1
	for i < n || j < m {
		if i < n && j < m && old[i] == new[j] {
			ops = append(ops, Line{Op: OpEqual, Content: old[i], OldNum: oldNum, NewNum: newNum})
			i++
			j++
			oldNum++
			newNum++
		} else if j < m && (i >= n || lcs[i][j+1] >= lcs[i+1][j]) {
			ops = append(ops, Line{Op: OpInsert, Content: new[j], NewNum: newNum})
			j++
			newNum++
		} else if i < n {
			ops = append(ops, Line{Op: OpDelete, Content: old[i], OldNum: oldNum})
			i++
			oldNum++
		}
	}
	return ops
}

// groupHunks splits ops into hunks with contextLines of surrounding context.
func groupHunks(ops []Line, contextLines int) []Hunk {
	if len(ops) == 0 {
		return nil
	}

	// Find change ranges.
	type changeRange struct{ start, end int }
	var changes []changeRange
	for i, op := range ops {
		if op.Op != OpEqual {
			if len(changes) == 0 || i > changes[len(changes)-1].end+contextLines*2 {
				changes = append(changes, changeRange{i, i})
			} else {
				changes[len(changes)-1].end = i
			}
		}
	}

	var hunks []Hunk
	for _, cr := range changes {
		start := cr.start - contextLines
		if start < 0 {
			start = 0
		}
		end := cr.end + contextLines + 1
		if end > len(ops) {
			end = len(ops)
		}

		h := Hunk{}
		for i := start; i < end; i++ {
			h.Lines = append(h.Lines, ops[i])
		}

		// Compute hunk header.
		if len(h.Lines) > 0 {
			for _, l := range h.Lines {
				if l.OldNum > 0 && h.OldStart == 0 {
					h.OldStart = l.OldNum
				}
				if l.NewNum > 0 && h.NewStart == 0 {
					h.NewStart = l.NewNum
				}
				if l.Op == OpEqual || l.Op == OpDelete {
					h.OldCount++
				}
				if l.Op == OpEqual || l.Op == OpInsert {
					h.NewCount++
				}
			}
			if h.OldStart == 0 {
				h.OldStart = 1
			}
			if h.NewStart == 0 {
				h.NewStart = 1
			}
		}
		hunks = append(hunks, h)
	}
	return hunks
}
