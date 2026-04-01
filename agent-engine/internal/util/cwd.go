package util

import (
	"sync"
)

var (
	cwdMu       sync.Mutex
	globalCwd   string
	agentCwdKey = make(map[string]string) // agentID -> cwd override
)

// SetGlobalCwd sets the process-wide current working directory used when
// no per-agent override is set.
func SetGlobalCwd(cwd string) {
	cwdMu.Lock()
	defer cwdMu.Unlock()
	globalCwd = cwd
}

// GetGlobalCwd returns the process-wide current working directory.
func GetGlobalCwd() string {
	cwdMu.Lock()
	defer cwdMu.Unlock()
	return globalCwd
}

// SetAgentCwd sets a per-agent CWD override (used for worktree isolation).
func SetAgentCwd(agentID, cwd string) {
	cwdMu.Lock()
	defer cwdMu.Unlock()
	agentCwdKey[agentID] = cwd
}

// ClearAgentCwd removes the per-agent CWD override.
func ClearAgentCwd(agentID string) {
	cwdMu.Lock()
	defer cwdMu.Unlock()
	delete(agentCwdKey, agentID)
}

// GetCwd returns the effective CWD for the given agentID.
// If agentID is empty or has no override, returns the global CWD.
func GetCwd(agentID string) string {
	cwdMu.Lock()
	defer cwdMu.Unlock()
	if agentID != "" {
		if override, ok := agentCwdKey[agentID]; ok {
			return override
		}
	}
	return globalCwd
}

// RunWithCwdOverride temporarily overrides the global CWD for the duration of
// fn and restores it afterwards, even if fn panics.
// agentID="" targets the global CWD.
func RunWithCwdOverride(agentID, newCwd string, fn func() error) error {
	cwdMu.Lock()
	prev := globalCwd
	if agentID != "" {
		prev = agentCwdKey[agentID]
		agentCwdKey[agentID] = newCwd
	} else {
		globalCwd = newCwd
	}
	cwdMu.Unlock()

	defer func() {
		cwdMu.Lock()
		if agentID != "" {
			if prev == "" {
				delete(agentCwdKey, agentID)
			} else {
				agentCwdKey[agentID] = prev
			}
		} else {
			globalCwd = prev
		}
		cwdMu.Unlock()
	}()

	return fn()
}
