package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Plugin command system deep implementation.
// Aligned with claude-code-main commands/plugin/plugin.tsx.
//
// Subcommands: list, install, uninstall, enable, disable, reload
// Interactive mode returns structured data for TUI plugin management panel.
// ──────────────────────────────────────────────────────────────────────────────

// PluginPanelData is the structured data for the plugin management TUI component.
type PluginPanelData struct {
	Subcommand string           `json:"subcommand"` // "list", "install", "uninstall", "enable", "disable", "reload"
	Plugins    []PluginViewData `json:"plugins,omitempty"`
	// For install/uninstall subcommands
	TargetPlugin string `json:"target_plugin,omitempty"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
}

// PluginViewData is a display-friendly view of a plugin.
type PluginViewData struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"` // "local", "registry", "git"
	Commands    int    `json:"commands"`
	Tools       int    `json:"tools"`
}

// DeepPluginCommand replaces the basic PluginCommand with full logic.
type DeepPluginCommand struct{ BaseCommand }

func (c *DeepPluginCommand) Name() string        { return "plugin" }
func (c *DeepPluginCommand) Description() string { return "Manage plugins" }
func (c *DeepPluginCommand) ArgumentHint() string {
	return "[list|install|uninstall|enable|disable|reload]"
}
func (c *DeepPluginCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepPluginCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepPluginCommand) ExecuteInteractive(ctx context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &PluginPanelData{Subcommand: "list"}

	if len(args) > 0 {
		data.Subcommand = strings.ToLower(args[0])
	}

	svc := getPluginService(ectx)

	switch data.Subcommand {
	case "list", "":
		data.Subcommand = "list"
		data.Plugins = listPlugins(svc)

	case "install", "add":
		data.Subcommand = "install"
		if len(args) < 2 {
			data.Error = "Usage: /plugin install <plugin-name-or-url>"
		} else {
			data.TargetPlugin = args[1]
			if svc != nil {
				if err := svc.InstallPlugin(ctx, data.TargetPlugin); err != nil {
					data.Error = fmt.Sprintf("Failed to install: %v", err)
				} else {
					data.Message = fmt.Sprintf("Plugin '%s' installed successfully", data.TargetPlugin)
					data.Plugins = listPlugins(svc)
				}
			} else {
				data.Error = "Plugin service not available"
			}
		}

	case "uninstall", "remove", "rm":
		data.Subcommand = "uninstall"
		if len(args) < 2 {
			data.Error = "Usage: /plugin uninstall <plugin-name>"
		} else {
			data.TargetPlugin = args[1]
			if svc != nil {
				if err := svc.UninstallPlugin(data.TargetPlugin); err != nil {
					data.Error = fmt.Sprintf("Failed to uninstall: %v", err)
				} else {
					data.Message = fmt.Sprintf("Plugin '%s' uninstalled", data.TargetPlugin)
					data.Plugins = listPlugins(svc)
				}
			} else {
				data.Error = "Plugin service not available"
			}
		}

	case "enable":
		if len(args) < 2 {
			data.Error = "Usage: /plugin enable <plugin-name>"
		} else {
			data.TargetPlugin = args[1]
			if svc != nil {
				if err := svc.EnablePlugin(data.TargetPlugin); err != nil {
					data.Error = fmt.Sprintf("Failed to enable: %v", err)
				} else {
					data.Message = fmt.Sprintf("Plugin '%s' enabled", data.TargetPlugin)
					data.Plugins = listPlugins(svc)
				}
			} else {
				data.Error = "Plugin service not available"
			}
		}

	case "disable":
		if len(args) < 2 {
			data.Error = "Usage: /plugin disable <plugin-name>"
		} else {
			data.TargetPlugin = args[1]
			if svc != nil {
				if err := svc.DisablePlugin(data.TargetPlugin); err != nil {
					data.Error = fmt.Sprintf("Failed to disable: %v", err)
				} else {
					data.Message = fmt.Sprintf("Plugin '%s' disabled", data.TargetPlugin)
					data.Plugins = listPlugins(svc)
				}
			} else {
				data.Error = "Plugin service not available"
			}
		}

	case "reload":
		if svc != nil {
			if _, err := svc.ReloadAll(); err != nil {
				data.Error = fmt.Sprintf("Failed to reload: %v", err)
			} else {
				data.Message = "All plugins reloaded"
				data.Plugins = listPlugins(svc)
			}
		} else {
			data.Error = "Plugin service not available"
		}

	default:
		data.Subcommand = "list"
		data.Plugins = listPlugins(svc)
	}

	return &InteractiveResult{
		Component: "plugin",
		Data:      data,
	}, nil
}

// getPluginService extracts PluginService from ExecContext safely.
func getPluginService(ectx *ExecContext) PluginService {
	if ectx == nil || ectx.Services == nil {
		return nil
	}
	return ectx.Services.Plugin
}

// listPlugins builds the plugin list view from available services.
func listPlugins(svc PluginService) []PluginViewData {
	if svc == nil {
		return nil
	}

	plugins := svc.ListPlugins()
	views := make([]PluginViewData, len(plugins))
	for i, p := range plugins {
		views[i] = PluginViewData{
			Name:        p.Name,
			Version:     p.Version,
			Description: "",
			Enabled:     p.Enabled,
			Source:      p.Repository,
			Commands:    len(svc.GetPluginCommands()),
			Tools:       0,
		}
	}
	return views
}

// ─── /reload-plugins deep implementation ────────────────────────────────────
// Aligned with claude-code-main commands/reload-plugins/index.ts.

// DeepReloadPluginsCommand replaces the basic ReloadPluginsCommand.
type DeepReloadPluginsCommand struct{ BaseCommand }

func (c *DeepReloadPluginsCommand) Name() string                  { return "reload-plugins" }
func (c *DeepReloadPluginsCommand) Description() string           { return "Reload all plugins" }
func (c *DeepReloadPluginsCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DeepReloadPluginsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepReloadPluginsCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	svc := getPluginService(ectx)
	if svc == nil {
		return "Plugin service not available.", nil
	}

	if _, err := svc.ReloadAll(); err != nil {
		return fmt.Sprintf("Failed to reload plugins: %v", err), nil
	}

	plugins := svc.ListPlugins()
	return fmt.Sprintf("Reloaded %d plugin(s).", len(plugins)), nil
}

// ─── /skills deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/skills/skills.tsx.

// SkillsViewData is the structured data for the skills TUI component.
type SkillsViewData struct {
	Skills  []SkillViewEntry `json:"skills,omitempty"`
	Message string           `json:"message,omitempty"`
}

// SkillViewEntry is a display-friendly skill.
type SkillViewEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"` // "builtin", "plugin", "mcp"
	Enabled     bool   `json:"enabled"`
}

// DeepSkillsCommand replaces the basic SkillsCommand.
type DeepSkillsCommand struct{ BaseCommand }

func (c *DeepSkillsCommand) Name() string                  { return "skills" }
func (c *DeepSkillsCommand) Description() string           { return "List available skills" }
func (c *DeepSkillsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepSkillsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepSkillsCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &SkillsViewData{}

	if ectx != nil && ectx.Services != nil && ectx.Services.Skill != nil {
		for _, source := range []struct {
			name string
			cmds []Command
		}{
			{"builtin", ectx.Services.Skill.GetBundledSkills()},
			{"dynamic", ectx.Services.Skill.GetDynamicSkills()},
		} {
			for _, cmd := range source.cmds {
				data.Skills = append(data.Skills, SkillViewEntry{
					Name:        cmd.Name(),
					Description: cmd.Description(),
					Source:      source.name,
					Enabled:     cmd.IsEnabled(ectx),
				})
			}
		}
	}

	if len(data.Skills) == 0 {
		data.Message = "No skills available."
	}

	return &InteractiveResult{
		Component: "skills",
		Data:      data,
	}, nil
}

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepPluginCommand{},
		&DeepReloadPluginsCommand{},
		&DeepSkillsCommand{},
	)
}
