package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Manager handles plugin discovery, loading, and lifecycle management.
type Manager struct {
	mu       sync.RWMutex
	registry *Registry
	hooks    *HookEngine
	dirs     []string // directories to scan for plugins
}

// NewManager creates a plugin manager with the given search directories.
func NewManager(searchDirs ...string) *Manager {
	return &Manager{
		registry: NewRegistry(),
		hooks:    NewHookEngine(),
		dirs:     searchDirs,
	}
}

// DefaultPluginDirs returns the standard plugin search paths.
func DefaultPluginDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, ".claude", "plugins"),
		filepath.Join(".", ".claude", "plugins"),
	}
	return dirs
}

// Registry returns the underlying plugin registry.
func (m *Manager) Registry() *Registry { return m.registry }

// HookEngine returns the underlying hook engine.
func (m *Manager) HookEngine() *HookEngine { return m.hooks }

// DiscoverAndLoad scans all configured directories for plugin binaries,
// loads them, and registers them. Returns the number loaded and any errors.
func (m *Manager) DiscoverAndLoad() (int, []error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var loaded int
	var errs []error

	for _, dir := range m.dirs {
		binaries, err := discoverPluginBinaries(dir)
		if err != nil {
			continue
		}
		for _, bin := range binaries {
			p, err := LoadPlugin(bin)
			if err != nil {
				errs = append(errs, fmt.Errorf("load %s: %w", bin, err))
				slog.Warn("plugin load failed", slog.String("path", bin), slog.Any("error", err))
				continue
			}
			if err := m.registry.Register(p); err != nil {
				p.Close()
				errs = append(errs, err)
				continue
			}
			slog.Info("plugin loaded", slog.String("name", p.Name()), slog.String("path", bin))
			loaded++
		}
	}
	return loaded, errs
}

// LoadFromPath loads a single plugin from a specific binary path.
func (m *Manager) LoadFromPath(path string) error {
	p, err := LoadPlugin(path)
	if err != nil {
		return err
	}
	if err := m.registry.Register(p); err != nil {
		p.Close()
		return err
	}
	slog.Info("plugin loaded", slog.String("name", p.Name()), slog.String("path", path))
	return nil
}

// Unload removes and kills a plugin by name.
func (m *Manager) Unload(name string) error {
	return m.registry.Unregister(name)
}

// ListPlugins returns info about all loaded plugins.
func (m *Manager) ListPlugins() []PluginInfo {
	plugins := m.registry.All()
	infos := make([]PluginInfo, len(plugins))
	for i, p := range plugins {
		infos[i] = PluginInfo{
			Name:        p.meta.Name,
			Description: p.meta.Description,
			Version:     p.meta.Version,
			Binary:      p.binary,
		}
	}
	return infos
}

// Close shuts down all plugins.
func (m *Manager) Close() {
	m.registry.CloseAll()
}

// PluginInfo describes a loaded plugin.
type PluginInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Binary      string `json:"binary"`
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
		// Skip hidden files and common non-plugin files.
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		binaries = append(binaries, filepath.Join(dir, name))
	}
	return binaries, nil
}
