package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/wall-ai/agent-engine/internal/skill"
)

// Manager handles plugin discovery, loading, and lifecycle management.
// It unifies external binary plugins, manifest-based plugins, and builtin plugins.
type Manager struct {
	mu sync.RWMutex

	// External binary plugin registry (go-plugin).
	externalRegistry *Registry
	// Manifest-based loaded plugins.
	loadedPlugins []*LoadedPlugin
	// Plugin hook registry for manifest plugins.
	pluginHookRegistry *PluginHookRegistry

	// Shared hook engine (receives hooks from all sources).
	hooks *HookEngine
	// Directories to scan for binary plugins.
	binaryDirs []string
	// Directories to scan for manifest-based plugins.
	manifestDirs []string
	// Plugin enabled state (name → enabled).
	enabledMap map[string]bool
}

// NewManager creates a unified plugin manager.
func NewManager(binaryDirs, manifestDirs []string) *Manager {
	return &Manager{
		externalRegistry:   NewRegistry(),
		pluginHookRegistry: NewPluginHookRegistry(),
		hooks:              NewHookEngine(),
		binaryDirs:         binaryDirs,
		manifestDirs:       manifestDirs,
		enabledMap:         make(map[string]bool),
	}
}

// DefaultPluginDirs returns the standard plugin search paths.
func DefaultPluginDirs() (binaryDirs, manifestDirs []string) {
	home, _ := os.UserHomeDir()
	if home != "" {
		binaryDirs = append(binaryDirs, filepath.Join(home, ".claude", "plugins"))
		manifestDirs = append(manifestDirs, filepath.Join(home, ".claude", "plugins"))
	}
	binaryDirs = append(binaryDirs, filepath.Join(".", ".claude", "plugins"))
	manifestDirs = append(manifestDirs, filepath.Join(".", ".claude", "plugins"))
	return
}

// ExternalRegistry returns the underlying external plugin registry.
func (m *Manager) ExternalRegistry() *Registry { return m.externalRegistry }

// HookEngine returns the shared hook engine.
func (m *Manager) HookEngine() *HookEngine { return m.hooks }

// SetEnabled sets the enabled state for a plugin.
func (m *Manager) SetEnabled(name string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabledMap[name] = enabled
}

// DiscoverAndLoad discovers and loads all plugin types:
// 1. Builtin plugins (hooks)
// 2. External binary plugins
// 3. Manifest-based plugins
func (m *Manager) DiscoverAndLoad() (int, []error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var totalLoaded int
	var allErrs []error

	// 1. Register builtin hooks.
	RegisterBuiltinHooks(m.hooks)

	// 2. Load external binary plugins.
	for _, dir := range m.binaryDirs {
		binaries, err := discoverPluginBinaries(dir)
		if err != nil {
			continue
		}
		for _, bin := range binaries {
			p, err := LoadPlugin(bin)
			if err != nil {
				allErrs = append(allErrs, fmt.Errorf("load binary %s: %w", bin, err))
				slog.Warn("binary plugin load failed", slog.String("path", bin), slog.Any("error", err))
				continue
			}
			if err := m.externalRegistry.Register(p); err != nil {
				p.Close()
				allErrs = append(allErrs, err)
				continue
			}
			slog.Info("binary plugin loaded", slog.String("name", p.Name()), slog.String("path", bin))
			totalLoaded++
		}
	}

	// 3. Load manifest-based plugins.
	result := LoadAllPlugins(m.manifestDirs, m.enabledMap)
	m.loadedPlugins = append(result.Enabled, result.Disabled...)

	for _, pe := range result.Errors {
		allErrs = append(allErrs, pe)
	}

	// Register hooks from enabled manifest plugins.
	for _, lp := range result.Enabled {
		if err := m.pluginHookRegistry.RegisterPluginHooks(lp, m.hooks); err != nil {
			slog.Warn("plugin hook registration failed",
				slog.String("plugin", lp.Name), slog.Any("err", err))
		}
		totalLoaded++
	}

	return totalLoaded, allErrs
}

// LoadFromPath loads a single external binary plugin from a specific path.
func (m *Manager) LoadFromPath(path string) error {
	p, err := LoadPlugin(path)
	if err != nil {
		return err
	}
	if err := m.externalRegistry.Register(p); err != nil {
		p.Close()
		return err
	}
	slog.Info("binary plugin loaded", slog.String("name", p.Name()), slog.String("path", path))
	return nil
}

// LoadManifestPlugin loads a single manifest-based plugin from a directory.
func (m *Manager) LoadManifestPlugin(dir string, source PluginSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lp, err := LoadPluginFromDir(dir, source)
	if err != nil {
		return err
	}
	m.loadedPlugins = append(m.loadedPlugins, lp)

	if lp.Enabled {
		if err := m.pluginHookRegistry.RegisterPluginHooks(lp, m.hooks); err != nil {
			slog.Warn("plugin hook registration failed",
				slog.String("plugin", lp.Name), slog.Any("err", err))
		}
	}

	slog.Info("manifest plugin loaded", slog.String("name", lp.Name), slog.String("path", dir))
	return nil
}

// Unload removes and kills a binary plugin by name.
func (m *Manager) Unload(name string) error {
	// Try external first.
	if err := m.externalRegistry.Unregister(name); err == nil {
		return nil
	}
	// Remove from manifest plugins.
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, lp := range m.loadedPlugins {
		if lp.Name == name {
			m.pluginHookRegistry.ClearPluginHooks(name)
			m.loadedPlugins = append(m.loadedPlugins[:i], m.loadedPlugins[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("plugin %q not found", name)
}

// ReloadPlugins clears caches and re-discovers all plugins.
func (m *Manager) ReloadPlugins() (int, []error) {
	m.Close()
	m.mu.Lock()
	m.loadedPlugins = nil
	m.hooks = NewHookEngine()
	m.pluginHookRegistry = NewPluginHookRegistry()
	m.externalRegistry = NewRegistry()
	m.mu.Unlock()
	return m.DiscoverAndLoad()
}

// GetAllSkills returns skills from all plugin sources: builtin plugins, manifest plugin
// commands, and manifest plugin skills.
func (m *Manager) GetAllSkills() []*skill.Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var skills []*skill.Skill

	// Builtin plugin skills.
	skills = append(skills, GetBuiltinPluginSkills(m.enabledMap)...)

	// Manifest plugin commands and skills.
	for _, lp := range m.loadedPlugins {
		if !lp.Enabled {
			continue
		}

		cmds, err := LoadPluginCommands(lp)
		if err == nil {
			skills = append(skills, cmds...)
		}

		sk, err := LoadPluginSkills(lp)
		if err == nil {
			skills = append(skills, sk...)
		}
	}

	return skills
}

// GetEnabledPlugins returns all enabled loaded plugins.
func (m *Manager) GetEnabledPlugins() []*LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var enabled []*LoadedPlugin
	for _, lp := range m.loadedPlugins {
		if lp.Enabled {
			enabled = append(enabled, lp)
		}
	}
	return enabled
}

// ListPlugins returns info about all loaded plugins (binary + manifest + builtin).
func (m *Manager) ListPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []PluginInfo

	// External binary plugins.
	for _, p := range m.externalRegistry.All() {
		infos = append(infos, PluginInfo{
			Name:        p.meta.Name,
			Description: p.meta.Description,
			Version:     p.meta.Version,
			Binary:      p.binary,
			Source:      string(SourceExternal),
			Enabled:     true,
		})
	}

	// Manifest-based plugins.
	for _, lp := range m.loadedPlugins {
		desc := ""
		ver := ""
		if lp.Manifest != nil {
			desc = lp.Manifest.Description
			ver = lp.Manifest.Version
		}
		infos = append(infos, PluginInfo{
			Name:        lp.Name,
			Description: desc,
			Version:     ver,
			Path:        lp.Path,
			Source:      string(lp.Source),
			Enabled:     lp.Enabled,
		})
	}

	// Builtin plugins.
	builtinDefsMu.RLock()
	for _, def := range builtinDefs {
		isEnabled := def.DefaultEnabled
		if v, ok := m.enabledMap[def.Name]; ok {
			isEnabled = v
		}
		infos = append(infos, PluginInfo{
			Name:        def.Name,
			Description: def.Description,
			Version:     def.Version,
			Source:      string(SourceBuiltin),
			Enabled:     isEnabled,
		})
	}
	builtinDefsMu.RUnlock()

	return infos
}

// GetPluginInfo returns info about a specific plugin by name.
func (m *Manager) GetPluginInfo(name string) *PluginInfo {
	for _, info := range m.ListPlugins() {
		if info.Name == name {
			return &info
		}
	}
	return nil
}

// Close shuts down all plugins.
func (m *Manager) Close() {
	m.externalRegistry.CloseAll()
}

// PluginInfo describes a loaded plugin.
type PluginInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Binary      string `json:"binary,omitempty"`
	Path        string `json:"path,omitempty"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
}

// discoverPluginBinaries finds executable files in a directory.
func discoverPluginBinaries(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var binaries []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// On Windows, look for .exe files; on Unix, check executable bit.
		if runtime.GOOS == "windows" {
			if !strings.HasSuffix(strings.ToLower(name), ".exe") {
				continue
			}
		} else {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.Mode()&0o111 == 0 {
				continue
			}
		}
		// Skip hidden files, common non-plugin files, and plugin.json.
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		if name == "plugin.json" {
			continue
		}
		binaries = append(binaries, filepath.Join(dir, name))
	}
	return binaries, nil
}
