package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// SkillTool wraps a loaded Skill as a Tool, injecting its prompt content
// as a user message to trigger the skill's workflow.
type SkillTool struct {
	skill *Skill
}

// NewSkillTool creates a Tool from a Skill.
func NewSkillTool(s *Skill) *SkillTool {
	return &SkillTool{skill: s}
}

func (t *SkillTool) Name() string           { return "skill_" + sanitiseName(t.skill.Meta.Name) }
func (t *SkillTool) UserFacingName() string { return t.skill.Meta.Name }
func (t *SkillTool) Description() string {
	if t.skill.Meta.Description != "" {
		return t.skill.Meta.Description
	}
	return "Run skill: " + t.skill.Meta.Name
}
func (t *SkillTool) IsReadOnly() bool        { return false }
func (t *SkillTool) IsConcurrencySafe() bool { return false }
func (t *SkillTool) MaxResultSizeChars() int { return 0 }
func (t *SkillTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *SkillTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"args":{"type":"object","description":"Optional arguments passed to the skill."}
		}
	}`)
}

func (t *SkillTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *SkillTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *SkillTool) Call(_ context.Context, _ json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		// Return the skill prompt as a text block; the engine will inject it
		// as a user message on the next turn.
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("[Skill: %s]\n\n%s", t.skill.Meta.Name, t.skill.RawMD),
		}
	}()
	return ch, nil
}

// Registry manages a collection of skills.
type Registry struct {
	skills []*Skill
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry { return &Registry{} }

// Add registers one or more skills.
func (r *Registry) Add(skills ...*Skill) { r.skills = append(r.skills, skills...) }

// All returns all registered skills.
func (r *Registry) All() []*Skill { return r.skills }

// AsTools converts all skills to Tool instances.
func (r *Registry) AsTools() []tool.Tool {
	tools := make([]tool.Tool, len(r.skills))
	for i, s := range r.skills {
		tools[i] = NewSkillTool(s)
	}
	return tools
}

func sanitiseName(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return strings.ToLower(sb.String())
}
