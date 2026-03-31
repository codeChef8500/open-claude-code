# Claude Code 源码深度解读 — 11 多 Agent 协调与 Swarm

> 覆盖文件：`tools/AgentTool/`（234KB）、`tasks/LocalAgentTask/`、`tasks/RemoteAgentTask/`、`coordinator/`、`hooks/useSwarmInitialization.ts`、`hooks/useSwarmPermissionPoller.ts`

---

## 1. 模块职责概述

Claude Code 支持多 Agent 协作模式，包括：
- **Sub-Agent（子 Agent）**：由主 Agent 通过 `AgentTool` 生成的临时 Agent
- **Swarm（群体）**：多个持久化 Agent 组成的团队（AGENT_SWARMS 特性）
- **Coordinator（协调者）**：多 Agent 模式下的权限决策协调者
- **Teammate（队友）**：使用 Bridge 在独立进程中运行的 Agent

---

## 2. Agent 执行模型

### 2.1 两种基本执行模式

```
前台模式（默认）：
AgentTool.call() → runAgent() → QueryEngine.ask()
    │ 阻塞等待                          │
    │                              Agent 执行任务
    └── 返回结果 ◄─────────────────────┘

后台模式（run_in_background: true）：
AgentTool.call() → registerAsyncAgent() → 立即返回 { taskId }
                        │
                        └──► LocalAgentTask 后台运行
                             └── QueryEngine.ask()（独立 event loop）
```

### 2.2 `runAgent()` — 核心执行函数（`AgentTool/runAgent.ts`，35KB）

```typescript
async function runAgent(
  input: AgentToolInput,
  context: ToolUseContext,
  canUseTool: CanUseToolFn,
  onProgress?: ToolCallProgress,
): Promise<AgentToolResult> {
  
  // 1. 确定 Agent 模型
  const model = getAgentModel(input.model, context)
  
  // 2. 构建系统提示
  const systemPrompt = await buildEffectiveSystemPrompt({
    basePrompt: getSystemPrompt(),
    agentType: input.subagent_type,
    // ...
  })
  
  // 3. 过滤可用工具（Agent 不能生成子 Agent，除非明确允许）
  const agentTools = assembleToolPool(permissionContext, mcpTools)
    .filter(tool => canAgentUseTool(tool))
  
  // 4. 构建初始消息
  const initialMessages: Message[] = [
    createUserMessage(input.prompt)
  ]
  
  // 5. 创建 QueryEngine 并执行
  const engine = new QueryEngine({ ...agentConfig })
  
  let result = ''
  for await (const message of engine.ask(initialMessages)) {
    // 收集结果 + 推送进度
    if (message.type === 'assistant') {
      result = extractTextContent(message)
    }
    if (onProgress) {
      onProgress({ type: 'agent', status: 'running', output: result })
    }
  }
  
  return { result, usage: engine.totalUsage }
}
```

---

## 3. `LocalAgentTask` — 本地异步 Agent 管理

```typescript
// tasks/LocalAgentTask/LocalAgentTask.ts

type LocalAgentTask = {
  id: string               // 任务 ID（= Agent ID）
  status: 'running' | 'completed' | 'failed' | 'cancelled'
  description: string
  startTime: number
  endTime?: number
  result?: string
  error?: string
  progress?: string        // 最新进度文本
  tokenCount?: number      // 累计 token 数
  abort: () => void        // 取消函数
}

// 注册异步 Agent
export async function registerAsyncAgent(
  agentId: AgentId,
  runner: () => Promise<void>,
): Promise<void> {
  const task: LocalAgentTask = {
    id: agentId,
    status: 'running',
    // ...
  }
  
  activeTasks.set(agentId, task)
  
  // 在后台运行（不等待）
  runner().then(
    () => completeAgentTask(agentId),
    (err) => failAgentTask(agentId, err)
  )
}

// 更新任务进度（供 AgentTool 的 onProgress 回调使用）
export function updateProgressFromMessage(
  agentId: AgentId,
  message: Message,
): void {
  const task = activeTasks.get(agentId)
  if (!task) return
  
  task.progress = extractSummary(message)
  task.tokenCount = getTokenCountFromTracker(agentId)
  
  // 通知 UI 更新（AppState.tasks）
  notifyTaskUpdate(task)
}
```

---

## 4. Swarm 模式（AGENT_SWARMS 特性）

Swarm 是持久化的多 Agent 团队，允许多个 Agent 并发工作并通过消息通信。

### 4.1 Team 管理

```typescript
// tools/TeamCreateTool
// 创建一个命名的 Agent 团队
async call(input: { team_name: string, members: AgentSpec[] }) {
  const team = await createTeam(input.team_name, input.members)
  return { team_id: team.id }
}

// tools/TeamDeleteTool  
// 解散团队并终止所有成员 Agent
async call(input: { team_id: string }) {
  await deleteTeam(input.team_id)
}
```

### 4.2 Agent 间消息传递（`SendMessageTool`）

```typescript
// tools/SendMessageTool
async call(input: { to: string, message: string }) {
  // 通过 UDS (Unix Domain Socket) 发送消息
  const inbox = getAgentInbox(input.to)
  await inbox.send({
    from: context.agentId,
    content: input.message,
    timestamp: Date.now(),
  })
  
  return { delivered: true }
}
```

### 4.3 消息收件箱轮询（`useInboxPoller`）

```typescript
// hooks/useInboxPoller.ts (34KB)
function useInboxPoller({ onMessage }) {
  useEffect(() => {
    const poll = setInterval(async () => {
      const messages = await pollUDSInbox(currentAgentId)
      for (const msg of messages) {
        // 将收到的消息注入到当前 Agent 的对话
        onMessage({
          type: 'system',
          subtype: 'teammate_message',
          from: msg.from,
          content: msg.content,
        })
      }
    }, POLL_INTERVAL_MS)
    
    return () => clearInterval(poll)
  }, [currentAgentId])
}
```

### 4.4 Swarm 权限轮询（`useSwarmPermissionPoller`）

```typescript
// 在 Swarm Worker 模式下，权限请求通过轮询发送给 Coordinator
function useSwarmPermissionPoller({ onPermissionDecision }) {
  useEffect(() => {
    const poll = setInterval(async () => {
      // 检查 Coordinator 是否有对待决权限的响应
      const responses = await pollPermissionResponses()
      for (const resp of responses) {
        resolvePermission(resp.toolUseId, resp.decision)
      }
    }, PERMISSION_POLL_INTERVAL_MS)
    
    return () => clearInterval(poll)
  }, [])
}
```

---

## 5. Coordinator 模式

当 `awaitAutomatedChecksBeforeDialog` 为 true 时（多 Agent 协调场景），权限检查会先等待 Coordinator 决策：

```typescript
// hooks/toolPermission/handlers/coordinatorHandler.ts
async function handleCoordinatorPermission({ ctx, ... }) {
  // 1. 等待自动分类器检查（BashClassifier / YoloClassifier）
  const classifierResult = await waitForClassifierCheck(pendingClassifierCheck)
  
  // 2. 如果分类器返回确定结果，直接使用
  if (classifierResult?.decision === 'allow') {
    return ctx.buildAllow(input, { decisionReason: classifierResult })
  }
  
  if (classifierResult?.decision === 'deny') {
    return ctx.buildDeny(classifierResult.reason)
  }
  
  // 3. 分类器不确定 → 返回 null，让上层弹框
  return null
}
```

---

## 6. Worktree 隔离（`tools/AgentTool/forkSubagent.ts`）

当 `isolation: 'worktree'` 时，子 Agent 在独立的 Git worktree 中工作：

```typescript
// 创建 Git worktree
async function createAgentWorktree(agentId: AgentId): Promise<string> {
  const worktreePath = join(tmpdir(), `claude-agent-${agentId}`)
  
  // git worktree add <path> HEAD
  await exec(`git worktree add ${worktreePath} HEAD`)
  
  return worktreePath
}

// Agent 完成后检查变更
async function hasWorktreeChanges(path: string): Promise<boolean> {
  const diff = await exec(`git -C ${path} diff --name-only HEAD`)
  return diff.stdout.trim().length > 0
}

// Agent 完成后移除 worktree（可选：合并到主分支）
async function removeAgentWorktree(path: string): Promise<void> {
  await exec(`git worktree remove ${path} --force`)
}
```

---

## 7. Remote Agent（`tasks/RemoteAgentTask/`）

在 `isolation: 'remote'`（ant-only）模式下，Agent 在远程 CCR（Cloud Code Runner）环境执行：

```typescript
// 检查是否符合远程 Agent 条件
async function checkRemoteAgentEligibility(
  input: AgentToolInput,
): Promise<{ eligible: boolean; reason?: string }> {
  // 1. 用户必须有 CCR 配额
  if (!hasRemoteRunnerQuota()) return { eligible: false, reason: '无远程运行配额' }
  
  // 2. 任务不能要求访问本地文件系统（除非通过挂载）
  if (requiresLocalFS(input)) return { eligible: false, reason: '需要本地文件访问' }
  
  return { eligible: true }
}

// 注册远程 Agent 任务
async function registerRemoteAgentTask(
  agentId: AgentId,
  input: AgentToolInput,
): Promise<string> {
  const task = await createRemoteTask({
    agentId,
    prompt: input.prompt,
    tools: input.tools,
    environment: buildRemoteEnvironment(),
  })
  
  return task.sessionUrl  // 远程会话 URL
}
```

---

## 8. Agent 颜色系统

每个 Agent 在 UI 中有独特的颜色，方便区分多 Agent 输出：

```typescript
// AgentTool/agentColorManager.ts
const AGENT_COLORS: AgentColorName[] = [
  'blue', 'green', 'yellow', 'magenta', 'cyan', 'red',
  'blueBright', 'greenBright', 'yellowBright', 'magentaBright',
]

// bootstrap/state.ts 中维护全局 agentColorMap
// 确保不同 Agent 拿到不同颜色，相同 Agent ID 总是相同颜色
export function setAgentColor(agentId: string): AgentColorName {
  if (!state.agentColorMap.has(agentId)) {
    const nextColor = AGENT_COLORS[state.agentColorMap.size % AGENT_COLORS.length]
    state.agentColorMap.set(agentId, nextColor)
  }
  return state.agentColorMap.get(agentId)!
}
```

---

## 9. 多 Agent 数据流

```
主 Claude（协调者）
    │
    ├── AgentTool.call({ prompt: '执行任务A', run_in_background: false })
    │       │
    │       └── runAgent()
    │               │
    │               └── QueryEngine.ask()  ← 子 Agent A（阻塞）
    │                       ├── BashTool.call()
    │                       └── FileEditTool.call()
    │
    ├── AgentTool.call({ prompt: '执行任务B', run_in_background: true })
    │       │
    │       └── registerAsyncAgent()      ← 后台 Agent B（非阻塞）
    │               └── runAgent()（后台）
    │
    └── 继续其他工作...
            ↓
    AgentTool.call({ prompt: '执行任务C' })  ← Agent C 与 B 并发
            │
    B 和 C 同时运行，结果通过 progress 回调推送到 UI
```

---

## 10. 设计模式

### 10.1 分层任务管理
- `LocalAgentTask`：管理任务生命周期（running/completed/failed）
- `AppState.tasks`：提供 UI 可见的任务快照
- `useTasksV2`：React hook 将任务变化同步到 UI

### 10.2 推拉混合消息模型
- **推（Push）**：Agent 主动 `SendMessage` 到队友收件箱
- **拉（Pull）**：`useInboxPoller` 定期轮询收件箱

### 10.3 隔离级别递进

| 隔离级别 | 实现 | 适用场景 |
|---------|------|---------|
| 无隔离（默认）| 同进程、同工作目录 | 简单子任务 |
| worktree 隔离 | Git worktree + CWD 切换 | 代码修改任务（防冲突）|
| remote 隔离 | CCR 远程环境 | 需要独立计算资源的大任务 |
