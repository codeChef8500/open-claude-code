// Package toolset provides the default set of tools for the agent engine.
// Both the CLI bootstrap path and the public SDK use this to ensure
// consistent tool registration.
package toolset

import (
	"runtime"

	"github.com/wall-ai/agent-engine/internal/skill"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/tool/agentool"
	"github.com/wall-ai/agent-engine/internal/tool/askuser"
	"github.com/wall-ai/agent-engine/internal/tool/bash"
	"github.com/wall-ai/agent-engine/internal/tool/brief"
	"github.com/wall-ai/agent-engine/internal/tool/cron"
	"github.com/wall-ai/agent-engine/internal/tool/fileedit"
	"github.com/wall-ai/agent-engine/internal/tool/fileread"
	"github.com/wall-ai/agent-engine/internal/tool/filewrite"
	"github.com/wall-ai/agent-engine/internal/tool/glob"
	"github.com/wall-ai/agent-engine/internal/tool/grep"
	"github.com/wall-ai/agent-engine/internal/tool/listpeers"
	"github.com/wall-ai/agent-engine/internal/tool/notebookedit"
	"github.com/wall-ai/agent-engine/internal/tool/planmode"
	"github.com/wall-ai/agent-engine/internal/tool/powershell"
	"github.com/wall-ai/agent-engine/internal/tool/sendmessage"
	"github.com/wall-ai/agent-engine/internal/tool/sleep"
	"github.com/wall-ai/agent-engine/internal/tool/taskcreate"
	"github.com/wall-ai/agent-engine/internal/tool/taskget"
	"github.com/wall-ai/agent-engine/internal/tool/tasklist"
	"github.com/wall-ai/agent-engine/internal/tool/taskstop"
	"github.com/wall-ai/agent-engine/internal/tool/taskupdate"
	"github.com/wall-ai/agent-engine/internal/tool/teamcreate"
	"github.com/wall-ai/agent-engine/internal/tool/teamdelete"
	"github.com/wall-ai/agent-engine/internal/tool/todo"
	"github.com/wall-ai/agent-engine/internal/tool/webfetch"
	"github.com/wall-ai/agent-engine/internal/tool/websearch"
	"github.com/wall-ai/agent-engine/internal/tool/worktree"
)

// SkillRegistryOption optionally injects a pre-populated skill registry.
// When nil, an empty registry is used (skills are added later by the caller).
type SkillRegistryOption = *skill.Registry

// DefaultTools returns the standard set of tools, using the given sub-agent
// runner. Pass nil for runner to disable sub-agent spawning (e.g. in child
// agents to prevent infinite recursion).
// skillReg may be nil; the SkillTool will be registered with an empty registry
// and callers can add skills before the first query.
func DefaultTools(runner agentool.SubAgentRunner, skillRegs ...SkillRegistryOption) []tool.Tool {
	var reg *skill.Registry
	if len(skillRegs) > 0 && skillRegs[0] != nil {
		reg = skillRegs[0]
	} else {
		reg = skill.NewRegistry()
	}
	tools := []tool.Tool{
		// Core file + shell tools
		fileread.New(),
		fileedit.New(),
		filewrite.New(),
		grep.New(),
		glob.New(),
		// Web tools
		webfetch.New(),
		websearch.New("", ""),
		// Interaction tools
		askuser.New(),
		todo.New(),
		sendmessage.New(),
		sleep.New(),
		taskstop.New(),
		// Background task management
		taskcreate.New(),
		taskget.New(),
		tasklist.New(),
		taskupdate.New(),
		// Notebook / document
		notebookedit.New(),
		// Agent coordination
		brief.New(),
		agentool.New(runner),
		// Plan mode
		planmode.NewEnterPlanMode(),
		planmode.NewExitPlanMode(),
		// Scheduled tasks
		cron.New(),
		// Team / multi-agent
		teamcreate.New(),
		teamdelete.New(),
		listpeers.New(),
		// Git worktree
		worktree.NewEnter(),
		worktree.NewExit(),
		worktree.NewList(),
		// Skills (unified SkillTool from internal/skill)
		skill.NewSkillTool(reg),
	}

	// Register platform-appropriate shell tool.
	if runtime.GOOS == "windows" {
		tools = append(tools, powershell.New())
	}
	tools = append(tools, bash.New())

	return tools
}
