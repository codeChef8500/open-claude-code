package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// /config — full implementation
// Aligned with claude-code-main commands/config/config.tsx.
//
// Supports subcommands: get, set, list (default).
// Interactive mode returns structured data for TUI config panel.
// ──────────────────────────────────────────────────────────────────────────────

// ConfigPanelData is the structured data for the config panel TUI component.
type ConfigPanelData struct {
	// Subcommand: "get", "set", "list" (default)
	Subcommand string `json:"subcommand"`
	// Key for get/set operations
	Key string `json:"key,omitempty"`
	// Value for set operations
	Value string `json:"value,omitempty"`
	// Entries is the list of all config entries (for list view)
	Entries []ConfigDisplayEntry `json:"entries,omitempty"`
	// Current session state
	CurrentModel      string `json:"current_model"`
	PermissionMode    string `json:"permission_mode"`
	AutoMode          bool   `json:"auto_mode"`
	Verbose           bool   `json:"verbose"`
	PlanMode          bool   `json:"plan_mode"`
	FastMode          bool   `json:"fast_mode"`
	EffortLevel       string `json:"effort_level"`
	// Config file paths
	ProjectConfigPath string `json:"project_config_path,omitempty"`
	UserConfigPath    string `json:"user_config_path,omitempty"`
}

// ConfigDisplayEntry is a single config item for display.
type ConfigDisplayEntry struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Source  string `json:"source"` // "project", "user", "default", "env"
	Type    string `json:"type"`   // "string", "bool", "number", "array"
	Mutable bool   `json:"mutable"`
}

// DeepConfigCommand replaces the basic ConfigCommand with full logic.
type DeepConfigCommand struct{ BaseCommand }

func (c *DeepConfigCommand) Name() string                  { return "config" }
func (c *DeepConfigCommand) Aliases() []string             { return []string{"settings"} }
func (c *DeepConfigCommand) Description() string           { return "Open config panel" }
func (c *DeepConfigCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepConfigCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DeepConfigCommand) ArgumentHint() string          { return "[get|set|list] [key] [value]" }

func (c *DeepConfigCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := buildConfigPanelData(args, ectx)

	// If "set" subcommand with key+value and services available, apply immediately.
	if data.Subcommand == "set" && data.Key != "" && data.Value != "" {
		if ectx != nil && ectx.Services != nil && ectx.Services.Config != nil {
			if err := ectx.Services.Config.Set(data.Key, data.Value); err != nil {
				data.Value = fmt.Sprintf("Error: %v", err)
			}
		}
	}

	// If "get" subcommand with key and services available, fetch value.
	if data.Subcommand == "get" && data.Key != "" {
		if ectx != nil && ectx.Services != nil && ectx.Services.Config != nil {
			if val, ok := ectx.Services.Config.Get(data.Key); ok {
				data.Value = fmt.Sprintf("%v", val)
			} else {
				data.Value = "(not set)"
			}
		}
	}

	// For "list" (default), populate entries from ConfigService.
	if data.Subcommand == "list" || data.Subcommand == "" {
		data.Subcommand = "list"
		if ectx != nil && ectx.Services != nil && ectx.Services.Config != nil {
			for _, entry := range ectx.Services.Config.List() {
				data.Entries = append(data.Entries, ConfigDisplayEntry{
					Key:     entry.Key,
					Value:   fmt.Sprintf("%v", entry.Value),
					Source:  entry.Source,
					Type:    inferConfigType(entry.Value),
					Mutable: true,
				})
			}
			data.ProjectConfigPath = ectx.Services.Config.ProjectPath()
			data.UserConfigPath = ectx.Services.Config.UserPath()
		}
		// Always include session state entries.
		data.Entries = appendSessionEntries(data.Entries, ectx)
	}

	return &InteractiveResult{
		Component: "config",
		Data:      data,
	}, nil
}

// buildConfigPanelData parses args and populates ConfigPanelData.
func buildConfigPanelData(args []string, ectx *ExecContext) *ConfigPanelData {
	data := &ConfigPanelData{}
	if ectx != nil {
		data.CurrentModel = ectx.Model
		data.PermissionMode = ectx.PermissionMode
		data.AutoMode = ectx.AutoMode
		data.Verbose = ectx.Verbose
		data.PlanMode = ectx.PlanModeActive
		data.FastMode = ectx.FastMode
		data.EffortLevel = ectx.EffortLevel
	}

	if len(args) == 0 {
		data.Subcommand = "list"
		return data
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "get":
		data.Subcommand = "get"
		if len(args) > 1 {
			data.Key = args[1]
		}
	case "set":
		data.Subcommand = "set"
		if len(args) > 1 {
			data.Key = args[1]
		}
		if len(args) > 2 {
			data.Value = strings.Join(args[2:], " ")
		}
	case "list":
		data.Subcommand = "list"
	default:
		// Treat as key for "get"
		data.Subcommand = "get"
		data.Key = args[0]
	}
	return data
}

// appendSessionEntries adds session-derived config entries.
func appendSessionEntries(entries []ConfigDisplayEntry, ectx *ExecContext) []ConfigDisplayEntry {
	if ectx == nil {
		return entries
	}
	sessionEntries := []ConfigDisplayEntry{
		{Key: "model", Value: ectx.Model, Source: "session", Type: "string", Mutable: true},
		{Key: "permission_mode", Value: ectx.PermissionMode, Source: "session", Type: "string", Mutable: true},
		{Key: "auto_mode", Value: fmt.Sprintf("%v", ectx.AutoMode), Source: "session", Type: "bool", Mutable: true},
		{Key: "verbose", Value: fmt.Sprintf("%v", ectx.Verbose), Source: "session", Type: "bool", Mutable: true},
		{Key: "plan_mode", Value: fmt.Sprintf("%v", ectx.PlanModeActive), Source: "session", Type: "bool", Mutable: true},
		{Key: "fast_mode", Value: fmt.Sprintf("%v", ectx.FastMode), Source: "session", Type: "bool", Mutable: true},
		{Key: "effort", Value: ectx.EffortLevel, Source: "session", Type: "string", Mutable: true},
	}
	// Only add session entries that aren't already in the list.
	existing := make(map[string]bool)
	for _, e := range entries {
		existing[e.Key] = true
	}
	for _, se := range sessionEntries {
		if !existing[se.Key] {
			entries = append(entries, se)
		}
	}
	return entries
}

// inferConfigType infers the display type of a config value.
func inferConfigType(v interface{}) string {
	switch v.(type) {
	case bool:
		return "bool"
	case int, int64, float64:
		return "number"
	case []interface{}, []string:
		return "array"
	default:
		return "string"
	}
}

func init() {
	defaultRegistry.RegisterOrReplace(&DeepConfigCommand{})
}
