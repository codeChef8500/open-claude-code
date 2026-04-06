package command

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// /clear — full implementation
// Aligned with claude-code-main commands/clear/conversation.ts + caches.ts.
//
// The clear command performs a complete session reset:
//  1. Execute SessionEnd hooks (with timeout)
//  2. Send cache eviction analytics hint
//  3. Compute preserved background task agent IDs
//  4. Clear conversation messages
//  5. Regenerate conversation ID
//  6. Clear all session caches (~25 categories)
//  7. Reset working directory to original
//  8. Clear AppState (tasks, attribution, file history, MCP state)
//  9. Regenerate session ID
// 10. Reset session file pointer
// 11. Relink preserved task outputs
// 12. Save mode and worktree state
// 13. Execute SessionStart hooks
// ──────────────────────────────────────────────────────────────────────────────

// ClearConversation performs the full conversation clear sequence.
// This is the Go equivalent of claude-code-main's clearConversation().
// Returns a status message describing what was done.
func ClearConversation(ctx context.Context, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "__clear_history__", nil
	}

	svc := ectx.Services
	if svc == nil {
		// Fallback: no services available, just signal clear.
		return "__clear_history__", nil
	}

	var steps []string

	// 1. Execute SessionEnd hooks (timeout 1.5s)
	if svc.Hook != nil {
		hookCtx, hookCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		err := svc.Hook.ExecuteSessionEndHooks(hookCtx)
		hookCancel()
		if err != nil {
			steps = append(steps, fmt.Sprintf("SessionEnd hooks: %v", err))
		} else {
			steps = append(steps, "SessionEnd hooks executed")
		}
	}

	// 2. Send cache eviction analytics hint
	if svc.Analytics != nil {
		svc.Analytics.SendCacheEvictionHint()
	}

	// 3. Compute preserved background task agent IDs
	var preservedAgentIDs []string
	if svc.Task != nil {
		preservedAgentIDs = svc.Task.GetPreservedAgentIDs()
	}

	// 4. Clear conversation messages
	if ectx.SetMessages != nil {
		ectx.SetMessages(func(_ interface{}) interface{} {
			// Return empty slice — concrete type depends on engine.
			return []interface{}{}
		})
		steps = append(steps, "Messages cleared")
	}

	// 5. Regenerate conversation ID
	if svc.Session != nil {
		newConvID := svc.Session.RegenerateConversationID()
		steps = append(steps, fmt.Sprintf("New conversation: %s", truncateID(newConvID)))
	}

	// 6. Clear all session caches
	if svc.Cache != nil {
		svc.Cache.ClearSessionCaches(preservedAgentIDs)
		steps = append(steps, "Session caches cleared")
	}

	// 7. Reset working directory to original
	if svc.AppState != nil {
		svc.AppState.ResetCWD()
	}

	// 8. Clear AppState
	if svc.AppState != nil {
		svc.AppState.ClearFileHistory()
		svc.AppState.ClearAttribution()
		svc.AppState.ClearMCPState()
		svc.AppState.ClearPlanSlug()
		svc.AppState.ClearSessionMetadata()
		steps = append(steps, "AppState reset")
	}

	// 9. Regenerate session ID
	if svc.Session != nil {
		newSessID := svc.Session.RegenerateSessionID()
		steps = append(steps, fmt.Sprintf("New session: %s", truncateID(newSessID)))
	}

	// 10. Reset session file pointer
	if svc.Session != nil {
		svc.Session.ResetSessionFilePointer()
	}

	// 11. Relink preserved task outputs
	if svc.Task != nil && len(preservedAgentIDs) > 0 {
		if err := svc.Task.RelinkTaskOutputs(preservedAgentIDs); err != nil {
			steps = append(steps, fmt.Sprintf("Task relink warning: %v", err))
		}
	}

	// 12. Save mode and worktree state
	if svc.AppState != nil {
		svc.AppState.SaveModeState()
		svc.AppState.SaveWorktreeState()
	}

	// 13. Execute SessionStart hooks
	if svc.Hook != nil {
		if err := svc.Hook.ExecuteSessionStartHooks(ctx); err != nil {
			steps = append(steps, fmt.Sprintf("SessionStart hooks: %v", err))
		} else {
			steps = append(steps, "SessionStart hooks executed")
		}
	}

	// Also clear the command cache in the registry.
	if svc.Cache != nil {
		svc.Cache.ClearCommandCache()
		svc.Cache.ClearSkillCache()
		svc.Cache.ClearDynamicSkills()
	}

	if len(steps) == 0 {
		return "__clear_history__", nil
	}

	// Return both the signal and a summary.
	return "__clear_history__\n" + strings.Join(steps, "\n"), nil
}

// truncateID returns the first 8 chars of an ID for display.
func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8] + "..."
	}
	return id
}

// ──────────────────────────────────────────────────────────────────────────────
// ClearSessionCaches — detailed cache clearing implementation.
// Aligned with claude-code-main commands/clear/caches.ts clearSessionCaches().
//
// This lists every cache category that needs clearing. The CacheService
// implementation should clear all of these when ClearSessionCaches is called.
// ──────────────────────────────────────────────────────────────────────────────

// CacheCategoryList documents the ~25 cache categories cleared during /clear.
// These align with claude-code-main's clearSessionCaches().
var CacheCategoryList = []string{
	"user_context",                // User context cache
	"system_context",              // System context cache
	"git_status",                  // Git status cache
	"file_suggestions",            // File suggestion cache
	"command_list",                // Command/skill list memoization
	"prompt_cache_break",          // Prompt cache break detection
	"system_prompt_injections",    // System prompt injection cache
	"post_compact_cleanup",        // Post-compaction cleanup state
	"stored_image_paths",          // Stored image paths
	"session_ingress",             // Session ingress cache
	"swarm_permission_callbacks",  // Swarm permission callbacks
	"repo_detection",              // Repository detection cache
	"bash_command_prefix",         // Bash command prefix cache
	"dump_prompts_state",          // Dump prompts state
	"invoked_skills",              // Invoked skills cache
	"git_dir_resolution",          // Git dir resolution cache
	"dynamic_skills",              // Dynamically discovered skills
	"lsp_diagnostics",             // LSP diagnostics state
	"magic_docs_tracking",         // Magic docs tracking
	"session_env_vars",            // Session environment variables
	"webfetch_url",                // WebFetch URL cache
	"toolsearch_descriptions",     // ToolSearch description cache
	"agent_definitions",           // Agent definition cache
	"skilltool_prompt",            // SkillTool prompt cache
	"skill_index",                 // Skill search index (if experimental)
}
