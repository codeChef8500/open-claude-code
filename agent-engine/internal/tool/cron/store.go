package cron

import (
	"sync"

	"github.com/wall-ai/agent-engine/internal/daemon"
	"github.com/wall-ai/agent-engine/internal/state"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ─── Global cron store singleton ─────────────────────────────────────────────

var (
	globalStore     *daemon.CronTaskStore
	globalStoreMu   sync.Mutex
	globalStoreOnce sync.Once
)

// SetGlobalCronStore sets the global CronTaskStore used by cron tools.
// Called once during daemon/session initialization.
func SetGlobalCronStore(s *daemon.CronTaskStore) {
	globalStoreMu.Lock()
	defer globalStoreMu.Unlock()
	globalStore = s
}

// getCronStore returns the CronTaskStore from the global singleton.
func getCronStore(_ *tool.UseContext) *daemon.CronTaskStore {
	globalStoreMu.Lock()
	defer globalStoreMu.Unlock()
	return globalStore
}

// ─── AppState helpers ────────────────────────────────────────────────────────

// getAppState extracts the *state.AppState from UseContext.GetAppState().
func getAppState(uctx *tool.UseContext) *state.AppState {
	if uctx == nil || uctx.GetAppState == nil {
		return nil
	}
	v := uctx.GetAppState()
	if as, ok := v.(*state.AppState); ok {
		return as
	}
	return nil
}

// isKairosOrSchedulingEnabled checks if KAIROS or scheduling is enabled.
func isKairosOrSchedulingEnabled(uctx *tool.UseContext) bool {
	as := getAppState(uctx)
	if as == nil {
		return false
	}
	return as.KairosActive || as.ScheduledTasksEnabled
}

// getTeamAgentID returns the team agent ID if in a team context.
func getTeamAgentID(uctx *tool.UseContext) string {
	as := getAppState(uctx)
	if as == nil || as.TeamContext == nil {
		return ""
	}
	return as.TeamContext.SelfAgentID
}

// isTeammate returns true if the current context is a teammate.
func isTeammate(uctx *tool.UseContext) bool {
	return getTeamAgentID(uctx) != ""
}
