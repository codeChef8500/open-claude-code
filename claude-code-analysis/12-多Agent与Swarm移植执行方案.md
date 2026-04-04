# 多Agent协调与Swarm完整移植执行方案

> 目标：将 `claude-code-main` (TypeScript) 的全部多Agent协调与Swarm功能1:1复制到 `agent-engine` (Go)

## 一、现状差异分析

### 1.1 agent-engine 已有基础设施

| 模块 | 文件 | 状态 |
|------|------|------|
| AgentDefinition/AgentTask/AgentMessage | `internal/agent/types.go` | ✅ 基础结构存在，需扩展 |
| SubagentContext | `internal/agent/subagent_context.go` | ✅ 权限继承+通信通道已有 |
| Coordinator | `internal/agent/coordinator.go` | ✅ 基本SpawnAgent/WaitAgent |
| Pool | `internal/agent/pool.go` | ✅ 有界并发池 |
| TaskFramework | `internal/agent/taskframework.go` | ✅ UI状态+淘汰机制 |
| TaskManager | `internal/agent/taskmanager.go` | ✅ 生命周期追踪 |
| Mailbox/MailboxRegistry | `internal/agent/mailbox.go` | ✅ 优先级+状态追踪 |
| MessageBus | `internal/agent/messaging.go` | ✅ 频道路由 |
| WorktreeManager | `internal/agent/worktree.go` | ✅ 基础创建/删除 |
| DiskOutput | `internal/agent/diskoutput.go` | ✅ 异步磁盘写入 |
| StallWatchdog | `internal/agent/stallwatchdog.go` | ✅ 停滞检测 |
| Color | `internal/agent/color.go` | ✅ ANSI颜色分配 |
| AgentTool (Task) | `internal/tool/agentool/` | ⚠️ 基础版，缺4路决策树 |
| SendMessageTool | `internal/tool/sendmessage/` | ⚠️ 基础版，缺邮箱文件系统 |
| Tool orchestration | `internal/tool/orchestration.go` | ✅ 带Hook的批量执行 |

### 1.2 需要新增/大幅扩展的模块

| 功能 | claude-code-main源文件 | 优先级 |
|------|----------------------|--------|
| Agent定义加载体系 | `loadAgentsDir.ts` | P0 |
| 内置Agent定义 | `builtInAgents.ts`, `built-in/*.ts` | P0 |
| Agent决策树(4路路由) | `AgentTool.tsx` L239-1262 | P0 |
| Fork Subagent | `forkSubagent.ts` | P0 |
| Agent运行核心 | `runAgent.ts` | P0 |
| 工具权限过滤 | `agentToolUtils.ts`, `constants/tools.ts` | P0 |
| 异步Agent生命周期 | `agentToolUtils.ts` runAsyncAgentLifecycle | P0 |
| Agent通知机制 | `agentToolUtils.ts` enqueueAgentNotification | P0 |
| Agent记忆系统 | `agentMemory.ts` | P1 |
| Coordinator模式 | `coordinatorMode.ts` | P1 |
| Agent恢复 | `resumeAgent.ts` | P1 |
| Agent专属MCP | `runAgent.ts` initializeAgentMcpServers | P1 |
| InProcess Teammate | `InProcessTeammateTask/` | P1 |
| TeamCreate/Delete工具 | `TeamCreateTool/`, `TeamDeleteTool/` | P1 |
| tmux Teammate | swarm backends | P2 |
| Remote Agent(CCR) | AgentTool remote isolation | P2 |
| Agent Prompt生成 | `prompt.ts` | P1 |
| 前台→后台转换 | AgentTool sync-to-background | P2 |
| Agent恢复磁盘转录 | `resumeAgent.ts` transcript | P2 |

---

## 二、执行阶段总览

```
Phase 1: Agent定义体系 + 工具权限过滤 (基础层)
Phase 2: Agent运行核心 + Fork机制 (执行层)
Phase 3: 异步生命周期 + 通知机制 (异步层)
Phase 4: AgentTool 4路决策树重构 (路由层)
Phase 5: Coordinator模式 + Agent记忆 (协调层)
Phase 6: InProcess Teammate + Team工具 (Swarm层)
Phase 7: tmux Teammate + Remote Agent (扩展层)
Phase 8: Agent恢复 + 前台转后台 (高级层)
```

---

## 三、Phase 1: Agent定义体系 + 工具权限过滤

### 3.1 扩展 AgentDefinition

**源码参考**: `loadAgentsDir.ts` BaseAgentDefinition

当前 `AgentDefinition` 只有9个字段，需扩展到完整对齐。

**新增字段** (在 `internal/agent/types.go`):

- `AgentType string` — agent类型名 ("general-purpose", "explore", "plan", 自定义)
- `Source string` — 来源 ("builtin", "custom", "plugin")
- `WhenToUse string` — 何时使用此agent的描述
- `Model string` — 模型覆盖
- `Effort string` — "low"/"medium"/"high"
- `Background bool` — 默认异步执行
- `Isolation string` — "worktree"/"remote"/""
- `DisallowedTools []string` — 禁止使用的工具
- `Skills []string` — 预加载技能
- `InitialPrompt string` — 初始提示词
- `OmitClaudeMd bool` — 省略CLAUDE.md以优化token
- `CriticalSystemReminder string` — 关键系统提醒
- `PermissionMode string` — "acceptEdits"/"bypass"等
- `Memory string` — "user"/"project"/"local"
- `McpServers []AgentMcpServerSpec` — agent专属MCP服务器
- `RequiredMcpServers []string` — 必需MCP服务器名称模式
- `Hooks map[string][]HookCommand` — agent级别hook

**新增类型**:

```go
type AgentMcpServerSpec struct {
    Name    string            `json:"name"`
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    URL     string            `json:"url,omitempty"`
}

type HookCommand struct {
    Command string `json:"command"`
    Timeout int    `json:"timeout,omitempty"`
}
```

**步骤**:
1. 扩展 `types.go` 中 `AgentDefinition`
2. 添加 `AgentMcpServerSpec`, `HookCommand`
3. 添加 `AgentDefinitionSource` 常量
4. 更新 coordinator.go, pool.go 等所有引用处

### 3.2 新建 Agent定义加载器

**新文件**: `internal/agent/loader.go`

**源码参考**: `loadAgentsDir.ts` — 从多源加载agent定义

**功能**:

```go
// AgentLoader 从多个源加载和合并agent定义
type AgentLoader struct {
    builtInAgents  []AgentDefinition
    customAgents   []AgentDefinition  // 从 .claude/agents/*.md 加载
    pluginAgents   []AgentDefinition  // 从 MCP server 加载
}

func NewAgentLoader() *AgentLoader
func (l *AgentLoader) LoadBuiltIn() []AgentDefinition
func (l *AgentLoader) LoadCustom(projectDir string) ([]AgentDefinition, error)
func (l *AgentLoader) LoadPlugin(mcpTools []string) []AgentDefinition
func (l *AgentLoader) MergeAll() []AgentDefinition  // 优先级: plugin > custom > builtin
func (l *AgentLoader) FindByType(agentType string) (*AgentDefinition, bool)
```

**Markdown Frontmatter 解析** — 自定义agent从 `.claude/agents/*.md` 加载:
```markdown
---
agent_type: my-reviewer
when_to_use: Use for code review tasks
tools: [Read, Grep, Glob]
disallowed_tools: [FileWrite]
model: sonnet
permission_mode: acceptEdits
memory: project
---
You are a code reviewer...
```

**步骤**:
1. 新建 `loader.go`
2. 实现 frontmatter YAML 解析 (使用 `gopkg.in/yaml.v3`)
3. 实现3源合并逻辑，plugin > custom > builtin 优先级
4. 添加 `FilterDeniedAgents()` — 根据权限规则过滤agent
5. 添加 `FilterAgentsByMcpRequirements()` — 根据MCP可用性过滤

### 3.3 内置Agent定义

**新文件**: `internal/agent/builtin_agents.go`

**源码参考**: `builtInAgents.ts`, `built-in/generalPurposeAgent.ts`, `exploreAgent.ts`, `planAgent.ts`

```go
var GeneralPurposeAgent = AgentDefinition{
    AgentType:  "general-purpose",
    Source:     SourceBuiltIn,
    WhenToUse:  "General-purpose coding agent for any task",
    MaxTurns:   200,
    // AllowedTools: nil = all tools
}

var ExploreAgent = AgentDefinition{
    AgentType:      "explore",
    Source:         SourceBuiltIn,
    WhenToUse:      "Read-only exploration and research",
    OmitClaudeMd:  true,
    AllowedTools:   []string{"Read", "Grep", "Glob", "Bash", "lsp"},
    DisallowedTools: nil,
}

var PlanAgent = AgentDefinition{
    AgentType:      "plan",
    Source:         SourceBuiltIn,
    WhenToUse:      "Planning and architecture discussion",
    OmitClaudeMd:  true,
    AllowedTools:   []string{"Read", "Grep", "Glob", "Bash"},
}

var ForkAgent = AgentDefinition{
    AgentType: "__fork__",
    Source:    SourceBuiltIn,
    WhenToUse: "Fork current context for parallel work",
    Background: true,
}

func GetBuiltInAgents(coordinatorMode bool) []AgentDefinition
```

### 3.4 工具权限过滤系统

**新文件**: `internal/agent/toolfilter.go`

**源码参考**: `agentToolUtils.ts` filterToolsForAgent, `constants/tools.ts`

```go
// 层级工具过滤常量 — 对齐 constants/tools.ts
var AllAgentDisallowedTools = map[string]bool{
    "TaskOutput":    true,
    "ExitPlanMode":  true,
    "EnterPlanMode": true,
    "Task":          true,  // 防止递归(非ant用户)
    "AskUserQuestion": true,
    "TaskStop":      true,
}

var AsyncAgentAllowedTools = map[string]bool{
    "Read": true, "WebSearch": true, "TodoWrite": true,
    "Grep": true, "WebFetch": true, "Glob": true,
    "Bash": true, "PowerShell": true,
    "FileEdit": true, "FileWrite": true,
    "NotebookEdit": true, "Skill": true,
    "SyntheticOutput": true, "ToolSearch": true,
    "EnterWorktree": true, "ExitWorktree": true,
}

var InProcessTeammateAllowedTools = map[string]bool{
    "TaskCreate": true, "TaskGet": true,
    "TaskList": true, "TaskUpdate": true,
    "SendMessage": true,
    "CronCreate": true, "CronDelete": true, "CronList": true,
}

var CoordinatorModeAllowedTools = map[string]bool{
    "Task": true, "TaskStop": true,
    "SendMessage": true, "SyntheticOutput": true,
}

// FilterToolsForAgent 根据agent类型和上下文过滤可用工具
func FilterToolsForAgent(
    allTools []string,
    agentDef *AgentDefinition,
    isAsync bool,
    isInProcessTeammate bool,
    isCoordinator bool,
) []string

// ResolveAgentTools 解析agent frontmatter中的tools/disallowedTools
func ResolveAgentTools(
    agentDef *AgentDefinition,
    availableTools []string,
) []string
```

**步骤**:
1. 新建 `toolfilter.go`
2. 定义4层过滤常量
3. 实现 `FilterToolsForAgent` — 分层应用过滤逻辑
4. 实现 `ResolveAgentTools` — 处理通配符展开和黑白名单合并
5. 单元测试覆盖各场景

---

## 四、Phase 2: Agent运行核心 + Fork机制

### 4.1 Agent运行器 (runAgent)

**新文件**: `internal/agent/runner.go`

**源码参考**: `runAgent.ts` — agent执行核心

`runAgent.ts` 是整个multi-agent系统的执行引擎，负责:
1. 生成agentId
2. 解析模型 (agent定义 → 用户覆盖 → 全局默认)
3. 执行 SubagentStart hooks
4. 注册frontmatter hooks
5. 预加载skills
6. 初始化agent专属MCP服务器
7. 构建 AgentOptions (隔离上下文)
8. 创建 ToolUseContext
9. 启动query loop并yield消息流

```go
// AgentRunner 执行一个agent任务
type AgentRunner struct {
    engine     *engine.Engine
    registry   *tool.Registry
    loader     *AgentLoader
    worktreeMgr *WorktreeManager
}

// RunAgentParams 对齐 runAgent.ts 的参数
type RunAgentParams struct {
    AgentDefinition   *AgentDefinition
    PromptMessages    []*engine.Message
    ParentContext     *SubagentContext
    IsAsync           bool
    QuerySource       string
    ModelOverride     string
    AvailableTools    []string
    WorktreePath      string
    Description       string
    // Fork专用
    ForkContextMessages []*engine.Message
    UseExactTools       bool
    OverrideSystemPrompt []string
    OverrideAgentID     string
    AbortCh             <-chan struct{}
}

// RunAgentResult 是agent运行的流式结果
type RunAgentResult struct {
    Messages chan *engine.Message  // 流式消息输出
    AgentID  string
    Error    error
}

func (r *AgentRunner) RunAgent(ctx context.Context, params RunAgentParams) *RunAgentResult
```

**核心逻辑** (对齐runAgent.ts初始化序列):

```
1. agentId = params.OverrideAgentID || generateAgentId()
2. resolvedModel = resolveAgentModel(def.Model, parentModel, params.ModelOverride)
3. executeHooks("SubagentStart", agentId, def)
4. if def.Hooks != nil { registerFrontmatterHooks(def.Hooks) }
5. if def.Skills != nil { preloadSkills(def.Skills) }
6. if def.McpServers != nil { initializeAgentMcpServers(def.McpServers) }
7. agentOptions = buildAgentOptions(params) // 隔离上下文
8. toolUseCtx = buildToolUseContext(agentOptions)
9. filteredTools = FilterToolsForAgent(params.AvailableTools, def, params.IsAsync, ...)
10. queryLoop = engine.NewQueryLoop(resolvedModel, filteredTools, systemPrompt)
11. for msg := range queryLoop.Run(ctx, params.PromptMessages) { yield msg }
12. cleanup: MCP disconnect, hooks("SubagentEnd"), skill cleanup
```

**关键设计决策**:
- Agent运行器是**独立Engine实例** — 每个agent拥有自己的query loop
- 通过 `SubagentContext.DeriveChild()` 创建子agent上下文
- 权限上下文隔离: 异步agent强制 `permissionMode = "acceptEdits"`, 避免阻塞prompt

**步骤**:
1. 新建 `runner.go`
2. 实现 `RunAgent` 方法 — 完整初始化序列
3. 实现 `resolveAgentModel()` — 模型解析优先级链
4. 实现 `buildAgentOptions()` — 构建隔离上下文
5. 实现 `buildToolUseContext()` — 创建工具执行上下文
6. 集成已有的 `engine.Engine` query loop

### 4.2 Fork Subagent

**新文件**: `internal/agent/fork.go`

**源码参考**: `forkSubagent.ts`

Fork是最复杂的agent类型，核心价值是**prompt cache共享** — 子agent继承父agent的完整对话历史，API请求前缀字节相同，从而复用缓存。

```go
// ForkConfig 控制fork行为
type ForkConfig struct {
    Enabled bool
}

// IsForkSubagentEnabled 检查fork功能是否启用
func IsForkSubagentEnabled() bool

// IsInForkChild 检查当前上下文是否在fork子进程中(防递归)
func IsInForkChild(messages []*engine.Message) bool

// BuildForkedMessages 构建fork子agent的消息序列
// 关键: 保持与父agent字节相同的API请求前缀
func BuildForkedMessages(
    directive string,            // fork子agent的任务指令
    parentAssistantMsg *engine.Message,  // 父agent当前的assistant消息
) []*engine.Message

// BuildChildMessage 构建fork子agent的directive消息
func BuildChildMessage(directive string) *engine.Message
```

**Fork消息构建逻辑** (对齐 `buildForkedMessages`):

```
输入: directive (任务), assistantMessage (父agent当前回复)

1. 克隆父agent的assistant消息(包含所有tool_use块)
2. 为每个tool_use块生成占位tool_result:
   - 当前fork的tool_use → 返回 "⏳ Running in parallel fork"
   - 其他tool_use → 返回 "⏳ Running in a parallel fork"
3. 追加 child directive 用户消息:
   内容模板:
   "[system: You are a parallel fork...]
    TASK: {directive}
    RULES:
    1. Work in your own worktree
    2. Use SyntheticOutput for final result
    3. Never fork again
    ..."

结果消息序列:
  [父assistant消息(含tool_use)] + [tool_results占位] + [child directive]
```

**递归保护**:
- `querySource` 检查: `agent:builtin:__fork__` → 拒绝
- 消息扫描回退: 检查消息中是否包含fork标记

**步骤**:
1. 新建 `fork.go`
2. 实现 `BuildForkedMessages` — 核心cache共享逻辑
3. 实现 `BuildChildMessage` — 严格的fork directive模板
4. 实现 `IsInForkChild` — 双重递归防护
5. 单元测试: 验证消息序列结构正确

### 4.3 Agent系统提示词构建

**新文件**: `internal/agent/prompt.go`

**源码参考**: `prompt.ts`

```go
// BuildAgentPrompt 构建AgentTool的描述，包含可用agent列表
func BuildAgentPrompt(
    agentDefs []AgentDefinition,
    isCoordinator bool,
    allowedAgentTypes []string,
    forkEnabled bool,
) string

// FormatAgentLine 格式化单个agent描述行
func FormatAgentLine(def AgentDefinition) string
// 输出格式: "- {agentType}: {whenToUse} (Tools: {tools})"

// BuildAgentSystemPrompt 为子agent构建系统提示词
func BuildAgentSystemPrompt(
    def *AgentDefinition,
    parentCtx *SubagentContext,
    additionalWorkDirs []string,
) ([]string, error)
```

**步骤**:
1. 新建 `prompt.go`
2. 实现 fork/non-fork 两套prompt模板
3. 实现agent listing (静态描述 vs attachment两种模式)

---

## 五、Phase 3: 异步生命周期 + 通知机制

### 5.1 异步Agent生命周期管理

**新文件**: `internal/agent/async_lifecycle.go`

**源码参考**: `agentToolUtils.ts` runAsyncAgentLifecycle

异步agent的完整生命周期:

```
注册 → 启动stream → 进度追踪 → 摘要生成 → 完成/失败/取消 →
worktree清理 → 通知主agent → 淘汰
```

```go
// AsyncAgentLifecycle 管理异步agent的完整生命周期
type AsyncAgentLifecycle struct {
    TaskID        string
    AgentID       string
    AbortCh       <-chan struct{}
    Runner        *AgentRunner
    Params        RunAgentParams
    Description   string
    TaskFramework *TaskFramework
    DiskOutput    *DiskOutput
    Watchdog      *StallWatchdog
    OnNotify      func(notification AgentNotification)
}

// AgentNotification 是agent完成时发给主agent的通知
type AgentNotification struct {
    TaskID      string `json:"task_id"`
    Description string `json:"description"`
    Status      string `json:"status"`  // "completed","failed","killed"
    Message     string `json:"message"` // 最终输出/错误信息
    OutputFile  string `json:"output_file"`
    Usage       AgentUsage `json:"usage"`
    WorktreePath  string `json:"worktree_path,omitempty"`
    WorktreeBranch string `json:"worktree_branch,omitempty"`
}

type AgentUsage struct {
    TotalTokens int `json:"total_tokens"`
    ToolUses    int `json:"tool_uses"`
    DurationMs  int `json:"duration_ms"`
}

// Run 执行异步agent生命周期 (在goroutine中调用)
func (lc *AsyncAgentLifecycle) Run(ctx context.Context) error
```

**Run 内部流程**:
```
1. taskFramework.Register(def) — 注册任务
2. taskManager.MarkRunning(agentID)
3. diskOutput = NewDiskOutput(dir, agentID, sessionID)
4. watchdog = NewStallWatchdog(diskOutput, agentID, notify)
5. go watchdog.Run(ctx)
6. for msg := range runner.RunAgent(ctx, params).Messages {
       diskOutput.Write(formatMessage(msg))
       updateProgress(msg)
       updateAsyncAgentProgress(taskID, progress)
   }
7. result = finalizeAgentResult(messages)
8. completeAsyncAgent(result) / failAsyncAgent(err)
9. worktreeResult = cleanupWorktreeIfNeeded()
10. enqueueNotification(taskID, result, worktreeResult)
```

### 5.2 Agent通知注入

**扩展**: `internal/agent/notification.go`

**源码参考**: `agentToolUtils.ts` enqueueAgentNotification

通知机制是异步agent与主agent通信的关键 — 当异步agent完成时，生成 `<task-notification>` XML注入到主agent的下一轮对话中。

```go
// NotificationQueue 管理待注入的agent通知
type NotificationQueue struct {
    mu    sync.Mutex
    queue []AgentNotification
}

func NewNotificationQueue() *NotificationQueue

// Enqueue 添加一个通知到队列
func (nq *NotificationQueue) Enqueue(n AgentNotification)

// Drain 取出所有待处理通知并清空队列
func (nq *NotificationQueue) Drain() []AgentNotification

// FormatAsXML 将通知格式化为<task-notification>XML
func FormatNotificationXML(n AgentNotification) string
```

**通知XML格式** (对齐claude-code-main):
```xml
<task-notification task_id="{taskID}" status="{status}">
  <description>{description}</description>
  <output_file>{outputFile}</output_file>
  <summary>{finalMessage}</summary>
  <usage total_tokens="{tokens}" tool_uses="{uses}" duration_ms="{ms}"/>
  <worktree path="{path}" branch="{branch}"/>  <!-- 可选 -->
</task-notification>
```

**注入时机**: 主agent的query loop在每轮开始前检查通知队列，将通知作为user-role消息注入。

**步骤**:
1. 新建 `async_lifecycle.go`
2. 新建 `notification.go`
3. 实现 `AsyncAgentLifecycle.Run()` — 完整异步流程
4. 实现 `NotificationQueue` — 线程安全队列
5. 实现 `FormatNotificationXML` — XML格式化
6. 在 `engine/queryloop.go` 中添加通知注入钩子点
7. 集成 DiskOutput + StallWatchdog (已有)

### 5.3 进度追踪与摘要

**新文件**: `internal/agent/progress.go`

**源码参考**: `agentToolUtils.ts` updateProgressFromMessage, startAgentSummarization

```go
// ProgressTracker 追踪agent执行进度
type ProgressTracker struct {
    TokenCount   int
    ToolUseCount int
    StartTime    time.Time
    LastActivity string
    Activities   []string // 最近5条
}

func NewProgressTracker() *ProgressTracker
func (pt *ProgressTracker) UpdateFromMessage(msg *engine.Message, tools []engine.Tool)
func (pt *ProgressTracker) GetUpdate() ProgressUpdate

type ProgressUpdate struct {
    TokenCount   int    `json:"token_count"`
    ToolUseCount int    `json:"tool_use_count"`
    DurationMs   int    `json:"duration_ms"`
    LastActivity string `json:"last_activity"`
}
```

---

## 六、Phase 4: AgentTool 4路决策树重构

### 6.1 重构 AgentTool

**源码参考**: `AgentTool.tsx` L239-1262

当前 `agentool.go` 只有基础同步/异步两条路径。需重构为完整4路决策树:

```
AgentTool.Call(input)
  ├─ teamName && name → 路径1: spawnTeammate()
  ├─ isolation=="remote" → 路径2: teleportToRemote()
  ├─ shouldRunAsync==true → 路径3: runAsyncAgentLifecycle()
  └─ else → 路径4: 同步runAgent() (支持中途转后台)
```

**扩展 Input**: 新增 `Prompt`, `SubagentType`, `Name`, `TeamName`, `Mode`, `Isolation`, `Cwd` 字段

**扩展 Output**: 支持4种status — `completed`, `async_launched`, `teammate_spawned`, `remote_launched`

**Call() 核心流程**:
1. 解析teamName (显式参数 || appState继承)
2. 路径1: teammate spawn (InProcess/tmux)
3. 选择agent定义 (fork路径 vs 命名agent)
4. MCP可用性检查
5. 路径2: remote isolation
6. Worktree隔离创建
7. 构建消息 (fork: BuildForkedMessages / normal: createUserMessage)
8. 路径3: 异步 (background/coordinator/fork)
9. 路径4: 同步 (含backgroundSignal race中途转后台)

**步骤**:
1. 重构 `agentool.go` 扩展Input/Output
2. 实现4路决策树
3. 集成 AgentRunner, AsyncAgentLifecycle
4. 实现 resolveAgentDefinition, resolveTeamName
5. 集成 worktree 创建+清理+变更检测
6. 更新 MapToolResultToBlockParam 4种output格式

### 6.2 扩展 Worktree 集成

**扩展**: `internal/agent/worktree.go`

新增方法:
- `HasChanges(worktreePath, headCommit string) (bool, error)` — git diff检测
- `CreateForAgent(agentID, repoDir string) (*WorktreeInfo, error)` — 返回完整信息
- `CleanupIfNoChanges(info *WorktreeInfo) (*WorktreeResult, error)` — 无变更则删除
- `BuildWorktreeNotice(originalCwd, worktreePath string) string` — fork路径转换提示

---

## 七、Phase 5: Coordinator模式 + Agent记忆

### 7.1 Coordinator模式

**扩展**: `internal/agent/coordinator.go` + 新建 `internal/agent/coordinator_mode.go`

**源码参考**: `coordinatorMode.ts`

Coordinator模式是专门的编排模式，主agent只负责分配任务给worker，自己不直接写代码。

```go
// CoordinatorMode 管理coordinator模式的状态和行为
type CoordinatorMode struct {
    Enabled       bool
    ScratchpadDir string // 共享便笺目录
}

func IsCoordinatorMode() bool
func GetCoordinatorSystemPrompt() string
func GetCoordinatorUserContext(tools []engine.Tool) string
```

**Coordinator系统提示词要点** (对齐coordinatorMode.ts):
- 角色: 编排者，不直接实现
- 工具: 仅 Task, TaskStop, SendMessage, SyntheticOutput
- 工作流4阶段: 研究→综合→实施→验证
- 并发管理: 最多N个并行worker
- 失败处理: worker失败时的重试/替代策略
- Scratchpad: 共享目录供worker间传递文件

### 7.2 Agent记忆系统

**新文件**: `internal/agent/memory.go`

**源码参考**: `agentMemory.ts`

```go
type AgentMemoryScope string
const (
    MemoryScopeUser    AgentMemoryScope = "user"
    MemoryScopeProject AgentMemoryScope = "project"
    MemoryScopeLocal   AgentMemoryScope = "local"
)

// GetAgentMemoryDir 返回agent记忆存储目录
func GetAgentMemoryDir(scope AgentMemoryScope, agentType string) (string, error)
// user → ~/.claude/agent-memory/{sanitized_type}/
// project → {projectDir}/.claude/agent-memory/{sanitized_type}/
// local → {projectDir}/.claude.local/agent-memory/{sanitized_type}/

// GetAgentMemoryEntrypoint 返回记忆入口文件路径
func GetAgentMemoryEntrypoint(scope AgentMemoryScope, agentType string) string

// LoadAgentMemoryPrompt 加载记忆内容并生成提示词
func LoadAgentMemoryPrompt(scope AgentMemoryScope, agentType string) (string, error)

// IsAgentMemoryPath 检查路径是否在agent记忆目录内
func IsAgentMemoryPath(path string) bool

// SanitizeAgentTypeForPath 将agent类型名转为安全路径名
func SanitizeAgentTypeForPath(agentType string) string
```

**记忆提示词模板**:
```
<agent-memory scope="{scope}">
{memory_content}
</agent-memory>

Guidelines:
- You can update your memory using FileWrite to {entrypoint_path}
- Memory persists across sessions
- Keep memory concise and actionable
```

---

## 八、Phase 6: InProcess Teammate + Team工具

### 8.1 InProcess Teammate

**新文件**: `internal/agent/inprocess_teammate.go`

**源码参考**: `InProcessTeammateTask/types.ts`, `InProcessTeammateTask.tsx`

InProcess Teammate在同一进程内运行，通过goroutine和channel通信，无需tmux。

```go
type TeammateIdentity struct {
    AgentID          string `json:"agent_id"`
    AgentName        string `json:"agent_name"`
    TeamName         string `json:"team_name"`
    Color            string `json:"color,omitempty"`
    PlanModeRequired bool   `json:"plan_mode_required"`
    ParentSessionID  string `json:"parent_session_id"`
}

type InProcessTeammate struct {
    Identity         TeammateIdentity
    Runner           *AgentRunner
    Mailbox          *Mailbox
    AbortCh          chan struct{}
    CurrentAbortCh   chan struct{} // 中止当前turn
    IsIdle           bool
    ShutdownRequested bool
    PendingMessages  []string
    Messages         []*engine.Message // UI缓冲(上限50)
    PermissionMode   string
    OnIdleCallbacks  []func()
}

func NewInProcessTeammate(identity TeammateIdentity, runner *AgentRunner) *InProcessTeammate

// Run 启动teammate的事件循环
func (t *InProcessTeammate) Run(ctx context.Context) error

// SendPendingMessage 向teammate发送用户消息
func (t *InProcessTeammate) SendPendingMessage(msg string)

// RequestShutdown 请求teammate优雅关闭
func (t *InProcessTeammate) RequestShutdown()

// Kill 立即终止teammate
func (t *InProcessTeammate) Kill()
```

**事件循环** (对齐InProcessTeammateTask.tsx):
```
while !shutdown {
    1. 检查邮箱是否有新消息
    2. 检查pendingUserMessages队列
    3. 如果有消息 → 执行一轮query loop
    4. 如果空闲 → 等待新消息 (select on channels)
    5. 更新进度和token计数
    6. 处理shutdown请求 → 清理并退出
}
```

### 8.2 TeamCreate/TeamDelete 工具

**新文件**: `internal/tool/teamcreate/teamcreate.go` (已存在骨架，需扩展)

**源码参考**: `TeamCreateTool.ts`

```go
type TeamCreateInput struct {
    TeamName    string `json:"team_name"`
    Description string `json:"description,omitempty"`
    AgentType   string `json:"agent_type,omitempty"`
}

type TeamCreateOutput struct {
    TeamName     string `json:"team_name"`
    TeamFilePath string `json:"team_file_path"`
    LeadAgentID  string `json:"lead_agent_id"`
}
```

**TeamFile 格式** (磁盘持久化):
```go
type TeamFile struct {
    Name          string       `json:"name"`
    Description   string       `json:"description,omitempty"`
    CreatedAt     int64        `json:"created_at"`
    LeadAgentID   string       `json:"lead_agent_id"`
    LeadSessionID string       `json:"lead_session_id"`
    Members       []TeamMember `json:"members"`
}

type TeamMember struct {
    AgentID       string   `json:"agent_id"`
    Name          string   `json:"name"`
    AgentType     string   `json:"agent_type"`
    Model         string   `json:"model,omitempty"`
    JoinedAt      int64    `json:"joined_at"`
    Cwd           string   `json:"cwd"`
    Subscriptions []string `json:"subscriptions,omitempty"`
}
```

**步骤**:
1. 新建 `internal/agent/team.go` — TeamFile管理
2. 扩展 teamcreate 工具
3. 扩展 teamdelete 工具
4. 实现 `SpawnTeammate()` 函数 — InProcess/tmux分支

### 8.3 扩展 SendMessage 工具

**扩展**: `internal/tool/sendmessage/sendmessage.go`

**源码参考**: `SendMessageTool.ts`

当前SendMessage非常基础，需要扩展支持:

1. **邮箱文件系统集成** — 写入 `~/.claude/teams/{teamName}/mailbox/{recipient}.json`
2. **广播** — `to: "*"` 发送给所有teammate
3. **结构化消息** — shutdown_request, shutdown_response, plan_approval_response
4. **UDS/Bridge路由** — 本地socket和远程peer通信
5. **颜色路由** — sender/target颜色传递

```go
type SendMessageInput struct {
    To      string      `json:"to"`
    Summary string      `json:"summary,omitempty"`
    Message interface{} `json:"message"` // string 或 结构化消息
}

// 结构化消息类型
type ShutdownRequest struct {
    Type   string `json:"type"` // "shutdown_request"
    Reason string `json:"reason,omitempty"`
}

type ShutdownResponse struct {
    Type      string `json:"type"` // "shutdown_response"
    RequestID string `json:"request_id"`
    Approve   bool   `json:"approve"`
    Reason    string `json:"reason,omitempty"`
}
```

---

## 九、Phase 7: tmux Teammate + Remote Agent

### 9.1 tmux Teammate Backend

**新文件**: `internal/agent/tmux_backend.go`

tmux teammate作为独立进程运行，通过tmux管理生命周期:

```go
type TmuxBackend struct {
    SessionName string
}

func (tb *TmuxBackend) SpawnTeammate(params TeammateSpawnParams) (*TeammateHandle, error)
func (tb *TmuxBackend) ListPanes() ([]TmuxPane, error)
func (tb *TmuxBackend) KillPane(paneID string) error
func (tb *TmuxBackend) SendKeys(paneID, keys string) error
```

**spawn流程**:
1. 创建tmux pane (split-window)
2. 在pane中启动 agent-engine 子进程 (teammate模式)
3. 传递环境变量: TEAM_NAME, AGENT_NAME, PARENT_SESSION_ID
4. 子进程读取TeamFile获取配置
5. 子进程通过邮箱文件系统通信

### 9.2 Remote Agent (CCR)

**新文件**: `internal/agent/remote_backend.go`

远程agent在容器中运行:

```go
type RemoteBackend struct {
    CCRURL string // Container runtime URL
}

func (rb *RemoteBackend) LaunchRemoteAgent(params RemoteAgentParams) (*RemoteSession, error)
func (rb *RemoteBackend) GetSessionStatus(sessionID string) (*RemoteSessionStatus, error)
func (rb *RemoteBackend) CancelSession(sessionID string) error

type RemoteAgentParams struct {
    InitialMessage string
    Description    string
    Signal         <-chan struct{}
}

type RemoteSession struct {
    ID    string
    Title string
    URL   string
}
```

**注意**: Remote Agent是ant-only功能，需要预留接口但可以暂时返回 "not supported" 错误。

---

## 十、Phase 8: Agent恢复 + 前台转后台

### 10.1 Agent恢复机制

**新文件**: `internal/agent/resume.go`

**源码参考**: `resumeAgent.ts`

从磁盘转录恢复中断的异步agent:

```go
type AgentMetadata struct {
    AgentType    string `json:"agent_type"`
    Description  string `json:"description"`
    WorktreePath string `json:"worktree_path,omitempty"`
}

// WriteAgentMetadata 写入agent元数据到磁盘
func WriteAgentMetadata(agentID string, meta AgentMetadata) error

// ReadAgentMetadata 读取agent元数据
func ReadAgentMetadata(agentID string) (*AgentMetadata, error)

// GetAgentTranscript 读取agent的对话转录
func GetAgentTranscript(agentID string) ([]*engine.Message, error)

// ResumeAgentBackground 从磁盘恢复agent
func ResumeAgentBackground(params ResumeParams) (*ResumeResult, error)
```

**恢复流程**:
1. 读取转录 + 元数据
2. 过滤无效消息 (orphaned thinking, unresolved tool_use)
3. 重建content replacement state
4. 验证worktree是否仍存在
5. 确定agent定义 (fork → ForkAgent, 命名 → 查找, 默认 → GeneralPurpose)
6. 重建系统提示词 (fork需要父级系统提示词)
7. 启动异步生命周期 (复用Phase 3)

### 10.2 前台→后台转换

**扩展**: AgentTool 同步路径

**源码参考**: `AgentTool.tsx` L868-1052 — backgroundSignal race

同步agent运行时，用户可以将其转为后台:

```go
// ForegroundAgent 跟踪前台运行的同步agent
type ForegroundAgent struct {
    AgentID         string
    BackgroundSignal chan struct{} // 关闭时触发转后台
    CancelAutoBackground func()
}

// RegisterAgentForeground 注册前台agent
func RegisterAgentForeground(params ForegroundParams) *ForegroundAgent

// UnregisterAgentForeground 取消前台注册
func UnregisterAgentForeground(agentID string)
```

**转换逻辑**:
1. 同步路径使用 `select` race: 下一条消息 vs backgroundSignal
2. backgroundSignal触发时:
   - 释放前台iterator
   - 用已收集的messages重新启动runAgent (isAsync=true)
   - 立即返回 async_launched
3. 自动转后台: 超过阈值时间自动触发 (getAutoBackgroundMs)

---

## 十一、跨Phase集成点

### 11.1 engine/queryloop.go 修改

1. **通知注入**: 在每轮开始前检查 NotificationQueue，注入 user-role 消息
2. **Agent上下文传播**: 将 SubagentContext 传入 UseContext
3. **流式消息输出**: 支持 channel 输出用于异步agent

### 11.2 UseContext 扩展

```go
// engine/tool_iface.go UseContext 新增字段:
SubagentCtx      *agent.SubagentContext
NotificationQueue *agent.NotificationQueue
TeamContext       *agent.TeamContext
AgentRunner       *agent.AgentRunner
```

### 11.3 engine.Engine 子agent工厂

```go
// 在 engine.go 中新增:
func (e *Engine) SpawnChildEngine(params agent.RunAgentParams) *Engine
```

---

## 十二、文件清单总结

### 新建文件

| 文件 | Phase | 功能 |
|------|-------|------|
| `internal/agent/loader.go` | P1 | Agent定义加载器 |
| `internal/agent/builtin_agents.go` | P1 | 内置agent定义 |
| `internal/agent/toolfilter.go` | P1 | 工具权限过滤 |
| `internal/agent/runner.go` | P2 | Agent运行核心 |
| `internal/agent/fork.go` | P2 | Fork子agent |
| `internal/agent/prompt.go` | P2 | Agent提示词构建 |
| `internal/agent/async_lifecycle.go` | P3 | 异步生命周期 |
| `internal/agent/notification.go` | P3 | 通知队列+XML |
| `internal/agent/progress.go` | P3 | 进度追踪 |
| `internal/agent/coordinator_mode.go` | P5 | Coordinator模式 |
| `internal/agent/memory.go` | P5 | Agent记忆系统 |
| `internal/agent/inprocess_teammate.go` | P6 | InProcess队友 |
| `internal/agent/team.go` | P6 | TeamFile管理 |
| `internal/agent/tmux_backend.go` | P7 | tmux后端 |
| `internal/agent/remote_backend.go` | P7 | 远程agent后端 |
| `internal/agent/resume.go` | P8 | Agent恢复 |

### 需修改文件

| 文件 | 变更 |
|------|------|
| `internal/agent/types.go` | 扩展AgentDefinition 17+字段 |
| `internal/agent/worktree.go` | 添加HasChanges, CreateForAgent等 |
| `internal/agent/coordinator.go` | 集成CoordinatorMode |
| `internal/tool/agentool/agentool.go` | 重构为4路决策树 |
| `internal/tool/sendmessage/sendmessage.go` | 邮箱+广播+结构化消息 |
| `internal/engine/tool_iface.go` | UseContext新增agent字段 |
| `internal/engine/queryloop.go` | 通知注入钩子 |

---

## 十三、依赖关系与执行顺序

```
Phase 1 (基础层) ← 无依赖
  ↓
Phase 2 (执行层) ← 依赖 Phase 1 的 AgentDefinition + ToolFilter
  ↓
Phase 3 (异步层) ← 依赖 Phase 2 的 AgentRunner
  ↓
Phase 4 (路由层) ← 依赖 Phase 2 + 3
  ↓
Phase 5 (协调层) ← 依赖 Phase 4 的 AgentTool
  ↓
Phase 6 (Swarm层) ← 依赖 Phase 4 + 5
  ↓
Phase 7 (扩展层) ← 依赖 Phase 6
  ↓
Phase 8 (高级层) ← 依赖 Phase 3 + 4
```

**预估工作量**:
- Phase 1: ~3天 (类型扩展+加载器+过滤器)
- Phase 2: ~5天 (runner是核心，fork逻辑复杂)
- Phase 3: ~3天 (异步生命周期+通知)
- Phase 4: ~4天 (4路决策树重构)
- Phase 5: ~3天 (coordinator+记忆)
- Phase 6: ~4天 (InProcess teammate+Team工具)
- Phase 7: ~3天 (tmux+remote stub)
- Phase 8: ~2天 (恢复+转后台)

**总计: ~27天**
