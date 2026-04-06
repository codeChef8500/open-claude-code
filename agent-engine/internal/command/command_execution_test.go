package command

import (
	"context"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Phase 3: Per-Command Execution Tests
// Verifies each command's execution logic, output format, and argument handling.
// ──────────────────────────────────────────────────────────────────────────────

func execLocal(t *testing.T, name string, args []string, ectx *ExecContext) string {
	t.Helper()
	cmd := Default().Find(name)
	if cmd == nil {
		t.Fatalf("command /%s not found", name)
	}
	lc, ok := cmd.(LocalCommand)
	if !ok {
		t.Fatalf("/%s is not a LocalCommand", name)
	}
	out, err := lc.Execute(context.Background(), args, ectx)
	if err != nil {
		t.Fatalf("/%s Execute error: %v", name, err)
	}
	return out
}

func execInteractive(t *testing.T, name string, args []string, ectx *ExecContext) *InteractiveResult {
	t.Helper()
	cmd := Default().Find(name)
	if cmd == nil {
		t.Fatalf("command /%s not found", name)
	}
	ic, ok := cmd.(InteractiveCommand)
	if !ok {
		t.Fatalf("/%s is not an InteractiveCommand", name)
	}
	r, err := ic.ExecuteInteractive(context.Background(), args, ectx)
	if err != nil {
		t.Fatalf("/%s ExecuteInteractive error: %v", name, err)
	}
	if r == nil {
		t.Fatalf("/%s returned nil InteractiveResult", name)
	}
	return r
}

func execPrompt(t *testing.T, name string, args []string, ectx *ExecContext) string {
	t.Helper()
	cmd := Default().Find(name)
	if cmd == nil {
		t.Fatalf("command /%s not found", name)
	}
	pc, ok := cmd.(PromptCommand)
	if !ok {
		t.Fatalf("/%s is not a PromptCommand", name)
	}
	content, err := pc.PromptContent(args, ectx)
	if err != nil {
		t.Fatalf("/%s PromptContent error: %v", name, err)
	}
	return content
}

// ── 3.1 Local Command Tests ────────────────────────────────────────────────

func TestHelpOutput(t *testing.T) {
	r := execInteractive(t, "help", nil, newTestEctx())
	if r.Component != "help" {
		t.Errorf("/help component should be 'help', got: %s", r.Component)
	}
	data, ok := r.Data.(*HelpViewData)
	if !ok {
		t.Fatalf("/help data should be *HelpViewData, got: %T", r.Data)
	}
	if len(data.BuiltinCommands) < 5 {
		t.Errorf("/help should list many builtin commands, got %d", len(data.BuiltinCommands))
	}
	if !strings.Contains(data.FallbackText, "Available commands") {
		t.Errorf("/help fallback text should contain 'Available commands', got: %s", data.FallbackText[:min(len(data.FallbackText), 120)])
	}
}

func TestClearOutput(t *testing.T) {
	out := execLocal(t, "clear", nil, newTestEctx())
	if out != "__clear_history__" {
		t.Errorf("/clear should return '__clear_history__', got: %q", out)
	}
}

func TestCompactOutput(t *testing.T) {
	out := execLocal(t, "compact", nil, newTestEctx())
	if out != "__compact__" {
		t.Errorf("/compact should return '__compact__', got: %q", out)
	}
}

func TestStatusOutput(t *testing.T) {
	ectx := newTestEctx()
	r := execInteractive(t, "status", nil, ectx)
	if r.Component != "status" {
		t.Errorf("/status component should be 'status', got: %s", r.Component)
	}
	data, ok := r.Data.(*StatusViewDataV2)
	if !ok {
		t.Fatalf("/status data should be *StatusViewDataV2, got: %T", r.Data)
	}
	if data.SessionID != ectx.SessionID {
		t.Errorf("/status SessionID mismatch: got %s, want %s", data.SessionID, ectx.SessionID)
	}
	if !strings.Contains(data.FallbackText, "Session") {
		t.Errorf("/status fallback text should contain 'Session', got: %s", data.FallbackText)
	}
}

func TestVersionOutput(t *testing.T) {
	out := execLocal(t, "version", nil, newTestEctx())
	if !strings.Contains(out, "Agent Engine") {
		t.Errorf("/version should contain 'Agent Engine', got: %s", out)
	}
	if !strings.Contains(out, "go1") {
		t.Errorf("/version should contain Go version, got: %s", out)
	}
}

func TestQuitOutput(t *testing.T) {
	out := execLocal(t, "quit", nil, newTestEctx())
	if out != "__quit__" {
		t.Errorf("/quit should return '__quit__', got: %q", out)
	}
}

func TestCostOutput(t *testing.T) {
	ectx := newTestEctx()
	ectx.CostUSD = 1.2345
	ectx.TotalTokens = 50000
	ectx.Model = "claude-sonnet-4-20250514"
	out := execLocal(t, "cost", nil, ectx)
	if out == "" {
		t.Error("/cost should return non-empty output")
	}
}

func TestVimToggleOn(t *testing.T) {
	out := execLocal(t, "vim", []string{"on"}, newTestEctx())
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "vim") {
		t.Errorf("/vim on should mention 'vim', got: %s", out)
	}
}

func TestVimToggleOff(t *testing.T) {
	out := execLocal(t, "vim", []string{"off"}, newTestEctx())
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "vim") {
		t.Errorf("/vim off should mention 'vim', got: %s", out)
	}
}

func TestRenameWithArgs(t *testing.T) {
	out := execLocal(t, "rename", []string{"test-session-name"}, newTestEctx())
	if !strings.Contains(out, "test-session-name") {
		t.Errorf("/rename should echo the new name, got: %s", out)
	}
}

func TestRenameNoArgs(t *testing.T) {
	out := execLocal(t, "rename", nil, newTestEctx())
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "usage") && !strings.Contains(lower, "rename") {
		t.Errorf("/rename with no args should show usage hint, got: %s", out)
	}
}

func TestCopyNoClipboard(t *testing.T) {
	out := execLocal(t, "copy", nil, newTestEctx())
	// Without clipboard service, should gracefully handle
	if out == "" {
		t.Error("/copy should return non-empty output even without clipboard service")
	}
}

func TestAdvisorNoArgs(t *testing.T) {
	out := execLocal(t, "advisor", nil, newTestEctx())
	if !strings.Contains(strings.ToLower(out), "not set") && !strings.Contains(strings.ToLower(out), "advisor") {
		t.Errorf("/advisor should mention status, got: %s", out)
	}
}

func TestAdvisorWithModel(t *testing.T) {
	out := execLocal(t, "advisor", []string{"opus"}, newTestEctx())
	if !strings.Contains(out, "opus") {
		t.Errorf("/advisor opus should mention 'opus', got: %s", out)
	}
}

func TestVerboseToggle(t *testing.T) {
	out := execLocal(t, "verbose", nil, newTestEctx())
	if out == "" {
		t.Error("/verbose should return non-empty output")
	}
}

func TestAutoModeToggle(t *testing.T) {
	out := execLocal(t, "auto-mode", nil, newTestEctx())
	if out == "" {
		t.Error("/auto-mode should return non-empty output")
	}
}

func TestLogoutOutput(t *testing.T) {
	out := execLocal(t, "logout", nil, newTestEctx())
	if out == "" {
		t.Error("/logout should return non-empty output")
	}
}

func TestHeapdumpOutput(t *testing.T) {
	out := execLocal(t, "heapdump", nil, newTestEctx())
	if !strings.Contains(strings.ToLower(out), "heap") && !strings.Contains(strings.ToLower(out), "pprof") {
		t.Errorf("/heapdump should mention heap/pprof, got: %s", out)
	}
}

func TestStickersOutput(t *testing.T) {
	out := execLocal(t, "stickers", nil, newTestEctx())
	if !strings.Contains(out, "sticker") {
		t.Errorf("/stickers should mention sticker, got: %s", out)
	}
}

func TestReloadPluginsOutput(t *testing.T) {
	out := execLocal(t, "reload-plugins", nil, newTestEctx())
	if out == "" {
		t.Error("/reload-plugins should return non-empty output")
	}
}

// ── 3.2 Interactive Command Tests ──────────────────────────────────────────

func TestModelPickerData(t *testing.T) {
	r := execInteractive(t, "model", nil, newTestEctx())
	if r.Component != "model" {
		t.Errorf("expected component 'model', got %q", r.Component)
	}
}

func TestModelWithArgs(t *testing.T) {
	r := execInteractive(t, "model", []string{"gpt-4"}, newTestEctx())
	if r.Component != "model" {
		t.Errorf("expected component 'model', got %q", r.Component)
	}
	if r.Data == nil {
		t.Error("model with args should have Data")
	}
}

func TestConfigPanel(t *testing.T) {
	r := execInteractive(t, "config", nil, newTestEctx())
	if r.Component != "config" {
		t.Errorf("expected component 'config', got %q", r.Component)
	}
}

func TestMcpPanel(t *testing.T) {
	r := execInteractive(t, "mcp", nil, newTestEctx())
	if r.Component != "mcp" {
		t.Errorf("expected component 'mcp', got %q", r.Component)
	}
}

func TestDoctorPanel(t *testing.T) {
	r := execInteractive(t, "doctor", nil, newTestEctx())
	if r.Component != "doctor" {
		t.Errorf("expected component 'doctor', got %q", r.Component)
	}
}

func TestThemePickerData(t *testing.T) {
	ectx := newTestEctx()
	ectx.Theme = "dark"
	r := execInteractive(t, "theme", nil, ectx)
	if r.Component != "theme" {
		t.Errorf("expected component 'theme', got %q", r.Component)
	}
	if r.Data == nil {
		t.Error("theme should have Data")
	}
}

func TestColorPickerData(t *testing.T) {
	r := execInteractive(t, "color", []string{"red"}, newTestEctx())
	if r.Component != "color" {
		t.Errorf("expected component 'color', got %q", r.Component)
	}
}

func TestExportWithFormat(t *testing.T) {
	r := execInteractive(t, "export", []string{"json"}, newTestEctx())
	if r.Component != "export" {
		t.Errorf("expected component 'export', got %q", r.Component)
	}
}

func TestKeybindingsData(t *testing.T) {
	r := execInteractive(t, "keybindings", nil, newTestEctx())
	if r.Component != "keybindings" {
		t.Errorf("expected component 'keybindings', got %q", r.Component)
	}
}

func TestOutputStyleData(t *testing.T) {
	r := execInteractive(t, "output-style", nil, newTestEctx())
	if r.Component != "output-style" {
		t.Errorf("expected component 'output-style', got %q", r.Component)
	}
}

func TestResumeWithSessionID(t *testing.T) {
	r := execInteractive(t, "resume", []string{"abc123"}, newTestEctx())
	if r.Component != "resume" {
		t.Errorf("expected component 'resume', got %q", r.Component)
	}
}

func TestLoginPanel(t *testing.T) {
	r := execInteractive(t, "login", nil, newTestEctx())
	if r.Component != "login" {
		t.Errorf("expected component 'login', got %q", r.Component)
	}
}

func TestPlanPanel(t *testing.T) {
	r := execInteractive(t, "plan", nil, newTestEctx())
	if r.Component != "plan" {
		t.Errorf("expected component 'plan', got %q", r.Component)
	}
}

func TestFastPanel(t *testing.T) {
	r := execInteractive(t, "fast", nil, newTestEctx())
	if r.Component != "fast" {
		t.Errorf("expected component 'fast', got %q", r.Component)
	}
}

func TestEffortPanel(t *testing.T) {
	r := execInteractive(t, "effort", nil, newTestEctx())
	if r.Component != "effort" {
		t.Errorf("expected component 'effort', got %q", r.Component)
	}
}

func TestBranchWithName(t *testing.T) {
	r := execInteractive(t, "branch", []string{"my-branch"}, newTestEctx())
	if r.Component != "branch" {
		t.Errorf("expected component 'branch', got %q", r.Component)
	}
	if r.Data == nil {
		t.Error("branch with name should have Data")
	}
}

func TestPermissionsPanel(t *testing.T) {
	r := execInteractive(t, "permissions", nil, newTestEctx())
	if r.Component != "permissions" {
		t.Errorf("expected component 'permissions', got %q", r.Component)
	}
}

func TestMemoryPanel(t *testing.T) {
	r := execInteractive(t, "memory", nil, newTestEctx())
	if r.Component != "memory" {
		t.Errorf("expected component 'memory', got %q", r.Component)
	}
}

func TestPluginPanel(t *testing.T) {
	r := execInteractive(t, "plugin", nil, newTestEctx())
	if r.Component != "plugin" {
		t.Errorf("expected component 'plugin', got %q", r.Component)
	}
}

func TestSkillsPanel(t *testing.T) {
	r := execInteractive(t, "skills", nil, newTestEctx())
	if r.Component != "skills" {
		t.Errorf("expected component 'skills', got %q", r.Component)
	}
}

func TestAgentsPanel(t *testing.T) {
	r := execInteractive(t, "agents", nil, newTestEctx())
	if r.Component != "agents" {
		t.Errorf("expected component 'agents', got %q", r.Component)
	}
}

func TestTasksPanel(t *testing.T) {
	r := execInteractive(t, "tasks", nil, newTestEctx())
	if r.Component != "tasks" {
		t.Errorf("expected component 'tasks', got %q", r.Component)
	}
}

func TestSessionPanel(t *testing.T) {
	r := execInteractive(t, "session", nil, newTestEctx())
	if r.Component != "session" {
		t.Errorf("expected component 'session', got %q", r.Component)
	}
}

func TestDiffPanel(t *testing.T) {
	r := execInteractive(t, "diff", nil, newTestEctx())
	if r.Component != "diff" {
		t.Errorf("expected component 'diff', got %q", r.Component)
	}
}

func TestRewindPanel(t *testing.T) {
	r := execInteractive(t, "rewind", nil, newTestEctx())
	if r.Component != "rewind" {
		t.Errorf("expected component 'rewind', got %q", r.Component)
	}
}

func TestAddDirWithPath(t *testing.T) {
	r := execInteractive(t, "add-dir", []string{"/some/path"}, newTestEctx())
	if r.Component != "add-dir" {
		t.Errorf("expected component 'add-dir', got %q", r.Component)
	}
}

func TestHooksPanel(t *testing.T) {
	r := execInteractive(t, "hooks", nil, newTestEctx())
	if r.Component != "hooks" {
		t.Errorf("expected component 'hooks', got %q", r.Component)
	}
}

func TestFeedbackPanel(t *testing.T) {
	r := execInteractive(t, "feedback", nil, newTestEctx())
	if r.Component != "feedback" {
		t.Errorf("expected component 'feedback', got %q", r.Component)
	}
}

func TestStatsPanel(t *testing.T) {
	r := execInteractive(t, "stats", nil, newTestEctx())
	if r.Component != "stats" {
		t.Errorf("expected component 'stats', got %q", r.Component)
	}
}

func TestDesktopPanel(t *testing.T) {
	r := execInteractive(t, "desktop", nil, newTestEctx())
	if r.Component != "desktop" {
		t.Errorf("expected component 'desktop', got %q", r.Component)
	}
}

func TestPrivacySettingsPanel(t *testing.T) {
	r := execInteractive(t, "privacy-settings", nil, newTestEctx())
	if r.Component != "privacy-settings" {
		t.Errorf("expected component 'privacy-settings', got %q", r.Component)
	}
}

func TestUpgradePanel(t *testing.T) {
	r := execInteractive(t, "upgrade", nil, newTestEctx())
	if r.Component != "upgrade" {
		t.Errorf("expected component 'upgrade', got %q", r.Component)
	}
}

func TestBtwWithMessage(t *testing.T) {
	r := execInteractive(t, "btw", []string{"something", "important"}, newTestEctx())
	if r.Component != "btw" {
		t.Errorf("expected component 'btw', got %q", r.Component)
	}
}

func TestReleaseNotesPanel(t *testing.T) {
	r := execInteractive(t, "release-notes", nil, newTestEctx())
	if r.Component != "release-notes" {
		t.Errorf("expected component 'release-notes', got %q", r.Component)
	}
}

func TestTerminalSetupPanel(t *testing.T) {
	r := execInteractive(t, "terminal-setup", nil, newTestEctx())
	if r.Component != "terminal-setup" {
		t.Errorf("expected component 'terminal-setup', got %q", r.Component)
	}
}

func TestMobilePanel(t *testing.T) {
	r := execInteractive(t, "mobile", nil, newTestEctx())
	if r.Component != "mobile" {
		t.Errorf("expected component 'mobile', got %q", r.Component)
	}
}

func TestChromePanel(t *testing.T) {
	r := execInteractive(t, "chrome", nil, newTestEctx())
	if r.Component != "chrome" {
		t.Errorf("expected component 'chrome', got %q", r.Component)
	}
}

func TestIDEPanel(t *testing.T) {
	r := execInteractive(t, "ide", nil, newTestEctx())
	if r.Component != "ide" {
		t.Errorf("expected component 'ide', got %q", r.Component)
	}
}

func TestSandboxTogglePanel(t *testing.T) {
	r := execInteractive(t, "sandbox-toggle", nil, newTestEctx())
	if r.Component != "sandbox-toggle" {
		t.Errorf("expected component 'sandbox-toggle', got %q", r.Component)
	}
}

func TestRateLimitOptionsPanel(t *testing.T) {
	r := execInteractive(t, "rate-limit-options", nil, newTestEctx())
	if r.Component != "rate-limit-options" {
		t.Errorf("expected component 'rate-limit-options', got %q", r.Component)
	}
}

func TestInstallGitHubAppPanel(t *testing.T) {
	r := execInteractive(t, "install-github-app", nil, newTestEctx())
	if r.Component != "install-github-app" {
		t.Errorf("expected component 'install-github-app', got %q", r.Component)
	}
}

func TestInstallSlackAppPanel(t *testing.T) {
	r := execInteractive(t, "install-slack-app", nil, newTestEctx())
	if r.Component != "install-slack-app" {
		t.Errorf("expected component 'install-slack-app', got %q", r.Component)
	}
}

func TestRemoteEnvPanel(t *testing.T) {
	r := execInteractive(t, "remote-env", nil, newTestEctx())
	if r.Component != "remote-env" {
		t.Errorf("expected component 'remote-env', got %q", r.Component)
	}
}

func TestRemoteSetupPanel(t *testing.T) {
	r := execInteractive(t, "remote-setup", nil, newTestEctx())
	if r.Component != "remote-setup" {
		t.Errorf("expected component 'remote-setup', got %q", r.Component)
	}
}

func TestThinkbackPanel(t *testing.T) {
	r := execInteractive(t, "thinkback", nil, newTestEctx())
	if r.Component != "thinkback" {
		t.Errorf("expected component 'thinkback', got %q", r.Component)
	}
}

func TestThinkbackPlayPanel(t *testing.T) {
	r := execInteractive(t, "thinkback-play", nil, newTestEctx())
	if r.Component != "thinkback-play" {
		t.Errorf("expected component 'thinkback-play', got %q", r.Component)
	}
}

func TestUltrareviewPanel(t *testing.T) {
	r := execInteractive(t, "ultrareview", nil, newTestEctx())
	if r.Component != "ultrareview" {
		t.Errorf("expected component 'ultrareview', got %q", r.Component)
	}
}

func TestWorkflowPanel(t *testing.T) {
	r := execInteractive(t, "workflow", nil, newTestEctx())
	if r.Component != "workflow" {
		t.Errorf("expected component 'workflow', got %q", r.Component)
	}
}

func TestUsagePanel(t *testing.T) {
	r := execInteractive(t, "usage", nil, newTestEctx())
	if r.Component != "usage" {
		t.Errorf("expected component 'usage', got %q", r.Component)
	}
}

func TestExtraUsagePanel(t *testing.T) {
	r := execInteractive(t, "extra-usage", nil, newTestEctx())
	if r.Component != "extra-usage" {
		t.Errorf("expected component 'extra-usage', got %q", r.Component)
	}
}

func TestPassesPanel(t *testing.T) {
	r := execInteractive(t, "passes", nil, newTestEctx())
	if r.Component != "passes" {
		t.Errorf("expected component 'passes', got %q", r.Component)
	}
}

// ── 3.3 Prompt Command Tests ───────────────────────────────────────────────

func TestCommitPromptContent(t *testing.T) {
	content := execPrompt(t, "commit", nil, newTestEctx())
	if !strings.Contains(content, "Git Safety Protocol") {
		t.Error("/commit prompt should contain 'Git Safety Protocol'")
	}
	if !strings.Contains(content, "git diff") {
		t.Error("/commit prompt should mention 'git diff'")
	}
}

func TestReviewPromptContent(t *testing.T) {
	content := execPrompt(t, "review", nil, newTestEctx())
	if !strings.Contains(content, "Code Review") {
		t.Error("/review prompt should contain 'Code Review'")
	}
}

func TestReviewWithArgs(t *testing.T) {
	content := execPrompt(t, "review", []string{"src/"}, newTestEctx())
	if !strings.Contains(content, "Code Review") {
		t.Error("/review with args should still contain 'Code Review'")
	}
}

func TestSecurityReviewPrompt(t *testing.T) {
	content := execPrompt(t, "security-review", nil, newTestEctx())
	if !strings.Contains(content, "SECURITY CATEGORIES") {
		t.Error("/security-review should contain 'SECURITY CATEGORIES'")
	}
	if !strings.Contains(content, "CONFIDENCE SCORING") {
		t.Error("/security-review should contain 'CONFIDENCE SCORING'")
	}
}

func TestInitPrompt(t *testing.T) {
	content := execPrompt(t, "init", nil, newTestEctx())
	if !strings.Contains(content, "CLAUDE.md") {
		t.Error("/init prompt should mention 'CLAUDE.md'")
	}
}

func TestCommitPushPrPrompt(t *testing.T) {
	content := execPrompt(t, "commit-push-pr", nil, newTestEctx())
	if !strings.Contains(content, "Git Safety Protocol") {
		t.Error("/commit-push-pr should contain 'Git Safety Protocol'")
	}
}

func TestPRCommentsPrompt(t *testing.T) {
	content := execPrompt(t, "pr-comments", []string{"123"}, newTestEctx())
	if !strings.Contains(content, "PR") {
		t.Error("/pr-comments should mention 'PR'")
	}
	if !strings.Contains(content, "123") {
		t.Error("/pr-comments 123 should include the PR number in prompt")
	}
}

func TestInsightsPrompt(t *testing.T) {
	content := execPrompt(t, "insights", nil, newTestEctx())
	if !strings.Contains(content, "Architecture") {
		t.Error("/insights should mention 'Architecture'")
	}
}

func TestInitVerifiersPrompt(t *testing.T) {
	content := execPrompt(t, "init-verifiers", nil, newTestEctx())
	if !strings.Contains(content, "verifier") {
		t.Error("/init-verifiers should mention 'verifier'")
	}
}

func TestBridgeKickPrompt(t *testing.T) {
	content := execPrompt(t, "bridge-kick", nil, newTestEctx())
	if content == "" {
		t.Error("/bridge-kick should return non-empty prompt")
	}
}

// ── 3.4 Buddy Signal Tests ────────────────────────────────────────────────

func TestBuddyShowSignal(t *testing.T) {
	out := execLocal(t, "buddy", nil, newTestEctx())
	if !strings.HasPrefix(out, "__buddy_") {
		t.Errorf("/buddy should return a buddy signal, got: %q", out)
	}
}

func TestBuddyPetSignal(t *testing.T) {
	out := execLocal(t, "buddy", []string{"pet"}, newTestEctx())
	if out != "__buddy_pet__" {
		t.Errorf("/buddy pet should return '__buddy_pet__', got: %q", out)
	}
}

func TestBuddyMuteSignal(t *testing.T) {
	out := execLocal(t, "buddy", []string{"mute"}, newTestEctx())
	if out != "__buddy_mute__" {
		t.Errorf("/buddy mute should return '__buddy_mute__', got: %q", out)
	}
}

func TestBuddyUnmuteSignal(t *testing.T) {
	out := execLocal(t, "buddy", []string{"unmute"}, newTestEctx())
	if out != "__buddy_unmute__" {
		t.Errorf("/buddy unmute should return '__buddy_unmute__', got: %q", out)
	}
}

func TestBuddyStatsSignal(t *testing.T) {
	out := execLocal(t, "buddy", []string{"stats"}, newTestEctx())
	if out != "__buddy_stats__" {
		t.Errorf("/buddy stats should return '__buddy_stats__', got: %q", out)
	}
}

// ── 3.5 Disabled Command Behavior ──────────────────────────────────────────

func TestFilesCommandDisabled(t *testing.T) {
	cmd := Default().Find("files")
	if cmd == nil {
		t.Fatal("/files not found")
	}
	if cmd.IsEnabled(newTestEctx()) {
		t.Error("/files should be disabled (ant-only)")
	}
}

func TestTagCommandDisabled(t *testing.T) {
	cmd := Default().Find("tag")
	if cmd == nil {
		t.Fatal("/tag not found")
	}
	if cmd.IsEnabled(newTestEctx()) {
		t.Error("/tag should be disabled (ant-only)")
	}
}
