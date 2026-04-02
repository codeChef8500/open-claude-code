package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadHooksFromSettings loads hook configurations from a settings JSON file.
// The file should contain a "hooks" key mapping event names to arrays of hook configs.
func LoadHooksFromSettings(path string) (HooksSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read hooks config %q: %w", path, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse hooks config %q: %w", path, err)
	}

	hooksJSON, ok := raw["hooks"]
	if !ok {
		return nil, nil
	}

	var rawHooks map[string]json.RawMessage
	if err := json.Unmarshal(hooksJSON, &rawHooks); err != nil {
		return nil, fmt.Errorf("parse hooks section: %w", err)
	}

	settings := make(HooksSettings)
	for eventName, cfgJSON := range rawHooks {
		event := HookEvent(eventName)
		if !isValidEvent(event) {
			continue
		}
		var configs []HookConfig
		if err := json.Unmarshal(cfgJSON, &configs); err != nil {
			return nil, fmt.Errorf("parse hooks for event %q: %w", eventName, err)
		}
		settings[event] = configs
	}

	return settings, nil
}

// LoadHooksFromDir loads and merges hook settings from multiple files in a directory.
// Files are loaded in alphabetical order; later files override earlier ones per-event.
func LoadHooksFromDir(dir string) (HooksSettings, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	merged := make(HooksSettings)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		settings, err := LoadHooksFromSettings(path)
		if err != nil {
			continue
		}
		for event, configs := range settings {
			merged[event] = append(merged[event], configs...)
		}
	}
	return merged, nil
}

// MergeHooksSettings merges multiple HooksSettings into one.
// Later settings append hooks to the same event.
func MergeHooksSettings(layers ...HooksSettings) HooksSettings {
	merged := make(HooksSettings)
	for _, layer := range layers {
		for event, configs := range layer {
			merged[event] = append(merged[event], configs...)
		}
	}
	return merged
}

// ValidateHookConfig checks a hook config for common errors.
func ValidateHookConfig(cfg HookConfig) []string {
	var errs []string
	if cfg.Command == "" {
		errs = append(errs, "command is required")
	}
	if !isValidEvent(cfg.Event) {
		errs = append(errs, fmt.Sprintf("unknown event %q", cfg.Event))
	}
	if cfg.TimeoutSeconds < 0 {
		errs = append(errs, "timeout_seconds must be >= 0")
	}
	if cfg.TimeoutSeconds > 300 {
		errs = append(errs, "timeout_seconds exceeds 300s max")
	}
	return errs
}

// ValidateSettings validates all hooks in a settings map.
func ValidateSettings(settings HooksSettings) map[HookEvent][]string {
	issues := make(map[HookEvent][]string)
	for event, configs := range settings {
		for i, cfg := range configs {
			cfg.Event = event
			errs := ValidateHookConfig(cfg)
			if len(errs) > 0 {
				for _, e := range errs {
					issues[event] = append(issues[event], fmt.Sprintf("hook[%d]: %s", i, e))
				}
			}
		}
	}
	if len(issues) == 0 {
		return nil
	}
	return issues
}

func isValidEvent(event HookEvent) bool {
	for _, e := range AllHookEvents {
		if e == event {
			return true
		}
	}
	return false
}
