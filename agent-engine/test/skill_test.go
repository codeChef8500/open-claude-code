package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/skill"
)

func TestSkillLoaderFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: my-skill
description: Does something useful
version: "1.0"
tags: [go, test]
---
# My Skill

This skill does something useful.
`
	path := filepath.Join(dir, "my-skill.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	s, err := skill.LoadSkillFile(path)
	require.NoError(t, err)
	assert.Equal(t, "my-skill", s.Meta.Name)
	assert.Equal(t, "Does something useful", s.Meta.Description)
	assert.Equal(t, "1.0", s.Meta.Version)
	assert.Contains(t, s.Meta.Tags, "go")
	assert.Contains(t, s.RawMD, "This skill does something useful")
}

func TestSkillLoaderNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := "# Plain skill\n\nNo frontmatter here.\n"
	path := filepath.Join(dir, "plain-skill.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	s, err := skill.LoadSkillFile(path)
	require.NoError(t, err)
	// Name should be derived from filename.
	assert.Equal(t, "plain-skill", s.Meta.Name)
}

func TestSkillDiscoverDir(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md"} {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, name),
			[]byte("# "+name+"\nContent."),
			0o644,
		))
	}
	// Non-MD file should be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("nope"), 0o644))

	skills, err := skill.LoadSkillDir(dir)
	require.NoError(t, err)
	assert.Len(t, skills, 3)
}

func TestSkillToolCall(t *testing.T) {
	s := &skill.Skill{
		Meta:  skill.SkillMeta{Name: "test-skill", Description: "A test skill"},
		RawMD: "## Step 1\nDo something.",
	}
	reg := skill.NewRegistry()
	reg.Add(s)
	st := skill.NewSkillTool(reg)
	ctx := context.Background()

	uctx := &engine.UseContext{WorkDir: t.TempDir()}
	ch, err := st.Call(ctx, []byte(`{"skill":"test-skill"}`), uctx)
	require.NoError(t, err)

	var out string
	for b := range ch {
		out += b.Text
	}
	assert.Contains(t, out, "test-skill")
	assert.Contains(t, out, "Step 1")
}
