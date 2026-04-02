package diff

import (
	"strings"
	"testing"
)

func TestCompute_NoChanges(t *testing.T) {
	content := "line1\nline2\nline3\n"
	r := Compute(content, content, "test.txt")
	if r.HasChanges() {
		t.Error("expected no changes")
	}
	if r.Added != 0 || r.Removed != 0 {
		t.Errorf("expected 0 added/removed, got +%d -%d", r.Added, r.Removed)
	}
}

func TestCompute_SimpleEdit(t *testing.T) {
	old := "line1\nline2\nline3\n"
	new := "line1\nmodified\nline3\n"
	r := Compute(old, new, "test.txt")
	if !r.HasChanges() {
		t.Error("expected changes")
	}
	if r.Added != 1 || r.Removed != 1 {
		t.Errorf("expected +1 -1, got +%d -%d", r.Added, r.Removed)
	}
}

func TestCompute_Additions(t *testing.T) {
	old := "a\nb\n"
	new := "a\nb\nc\nd\n"
	r := Compute(old, new, "f.txt")
	if r.Added != 2 {
		t.Errorf("expected 2 added, got %d", r.Added)
	}
	if r.Removed != 0 {
		t.Errorf("expected 0 removed, got %d", r.Removed)
	}
}

func TestCompute_Deletions(t *testing.T) {
	old := "a\nb\nc\nd\n"
	new := "a\nd\n"
	r := Compute(old, new, "f.txt")
	if r.Removed != 2 {
		t.Errorf("expected 2 removed, got %d", r.Removed)
	}
}

func TestResult_Format(t *testing.T) {
	old := "line1\nline2\nline3\n"
	new := "line1\nchanged\nline3\n"
	r := Compute(old, new, "test.txt")
	out := r.Format()
	if !strings.Contains(out, "--- test.txt") {
		t.Error("expected --- header")
	}
	if !strings.Contains(out, "+++ test.txt") {
		t.Error("expected +++ header")
	}
	if !strings.Contains(out, "-line2") {
		t.Error("expected -line2")
	}
	if !strings.Contains(out, "+changed") {
		t.Error("expected +changed")
	}
}

func TestResult_Summary(t *testing.T) {
	r := Compute("a\n", "a\nb\n", "x.go")
	s := r.Summary()
	if !strings.Contains(s, "+1") {
		t.Errorf("expected +1 in summary, got %q", s)
	}
}
