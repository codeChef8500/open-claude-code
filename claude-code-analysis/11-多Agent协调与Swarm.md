# Claude Code 源码深度解读 — 11 多 Agent 协调与 Swarm

> 覆盖文件：`tools/AgentTool/AgentTool.tsx`（234KB）、`tools/AgentTool/runAgent.ts`（35KB）、`tools/AgentTool/forkSubagent.ts`、`tools/AgentTool/loadAgentsDir.ts`、`tools/AgentTool/agentToolUtils.ts`、`tools/AgentTool/agentMemory.ts`、`tools/AgentTool/resumeAgent.ts`、`constants/tools.ts`、`tasks/LocalAgentTask/`、`tasks/RemoteAgentTask/`

---

## 1. 模块职责概述

Claude Code 支持多 Agent 协作模式，包括：

- **Sub-Agent（子 Agent）**：由主 Agent 通过 `AgentTool` 生成的临时 Agent（同步或后台）
- **Fork Agent**：继承完整父对话上下文的分叉工作者（feature `FORK_SUBAGENT`）
- **Swarm Teammate（队友）**：使用独立 tmux 进程运行的持久化 Agent（`AGENT_SWARMS` 特性）
- **In-Process Teammate**：在同进程中以独立事件循环运行的 Agent（更高效）
- **Remote Agent**：在 CCR 云端容器中运行的 Agent（ant-only）
- **Coordinator**：多 Agent 模式下专职调度的协调者（`COORDINATOR_MODE`）

---

## 2. Agent 定义体系（`loadAgentsDir.ts`）

### 2.1 三类 Agent 来源

```typescript
// 来源优先级（低→高，后者覆盖前者）：
// built-in < plugin < user < project < flag < managed(policy)

export type AgentDefinition =
  | BuiltInAgentDefinition    // source: 'built-in'（内置，代码中定义）
  | CustomAgentDefinition     // source: 'userSettings'|'projectSettings'|'policySettings'|'flagSettings'
  | PluginAgentDefinition     // source: 'plugin'
```

### 2.2 Agent Frontmatter 完整字段

```typescript
// .claude/agents/<name>.md 的 YAML frontmatter 支持的完整字段：
type AgentDefinitionFields = {
  agentType: string          // Agent 类型名，全局唯一标识
  whenToUse: string          // 给 LLM 看的调用提示
  tools?: string[]           // ['*'] = 通配（允许所有工具），['Bash', 'Read'] = 精确限制
  disallowedTools?: string[] // 额外禁止的工具（在 tools 过滤后再排除）
  skills?: string[]          // 预加载的 Skill（斜线命令）列表
  mcpServers?: AgentMcpServerSpec[] // 专属 MCP 服务器（字符串引用 or 内联配置）
  hooks?: HooksSettings      // 注册到 Agent 生命周期的钩子
  color?: AgentColorName     // UI 颜色预设（覆盖自动分配）
  model?: string             // 'inherit'|'sonnet'|'opus'|'haiku' 等
  effort?: EffortValue       // 思考力度级别
  permissionMode?: PermissionMode // 'default'|'acceptEdits'|'plan'|'bypassPermissions'|'bubble'
  maxTurns?: number          // 最大 agentic turns 数（防止无限循环）
  memory?: 'user'|'project'|'local' // 持久化记忆范围
  isolation?: 'worktree'|'remote'   // 隔离模式（'remote' 仅 ant-only）
  background?: boolean       // true = 总是以后台模式运行
  initialPrompt?: string     // 追加到第一轮用户消息（支持斜线命令）
  omitClaudeMd?: boolean     // 不加载 CLAUDE.md 层级（节省 token）
  criticalSystemReminder_EXPERIMENTAL?: string // 每轮重注入的关键提示
  requiredMcpServers?: string[] // 必须配置的 MCP 服务器（缺少则 Agent 不可用）
}
```

### 2.3 内置 Agent 清单（`builtInAgents.ts`）

| 内置 Agent | 权限模式 | 工具范围 | 特点 |
|-----------|---------|---------|------|
| `general-purpose` | acceptEdits | `['*']` | 默认子 Agent，通用 |
| `Explore` | acceptEdits | Read + Grep + Glob | omitClaudeMd + omitGitStatus |
| `Plan` | acceptEdits | Read | omitClaudeMd + omitGitStatus |
| `fork` | bubble | `['*']` + useExactTools | 继承父对话，缓存友好 |

---

## 3. `AgentTool.call()` 决策树（核心路由逻辑）

```
AgentTool.call({
  prompt, subagent_type, description, model, run_in_background,
  name, team_name, mode, isolation, cwd
})
           │
           ├─ [1] teamName && name
           │       └── spawnTeammate() → { status: 'teammate_spawned', tmux_session_name, ... }
           │
           ├─ [2] effectiveIsolation === 'remote' (ant-only)
           │       └── teleportToRemote() → registerRemoteAgentTask()
           │               → { status: 'remote_launched', taskId, sessionUrl }
           │
           ├─ [3] shouldRunAsync === true
           │       │  (条件之一满足即为 true，见 3.1)
           │       └── registerAsyncAgent() + void runAsyncAgentLifecycle()
           │               → { status: 'async_launched', agentId, outputFile }
           │
           └─ [4] 默认同步路径
                   └── for await (runAgent(...)) → 流式执行
                           → { status: 'completed', content, usage }
```

### 3.1 `shouldRunAsync` 触发条件

```typescript
const shouldRunAsync =
  run_in_background === true ||          // 显式后台标志
  selectedAgent.background === true ||   // Agent 定义中 background: true
  isCoordinator ||                       // 协调者模式（所有子 Agent 都异步）
  isForkSubagentEnabled() ||             // Fork 实验模式（全部异步）
  (feature('KAIROS') && appState.kairosEnabled) ||  // KAIROS 守护进程模式
  (proactiveModule?.isProactiveActive() ?? false)   // Proactive 模式
```

### 3.2 输出 Schema

```typescript
// 四种返回状态，对应四条分支
type Output =
  | { status: 'completed';        agentId, content, usage, totalTokens, ... }
  | { status: 'async_launched';   agentId, outputFile, canReadOutputFile }
  | { status: 'teammate_spawned'; tmux_session_name, tmux_window_name, tmux_pane_id, ... }
  | { status: 'remote_launched';  taskId, sessionUrl, outputFile }
```

---

## 4. Agent 执行核心（`runAgent.ts`，35KB）

### 4.1 初始化序列

```typescript
// 1. 确定 agentId（早期生成，用于 worktree slug）
const agentId = override?.agentId ?? createAgentId()

// 2. 解析模型
const resolvedAgentModel = getAgentModel(agentDef.model, parentModel, modelParam, permissionMode)

// 3. 执行 SubagentStart hooks（可注入额外上下文到初始消息）
for await (const hookResult of executeSubagentStartHooks(agentId, agentType, signal)) {
  additionalContexts.push(...hookResult.additionalContexts)
}

// 4. 注册 frontmatter hooks（生命周期钩子，isAgent=true 将 Stop → SubagentStop）
if (agentDefinition.hooks && hooksAllowedForThisAgent) {
  registerFrontmatterHooks(rootSetAppState, agentId, agentDefinition.hooks, ..., true)
}

// 5. 预加载 skills（并发加载，添加为初始用户消息）
const loaded = await Promise.all(validSkills.map(async s => ({
  content: await s.getPromptForCommand('', toolUseContext)
})))

// 6. 初始化 Agent 专属 MCP 服务器
const { clients, tools, cleanup } = await initializeAgentMcpServers(agentDef, parentMcpClients)

// 7. 构建 AgentOptions（隔离上下文）
const agentOptions = {
  isNonInteractiveSession: isAsync ? true : parent.isNonInteractive,
  thinkingConfig: useExactTools ? parent.thinkingConfig : { type: 'disabled' },
  mainLoopModel: resolvedAgentModel,
  tools: mergeDedup([resolvedTools, agentMcpTools]),
  ...
}

// 8. 创建 ToolUseContext
const agentToolUseContext = createSubagentContext(toolUseContext, {
  agentId, options: agentOptions,
  shareSetAppState: !isAsync,  // 同步 Agent 共享父状态写入
  abortController: isAsync ? new AbortController() : parent.abortController,
  ...
})
```

### 4.2 权限上下文隔离逻辑

```typescript
// 异步 Agent 的权限上下文修改
const agentGetAppState = () => {
  let ctx = state.toolPermissionContext

  // a. 覆盖权限模式（bypassPermissions/acceptEdits 不可被子 Agent 降级）
  if (agentPermissionMode && mode !== 'bypassPermissions' && mode !== 'acceptEdits') {
    ctx = { ...ctx, mode: agentPermissionMode }
  }

  // b. 异步 Agent 默认不弹权限对话框（无 UI）
  const shouldAvoidPrompts = permissionMode === 'bubble' ? false : isAsync
  if (shouldAvoidPrompts) ctx = { ...ctx, shouldAvoidPermissionPrompts: true }

  // c. 异步 + 可弹框（bubble 模式）→ 先等自动分类器再弹
  if (isAsync && !shouldAvoidPrompts) {
    ctx = { ...ctx, awaitAutomatedChecksBeforeDialog: true }
  }

  // d. allowedTools 覆盖 session 级权限（保留 cliArg 级别）
  if (allowedTools !== undefined) {
    ctx = { ...ctx, alwaysAllowRules: { cliArg: prev.cliArg, session: allowedTools } }
  }
  return { ...state, toolPermissionContext: ctx }
}
```

### 4.3 Token 节省优化（fleet-wide）

```typescript
// Explore/Plan 子 Agent 跳过 CLAUDE.md（saves ~5-15 Gtok/week）
const shouldOmitClaudeMd = agentDef.omitClaudeMd &&
  !override?.userContext &&
  getFeatureValue_CACHED_MAY_BE_STALE('tengu_slim_subagent_claudemd', true)

// Explore/Plan 子 Agent 跳过 gitStatus（saves ~1-3 Gtok/week）
const resolvedSystemContext =
  agentType === 'Explore' || agentType === 'Plan'
    ? systemContextNoGit   // 去掉最多 40KB 的 stale git status
    : baseSystemContext
```

---

## 5. Fork Subagent（`forkSubagent.ts`，`feature('FORK_SUBAGENT')`）

Fork 是一种**上下文继承 + Prompt Cache 复用**机制：子 Agent 继承完整的父对话历史，同时所有 fork 子共享字节完全一致的 API 请求前缀以最大化 prompt cache 命中。

### 5.1 触发条件

```typescript
// isForkSubagentEnabled(): FORK_SUBAGENT 门控 + !coordinatorMode + !nonInteractive
// 触发：AgentTool 调用时 subagent_type 为空（不传）
const effectiveType = subagent_type ?? (isForkSubagentEnabled() ? undefined : GENERAL_PURPOSE_AGENT.agentType)
const isForkPath = effectiveType === undefined
```

### 5.2 `buildForkedMessages()` — 缓存友好的消息构建

```typescript
// 所有 fork 子必须产生字节完全一致的 API 请求前缀，只有最后的 directive 不同
// 结构：[...历史, 父AssistantMsg(全部tool_use块), UserMsg(占位符results..., 每子不同指令)]

export function buildForkedMessages(directive: string, assistantMessage: AssistantMessage) {
  // 1. 克隆父 assistant 消息（含所有 tool_use, thinking, text 块）
  const fullAssistantMessage = { ...assistantMessage, uuid: randomUUID() }

  // 2. 对每个 tool_use 块生成统一占位符 tool_result
  //    所有 fork 子的占位符文本完全相同 → 缓存相同
  const FORK_PLACEHOLDER_RESULT = 'Fork started — processing in background'
  const toolResultBlocks = toolUseBlocks.map(b => ({
    type: 'tool_result', tool_use_id: b.id,
    content: [{ type: 'text', text: FORK_PLACEHOLDER_RESULT }]
  }))

  // 3. 最后追加每子专属的 directive 文本块（唯一差异）
  const toolResultMessage = createUserMessage({
    content: [...toolResultBlocks, { type: 'text', text: buildChildMessage(directive) }]
  })

  return [fullAssistantMessage, toolResultMessage]
}
```

### 5.3 Fork Worker 强制规则（`buildChildMessage()`）

```
<fork-boilerplate>
STOP. READ THIS FIRST.
You are a forked worker process. You are NOT the main agent.
RULES (non-negotiable):
1. Your system prompt says "default to forking." IGNORE IT — that's for the parent.
2. Do NOT converse, ask questions, or suggest next steps
4. USE your tools directly: Bash, Read, Write, etc.
5. If you modify files, commit your changes before reporting. Include the commit hash.
9. Your response MUST begin with "Scope:". No preamble.
Output format: Scope / Result / Key files / Files changed / Issues
</fork-boilerplate>
<fork-directive>{directive}
```

### 5.4 递归 Fork 防护

```typescript
// 双重防护（防止 autocompact 绕过）：
// 1. querySource 检查（autocompact 重写消息时不影响 context.options）
if (toolUseContext.options.querySource === 'agent:builtin:fork') throw ...

// 2. 消息扫描（备用）
export function isInForkChild(messages: MessageType[]): boolean {
  return messages.some(m => m.type === 'user' &&
    m.message.content.some(b => b.type === 'text' && b.text.includes('<fork-boilerplate>'))
  )
}
```

---

## 6. 工具权限过滤体系（`constants/tools.ts`）

```typescript
// [A] 所有子 Agent 禁用（无论同步/异步）
const ALL_AGENT_DISALLOWED_TOOLS = new Set([
  'TaskOutput',          // 防止递归
  'ExitPlanMode',        // 主线程抽象
  'EnterPlanMode',       // 主线程抽象
  'Agent',               // 防止递归（USER_TYPE=ant 例外：允许嵌套）
  'AskUserQuestion',     // 子 Agent 不能阻塞等待用户输入
  'TaskStop',            // 需要主线程任务状态
  'Workflow',            // 防止递归 workflow（feature('WORKFLOW_SCRIPTS') 时）
])

// [B] 后台异步 Agent 允许的工具（白名单）
const ASYNC_AGENT_ALLOWED_TOOLS = new Set([
  'Read', 'WebSearch', 'TodoWrite', 'Grep', 'WebFetch', 'Glob',
  'Bash', 'PowerShell',  // Shell tools
  'Edit', 'Write', 'NotebookEdit',
  'Skill', 'SyntheticOutput', 'ToolSearch',
  'EnterWorktree', 'ExitWorktree',
  // 注意：Agent tool 本身不在此列表 → 后台 Agent 不能再生成后台 Agent
])

// [C] 进程内队友（in-process teammate）额外允许
const IN_PROCESS_TEAMMATE_ALLOWED_TOOLS = new Set([
  'TaskCreate', 'TaskGet', 'TaskList', 'TaskUpdate',
  'SendMessage',
  // feature('AGENT_TRIGGERS') 时：
  'CronCreate', 'CronDelete', 'CronList',
])

// [D] 协调者模式（Coordinator）专用
const COORDINATOR_MODE_ALLOWED_TOOLS = new Set([
  'Agent', 'TaskStop', 'SendMessage', 'SyntheticOutput',
])
```

**工具过滤优先级：**
```
ALL_AGENT_DISALLOWED_TOOLS
    → CUSTOM_AGENT_DISALLOWED_TOOLS（自定义 Agent 的额外限制）
         → ASYNC_AGENT_ALLOWED_TOOLS（异步 Agent 的白名单）
              → IN_PROCESS_TEAMMATE_ALLOWED_TOOLS（队友额外补充）
                   → agent frontmatter tools/disallowedTools（最终精确裁剪）
```

---

## 7. Agent 持久记忆系统（`agentMemory.ts`）

```typescript
// 三种存储范围的记忆目录
type AgentMemoryScope = 'user' | 'project' | 'local'

// user scope:   ~/.claude/agent-memory/<agentType>/MEMORY.md
// project scope: .claude/agent-memory/<agentType>/MEMORY.md（进入 VCS）
// local scope:  .claude/agent-memory-local/<agentType>/MEMORY.md（不进 VCS）

// 云端 remote 环境下（CLAUDE_CODE_REMOTE_MEMORY_DIR）：
// local scope → <remoteDir>/projects/<gitRoot>/agent-memory-local/<agentType>/MEMORY.md

// 记忆快照机制（Memory Snapshot）：
// 1. project 中可包含 MEMORY_SNAPSHOT.md 作为初始记忆模板
// 2. 用户首次使用时从快照 copy 到 local/user scope
// 3. 项目更新快照后，提示用户决定是否合并（prompt-update action）
export async function loadAgentMemoryPrompt(agentType: string, scope: AgentMemoryScope): Promise<string> {
  // 同步调用，fire-and-forget mkdir（在 API round-trip 前完成）
  void ensureMemoryDirExists(memoryDir)
  return buildMemoryPrompt({ displayName: 'Persistent Agent Memory', memoryDir, extraGuidelines: [scopeNote] })
}
```

---

## 8. Agent 专属 MCP 服务器（`runAgent.ts` `initializeAgentMcpServers()`）

```typescript
// Agent frontmatter 中声明：
// mcpServers:
//   - "slack"                    ← 引用现有配置（共享连接，不清理）
//   - myServer: { command: ... } ← 内联定义（Agent 专属，完成后清理）

async function initializeAgentMcpServers(agentDef, parentClients) {
  for (const spec of agentDef.mcpServers) {
    if (typeof spec === 'string') {
      // 引用：getMcpConfigByName() → connectToServer()（已 memoize，共享）
      // 不加入 newlyCreatedClients → cleanup() 时不断开
    } else {
      // 内联：直接创建连接
      // 加入 newlyCreatedClients → agent 完成后 client.cleanup()
    }
  }
  // 返回：父 MCP clients + Agent 专属 clients 的合并集合
  return { clients: [...parentClients, ...agentClients], tools: agentTools, cleanup }
}
```

---

## 9. 后台 Agent 生命周期（`runAsyncAgentLifecycle()`）

```typescript
// agentToolUtils.ts
async function runAsyncAgentLifecycle({ taskId, abortController, makeStream, metadata, ... }) {
  // 1. makeStream() 创建 runAgent() 异步生成器
  const stream = makeStream(onCacheSafeParams)

  // 2. 可选：启动 Agent 摘要器（coordinator / fork / SDK 进度）
  if (enableSummarization) {
    startAgentSummarization({ agentId, onCacheSafeParams })  // 后台定期摘要
  }

  // 3. 消费消息流，更新进度
  for await (const msg of stream) {
    updateProgressFromMessage(agentId, msg)  // 推送进度到 AppState.tasks
  }

  // 4. 完成后清理 worktree（如有）
  const worktreeResult = await getWorktreeResult()  // 检查是否有变更，决定保留/删除

  // 5. completeAsyncAgent() → 状态切 'completed' + 写磁盘输出文件
  await completeAsyncAgent(taskId, { result, worktreeResult })

  // 6. enqueueAgentNotification() → 通知主 Agent（task-notification 消息）
  enqueueAgentNotification(taskId, description, result)
}
```

**自动后台转换（Auto-Background）：**
```typescript
const PROGRESS_THRESHOLD_MS = 2_000    // 2秒后显示"转为后台"提示
const AUTO_BACKGROUND_MS     = 120_000  // 2分钟后自动转为后台

// GrowthBook gate: tengu_auto_background_agents
// 或 env: CLAUDE_AUTO_BACKGROUND_TASKS=1
function getAutoBackgroundMs(): number {
  return (isEnvTruthy(process.env.CLAUDE_AUTO_BACKGROUND_TASKS) ||
    getFeatureValue_CACHED_MAY_BE_STALE('tengu_auto_background_agents', false))
    ? 120_000 : 0
}
```

---

## 10. Agent 恢复（`resumeAgent.ts`）

```typescript
// 从磁盘 transcript 恢复一个已中断的后台 Agent
export async function resumeAgentBackground({ agentId, prompt, toolUseContext, canUseTool }) {
  // 1. 加载历史 transcript（过滤孤立 thinking、unresolved tool_use、空白消息）
  const [transcript, meta] = await Promise.all([getAgentTranscript(agentId), readAgentMetadata(agentId)])
  const resumedMessages = filterOrphanedThinkingOnlyMessages(
    filterUnresolvedToolUses(transcript.messages)
  )

  // 2. 重建 contentReplacementState（确保 prompt cache 稳定性）
  const resumedReplacementState = reconstructForSubagentResume(
    toolUseContext.contentReplacementState, resumedMessages, transcript.contentReplacements
  )

  // 3. worktree 检查（bump mtime 防止 stale-cleanup 竞争）
  if (meta?.worktreePath) await fsp.utimes(worktreePath, now, now)

  // 4. Fork 恢复：重建父系统提示（byte-exact cache prefix）
  if (isResumedFork && toolUseContext.renderedSystemPrompt) {
    forkParentSystemPrompt = toolUseContext.renderedSystemPrompt
  }

  // 5. 接后台 Agent 生命周期继续运行
  void runAsyncAgentLifecycle({ taskId: agentId, makeStream: () => runAgent({ ...params }) })
}
```

---

## 11. Worktree 隔离详解（`isolation: 'worktree'`）

```typescript
// 创建 worktree（slug = "agent-{agentId前8位}"）
const slug = `agent-${earlyAgentId.slice(0, 8)}`
worktreeInfo = await createAgentWorktree(slug)
// → utils/worktree.ts: git worktree add <path> HEAD

// 注入路径翻译提示（仅 fork + worktree 组合时）
if (isForkPath && worktreeInfo) {
  promptMessages.push(createUserMessage({
    content: `You've inherited the conversation context above from a parent agent working in ${parentCwd}.
You are operating in an isolated git worktree at ${worktreeCwd} — same repository, same relative
file structure, separate working copy. Paths in the inherited context refer to the parent's working
directory; translate them to your worktree root. Re-read files before editing if the parent may have
modified them since they appear in the context. Your changes stay in this worktree and will not
affect the parent's files.`
  }))
}

// Agent 完成后的 worktree 清理
const changed = await hasWorktreeChanges(worktreePath, headCommit)  // git diff vs HEAD commit
if (!changed) {
  await removeAgentWorktree(worktreePath, worktreeBranch, gitRoot)  // 无变更 → 删除
  // 清除 metadata 中的 worktreePath（防止 resume 引用已删 worktree）
} else {
  keepWorktree(worktreePath, worktreeBranch)  // 有变更 → 保留，返回路径给调用者
}
```

---

## 12. 完整多 Agent 数据流

```
主 Claude
    │
    ├─[1] AgentTool({ name: 'worker1', team_name: 'myteam', prompt: '...' })
    │       └── spawnTeammate() → tmux 新窗口 → 独立 claude 进程
    │               CLAUDE_CODE_SESSION_KIND=daemon-worker
    │               ↔ SendMessageTool（UDS Socket）互通
    │
    ├─[2] AgentTool({ subagent_type: 'Explore', run_in_background: false })
    │       └── runAgent() [同步阻塞] → query() 迭代器消费
    │               │  工具：Read + Grep + Glob（omitClaudeMd + omitGitStatus）
    │               └── 完成后返回 { status: 'completed', content }
    │
    ├─[3] AgentTool({ run_in_background: true, isolation: 'worktree' })
    │       └── createAgentWorktree('agent-abc12345')
    │               └── registerAsyncAgent()
    │                       └── void runAsyncAgentLifecycle()
    │                               [独立 AbortController，不受 ESC 影响]
    │                               └── 完成 → enqueueAgentNotification()
    │                                          → <task-notification> 注入主对话
    │
    └─[4] AgentTool({ subagent_type: undefined })  [fork 实验开启时]
            └── buildForkedMessages(directive, assistantMessage)
                    [父 assistant msg + 占位 tool_results + 每子专属 directive]
                    └── runAgent({ useExactTools: true, forkContextMessages: history })
                            [继承父 thinkingConfig + 工具列表，cache-identical prefix]
```

---

## 13. 设计亮点

### 13.1 Fork 的 Prompt Cache 工程

Fork 子 Agent 的核心设计难点是让多个并发子 Agent 共享同一 prompt cache slot。解决方案：
1. **相同前缀**：所有子共享 `[历史 + 父 assistant msg + 占位 placeholder results]`（字节完全一致）
2. **差异后置**：每子的专属指令追加在最后的 text 块（缓存不命中的唯一部分）
3. **useExactTools**：继承父的精确工具数组和 thinkingConfig，避免工具定义序列化差异破坏缓存
4. **autocompact 防护**：`querySource` 写入 `context.options`（非消息，autocompact 不重写）

### 13.2 隔离与效率的权衡矩阵

| 模式 | AbortController | setAppState | 权限弹框 | 工具集 | 适用场景 |
|------|-----------------|-------------|---------|--------|---------|
| 同步子 Agent | 共享父 | 共享 | 弹框 | 完整 | 简单阻塞子任务 |
| 后台子 Agent | 独立（不受 ESC）| 通过 rootSetAppState | 不弹（auto-deny）| 白名单 | 独立长任务 |
| bubble 后台 | 独立 | rootSetAppState | 先分类器再弹 | 父工具集 | 需用户确认的后台任务 |
| Fork 子 | 独立 | rootSetAppState | 先分类器再弹 | 父工具集（精确复制）| 并行拆解，cache 共享 |
| Coordinator | 独立 | - | 通过协调者 | Agent+Stop+Send | 纯调度，不执行 |

### 13.3 工具禁用的分层设计
- `ALL_AGENT_DISALLOWED_TOOLS` 防止关键递归（AgentTool、AskUserQuestion）
- `ASYNC_AGENT_ALLOWED_TOOLS` 白名单保证异步 Agent 安全（无 UI 交互工具）
- frontmatter `disallowedTools` 允许 Agent 自我约束（如 Explore 不写文件）

### 13.4 MCP 的生命周期分类
- **引用型 MCP**（字符串名）：memoized 共享连接，Agent 结束不断开（下次 Agent 复用）
- **内联型 MCP**（对象定义）：Agent 专属创建，`cleanup()` 在 Agent 完成时调用

### 13.5 Slim Subagent 优化
通过 `omitClaudeMd` + `omitGitStatus` 减少 Explore/Plan 子 Agent 的上下文：
- **34M+ Explore 调用/周** × 省 ~5KB CLAUDE.md ≈ **5-15 Gtok/周节省**
- **Read-only Agent** 根本不需要 commit/PR/lint 指导，省得彻底干净
