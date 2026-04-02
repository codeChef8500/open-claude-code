package skill

import (
	"os"
	"path/filepath"
	"testing"
)

const testSkillMD = `---
name: test-skill
description: A test skill for unit testing
version: "1.0"
tags:
  - test
  - ci
allowed_tools:
  - Read
  - Bash
---

# Test Skill

This is the test skill body.
`

func TestLoadSkillFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte(testSkillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSkillFile(path)
	if err != nil {
		t.Fatalf("LoadSkillFile: %v", err)
	}
	if s.Meta.Name != "test-skill" {
		t.Errorf("name: expected 'test-skill', got %q", s.Meta.Name)
	}
	if s.Meta.Description != "A test skill for unit testing" {
		t.Errorf("description mismatch: %q", s.Meta.Description)
	}
	if s.Meta.Version != "1.0" {
		t.Errorf("version: expected '1.0', got %q", s.Meta.Version)
	}
	if len(s.Meta.Tags) != 2 {
		t.Errorf("tags: expected 2, got %d", len(s.Meta.Tags))
	}
	if len(s.Meta.AllowedTools) != 2 {
		t.Errorf("allowed_tools: expected 2, got %d", len(s.Meta.AllowedTools))
	}
	if s.Prompt == "" {
		t.Error("expected non-empty rendered prompt")
	}
	if s.FilePath != path {
		t.Errorf("FilePath: expected %q, got %q", path, s.FilePath)
	}
}

func TestLoadSkillFile_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.md")
	if err := os.WriteFile(path, []byte("# Just a heading\n\nSome content."), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSkillFile(path)
	if err != nil {
		t.Fatalf("LoadSkillFile: %v", err)
	}
	// Name should be derived from filename.
	if s.Meta.Name != "plain" {
		t.Errorf("expected name 'plain', got %q", s.Meta.Name)
	}
}

func TestParseSkillBytes(t *testing.T) {
	s, err := ParseSkillBytes([]byte(testSkillMD), "from-bytes.md")
	if err != nil {
		t.Fatalf("ParseSkillBytes: %v", err)
	}
	if s.Meta.Name != "test-skill" {
		t.Errorf("expected 'test-skill', got %q", s.Meta.Name)
	}
}

func TestLoadSkillDir(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "ignore.txt"} {
		content := "---\nname: " + name + "\n---\n# " + name
		_ = os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}

	skills, err := LoadSkillDir(dir)
	if err != nil {
		t.Fatalf("LoadSkillDir: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills (only .md), got %d", len(skills))
	}
}

func TestLoadSkillDir_Empty(t *testing.T) {
	dir := t.TempDir()
	skills, err := LoadSkillDir(dir)
	if err != nil {
		t.Fatalf("LoadSkillDir: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	skills := []*Skill{
		{Meta: SkillMeta{Name: "a"}},
		{Meta: SkillMeta{Name: "b"}},
	}
	results := Search(skills, "")
	if len(results) != 2 {
		t.Errorf("expected 2 results for empty query, got %d", len(results))
	}
}

func TestSearch_NameMatch(t *testing.T) {
	skills := []*Skill{
		{Meta: SkillMeta{Name: "deploy", Description: "Deploy the app"}},
		{Meta: SkillMeta{Name: "test", Description: "Run tests"}},
	}
	results := Search(skills, "deploy")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Skill.Meta.Name != "deploy" {
		t.Errorf("expected 'deploy', got %q", results[0].Skill.Meta.Name)
	}
}

func TestSearch_TagMatch(t *testing.T) {
	skills := []*Skill{
		{Meta: SkillMeta{Name: "alpha", Tags: []string{"ci", "deploy"}}},
		{Meta: SkillMeta{Name: "beta", Tags: []string{"lint"}}},
	}
	results := Search(skills, "ci")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Skill.Meta.Name != "alpha" {
		t.Errorf("expected 'alpha', got %q", results[0].Skill.Meta.Name)
	}
}

func TestTopSkills(t *testing.T) {
	skills := []*Skill{
		{Meta: SkillMeta{Name: "a", Description: "alpha"}},
		{Meta: SkillMeta{Name: "b", Description: "beta"}},
		{Meta: SkillMeta{Name: "c", Description: "gamma"}},
	}
	top := TopSkills(skills, "alpha", 2)
	if len(top) != 1 {
		t.Errorf("expected 1 top skill, got %d", len(top))
	}
}

func TestManagedRegistry(t *testing.T) {
	reg := NewManagedRegistry()

	s1 := &Skill{Meta: SkillMeta{Name: "one", Description: "first"}, Prompt: "p1", RawMD: "r1"}
	s2 := &Skill{Meta: SkillMeta{Name: "two", Description: "second"}, Prompt: "p2", RawMD: "r2"}

	if err := reg.Register(s1); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := reg.Register(s2); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Duplicate.
	if err := reg.Register(s1); err == nil {
		t.Error("expected error on duplicate register")
	}

	if reg.Count() != 2 {
		t.Errorf("expected 2, got %d", reg.Count())
	}

	got, ok := reg.Get("one")
	if !ok || got.Meta.Name != "one" {
		t.Error("expected to find 'one'")
	}

	reg.Unregister("one")
	if reg.Count() != 1 {
		t.Errorf("expected 1 after unregister, got %d", reg.Count())
	}

	names := reg.Names()
	if len(names) != 1 || names[0] != "two" {
		t.Errorf("expected ['two'], got %v", names)
	}
}

func TestManagedRegistry_Upsert(t *testing.T) {
	reg := NewManagedRegistry()
	s := &Skill{Meta: SkillMeta{Name: "x", Description: "v1"}, Prompt: "p", RawMD: "r"}
	reg.Upsert(s)

	s2 := &Skill{Meta: SkillMeta{Name: "x", Description: "v2"}, Prompt: "p2", RawMD: "r2"}
	reg.Upsert(s2)

	got, _ := reg.Get("x")
	if got.Meta.Description != "v2" {
		t.Errorf("expected v2 after upsert, got %q", got.Meta.Description)
	}
	if reg.Count() != 1 {
		t.Errorf("expected 1, got %d", reg.Count())
	}
}

func TestManagedRegistryFrom(t *testing.T) {
	skills := []*Skill{
		{Meta: SkillMeta{Name: "a"}, Prompt: "p", RawMD: "r"},
		{Meta: SkillMeta{Name: "b"}, Prompt: "p", RawMD: "r"},
	}
	reg := NewManagedRegistryFrom(skills)
	if reg.Count() != 2 {
		t.Errorf("expected 2, got %d", reg.Count())
	}
}

func TestValidate(t *testing.T) {
	// Valid skill.
	s := &Skill{Meta: SkillMeta{Name: "good", Description: "desc"}, Prompt: "content", RawMD: "md"}
	issues := Validate(s)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}

	// Missing name.
	bad := &Skill{Meta: SkillMeta{}, Prompt: "content", RawMD: "md"}
	issues = Validate(bad)
	if len(issues) == 0 {
		t.Error("expected issues for missing name")
	}

	// No content.
	noContent := &Skill{Meta: SkillMeta{Name: "empty", Description: "d"}}
	issues = Validate(noContent)
	found := false
	for _, i := range issues {
		if i == "skill has no content" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'skill has no content' issue")
	}
}
