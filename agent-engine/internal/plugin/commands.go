package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/skill"
)

// LoadPluginCommands loads slash-command skills from a plugin.
// Scans the standard commands/ directory plus any additional paths from the manifest.
// Commands are namespaced as "pluginName:commandName".
func LoadPluginCommands(plugin *LoadedPlugin) ([]*skill.Skill, error) {
	if plugin == nil || plugin.Manifest == nil {
		return nil, nil
	}

	loadedPaths := make(map[string]struct{})
	var skills []*skill.Skill

	// 1. Load from standard commands/ directory.
	if plugin.CommandsPath != "" {
		cmds, err := loadCommandsFromDir(plugin.CommandsPath, plugin.Name, loadedPaths)
		if err == nil {
			skills = append(skills, cmds...)
		}
	}

	// 2. Load additional commands from manifest.
	if plugin.Manifest.Commands != nil {
		extra, err := resolveManifestCommands(plugin, loadedPaths)
		if err == nil {
			skills = append(skills, extra...)
		}
	}

	// Tag all skills with plugin source.
	for _, s := range skills {
		s.Meta.Source = "plugin"
		s.Meta.LoadedFrom = plugin.Path
	}

	return skills, nil
}

// LoadPluginSkills loads skills from a plugin's skills/ directory
// plus any additional paths from the manifest.
func LoadPluginSkills(plugin *LoadedPlugin) ([]*skill.Skill, error) {
	if plugin == nil || plugin.Manifest == nil {
		return nil, nil
	}

	var skills []*skill.Skill

	// 1. Load from standard skills/ directory.
	if plugin.SkillsPath != "" {
		loaded, err := skill.LoadSkillDir(plugin.SkillsPath)
		if err == nil {
			for _, s := range loaded {
				s.Meta.Name = namespacedName(plugin.Name, s.Meta.Name)
				skills = append(skills, s)
			}
		}
	}

	// 2. Load additional skill directories from manifest.
	if plugin.Manifest.Skills != nil {
		dirs := resolveManifestPaths(plugin.Path, plugin.Manifest.Skills)
		for _, dir := range dirs {
			loaded, err := skill.LoadSkillDir(dir)
			if err != nil {
				continue
			}
			for _, s := range loaded {
				s.Meta.Name = namespacedName(plugin.Name, s.Meta.Name)
				skills = append(skills, s)
			}
		}
	}

	// Tag all skills with plugin source.
	for _, s := range skills {
		s.Meta.Source = "plugin"
		s.Meta.LoadedFrom = plugin.Path
	}

	return skills, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// loadCommandsFromDir walks a directory for *.md files and SKILL.md directories,
// namespacing each command with the plugin name.
func loadCommandsFromDir(dir, pluginName string, loadedPaths map[string]struct{}) ([]*skill.Skill, error) {
	loaded, err := skill.LoadSkillDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []*skill.Skill
	for _, s := range loaded {
		absPath := s.FilePath
		if absPath == "" && s.SkillDir != "" {
			absPath = s.SkillDir
		}
		if absPath != "" {
			if _, dup := loadedPaths[absPath]; dup {
				continue
			}
			loadedPaths[absPath] = struct{}{}
		}
		s.Meta.Name = namespacedName(pluginName, s.Meta.Name)
		skills = append(skills, s)
	}
	return skills, nil
}

// resolveManifestCommands handles the manifest's commands field, which can be:
// - a string path
// - an array of paths
// - an object mapping name → CommandMetadata
func resolveManifestCommands(plugin *LoadedPlugin, loadedPaths map[string]struct{}) ([]*skill.Skill, error) {
	raw := plugin.Manifest.Commands
	if len(raw) == 0 {
		return nil, nil
	}

	var skills []*skill.Skill

	// Try string (single path).
	var singlePath string
	if json.Unmarshal(raw, &singlePath) == nil {
		s, err := loadCommandFromPath(plugin.Path, plugin.Name, singlePath, loadedPaths)
		if err == nil && s != nil {
			skills = append(skills, s)
		}
		return skills, nil
	}

	// Try array of paths.
	var paths []string
	if json.Unmarshal(raw, &paths) == nil {
		for _, p := range paths {
			s, err := loadCommandFromPath(plugin.Path, plugin.Name, p, loadedPaths)
			if err == nil && s != nil {
				skills = append(skills, s)
			}
		}
		return skills, nil
	}

	// Try object mapping: { "name": CommandMetadata }.
	var cmdMap map[string]*CommandMetadata
	if json.Unmarshal(raw, &cmdMap) == nil {
		for name, meta := range cmdMap {
			s, err := commandMetadataToSkill(plugin.Path, plugin.Name, name, meta, loadedPaths)
			if err == nil && s != nil {
				skills = append(skills, s)
			}
		}
		return skills, nil
	}

	return nil, fmt.Errorf("commands field has unsupported format")
}

// loadCommandFromPath loads a single command from a relative path.
func loadCommandFromPath(pluginDir, pluginName, relPath string, loadedPaths map[string]struct{}) (*skill.Skill, error) {
	relPath = strings.TrimPrefix(relPath, "./")
	absPath := filepath.Join(pluginDir, relPath)

	if _, dup := loadedPaths[absPath]; dup {
		return nil, nil
	}

	// Check if it's a directory containing SKILL.md.
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		skillMD := filepath.Join(absPath, "SKILL.md")
		if _, err := os.Stat(skillMD); err != nil {
			return nil, fmt.Errorf("directory %s does not contain SKILL.md", absPath)
		}
		absPath = skillMD
	}

	loadedPaths[absPath] = struct{}{}

	s, err := skill.LoadSkillFile(absPath)
	if err != nil {
		return nil, err
	}
	s.Meta.Name = namespacedName(pluginName, s.Meta.Name)
	return s, nil
}

// commandMetadataToSkill converts a CommandMetadata entry to a Skill.
func commandMetadataToSkill(pluginDir, pluginName, name string, meta *CommandMetadata, loadedPaths map[string]struct{}) (*skill.Skill, error) {
	if meta == nil {
		return nil, fmt.Errorf("nil metadata for command %s", name)
	}

	var s *skill.Skill

	if meta.Source != "" {
		// Load from file.
		loaded, err := loadCommandFromPath(pluginDir, pluginName, meta.Source, loadedPaths)
		if err != nil {
			return nil, err
		}
		s = loaded
	} else if meta.Content != "" {
		// Inline content.
		parsed, err := skill.ParseSkillBytes([]byte(meta.Content), name+".md")
		if err != nil {
			return nil, err
		}
		s = parsed
	} else {
		return nil, fmt.Errorf("command %s must have either source or content", name)
	}

	if s == nil {
		return nil, nil
	}

	// Override with metadata.
	s.Meta.Name = namespacedName(pluginName, name)
	if meta.Description != "" {
		s.Meta.Description = meta.Description
	}
	if meta.ArgumentHint != "" {
		s.Meta.ArgumentHint = meta.ArgumentHint
	}
	if meta.Model != "" {
		s.Meta.Model = meta.Model
	}
	if len(meta.AllowedTools) > 0 {
		s.Meta.AllowedTools = meta.AllowedTools
	}

	return s, nil
}

// resolveManifestPaths resolves the manifest's skills/agents field
// (string, or array of strings) into absolute directory paths.
func resolveManifestPaths(pluginDir string, raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var paths []string

	// Try single path.
	var single string
	if json.Unmarshal(raw, &single) == nil {
		single = strings.TrimPrefix(single, "./")
		abs := filepath.Join(pluginDir, single)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			paths = append(paths, abs)
		}
		return paths
	}

	// Try array of paths.
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		for _, p := range arr {
			p = strings.TrimPrefix(p, "./")
			abs := filepath.Join(pluginDir, p)
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				paths = append(paths, abs)
			}
		}
	}

	return paths
}

// namespacedName returns "pluginName:cmdName" if pluginName is non-empty.
func namespacedName(pluginName, cmdName string) string {
	if pluginName == "" {
		return cmdName
	}
	return pluginName + ":" + cmdName
}

// SubstitutePluginVariables replaces plugin-specific ${...} variables in content.
//
//	${CLAUDE_PLUGIN_ROOT} → plugin directory path
//	${CLAUDE_PLUGIN_DATA} → ~/.claude/plugin-data/<name>/
//	${user_config.KEY}    → user config value
func SubstitutePluginVariables(content string, plugin *LoadedPlugin) string {
	if !strings.Contains(content, "${") {
		return content
	}

	result := content
	result = strings.ReplaceAll(result, "${CLAUDE_PLUGIN_ROOT}", plugin.Path)

	// Plugin data directory.
	home, _ := os.UserHomeDir()
	if home != "" {
		dataDir := filepath.Join(home, ".claude", "plugin-data", plugin.Name)
		result = strings.ReplaceAll(result, "${CLAUDE_PLUGIN_DATA}", dataDir)
	}

	// User config values.
	if plugin.UserConfigValues != nil {
		for k, v := range plugin.UserConfigValues {
			placeholder := "${user_config." + k + "}"
			val := fmt.Sprintf("%v", v)
			result = strings.ReplaceAll(result, placeholder, val)
		}
	}

	return result
}
