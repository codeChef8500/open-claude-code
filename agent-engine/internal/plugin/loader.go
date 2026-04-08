package plugin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadPluginFromDir loads a manifest-based plugin from a directory containing plugin.json.
func LoadPluginFromDir(dir string, source PluginSource) (*LoadedPlugin, error) {
	manifestPath := filepath.Join(dir, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if err := ValidateManifest(&manifest); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}

	lp := &LoadedPlugin{
		Name:     manifest.Name,
		Manifest: &manifest,
		Path:     dir,
		Source:   source,
		Enabled:  true, // default; caller can override
	}

	// Resolve standard directory paths.
	lp.CommandsPath = resolveSubdir(dir, "commands")
	lp.SkillsPath = resolveSubdir(dir, "skills")
	lp.AgentsPath = resolveSubdir(dir, "agents")

	// Resolve hooks config.
	hooksConfig, err := resolveHooksConfig(dir, manifest.Hooks)
	if err != nil {
		slog.Warn("plugin hooks config error", slog.String("plugin", manifest.Name), slog.Any("err", err))
	} else {
		lp.HooksConfig = hooksConfig
	}

	// Resolve LSP configs.
	lspConfigs, err := resolveLSPConfigs(dir, manifest.LSPServers)
	if err != nil {
		slog.Warn("plugin lsp config error", slog.String("plugin", manifest.Name), slog.Any("err", err))
	} else {
		lp.LSPConfigs = lspConfigs
	}

	// Merge settings.
	if manifest.Settings != nil {
		lp.MergedSettings = manifest.Settings
	}

	return lp, nil
}

// LoadAllPlugins scans directories for manifest-based plugins and loads them.
// enabledMap maps plugin name → enabled status. If nil, all are enabled.
func LoadAllPlugins(searchDirs []string, enabledMap map[string]bool) *PluginLoadResult {
	result := &PluginLoadResult{}
	seen := make(map[string]struct{})

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pluginDir := filepath.Join(dir, e.Name())
			manifestPath := filepath.Join(pluginDir, "plugin.json")
			if _, err := os.Stat(manifestPath); err != nil {
				continue
			}

			lp, err := LoadPluginFromDir(pluginDir, SourceMarketplace)
			if err != nil {
				result.Errors = append(result.Errors, PluginError{
					Name:    e.Name(),
					Path:    pluginDir,
					Message: err.Error(),
					Err:     err,
				})
				continue
			}

			// Skip duplicates.
			if _, dup := seen[lp.Name]; dup {
				continue
			}
			seen[lp.Name] = struct{}{}

			// Apply enabled state.
			if enabledMap != nil {
				if enabled, ok := enabledMap[lp.Name]; ok {
					lp.Enabled = enabled
				}
			}

			if lp.Enabled {
				result.Enabled = append(result.Enabled, lp)
			} else {
				result.Disabled = append(result.Disabled, lp)
			}
		}
	}

	return result
}

// LoadPluginHooksConfig reads hooks from the standard hooks/hooks.json path
// within a plugin, plus any additional hooks from the manifest.
func LoadPluginHooksConfig(pluginPath string, manifest *PluginManifest) (map[string]interface{}, error) {
	merged := make(map[string]interface{})

	// Standard hooks/hooks.json path.
	standardPath := filepath.Join(pluginPath, "hooks", "hooks.json")
	if data, err := os.ReadFile(standardPath); err == nil {
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err == nil {
			for k, v := range parsed {
				merged[k] = v
			}
		}
	}

	// Additional hooks from manifest.
	if manifest != nil && manifest.Hooks != nil {
		additional, err := resolveHooksConfig(pluginPath, manifest.Hooks)
		if err == nil && additional != nil {
			for k, v := range additional {
				merged[k] = v
			}
		}
	}

	if len(merged) == 0 {
		return nil, nil
	}
	return merged, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// resolveSubdir returns the absolute path to a subdirectory if it exists,
// or empty string if it doesn't.
func resolveSubdir(base, sub string) string {
	p := filepath.Join(base, sub)
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return p
	}
	return ""
}

// resolveHooksConfig handles the hooks field from the manifest, which can be:
// - a JSON path string (e.g., "./hooks.json")
// - an inline object
// - an array of paths/objects
func resolveHooksConfig(pluginDir string, raw json.RawMessage) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Try string (path to JSON file).
	var path string
	if err := json.Unmarshal(raw, &path); err == nil {
		return loadJSONFile(pluginDir, path)
	}

	// Try inline object.
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj, nil
	}

	// Try array.
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		merged := make(map[string]interface{})
		for _, item := range arr {
			var p string
			if json.Unmarshal(item, &p) == nil {
				if m, err := loadJSONFile(pluginDir, p); err == nil {
					for k, v := range m {
						merged[k] = v
					}
				}
				continue
			}
			var o map[string]interface{}
			if json.Unmarshal(item, &o) == nil {
				for k, v := range o {
					merged[k] = v
				}
			}
		}
		return merged, nil
	}

	return nil, fmt.Errorf("hooks field has unsupported format")
}

// resolveLSPConfigs handles the lspServers manifest field.
func resolveLSPConfigs(pluginDir string, raw json.RawMessage) (map[string]*LspServerConfig, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Try string (path to JSON file).
	var path string
	if err := json.Unmarshal(raw, &path); err == nil {
		m, err := loadJSONFile(pluginDir, path)
		if err != nil {
			return nil, err
		}
		return parseLSPMap(m)
	}

	// Try inline object.
	var obj map[string]*LspServerConfig
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj, nil
	}

	// Try array.
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		merged := make(map[string]*LspServerConfig)
		for _, item := range arr {
			var p string
			if json.Unmarshal(item, &p) == nil {
				m, err := loadJSONFile(pluginDir, p)
				if err == nil {
					if parsed, err := parseLSPMap(m); err == nil {
						for k, v := range parsed {
							merged[k] = v
						}
					}
				}
				continue
			}
			var o map[string]*LspServerConfig
			if json.Unmarshal(item, &o) == nil {
				for k, v := range o {
					merged[k] = v
				}
			}
		}
		return merged, nil
	}

	return nil, fmt.Errorf("lspServers field has unsupported format")
}

// loadJSONFile reads a JSON file from a relative path within a plugin directory.
func loadJSONFile(pluginDir, relPath string) (map[string]interface{}, error) {
	relPath = strings.TrimPrefix(relPath, "./")
	absPath := filepath.Join(pluginDir, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// parseLSPMap re-marshals a generic map into typed LspServerConfig values.
func parseLSPMap(m map[string]interface{}) (map[string]*LspServerConfig, error) {
	result := make(map[string]*LspServerConfig)
	for k, v := range m {
		data, err := json.Marshal(v)
		if err != nil {
			continue
		}
		var cfg LspServerConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		result[k] = &cfg
	}
	return result, nil
}
