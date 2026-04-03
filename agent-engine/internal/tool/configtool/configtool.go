package configtool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ── Supported settings ──────────────────────────────────────────────────────

// SettingDef describes a configurable setting.
type SettingDef struct {
	Key         string   `json:"key"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // "string", "boolean", "number"
	Options     []string `json:"options,omitempty"`
	ReadOnly    bool     `json:"readOnly,omitempty"`
}

// ConfigStore is the interface for reading/writing config values.
type ConfigStore interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}) error
}

// MapConfigStore is a simple in-memory config store.
type MapConfigStore struct {
	data map[string]interface{}
}

// NewMapConfigStore creates a MapConfigStore with initial data.
func NewMapConfigStore(initial map[string]interface{}) *MapConfigStore {
	if initial == nil {
		initial = make(map[string]interface{})
	}
	return &MapConfigStore{data: initial}
}

func (s *MapConfigStore) Get(key string) (interface{}, bool) {
	v, ok := s.data[key]
	return v, ok
}

func (s *MapConfigStore) Set(key string, value interface{}) error {
	s.data[key] = value
	return nil
}

// defaultSettings defines the supported settings and their metadata.
var defaultSettings = []SettingDef{
	{Key: "model", Description: "The model to use for conversations", Type: "string"},
	{Key: "theme", Description: "Color theme (light, dark, auto)", Type: "string", Options: []string{"light", "dark", "auto"}},
	{Key: "verbose", Description: "Enable verbose output", Type: "boolean"},
	{Key: "autoCompact", Description: "Enable automatic context compaction", Type: "boolean"},
	{Key: "maxTurns", Description: "Maximum conversation turns", Type: "number"},
	{Key: "permissions.defaultMode", Description: "Default permission mode (ask, auto, deny)", Type: "string", Options: []string{"ask", "auto", "deny"}},
	{Key: "notifications.enabled", Description: "Enable desktop notifications", Type: "boolean"},
}

// ── Tool implementation ─────────────────────────────────────────────────────

// ConfigTool allows the agent to view and modify configuration settings.
type ConfigTool struct {
	tool.BaseTool
	store    ConfigStore
	settings []SettingDef
}

// New creates a ConfigTool with the given config store.
func New(store ConfigStore) *ConfigTool {
	return &ConfigTool{
		store:    store,
		settings: defaultSettings,
	}
}

// NewWithSettings creates a ConfigTool with custom settings definitions.
func NewWithSettings(store ConfigStore, settings []SettingDef) *ConfigTool {
	return &ConfigTool{
		store:    store,
		settings: settings,
	}
}

func (t *ConfigTool) Name() string           { return "config" }
func (t *ConfigTool) UserFacingName() string { return "Config" }
func (t *ConfigTool) Description() string {
	return "View or modify configuration settings. Omit 'value' to read, provide 'value' to set."
}
func (t *ConfigTool) MaxResultSizeChars() int                  { return 100_000 }
func (t *ConfigTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *ConfigTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *ConfigTool) ShouldDefer() bool                        { return true }
func (t *ConfigTool) SearchHint() string                       { return "get or set configuration settings (theme, model)" }

func (t *ConfigTool) IsReadOnly(input json.RawMessage) bool {
	var args struct {
		Value interface{} `json:"value"`
	}
	_ = json.Unmarshal(input, &args)
	return args.Value == nil
}

func (t *ConfigTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"setting": {
				"type": "string",
				"description": "The setting key (e.g., \"theme\", \"model\", \"permissions.defaultMode\")."
			},
			"value": {
				"description": "The new value. Omit to get the current value.",
				"oneOf": [
					{"type": "string"},
					{"type": "boolean"},
					{"type": "number"}
				]
			}
		},
		"required": ["setting"]
	}`)
}

func (t *ConfigTool) Prompt(_ *tool.UseContext) string {
	var parts []string
	parts = append(parts, "Use this tool to get or set configuration settings.")
	parts = append(parts, "\nSupported settings:")
	for _, s := range t.settings {
		line := fmt.Sprintf("  - %s (%s): %s", s.Key, s.Type, s.Description)
		if len(s.Options) > 0 {
			line += fmt.Sprintf(" [options: %s]", strings.Join(s.Options, ", "))
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n")
}

func (t *ConfigTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var args struct {
		Setting string `json:"setting"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if args.Setting == "" {
		return fmt.Errorf("setting must not be empty")
	}
	return nil
}

func (t *ConfigTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	// Reads are auto-allowed. Writes need permission.
	var args struct {
		Value interface{} `json:"value"`
	}
	_ = json.Unmarshal(input, &args)
	if args.Value == nil {
		return nil // read is free
	}
	return nil // permission checked externally via hooks
}

func (t *ConfigTool) Call(_ context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)

	go func() {
		defer close(ch)

		var args struct {
			Setting string      `json:"setting"`
			Value   interface{} `json:"value"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}

		// Check setting exists.
		var settingDef *SettingDef
		for i := range t.settings {
			if t.settings[i].Key == args.Setting {
				settingDef = &t.settings[i]
				break
			}
		}
		if settingDef == nil {
			var keys []string
			for _, s := range t.settings {
				keys = append(keys, s.Key)
			}
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Unknown setting: %q. Available: %s", args.Setting, strings.Join(keys, ", ")),
				IsError: true,
			}
			return
		}

		// GET operation.
		if args.Value == nil {
			val, found := t.store.Get(args.Setting)
			if !found {
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: fmt.Sprintf("%s = (not set)", args.Setting),
				}
			} else {
				data, _ := json.Marshal(val)
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: fmt.Sprintf("%s = %s", args.Setting, string(data)),
				}
			}
			return
		}

		// SET operation.
		if settingDef.ReadOnly {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Setting %q is read-only.", args.Setting),
				IsError: true,
			}
			return
		}

		// Validate options.
		if len(settingDef.Options) > 0 {
			strVal := fmt.Sprintf("%v", args.Value)
			valid := false
			for _, opt := range settingDef.Options {
				if strVal == opt {
					valid = true
					break
				}
			}
			if !valid {
				ch <- &engine.ContentBlock{
					Type:    engine.ContentTypeText,
					Text:    fmt.Sprintf("Invalid value %q for %s. Options: %s", strVal, args.Setting, strings.Join(settingDef.Options, ", ")),
					IsError: true,
				}
				return
			}
		}

		// Coerce boolean strings.
		finalValue := args.Value
		if settingDef.Type == "boolean" {
			if strVal, ok := args.Value.(string); ok {
				switch strings.ToLower(strings.TrimSpace(strVal)) {
				case "true":
					finalValue = true
				case "false":
					finalValue = false
				default:
					ch <- &engine.ContentBlock{
						Type:    engine.ContentTypeText,
						Text:    fmt.Sprintf("%s requires true or false.", args.Setting),
						IsError: true,
					}
					return
				}
			}
		}

		previousVal, _ := t.store.Get(args.Setting)
		if err := t.store.Set(args.Setting, finalValue); err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Error setting %s: %v", args.Setting, err),
				IsError: true,
			}
			return
		}

		prevJSON, _ := json.Marshal(previousVal)
		newJSON, _ := json.Marshal(finalValue)
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Set %s to %s (was: %s)", args.Setting, string(newJSON), string(prevJSON)),
		}
	}()

	return ch, nil
}
