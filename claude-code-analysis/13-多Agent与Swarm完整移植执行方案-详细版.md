# 多Agent协调与Swarm完整移植执行方案（详细版）

> 基于 `claude-code-main`（TypeScript）源码深度分析，完整复制到 `agent-engine`（Go）

---

## 一、现状对比总览

### 1.1 claude-code-main 核心模块

| 模块 | 文件 | 职责 |
|------|------|------|
| AgentTool 入口 | `AgentTool.tsx` (1398行) | 4路决策树：teammate → remote → async → sync |
| 核心执行 | `runAgent.ts` (974行) | Agent生命周期：ID生成、hook执行、MCP初始化、工具过滤、查询循环 |
| Fork子Agent | `forkSubagent.ts` (211行) | 字节级相同前缀构造，prompt cache共享 |
| 工具集 | `agentToolUtils.ts` (687行) | 工具过滤、结果格式化、异步生命周期、handoff分类 |
| Agent定义加载 | `loadAgentsDir.ts` (756行) | 多源加载：builtin/custom/plugin/policy，frontmatter解析 |
| 异步生命周期 | `agentToolUtils.ts:runAsyncAgentLifecycle` | 进度跟踪、摘要、完成通知、错误处理 |
| 前台转后台 | `AgentTool.tsx:898-1052` | Race模式：消息迭代 vs backgroundSignal |
| 协调器模式 | `coordinatorMode.ts` | 限制工具集、强制异步 |
| InProcess队友 | `inProcessTeammate.ts` | 同进程goroutine协作，mailbox通信 |
| Agent记忆 | `agentMemory.ts` + `agentMemorySnapshot.ts` | user/project/local三级持久化 |
| 通知注入 | `LocalAgentTask.ts` | XML格式通知注入父对话 |
| Agent恢复 | `agentResume.ts` | 从磁盘transcript恢复中断Agent |
| Worktree隔离 | `agentWorktree.ts` | git worktree创建/清理/变更检测 |
| 团队管理 | `team.ts` / `TeamFile` | 团队创建/成员管理/消息路由 |

### 1.2 agent-engine 已有实现

| 模块 | 文件 | 完成度 | 差距 |
|------|------|--------|------|
| types.go | AgentDefinition结构体 | ★★★★☆ | 缺少 `getSystemPrompt` 回调机制 |
| runner.go | AgentRunner + executeLoop | ★★★☆☆ | 缺少hook执行、MCP初始化、fork消息构造集成 |
| toolfilter.go | FilterToolsForAgent | ★★★★☆ | 基本对齐，缺少 `Agent(x,y)` 参数化规则解析 |
| fork.go | BuildForkedMessages | ★★☆☆☆ | 缺少 tool_result placeholder 机制实现字节级缓存共享 |
| async_lifecycle.go | AsyncLifecycleManager | ★★★☆☆ | 缺少摘要、进度事件发射、disk output集成 |
| agentool.go | AgentTool 4路决策树 | ★★★☆☆ | 缺少 team_name/name 路由、fork决策、前台转后台 |
| notification.go | NotificationQueue + XML | ★★★★☆ | 基本完整 |
| coordinator_mode.go | CoordinatorMode | ★★★☆☆ | 缺少系统提示词集成、强制异步逻辑 |
| inprocess_teammate.go | InProcessTeammate | ★★☆☆☆ | 缺少 runLoop 中的 Agent 执行 + mailbox 消息处理 |
| loader.go | AgentLoader | ★★★☆☆ | 缺少 plugin agent 加载、JSON schema 验证 |
| memory.go | AgentMemory | ★★☆☆☆ | 需要验证 snapshot 机制和3级作用域 |
| resume.go | ResumeManager | ★★★☆☆ | checkpoint基础完成，缺少 conversation replay |
| team.go | TeamManager | ★★★☆☆ | 缺少 resolveTeamName 和 spawnTeammate 路由 |
| subagent_context.go | SubagentContext | ★★★☆☆ | 缺少 permission mode 覆盖逻辑 |
| worktree.go | WorktreeManager | ★★☆☆☆ | 基础结构，需要对齐变更检测和清理逻辑 |

---

## 二、关键差距分析

### 2.1 AgentTool 决策树差距

**claude-code-main 完整决策路径：**

```
AgentTool.call() 入口
│
├─ 检查 team_name → isAgentSwarmsEnabled()
├─ resolveTeamName({ team_name }, appState)
├─ 校验：teammate不能嵌套spawn、InProcess不能spawn后台
│
├─ [Path 1] teamName && name → spawnTeammate()
│   └─ 返回 { status: 'teammate_spawned', ... }
│
├─ 解析 effectiveType (fork路径 vs 显式subagent_type)
│   ├─ isForkSubagentEnabled() && !subagent_type → FORK_AGENT
│   └─ subagent_type 存在 → filterDeniedAgents() → find agent
│
├─ 检查 requiredMcpServers（含30s轮询等待）
├─ 解析 effectiveIsolation (param > agentDef)
│
├─ [Path 2] isolation === 'remote' → teleportToRemote()
│   └─ 返回 { status: 'remote_launched', taskId, sessionUrl }
│
├─ 构造 systemPrompt + promptMessages
│   ├─ Fork路径：继承父系统提示 + buildForkedMessages()
│   └─ 普通路径：getAgentSystemPrompt() + enhanceWithEnvDetails
│
├─ 计算 shouldRunAsync (background | coordinator | fork | assistant)
├─ 构建 workerTools (独立于父级)
├─ 创建 worktree (如需)
│
├─ [Path 3] shouldRunAsync → registerAsyncAgent() + void runAsyncAgentLifecycle()
│   └─ 返回 { status: 'async_launched', agentId, outputFile }
│
└─ [Path 4] Sync执行
    ├─ registerAgentForeground() (可后台化)
    ├─ while(true) { race(nextMessage, backgroundSignal) }
    │   ├─ backgroundSignal触发 → 前台转后台
    │   └─ 正常消息处理 + 进度更新
    └─ finalizeAgentTool() → 返回结果
```

**agent-engine 当前缺失：**
1. `team_name` + `name` 的 teammate spawn 路由
2. Fork子agent自动选择（`isForkSubagentEnabled` + `isInForkChild` 递归保护）
3. `requiredMcpServers` 轮询等待机制
4. 前台转后台 Race 模式
5. `buildForkedMessages` 的 tool_result placeholder 字节级缓存机制
6. `workerTools` 独立构建（不继承父级限制）
7. `shouldRunAsync` 的多条件合并判断

### 2.2 runAgent 执行核心差距

**claude-code-main runAgent 完整流程：**

```
runAgent() AsyncGenerator
│
├─ 1. agentId 生成 / override.agentId
├─ 2. Perfetto trace 注册
├─ 3. Fork context messages 过滤 (filterIncompleteToolCalls)
├─ 4. ReadFileState 克隆或创建
├─ 5. UserContext / SystemContext 解析
│   ├─ omitClaudeMd (Explore/Plan类型)
│   └─ omitGitStatus (Explore/Plan类型)
├─ 6. Permission mode 覆盖
│   ├─ agentPermissionMode 优先级判断
│   ├─ shouldAvoidPermissionPrompts (async → true)
│   ├─ awaitAutomatedChecksBeforeDialog
│   └─ allowedTools → session rules 替换
├─ 7. 工具解析
│   ├─ useExactTools → 直接使用父工具（fork缓存兼容）
│   └─ resolveAgentTools → 多层过滤
├─ 8. 系统提示词构造
│   ├─ override.systemPrompt（fork路径）
│   └─ getAgentSystemPrompt() + enhanceWithEnvDetails
├─ 9. AbortController
│   ├─ async → new（独立生命周期）
│   └─ sync → 共享父级
├─ 10. SubagentStart hooks 执行
├─ 11. Frontmatter hooks 注册
├─ 12. Skills 预加载
├─ 13. Agent MCP servers 初始化
│   ├─ 字符串引用 → getMcpConfigByName + connectToServer
│   └─ 内联定义 → dynamic scope + 独立清理
├─ 14. AgentOptions 构建
│   ├─ thinkingConfig: fork → 继承, 普通 → disabled
│   ├─ isNonInteractiveSession: fork → 继承, async → true
│   └─ querySource: fork → 传播（递归guard用）
├─ 15. SubagentContext 创建
├─ 16. Query循环 + transcript 记录
└─ 17. 清理: MCP servers, hooks, skills
```

**agent-engine runner.go 缺失：**
1. Hook 执行（SubagentStart/SubagentEnd）
2. Skills 预加载
3. Agent MCP servers 初始化
4. Fork context 消息过滤
5. Permission mode 覆盖逻辑（多层优先级）
6. Thinking config 控制
7. UserContext/SystemContext 的 omitClaudeMd / omitGitStatus
8. Transcript 记录
9. ContentReplacementState 管理

### 2.3 Fork 子Agent 差距

**claude-code-main 的 buildForkedMessages 核心机制：**

```
输入: directive(子任务), assistantMessage(父的最后消息)
输出: [clonedAssistantMsg, userMsgWithPlaceholders+directive]

关键设计:
1. 克隆父 assistant message（保留所有 tool_use blocks）
2. 为每个 tool_use 创建 tool_result（text = "Fork started — processing in background"）
3. 所有 fork children 的 tool_result placeholder 文本相同
4. 仅最后的 directive text block 不同
→ 实现字节级相同前缀 → prompt cache 命中
```

**agent-engine fork.go 当前实现：**
- 仅做简单消息克隆 + 追加子任务消息
- 完全缺少 tool_use → tool_result placeholder 机制
- 缺少父系统提示词继承
- 缺少递归 fork 保护

---

## 三、分阶段执行方案

### 阶段 1：Agent定义体系完善（预计 2-3 天）

#### 1.1 AgentDefinition 增强

**文件**: `internal/agent/types.go`

```go
// 新增字段
type AgentDefinition struct {
    // ... 现有字段 ...

    // 动态系统提示词生成器（builtin agents需要）
    GetSystemPrompt func(ctx AgentPromptContext) string `json:"-"`

    // 标记来源更细化
    SourceDetail string `json:"source_detail,omitempty"` // "userSettings","projectSettings","policySettings","flagSettings"

    // Plugin 元数据
    PluginName string `json:"plugin_name,omitempty"`

    // Effort level (int or string)
    EffortValue interface{} `json:"effort_value,omitempty"`

    // 待更新的 snapshot
    PendingSnapshotUpdate *SnapshotUpdate `json:"pending_snapshot_update,omitempty"`
}

type AgentPromptContext struct {
    ToolNames []string
    Model     string
    WorkDir   string
    TeamName  string
}

type SnapshotUpdate struct {
    Timestamp string `json:"snapshot_timestamp"`
}
```

#### 1.2 AgentDefinitionsResult 结构

**文件**: `internal/agent/loader.go`（修改）

```go
type AgentDefinitionsResult struct {
    ActiveAgents    []AgentDefinition
    AllAgents       []AgentDefinition
    FailedFiles     []FailedAgentFile
    AllowedAgentTypes []string  // 从 Agent(x,y) 工具规范解析
}

type FailedAgentFile struct {
    Path  string
    Error string
}
```

#### 1.3 getActiveAgentsFromList 优先级合并

**文件**: `internal/agent/loader.go`（修改）

实现与 claude-code-main 相同的优先级顺序：
```
builtin → plugin → userSettings → projectSettings → flagSettings → policySettings
```
后加载的同名 agent 覆盖先加载的。

#### 1.4 Builtin Agents 完善

**文件**: `internal/agent/builtin_agents.go`（修改）

对齐 claude-code-main 的内置 agent 类型：
- `general-purpose`：全工具访问
- `Explore`：只读搜索，omitClaudeMd=true, omitGitStatus=true
- `Plan`：规划模式，omitClaudeMd=true
- `fork`：合成 agent 定义（permissionMode: "bubble"）

```go
var ForkAgent = AgentDefinition{
    AgentType:      "fork",
    Source:         SourceBuiltIn,
    WhenToUse:      "Fork current conversation for parallel work",
    Background:     true,
    PermissionMode: "bubble",
    GetSystemPrompt: func(ctx AgentPromptContext) string {
        return "" // fork 继承父系统提示
    },
}
```

#### 1.5 Agent JSON Schema 验证

**新文件**: `internal/agent/schema.go`

```go
// ValidateAgentJSON validates an agent definition against the JSON schema.
// Aligned with loadAgentsDir.ts AgentJsonSchema.
func ValidateAgentJSON(data []byte) (*AgentDefinition, error) { ... }

// ValidateAgentsJSON validates multiple agent definitions.
func ValidateAgentsJSON(data []byte) (map[string]*AgentDefinition, error) { ... }
```

---

### 阶段 2：AgentRunner 核心执行完善（预计 3-4 天）

#### 2.1 Hook 执行集成

**文件**: `internal/agent/runner.go`（修改）

在 `RunAgent` 方法中添加：

```go
// Step 5.5: Execute SubagentStart hooks.
hookResults := r.executeSubagentStartHooks(ctx, agentID, effectiveDef.AgentType)
for _, hr := range hookResults {
    if len(hr.AdditionalContexts) > 0 {
        // Append hook context as initial messages.
        for _, ctx := range hr.AdditionalContexts {
            initialMessages = append(initialMessages, &engine.Message{
                Role: engine.RoleUser,
                Content: []*engine.ContentBlock{
                    {Type: engine.ContentTypeText, Text: ctx},
                },
            })
        }
    }
}

// Step 5.6: Register frontmatter hooks (scoped to agent lifecycle).
if len(effectiveDef.Hooks) > 0 {
    r.registerFrontmatterHooks(agentID, effectiveDef.Hooks)
}
```

**新文件**: `internal/agent/hooks.go`

```go
type HookResult struct {
    AdditionalContexts []string
    Error              error
}

func (r *AgentRunner) executeSubagentStartHooks(ctx context.Context, agentID, agentType string) []HookResult { ... }
func (r *AgentRunner) executeSubagentEndHooks(ctx context.Context, agentID, agentType string) { ... }
func (r *AgentRunner) registerFrontmatterHooks(agentID string, hooks map[string][]HookCommand) { ... }
```

#### 2.2 Agent MCP Servers 初始化

**新文件**: `internal/agent/mcp_init.go`

```go
type AgentMcpResult struct {
    Clients       []MCPClient       // 父 + 新创建的
    Tools         []engine.Tool     // agent-specific MCP tools
    CleanupFunc   func()            // 清理新创建的连接
}

// InitializeAgentMcpServers 初始化 agent 专属 MCP 服务器连接。
// 对齐 runAgent.ts 的 initializeAgentMcpServers 函数。
//
// 处理两种规格:
//   - 字符串引用: 按名称查找现有 MCP 配置
//   - 内联定义: { name: config } 格式，创建新连接
//
// 仅清理新创建的连接（内联定义），共享引用不清理。
func InitializeAgentMcpServers(
    agentDef *AgentDefinition,
    parentClients []MCPClient,
    mcpConfigLookup func(name string) *MCPConfig,
    connectFn func(name string, config *MCPConfig) (MCPClient, error),
) (*AgentMcpResult, error) { ... }
```

#### 2.3 Permission Mode 覆盖逻辑

**文件**: `internal/agent/runner.go`（修改）

```go
// resolvePermissionMode implements the multi-layer permission override logic
// from runAgent.ts agentGetAppState.
//
// Priority:
//   1. Parent bypassPermissions / acceptEdits / auto → never override
//   2. Agent definition permissionMode → apply
//   3. Async agents → shouldAvoidPermissionPrompts = true
//   4. Bubble mode → always show prompts
func (r *AgentRunner) resolvePermissionMode(
    parentMode string,
    agentDef *AgentDefinition,
    isAsync bool,
    canShowPermissionPrompts *bool,
) string { ... }
```

#### 2.4 工具解析增强

**文件**: `internal/agent/toolfilter.go`（修改）

添加 `resolveAgentTools` 完整实现：

```go
// ResolveAgentToolsFull implements the full tool resolution pipeline
// from agentToolUtils.ts resolveAgentTools.
//
// Handles:
//   - Wildcard expansion ('*' or undefined tools)
//   - Agent(x,y) parameterized tool specs → allowedAgentTypes
//   - DisallowedTools filtering
//   - Permission rule value parsing for tool specs
func ResolveAgentToolsFull(
    agentDef *AgentDefinition,
    availableTools []string,
    isAsync bool,
    isMainThread bool,
) *ResolvedAgentToolsResult {
    // ...
}

type ResolvedAgentToolsResult struct {
    HasWildcard       bool
    ValidTools        []string
    InvalidTools      []string
    ResolvedTools     []string
    AllowedAgentTypes []string
}
```

#### 2.5 executeLoop 增强

**文件**: `internal/agent/runner.go`（修改）

当前 `executeLoop` 在收到 `EventDone` 后 break，但 claude-code-main 的 agent 是一个完整的多轮查询循环。需要修改为：

```go
func (r *AgentRunner) executeLoop(ctx context.Context, eng *engine.Engine, task string, def AgentDefinition, initialMessages []*engine.Message) *AgentRunResult {
    // ... 现有代码 ...

    for turnCount < maxTurns {
        turnCount++
        eventCh := eng.SubmitMessage(ctx, params)

        var hasToolCalls bool
        for ev := range eventCh {
            // ... 处理事件 ...
            if ev.Type == engine.EventToolUse {
                hasToolCalls = true
            }
        }

        // 只在无工具调用时（end_turn）才结束
        if turnDone && !hasToolCalls {
            break
        }

        // 如果有工具调用，engine 内部处理后会返回新的 assistant 响应
        // 继续下一轮
        params = engine.QueryParams{
            Text:   "", // continuation, no new user message
            Source: engine.QuerySourceAgent,
        }
    }
    // ...
}
```

#### 2.6 UserContext / SystemContext 优化

**文件**: `internal/agent/prompt.go`（修改）

```go
// BuildAgentContext constructs the user/system context for an agent.
// Implements omitClaudeMd and omitGitStatus optimizations.
func BuildAgentContext(agentDef *AgentDefinition, parentCtx AgentPromptContext) (userCtx, sysCtx map[string]string) {
    // omitClaudeMd for read-only agents (Explore, Plan)
    if agentDef.OmitClaudeMd {
        delete(userCtx, "claudeMd")
    }
    // omitGitStatus for Explore/Plan
    if agentDef.AgentType == "Explore" || agentDef.AgentType == "Plan" {
        delete(sysCtx, "gitStatus")
    }
    return
}
```

---

### 阶段 3：Fork 子Agent 完整实现（预计 2-3 天）

#### 3.1 字节级缓存共享消息构造

**文件**: `internal/agent/fork.go`（重写）

```go
const ForkPlaceholderResult = "Fork started — processing in background"

// BuildForkedMessages constructs byte-identical API request prefixes for cache sharing.
//
// 对齐 forkSubagent.ts 的 buildForkedMessages:
//   1. 克隆父 assistant message（保留所有 tool_use blocks）
//   2. 为每个 tool_use 创建 tool_result（placeholder 文本相同）
//   3. 追加 per-child directive 作为最后的 text block
//   4. 所有 fork children 仅最后一个 text block 不同 → 缓存命中
func BuildForkedMessages(
    directive string,
    parentAssistantMsg *engine.Message,
    allParentMessages []*engine.Message,
) []*engine.Message {
    if parentAssistantMsg == nil {
        return []*engine.Message{buildChildMessage(directive)}
    }

    // 收集所有 tool_use blocks
    var toolUseBlocks []*engine.ContentBlock
    for _, block := range parentAssistantMsg.Content {
        if block.Type == engine.ContentTypeToolUse {
            toolUseBlocks = append(toolUseBlocks, block)
        }
    }

    if len(toolUseBlocks) == 0 {
        return []*engine.Message{buildChildMessage(directive)}
    }

    // 克隆父 assistant message
    clonedAssistant := cloneMessage(parentAssistantMsg)

    // 构建 tool_result blocks + directive
    var resultContent []*engine.ContentBlock
    for _, tu := range toolUseBlocks {
        resultContent = append(resultContent, &engine.ContentBlock{
            Type:      engine.ContentTypeToolResult,
            ToolUseID: tu.ToolUseID,
            Text:      ForkPlaceholderResult,
        })
    }
    resultContent = append(resultContent, &engine.ContentBlock{
        Type: engine.ContentTypeText,
        Text: buildChildInstructions(directive),
    })

    userMsg := &engine.Message{
        Role:    engine.RoleUser,
        Content: resultContent,
    }

    return []*engine.Message{clonedAssistant, userMsg}
}
```

#### 3.2 递归 Fork 保护

```go
// IsInForkChild checks if the current context is inside a fork child.
// Prevents recursive forking. Uses both querySource check and message scan.
func IsInForkChild(querySource string, messages []*engine.Message) bool {
    // Primary: querySource check (survives autocompact)
    if querySource == "agent:builtin:fork" {
        return true
    }
    // Fallback: scan messages for fork boilerplate
    for _, m := range messages {
        if m.Role == engine.RoleUser {
            for _, block := range m.Content {
                if block.Type == engine.ContentTypeText &&
                    strings.Contains(block.Text, "You are a forked worker agent") {
                    return true
                }
            }
        }
    }
    return false
}
```

#### 3.3 Worktree Notice 注入

```go
// BuildWorktreeNotice creates a notice for fork children in worktrees.
// Tells the child to translate paths and re-read stale files.
func BuildWorktreeNotice(parentCwd, worktreePath string) string {
    return fmt.Sprintf(`IMPORTANT: You are working in an isolated git worktree.
- Your working directory is: %s
- The parent's working directory was: %s
- Translate any absolute paths from the parent to your worktree.
- Re-read files you need — they may be stale.
- Do NOT modify files outside your worktree.`, worktreePath, parentCwd)
}
```

---

### 阶段 4：AgentTool 决策树重构（预计 3-4 天）

#### 4.1 Input Schema 扩展

**文件**: `internal/tool/agentool/agentool.go`（修改）

```go
type Input struct {
    Prompt          string `json:"prompt"`           // 原 task 重命名对齐
    SubagentType    string `json:"subagent_type,omitempty"`
    Description     string `json:"description,omitempty"`
    Model           string `json:"model,omitempty"`
    RunInBackground bool   `json:"run_in_background,omitempty"`
    Name            string `json:"name,omitempty"`       // teammate name → triggers teammate spawn
    TeamName        string `json:"team_name,omitempty"`  // team context
    Mode            string `json:"mode,omitempty"`       // "plan" for plan mode
    Isolation       string `json:"isolation,omitempty"`  // "worktree","remote"
    Cwd             string `json:"cwd,omitempty"`        // working directory override
}
```

#### 4.2 完整决策树实现

```go
func (t *AgentTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
    var in Input
    if err := json.Unmarshal(input, &in); err != nil {
        return nil, err
    }

    ch := make(chan *engine.ContentBlock, 4)
    go func() {
        defer close(ch)

        // 1. Resolve team name (from param or context)
        teamName := t.resolveTeamName(in.TeamName, uctx)

        // 2. Validation checks
        if teamName != "" && in.Name != "" {
            if t.isCurrentAgentTeammate() {
                ch <- errorBlock("Teammates cannot spawn other teammates")
                return
            }
            if t.isInProcessTeammate() && in.RunInBackground {
                ch <- errorBlock("In-process teammates cannot spawn background agents")
                return
            }
        }

        // 3. Decision tree
        // Path 1: Teammate spawn
        if teamName != "" && in.Name != "" {
            t.handleTeammateSpawn(ctx, in, teamName, uctx, ch)
            return
        }

        // Resolve agent definition
        selectedAgent, isFork := t.resolveAgent(in, uctx)

        // Fork recursive guard
        if isFork && IsInForkChild(uctx.QuerySource, uctx.Messages) {
            ch <- errorBlock("Fork is not available inside a forked worker")
            return
        }

        // Check required MCP servers
        if err := t.checkRequiredMcpServers(selectedAgent, uctx); err != nil {
            ch <- errorBlock(err.Error())
            return
        }

        // Resolve effective isolation
        effectiveIsolation := t.resolveIsolation(in.Isolation, selectedAgent)

        // Path 2: Remote isolation
        if effectiveIsolation == "remote" {
            t.handleRemotePath(ctx, in, selectedAgent, uctx, ch)
            return
        }

        // Determine async
        shouldRunAsync := t.shouldRunAsync(in, selectedAgent, uctx)

        // Build run params
        params := t.buildFullRunParams(in, selectedAgent, isFork, effectiveIsolation, uctx)

        // Path 3: Async execution
        if shouldRunAsync {
            t.handleAsyncPath(ctx, params, in, uctx, ch)
            return
        }

        // Path 4: Sync execution with foreground-to-background support
        t.handleSyncWithFgToBg(ctx, params, in, uctx, ch)
    }()
    return ch, nil
}
```

#### 4.3 resolveAgent 实现

```go
func (t *AgentTool) resolveAgent(in Input, uctx *tool.UseContext) (*agent.AgentDefinition, bool) {
    // Fork path: no subagent_type + fork enabled
    if in.SubagentType == "" && agent.IsForkSubagentEnabled() {
        return &agent.ForkAgent, true
    }

    // Explicit type
    effectiveType := in.SubagentType
    if effectiveType == "" {
        effectiveType = "general-purpose"
    }

    if t.cfg.Loader != nil {
        if def, ok := t.cfg.Loader.FindByType(effectiveType); ok {
            // Check denied agents
            if t.isAgentDenied(def, uctx) {
                return nil, false
            }
            return def, false
        }
    }

    return &agent.GeneralPurposeAgent, false
}
```

#### 4.4 shouldRunAsync 多条件合并

```go
func (t *AgentTool) shouldRunAsync(in Input, agentDef *agent.AgentDefinition, uctx *tool.UseContext) bool {
    if in.RunInBackground {
        return true
    }
    if agentDef != nil && agentDef.Background {
        return true
    }
    // Coordinator mode forces async
    if t.isCoordinatorMode() {
        return true
    }
    // Fork subagent forces async
    if agent.IsForkSubagentEnabled() && in.SubagentType == "" {
        return true
    }
    return false
}
```

---

### 阶段 5：异步生命周期完善（预计 2-3 天）

#### 5.1 runAsyncAgentLifecycle 完善

**文件**: `internal/agent/async_lifecycle.go`（修改）

```go
// RunAsyncAgentLifecycle drives a background agent from spawn to notification.
// Aligned with agentToolUtils.ts runAsyncAgentLifecycle.
func (m *AsyncLifecycleManager) RunAsyncAgentLifecycle(
    ctx context.Context,
    params RunAgentParams,
    metadata AgentMetadata,
    rootSetAppState func(update func(interface{}) interface{}),
    enableSummarization bool,
    getWorktreeResult func() (*WorktreeResult, error),
) error {
    agentMessages := make([]*engine.Message, 0)
    tracker := NewProgressTracker(params.ExistingAgentID)

    // Run agent and collect messages
    result := m.runner.RunAgent(ctx, params)

    // Progress tracking
    tracker.Update(result)

    // Completion notification
    if result.Error != nil {
        if ctx.Err() != nil {
            // Cancelled
            m.notifications.Push(Notification{
                Type:    NotificationTypeStatus,
                AgentID: params.ExistingAgentID,
                Message: "Agent was cancelled",
            })
        } else {
            // Failed
            m.notifications.Push(Notification{
                Type:    NotificationTypeError,
                AgentID: params.ExistingAgentID,
                Message: result.Error.Error(),
            })
        }
    } else {
        // Success
        worktreeResult, _ := getWorktreeResult()
        m.notifications.Push(Notification{
            Type:    NotificationTypeComplete,
            AgentID: params.ExistingAgentID,
            Message: formatCompletionNotification(result, worktreeResult),
        })
    }

    // Disk output
    if m.diskOutput != nil {
        m.diskOutput.WriteOutput(params.ExistingAgentID, result.Output)
    }

    return nil
}
```

#### 5.2 进度跟踪增强

**文件**: `internal/agent/progress.go`（修改）

```go
type ProgressUpdate struct {
    ToolUseCount int
    TokenCount   int
    LastActivity *ActivityInfo
}

type ActivityInfo struct {
    ToolName            string
    ActivityDescription string
    Timestamp           time.Time
}

// UpdateFromMessage processes a message and updates progress.
func (pt *ProgressTracker) UpdateFromMessage(msg *engine.Message, toolNames []string) {
    // Count tool uses in assistant messages
    if msg.Role == engine.RoleAssistant {
        for _, block := range msg.Content {
            if block.Type == engine.ContentTypeToolUse {
                pt.totalToolUses++
                pt.lastActivity = &ActivityInfo{
                    ToolName:  block.ToolName,
                    Timestamp: time.Now(),
                }
            }
        }
    }
}
```

#### 5.3 Summarization 集成

**新文件**: `internal/agent/summarization.go`

```go
// AgentSummarizer periodically generates summaries of a running agent's progress.
type AgentSummarizer struct {
    agentID string
    taskID  string
    stop    chan struct{}
    // ... LLM caller for generating summaries
}

func StartAgentSummarization(taskID, agentID string, caller engine.ModelCaller) *AgentSummarizer { ... }
func (s *AgentSummarizer) Stop() { ... }
```

---

### 阶段 6：前台转后台机制（预计 1-2 天）

#### 6.1 ForegroundAgent 结构

**文件**: `internal/agent/fg_to_bg.go`（修改/重写）

```go
// ForegroundAgentRegistration tracks a sync agent that can be backgrounded.
type ForegroundAgentRegistration struct {
    TaskID           string
    AgentID          string
    BackgroundSignal chan struct{}
    CancelAutoBg     func()
}

// RegisterAgentForeground creates a foreground agent registration
// that can be converted to background on demand.
func RegisterAgentForeground(
    agentID string,
    description string,
    autoBackgroundMs int,
) *ForegroundAgentRegistration {
    reg := &ForegroundAgentRegistration{
        TaskID:           agentID,
        AgentID:          agentID,
        BackgroundSignal: make(chan struct{}),
    }

    // Auto-background timer
    if autoBackgroundMs > 0 {
        timer := time.AfterFunc(time.Duration(autoBackgroundMs)*time.Millisecond, func() {
            close(reg.BackgroundSignal)
        })
        reg.CancelAutoBg = timer.Stop
    }

    return reg
}

// BackgroundAgent triggers the foreground-to-background conversion.
func (reg *ForegroundAgentRegistration) BackgroundAgent() {
    select {
    case <-reg.BackgroundSignal:
        // Already backgrounded
    default:
        close(reg.BackgroundSignal)
    }
}
```

#### 6.2 Sync 执行中的 Race 逻辑

在 AgentTool 的 sync 路径中实现 Race：

```go
func (t *AgentTool) handleSyncWithFgToBg(ctx context.Context, params agent.RunAgentParams, in Input, uctx *tool.UseContext, ch chan<- *engine.ContentBlock) {
    reg := RegisterAgentForeground(params.ExistingAgentID, in.Description, 0)

    // Create channels for the race
    type messageResult struct {
        result *agent.AgentRunResult
        err    error
    }
    messageCh := make(chan messageResult, 1)

    go func() {
        result := t.cfg.Runner.RunAgent(ctx, params)
        messageCh <- messageResult{result: result}
    }()

    select {
    case mr := <-messageCh:
        // Agent completed normally
        if mr.result.Error != nil {
            ch <- errorBlock(mr.result.Error.Error())
            return
        }
        ch <- &engine.ContentBlock{
            Type: engine.ContentTypeText,
            Text: agent.FormatAgentResult(mr.result, 50000),
        }

    case <-reg.BackgroundSignal:
        // Foreground-to-background conversion triggered
        // Return async result immediately
        ch <- &engine.ContentBlock{
            Type: engine.ContentTypeText,
            Text: fmt.Sprintf("Agent %s moved to background. Output: %s",
                params.ExistingAgentID,
                agent.GetTaskOutputPath(params.ExistingAgentID)),
        }
    }
}
```

---

### 阶段 7：InProcess Teammate 完善（预计 2-3 天）

#### 7.1 runLoop 实现完善

**文件**: `internal/agent/inprocess_teammate.go`（修改）

```go
// runLoop is the main processing loop for an in-process teammate.
// It polls the mailbox for incoming messages and processes them.
func (t *InProcessTeammate) runLoop(ctx context.Context) {
    defer close(t.done)

    // Process initial task
    if t.Definition.Task != "" {
        t.processTask(ctx, t.Definition.Task)
    }

    // Poll mailbox for incoming messages
    ticker := time.NewTicker(t.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            msgs := t.mailbox.Read()
            for _, msg := range msgs {
                if ctx.Err() != nil {
                    return
                }
                t.processMailboxMessage(ctx, msg)
            }
        }
    }
}

// processTask executes a task using the agent runner.
func (t *InProcessTeammate) processTask(ctx context.Context, task string) {
    params := agent.RunAgentParams{
        AgentDef:       &t.Definition,
        Task:           task,
        Background:     true, // in-process teammates are async
        TeamName:       t.TeamName,
    }

    result := t.runner.RunAgent(ctx, params)

    // Send result back to team leader via message bus
    if t.bus != nil {
        t.bus.Send(agent.AgentMessage{
            FromAgentID: t.AgentID,
            ToAgentID:   t.Definition.ParentID,
            Content:     result.Output,
        })
    }
}
```

#### 7.2 spawnTeammate 路由

**新文件**: `internal/agent/spawn_teammate.go`

```go
// SpawnTeammate creates and starts an in-process teammate.
// This is the Go equivalent of spawnTeammate() in AgentTool.tsx.
func SpawnTeammate(
    name string,
    prompt string,
    teamName string,
    agentDef *AgentDefinition,
    runner *AgentRunner,
    teamManager *TeamManager,
    registry *MailboxRegistry,
    bus *MessageBus,
) (*InProcessTeammate, error) {
    agentID := uuid.New().String()

    // Add to team
    if err := teamManager.AddMember(teamName, agentID, agentDef.AgentType, "worker"); err != nil {
        return nil, err
    }

    // Create teammate
    teammate := NewInProcessTeammate(InProcessTeammateConfig{
        AgentID:    agentID,
        Definition: *agentDef,
        TeamName:   teamName,
        Runner:     runner,
        Registry:   registry,
        Bus:        bus,
    })
    teammate.Definition.Task = prompt

    // Start the teammate
    if err := teammate.Start(context.Background()); err != nil {
        return nil, err
    }

    return teammate, nil
}
```

---

### 阶段 8：协调器模式增强（预计 1-2 天）

#### 8.1 系统提示词集成

**文件**: `internal/agent/coordinator.go`（修改）

```go
// CoordinatorSystemPrompt returns the system prompt for coordinator mode.
// Limits the coordinator to only using Task, TaskStop, and SendMessage tools.
const CoordinatorSystemPrompt = `You are a coordinator agent. Your role is to:
1. Break down complex tasks into independent work items
2. Spawn worker agents to handle each work item
3. Monitor progress and handle failures
4. Synthesize results when all workers complete

You may ONLY use these tools:
- Task: Spawn worker agents
- TaskStop: Cancel a running agent
- SendMessage: Communicate with agents

Do NOT attempt to do the work yourself. Delegate everything.`

// IsCoordinatorMode checks if the current session is in coordinator mode.
func IsCoordinatorMode() bool {
    return os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") == "true" ||
        os.Getenv("CLAUDE_CODE_COORDINATOR_MODE") == "1"
}
```

#### 8.2 强制异步和工具限制

```go
// GetCoordinatorToolFilter returns the tool filter for coordinator mode.
func GetCoordinatorToolFilter() func(toolName string) bool {
    return func(toolName string) bool {
        return CoordinatorModeAllowedTools[toolName]
    }
}
```

---

### 阶段 9：Agent 记忆系统（预计 1-2 天）

#### 9.1 三级记忆作用域

**文件**: `internal/agent/memory.go`（修改）

```go
type MemoryScope string

const (
    MemoryScopeUser    MemoryScope = "user"
    MemoryScopeProject MemoryScope = "project"
    MemoryScopeLocal   MemoryScope = "local"
)

// MemoryManager handles persistent agent memory across sessions.
type MemoryManager struct {
    baseDir string
}

// MemoryPaths returns the directory paths for each memory scope.
func (mm *MemoryManager) MemoryPaths(agentType string, scope MemoryScope) string {
    switch scope {
    case MemoryScopeUser:
        return filepath.Join(os.UserHomeDir(), ".claude", "agent-memory", agentType)
    case MemoryScopeProject:
        return filepath.Join(mm.baseDir, ".claude", "agent-memory", agentType)
    case MemoryScopeLocal:
        return filepath.Join(mm.baseDir, ".claude", "local-agent-memory", agentType)
    default:
        return ""
    }
}

// LoadMemory reads the agent's memory file and returns it as a prompt string.
func (mm *MemoryManager) LoadMemory(agentType string, scope MemoryScope) (string, error) { ... }

// SaveMemory persists the agent's memory to disk.
func (mm *MemoryManager) SaveMemory(agentType string, scope MemoryScope, content string) error { ... }
```

#### 9.2 Snapshot 机制

```go
// CheckMemorySnapshot checks if a newer snapshot is available.
func (mm *MemoryManager) CheckMemorySnapshot(agentType string, scope MemoryScope) (*SnapshotCheckResult, error) { ... }

// InitializeFromSnapshot copies a project snapshot to local memory.
func (mm *MemoryManager) InitializeFromSnapshot(agentType string, scope MemoryScope, timestamp string) error { ... }
```

---

### 阶段 10：Agent 恢复增强（预计 1-2 天）

#### 10.1 Conversation Replay

**文件**: `internal/agent/resume.go`（修改）

```go
// ResumeAgent loads a checkpoint and resumes agent execution.
// Aligned with claude-code-main's agentResume.ts.
func (r *AgentRunner) ResumeAgent(ctx context.Context, rm *ResumeManager, agentID string) (*AgentRunResult, error) {
    cp, err := rm.LoadCheckpoint(agentID)
    if err != nil || cp == nil {
        return nil, fmt.Errorf("no checkpoint found for agent %s", agentID)
    }

    // Reconstruct params from checkpoint
    params := RunAgentParams{
        AgentDef:        &cp.Definition,
        Task:            cp.Definition.Task,
        WorkDir:         cp.WorkDir,
        ExistingAgentID: cp.AgentID,
        Background:      cp.Background,
    }

    // If worktree was in use, restore it
    if cp.WorktreeDir != "" {
        params.WorkDir = cp.WorktreeDir
        params.IsolationMode = IsolationWorktree
    }

    return r.RunAgent(ctx, params), nil
}
```

#### 10.2 Checkpoint 自动保存

在 `executeLoop` 中添加定期 checkpoint：

```go
// Save checkpoint every N turns
if turnCount % 5 == 0 && r.resumeManager != nil {
    cp := &AgentCheckpoint{
        AgentID:      agentID,
        Definition:   def,
        Status:       AgentStatusRunning,
        TurnCount:    turnCount,
        MaxTurns:     maxTurns,
        WorkDir:      workDir,
        WorktreeDir:  worktreePath,
        MessageCount: len(allMessages),
    }
    _ = r.resumeManager.SaveCheckpoint(cp)
}
```

---

### 阶段 11：Worktree 隔离完善（预计 1 天）

#### 11.1 变更检测

**文件**: `internal/agent/worktree.go`（修改）

```go
// WorktreeHasChanges checks if the worktree has uncommitted changes.
// Aligned with claude-code-main's hasWorktreeChanges.
func WorktreeHasChanges(worktreePath string) (bool, error) {
    // git diff --quiet HEAD
    cmd := exec.Command("git", "-C", worktreePath, "diff", "--quiet", "HEAD")
    err := cmd.Run()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            // Exit code 1 means there are changes
            return exitErr.ExitCode() == 1, nil
        }
        return false, err
    }
    return false, nil // exit code 0 = no changes
}

// WorktreeHasChangesFromCommit checks against a specific commit.
func WorktreeHasChangesFromCommit(worktreePath, headCommit string) (bool, error) {
    cmd := exec.Command("git", "-C", worktreePath, "diff", "--quiet", headCommit)
    err := cmd.Run()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return exitErr.ExitCode() == 1, nil
        }
        return false, err
    }
    return false, nil
}
```

#### 11.2 清理逻辑完善

```go
// CleanupWorktreeIfNeeded removes worktree if no changes, keeps if changed.
// Returns worktree info for notification embedding.
func (wm *WorktreeManager) CleanupWorktreeIfNeeded(agentID, worktreePath, headCommit string) (*WorktreeResult, error) {
    if worktreePath == "" {
        return &WorktreeResult{}, nil
    }

    if headCommit != "" {
        changed, err := WorktreeHasChangesFromCommit(worktreePath, headCommit)
        if err == nil && !changed {
            _ = wm.RemoveWorktree(agentID, worktreePath)
            return &WorktreeResult{}, nil
        }
    }

    slog.Info("agent: keeping worktree with changes",
        slog.String("path", worktreePath))
    return &WorktreeResult{
        WorktreePath: worktreePath,
    }, nil
}
```

---

### 阶段 12：团队工具集成（预计 1-2 天）

#### 12.1 TeamCreate / TeamDelete 工具

已有 `internal/tool/teamcreate/` 和 `internal/tool/teamdelete/`，需要验证和完善：
- 确保与 TeamManager 正确集成
- 验证 MessageBus 订阅
- 添加权限检查

#### 12.2 SendMessage 工具增强

**文件**: `internal/tool/sendmessage/`（修改）

```go
// 增加 agentNameRegistry 查找
// 支持按 name 发送（非 agentID）
func (t *SendMessageTool) resolveRecipient(name string, uctx *tool.UseContext) (string, error) {
    // 1. 直接 agentID 查找
    // 2. 按 name 在 agentNameRegistry 查找
    // 3. 按 team member name 查找
}
```

---

## 四、文件变更清单

### 4.1 修改的现有文件

| 文件 | 变更内容 | 阶段 |
|------|----------|------|
| `internal/agent/types.go` | 增加 GetSystemPrompt 回调、SourceDetail、PluginName | P1 |
| `internal/agent/builtin_agents.go` | 完善 ForkAgent、对齐 Explore/Plan 定义 | P1 |
| `internal/agent/loader.go` | AgentDefinitionsResult、优先级合并、JSON验证 | P1 |
| `internal/agent/runner.go` | Hook执行、MCP初始化、Permission覆盖、executeLoop增强 | P2 |
| `internal/agent/toolfilter.go` | ResolveAgentToolsFull、Agent(x,y)解析 | P2 |
| `internal/agent/prompt.go` | omitClaudeMd/omitGitStatus优化 | P2 |
| `internal/agent/fork.go` | 完全重写：字节级缓存共享、递归保护 | P3 |
| `internal/tool/agentool/agentool.go` | Input扩展、完整4路决策树、resolveAgent | P4 |
| `internal/agent/async_lifecycle.go` | RunAsyncAgentLifecycle完善、进度事件 | P5 |
| `internal/agent/progress.go` | UpdateFromMessage、ActivityInfo | P5 |
| `internal/agent/fg_to_bg.go` | Race模式、ForegroundAgentRegistration | P6 |
| `internal/agent/inprocess_teammate.go` | runLoop完善、processTask | P7 |
| `internal/agent/coordinator_mode.go` | 系统提示词、强制异步 | P8 |
| `internal/agent/memory.go` | 三级作用域、Snapshot机制 | P9 |
| `internal/agent/resume.go` | Conversation replay、自动checkpoint | P10 |
| `internal/agent/worktree.go` | 变更检测完善、清理逻辑 | P11 |

### 4.2 新建文件

| 文件 | 职责 | 阶段 |
|------|------|------|
| `internal/agent/schema.go` | Agent JSON Schema 验证 | P1 |
| `internal/agent/hooks.go` | SubagentStart/End hook 执行框架 | P2 |
| `internal/agent/mcp_init.go` | Agent MCP servers 初始化 | P2 |
| `internal/agent/summarization.go` | 周期性进度摘要生成 | P5 |
| `internal/agent/spawn_teammate.go` | spawnTeammate 路由逻辑 | P7 |

---

## 五、阶段依赖关系

```
P1 (Agent定义) ──┬── P2 (Runner核心)
                 │       │
                 │       ├── P3 (Fork子Agent)
                 │       │
                 │       ├── P5 (异步生命周期)
                 │       │       │
                 │       │       └── P6 (前台转后台)
                 │       │
                 │       └── P4 (AgentTool决策树) ← 依赖 P3, P5, P6
                 │
                 ├── P7 (InProcess Teammate) ← 依赖 P2, P4
                 │
                 ├── P8 (协调器) ← 依赖 P4, P5
                 │
                 ├── P9 (Agent记忆) ← 独立
                 │
                 ├── P10 (Agent恢复) ← 依赖 P2
                 │
                 ├── P11 (Worktree) ← 依赖 P2
                 │
                 └── P12 (团队工具) ← 依赖 P7
```

---

## 六、工作量估计

| 阶段 | 内容 | 预计工时 | 优先级 |
|------|------|----------|--------|
| P1 | Agent定义体系完善 | 2-3天 | ★★★★★ |
| P2 | AgentRunner核心执行 | 3-4天 | ★★★★★ |
| P3 | Fork子Agent完整实现 | 2-3天 | ★★★★☆ |
| P4 | AgentTool决策树重构 | 3-4天 | ★★★★★ |
| P5 | 异步生命周期完善 | 2-3天 | ★★★★☆ |
| P6 | 前台转后台机制 | 1-2天 | ★★★☆☆ |
| P7 | InProcess Teammate | 2-3天 | ★★★☆☆ |
| P8 | 协调器模式增强 | 1-2天 | ★★★☆☆ |
| P9 | Agent记忆系统 | 1-2天 | ★★☆☆☆ |
| P10 | Agent恢复增强 | 1-2天 | ★★☆☆☆ |
| P11 | Worktree隔离完善 | 1天 | ★★☆☆☆ |
| P12 | 团队工具集成 | 1-2天 | ★★☆☆☆ |
| **总计** | | **20-31天** | |

---

## 七、测试策略

### 7.1 单元测试（每阶段必须）

```
internal/agent/pool_test.go        — 已有，扩展覆盖
internal/agent/toolfilter_test.go  — 新增 ResolveAgentToolsFull 测试
internal/agent/fork_test.go        — 新增字节级缓存验证
internal/agent/memory_test.go      — 新增三级作用域测试
internal/agent/resume_test.go      — 新增 checkpoint/replay 测试
```

### 7.2 集成测试

```
test/integration/agent_sync_test.go    — 同步 agent 完整流程
test/integration/agent_async_test.go   — 异步 agent + 通知
test/integration/agent_fork_test.go    — Fork + 缓存验证
test/integration/agent_team_test.go    — 团队协作流程
test/integration/agent_fgtobg_test.go  — 前台转后台
```

### 7.3 验证命令

```bash
# 编译检查
& "D:\Program Files\Go\bin\go.exe" build ./...

# 单元测试
& "D:\Program Files\Go\bin\go.exe" test ./internal/agent/... -v

# 集成测试
& "D:\Program Files\Go\bin\go.exe" test ./test/integration/... -v -timeout 120s
```

---

## 八、关键设计决策

### 8.1 Go 语言适配

| TypeScript 模式 | Go 等价实现 |
|------------------|-------------|
| `async function*` (AsyncGenerator) | `chan *engine.Message` 或 callback |
| `Promise.race()` | `select {}` 多路复用 |
| `void asyncFn()` (fire-and-forget) | `go func() { ... }()` |
| `AbortController` | `context.WithCancel` |
| `AsyncLocalStorage` | `context.Value` |
| `Map<string, AgentId>` (agentNameRegistry) | `sync.Map` 或 `map + sync.RWMutex` |
| `process.env.XXXX` | `os.Getenv("XXXX")` |
| Frontmatter解析 | `github.com/adrg/frontmatter` (已集成) |
| Zod schema验证 | 手动验证或 `github.com/santhosh-tekuri/jsonschema` |

### 8.2 并发安全

- 所有 Manager 结构体使用 `sync.RWMutex`
- Notification/Mailbox 使用有界 channel
- Context 传播确保级联取消
- 避免 goroutine 泄漏：`defer close(done)`

### 8.3 错误处理

- Agent 失败不应导致父 agent 崩溃
- 所有异步 agent 必须捕获 panic
- MCP 连接失败优雅降级（跳过该 MCP 工具集）
- Worktree 清理失败不阻塞通知发送

---

## 九、执行顺序建议

**推荐开发顺序：P1 → P2 → P4 → P3 → P5 → P6 → P7 → P8 → P11 → P9 → P10 → P12**

理由：
1. P1 (定义) 和 P2 (执行核心) 是所有后续阶段的基础
2. P4 (AgentTool) 是用户触达的入口，尽早完成方便测试
3. P3 (Fork) 是性能关键路径，但可以在 P4 之后补充
4. P5/P6 完善异步能力，P7/P8 构建团队协作
5. P9/P10/P11/P12 是增强功能，可以并行或延后
