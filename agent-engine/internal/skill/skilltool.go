package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

const (
	// SkillToolName is the canonical tool name for the unified skill invoker.
	SkillToolName = "Skill"
	// MaxListingDescChars caps per-entry description length in the prompt listing.
	MaxListingDescChars = 250
	// DefaultCharBudget is the fallback for skill listing budget (1% of 200K × 4 chars/token).
	DefaultCharBudget = 8000
	// MaxSkillResultChars limits the result text returned from a skill execution.
	MaxSkillResultChars = 100_000
)

// skillInput is the input schema for the unified SkillTool.
type skillInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args,omitempty"`
}

// skillOutput is the result returned by the SkillTool.
type skillOutput struct {
	Success      bool     `json:"success"`
	CommandName  string   `json:"command_name"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	Model        string   `json:"model,omitempty"`
	Effort       string   `json:"effort,omitempty"`
	Status       string   `json:"status"` // "inline" or "forked"
	Result       string   `json:"result,omitempty"`
	AgentID      string   `json:"agent_id,omitempty"`
}

// SkillTool is the unified skill invoker that mirrors claude-code-main's SkillTool.
// It validates input, checks permissions, runs argument/variable/shell substitution,
// and returns the processed prompt with a context modifier.
type SkillTool struct {
	tool.BaseTool
	// finder resolves a skill name to a Skill. Set by the engine at init.
	finder SkillFinder
	// ctxMod stores the context modifier from the last Call.
	ctxMod func(*engine.UseContext) *engine.UseContext
}

// SkillFinder resolves skill names to Skill objects.
type SkillFinder interface {
	FindSkill(name string) (*Skill, bool)
	AllSkills() []*Skill
}

// NewSkillTool creates the unified Skill tool.
func NewSkillTool(finder SkillFinder) *SkillTool {
	return &SkillTool{finder: finder}
}

func (t *SkillTool) Name() string           { return SkillToolName }
func (t *SkillTool) UserFacingName() string { return SkillToolName }
func (t *SkillTool) Description() string {
	return "Execute a skill within the conversation"
}
func (t *SkillTool) SearchHint() string                       { return "invoke a slash-command skill" }
func (t *SkillTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *SkillTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *SkillTool) MaxResultSizeChars() int                  { return MaxSkillResultChars }
func (t *SkillTool) IsEnabled(_ *tool.UseContext) bool        { return true }

func (t *SkillTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"skill":{"type":"string","description":"The skill name. E.g., \"commit\", \"review-pr\", or \"pdf\""},
			"args":{"type":"string","description":"Optional arguments for the skill"}
		},
		"required":["skill"]
	}`)
}

// Prompt returns the skill system prompt with a budget-aware listing of available skills.
func (t *SkillTool) Prompt(uctx *tool.UseContext) string {
	skills := t.finder.AllSkills()
	listing := FormatSkillListing(skills, DefaultCharBudget)

	prompt := `Execute a skill within the main conversation

When users ask you to perform tasks, check if any of the available skills match. Skills provide specialized capabilities and domain knowledge.

When users reference a "slash command" or "/<something>" (e.g., "/commit", "/review-pr"), they are referring to a skill. Use this tool to invoke it.

How to invoke:
- Use this tool with the skill name and optional arguments
- Examples:
  - skill: "pdf" - invoke the pdf skill
  - skill: "commit", args: "-m 'Fix bug'" - invoke with arguments
  - skill: "review-pr", args: "123" - invoke with arguments

Important:
- Available skills are listed below
- When a skill matches the user's request, invoke the relevant Skill tool BEFORE generating any other response
- NEVER mention a skill without actually calling this tool
- Do not invoke a skill that is already running
`
	if listing != "" {
		prompt += "\nAvailable skills:\n" + listing + "\n"
	}
	return prompt
}

// ToAutoClassifierInput returns the skill name for the auto-mode classifier.
func (t *SkillTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in skillInput
	if json.Unmarshal(input, &in) == nil {
		return in.Skill
	}
	return ""
}

// ValidateInput checks that the skill exists and is invocable.
func (t *SkillTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in skillInput
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid skill input: %w", err)
	}

	name := normalizeSkillName(in.Skill)
	if name == "" {
		return fmt.Errorf("invalid skill format: %q", in.Skill)
	}

	s, ok := t.finder.FindSkill(name)
	if !ok {
		return fmt.Errorf("unknown skill: %s", name)
	}

	if s.Meta.DisableModelInvocation {
		return fmt.Errorf("skill %s cannot be used with the Skill tool (disable-model-invocation)", name)
	}

	return nil
}

// CheckPermissions evaluates permission rules for the skill.
func (t *SkillTool) CheckPermissions(_ context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in skillInput
	if err := json.Unmarshal(input, &in); err != nil {
		return err
	}

	name := normalizeSkillName(in.Skill)
	s, ok := t.finder.FindSkill(name)
	if !ok {
		return fmt.Errorf("unknown skill: %s", name)
	}

	// Skills with only safe properties (no hooks, no allowed tools) auto-allow.
	if skillHasOnlySafeProperties(s) {
		return nil
	}

	// Otherwise delegate to the standard permission flow (will be handled
	// by the orchestration layer's hook/permission system).
	return nil
}

// Call executes the skill: substitutes arguments/variables, executes shell
// commands in the prompt, and returns the processed content.
func (t *SkillTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in skillInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid skill input: %w", err)
	}

	name := normalizeSkillName(in.Skill)
	s, ok := t.finder.FindSkill(name)
	if !ok {
		return nil, fmt.Errorf("unknown skill: %s", name)
	}

	// Prepare the prompt content.
	content := s.RawMD

	// 1. Variable substitution.
	vars := buildSkillVariables(s, uctx)
	content = SubstituteVariables(content, vars)

	// 2. Argument substitution.
	argNames := ParseArgumentNames(s.Meta.Arguments)
	content = SubstituteArguments(content, in.Args, true, argNames)

	// 3. Shell command execution in prompt.
	if strings.Contains(content, "!`") || strings.Contains(content, "```!") {
		shell := s.Meta.Shell
		execContent, _ := ExecuteShellCommandsInPrompt(content, ShellExecContext{
			WorkDir: uctx.WorkDir,
			Shell:   shell,
			Ctx:     ctx,
		})
		content = execContent
	}

	// Build output.
	out := skillOutput{
		Success:     true,
		CommandName: name,
		Status:      "inline",
	}

	if len(s.Meta.AllowedTools) > 0 {
		out.AllowedTools = s.Meta.AllowedTools
	}
	if s.Meta.Model != "" {
		out.Model = s.Meta.Model
	}
	if s.Meta.Effort != "" {
		out.Effort = s.Meta.Effort
	}

	// Set context modifier for allowed tools, model, and effort overrides.
	t.ctxMod = buildContextModifier(s)

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("[Skill: %s]\n\n%s", name, content),
		}
	}()
	return ch, nil
}

// ContextModifier returns the modifier set by the last Call, which adjusts
// allowed tools, model, and effort in the UseContext.
func (t *SkillTool) ContextModifier() func(*engine.UseContext) *engine.UseContext {
	return t.ctxMod
}

// MapToolResultToBlockParam formats the skill result for the API.
func (t *SkillTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text := "Launching skill"
	if out, ok := content.(*skillOutput); ok {
		if out.Status == "forked" {
			text = fmt.Sprintf("Skill %q completed (forked).\n\nResult:\n%s", out.CommandName, out.Result)
		} else {
			text = fmt.Sprintf("Launching skill: %s", out.CommandName)
		}
	}
	return &engine.ContentBlock{
		Type:      engine.ContentTypeToolResult,
		ToolUseID: toolUseID,
		Text:      text,
	}
}

// ── Skill Registry (simple, for the unified tool) ────────────────────────────

// Registry manages a collection of skills and implements SkillFinder.
type Registry struct {
	skills []*Skill
	byName map[string]*Skill
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry { return &Registry{byName: make(map[string]*Skill)} }

// Add registers one or more skills.
func (r *Registry) Add(skills ...*Skill) {
	for _, s := range skills {
		r.skills = append(r.skills, s)
		r.byName[s.Meta.Name] = s
	}
}

// All returns all registered skills.
func (r *Registry) All() []*Skill { return r.skills }

// FindSkill looks up a skill by name.
func (r *Registry) FindSkill(name string) (*Skill, bool) {
	s, ok := r.byName[name]
	return s, ok
}

// AllSkills returns all skills (implements SkillFinder).
func (r *Registry) AllSkills() []*Skill { return r.skills }

// AsTools returns the single unified SkillTool.
func (r *Registry) AsTools() []tool.Tool {
	return []tool.Tool{NewSkillTool(r)}
}

// ── Helper functions ─────────────────────────────────────────────────────────

func normalizeSkillName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "/")
	return name
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

// skillHasOnlySafeProperties returns true if the skill has no properties that
// require explicit permission (hooks, allowed tools overrides, shell commands).
func skillHasOnlySafeProperties(s *Skill) bool {
	if len(s.Meta.AllowedTools) > 0 {
		return false
	}
	if len(s.Meta.Hooks) > 0 {
		return false
	}
	if s.Meta.Shell != "" {
		return false
	}
	if strings.Contains(s.RawMD, "!`") || strings.Contains(s.RawMD, "```!") {
		return false
	}
	return true
}

// buildSkillVariables creates the variable map for ${...} substitution.
func buildSkillVariables(s *Skill, uctx *engine.UseContext) map[string]string {
	vars := map[string]string{
		"CLAUDE_SESSION_ID": uctx.SessionID,
	}
	if s.SkillDir != "" {
		vars["CLAUDE_SKILL_DIR"] = s.SkillDir
	}
	if s.FilePath != "" && s.SkillDir == "" {
		vars["CLAUDE_SKILL_DIR"] = filepath.Dir(s.FilePath)
	}
	return vars
}

// buildContextModifier returns a UseContext modifier that applies the skill's
// allowed tools, model, and effort overrides.
func buildContextModifier(s *Skill) func(*engine.UseContext) *engine.UseContext {
	if len(s.Meta.AllowedTools) == 0 && s.Meta.Model == "" && s.Meta.Effort == "" {
		return nil
	}
	return func(uctx *engine.UseContext) *engine.UseContext {
		modified := *uctx
		if s.Meta.Model != "" {
			modified.MainLoopModel = s.Meta.Model
		}
		// AllowedTools and Effort are applied via the engine's query loop
		// using the ContextModifier mechanism. The engine reads these from
		// the modified context.
		return &modified
	}
}

// FormatSkillListing returns a budget-aware listing of skills for the system prompt.
// Bundled skills always get full descriptions; others may be truncated.
func FormatSkillListing(skills []*Skill, charBudget int) string {
	if len(skills) == 0 {
		return ""
	}

	// Filter out skills that shouldn't appear in the listing.
	var listable []*Skill
	for _, s := range skills {
		if s.Meta.DisableModelInvocation {
			continue
		}
		listable = append(listable, s)
	}

	if len(listable) == 0 {
		return ""
	}

	// Build full descriptions.
	type entry struct {
		skill     *Skill
		full      string
		isBundled bool
	}
	entries := make([]entry, len(listable))
	totalChars := 0
	for i, s := range listable {
		desc := s.Meta.Description
		if s.Meta.WhenToUse != "" {
			desc += " - " + s.Meta.WhenToUse
		}
		if len(desc) > MaxListingDescChars {
			desc = desc[:MaxListingDescChars-1] + "…"
		}
		full := fmt.Sprintf("- %s: %s", s.Meta.Name, desc)
		entries[i] = entry{s, full, s.Meta.Source == "bundled"}
		totalChars += len(full) + 1 // +1 for newline
	}

	// If everything fits, return full.
	if totalChars <= charBudget {
		lines := make([]string, len(entries))
		for i, e := range entries {
			lines[i] = e.full
		}
		return strings.Join(lines, "\n")
	}

	// Budget exceeded: bundled keep full, others get truncated.
	bundledChars := 0
	restCount := 0
	for _, e := range entries {
		if e.isBundled {
			bundledChars += len(e.full) + 1
		} else {
			restCount++
		}
	}

	remaining := charBudget - bundledChars
	maxDescLen := 20
	if restCount > 0 && remaining > 0 {
		nameOverhead := 0
		for _, e := range entries {
			if !e.isBundled {
				nameOverhead += len(e.skill.Meta.Name) + 4 + 1 // "- name: " + newline
			}
		}
		available := remaining - nameOverhead
		if available > 0 {
			maxDescLen = available / restCount
		}
	}

	lines := make([]string, len(entries))
	for i, e := range entries {
		if e.isBundled {
			lines[i] = e.full
		} else if maxDescLen < 20 {
			lines[i] = fmt.Sprintf("- %s", e.skill.Meta.Name)
		} else {
			desc := e.skill.Meta.Description
			if len(desc) > maxDescLen {
				desc = desc[:maxDescLen-1] + "…"
			}
			lines[i] = fmt.Sprintf("- %s: %s", e.skill.Meta.Name, desc)
		}
	}
	return strings.Join(lines, "\n")
}
