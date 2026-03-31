# Claude Code 源码深度解读 — 19 KAIROS 助手守护进程模式

> 覆盖文件：`entrypoints/cli.tsx`（快速路径分发）、`main.tsx`（KAIROS 初始化段）、`src/assistant/`（`sessionHistory.ts`）、`bootstrap/state.ts`（`kairosActive` 标志）、`utils/concurrentSessions.ts`（PID 注册表）、`utils/cronScheduler.ts`（调度器）、`utils/cronTasksLock.ts`（调度器锁）、`cli/transports/ccrClient.ts`（CCR 传输层）、`tools/BriefTool/`（SendUserMessage）

---

## 1. 什么是 KAIROS？

**KAIROS**（希腊语：时机）是 Claude Code 的"助手守护进程模式"（Assistant Daemon Mode）。与普通的交互式会话不同，KAIROS 使 Claude Code 成为持久运行的后台助手进程，能够：

- **主动监听**：等待来自用户界面（Claude.ai 聊天）的消息，像真人助手一样实时响应
- **主动发起**：通过 `SendUserMessage`（BriefTool）在任务执行过程中主动向用户报告进度
- **持久存活**：跨对话保持状态，不会在会话结束后销毁
- **响应 tick**：在 Proactive 模式下接收定期 `<tick>` 触发，自主决定做什么

---

## 2. 特性门控体系

```
feature('KAIROS')           ← 编译时 DCE 门（bun:bundle）
      │
      ├── feature('KAIROS_BRIEF')     ← BriefTool 独立发布门
      ├── feature('KAIROS_CHANNELS')  ← 频道通知功能门
      ├── feature('DAEMON')           ← 守护进程主管/工作者模式
      └── GrowthBook gate: 'tengu_kairos'  ← 运行时用户授权门
```

```typescript
// main.tsx 中的关键初始化
const assistantModule = feature('KAIROS')
  ? require('./assistant/index.js') as typeof import('./assistant/index.js')
  : null

const kairosGate = feature('KAIROS')
  ? require('./assistant/gate.js') as typeof import('./assistant/gate.js')
  : null
```

---

## 3. 进程守护技术栈（核心）

KAIROS **不使用传统 Unix `daemon()` / `fork()+setsid()` 系统调用**，而是采用一套完整的**主管/工作者（Supervisor/Worker）进程模型**，结合 PID 文件注册表和文件锁实现多进程协调。

### 3.1 整体进程架构

```
操作系统进程树
├── claude daemon            ← Supervisor（主管进程，feature('DAEMON')）
│   │   来自: cli.tsx → daemonMain() → daemon/main.js [未在开源版中公开]
│   │   会话类型: CLAUDE_CODE_SESSION_KIND=daemon
│   │
│   └── claude --daemon-worker=assistant  ← Worker（工作者进程）
│       │   来自: cli.tsx → runDaemonWorker() → daemon/workerRegistry.js
│       │   会话类型: CLAUDE_CODE_SESSION_KIND=daemon-worker
│       │   标志: --assistant（跳过资格重检）
│       │
│       └── [队友子进程]  ← Swarm 队友（通过 AgentTool 产生）
│           │   会话类型: agent-id 不为 null → 不注册 PID 文件
│           └── ...
│
├── claude --bg "task"      ← 后台会话（feature('BG_SESSIONS')）
│   │   封装在 tmux 中，CLAUDE_CODE_SESSION_KIND=bg
│   │   /exit 时 detach，不杀进程
│   └── ...
│
└── claude "prompt"         ← 普通交互式会话
        会话类型: CLAUDE_CODE_SESSION_KIND=interactive（默认）
```

### 3.2 `cli.tsx` 快速路径分发（进程入口决策树）

```typescript
// entrypoints/cli.tsx — 启动时的快速路径优先级

// [1] --daemon-worker=<kind>（最高优先级，主管子进程）
if (feature('DAEMON') && args[0] === '--daemon-worker') {
  const { runDaemonWorker } = await import('../daemon/workerRegistry.js')
  await runDaemonWorker(args[1])  // args[1] = "assistant" 等 worker 类型
  return
}

// [2] daemon 主管模式
if (feature('DAEMON') && args[0] === 'daemon') {
  enableConfigs()
  initSinks()  // 初始化日志 sinks
  const { daemonMain } = await import('../daemon/main.js')
  await daemonMain(args.slice(1))
  return
}

// [3] --bg / ps / logs / attach / kill（后台会话管理）
if (feature('BG_SESSIONS') && /* 相关子命令 */) {
  const bg = await import('../cli/bg.js')
  await bg.handleBgFlag(args)
  return
}

// [4] 普通交互式会话（最重路径，最后加载）
const { main: cliMain } = await import('../main.js')
await cliMain()
```

### 3.3 PID 文件注册表（`utils/concurrentSessions.ts`）

**每个顶层会话**（不含队友子进程）在启动时写入 PID 文件，注册到全局进程注册表：

```typescript
// 会话类型枚举
export type SessionKind = 'interactive' | 'bg' | 'daemon' | 'daemon-worker'

// PID 文件位置：~/.claude/sessions/<pid>.json
// 由 CLAUDE_CODE_SESSION_KIND 环境变量指定类型（主管在 spawn 时注入）
export async function registerSession(): Promise<boolean> {
  if (getAgentId() != null) return false  // 队友不注册（避免污染 claude ps 输出）

  const kind: SessionKind = envSessionKind() ?? 'interactive'
  const pidFile = join(getSessionsDir(), `${process.pid}.json`)

  await writeFile(pidFile, jsonStringify({
    pid:       process.pid,
    sessionId: getSessionId(),
    cwd:       getOriginalCwd(),
    startedAt: Date.now(),
    kind,                    // interactive / bg / daemon / daemon-worker
    entrypoint: process.env.CLAUDE_CODE_ENTRYPOINT,
    // UDS_INBOX 特性下：进程间消息 socket 路径
    messagingSocketPath: process.env.CLAUDE_CODE_MESSAGING_SOCKET,
    // BG_SESSIONS 特性下：
    name:    process.env.CLAUDE_CODE_SESSION_NAME,
    logPath: process.env.CLAUDE_CODE_SESSION_LOG,
    agent:   process.env.CLAUDE_CODE_AGENT,
  }))

  // 会话切换（/resume）时更新 PID 文件中的 sessionId
  onSessionSwitch(id => void updatePidFile({ sessionId: id }))

  // 进程退出时删除 PID 文件
  registerCleanup(async () => { await unlink(pidFile) })
}
```

**`claude ps` 工作原理：**
```
读取 ~/.claude/sessions/*.json
    │
    ├── 对每个 <pid>.json：
    │     ├── isProcessRunning(pid) == true  → 统计为活跃会话
    │     └── isProcessRunning(pid) == false → 删除过期文件（WSL 除外）
    │
    └── 输出会话列表（含 kind / cwd / status / name 等字段）
```

### 3.4 后台会话（`--bg`，`feature('BG_SESSIONS')`）

```bash
# 创建后台会话（封装在 tmux 中）
claude --bg "帮我持续监控 CI 状态"
  └── 设置 CLAUDE_CODE_SESSION_KIND=bg
  └── 在 tmux 新窗口中启动
  └── /exit 时 detach（不杀进程）

# 管理命令
claude ps              # 列出所有活跃会话
claude logs <pid>      # tail -f CLAUDE_CODE_SESSION_LOG
claude attach <pid>    # tmux attach
claude kill <pid>      # SIGTERM → 进程
```

---

## 4. KAIROS 激活流程

```
.claude/settings.json  assistant: true
           │
           ▼
main.tsx: assistantModule?.isAssistantMode() === true
           │
           ├── [信任检查] checkHasTrustDialogAccepted() 必须通过
           │     └── 目录必须已通过信任对话框 ← 防止攻击者控制的 settings.json
           │
           ├── [授权检查] kairosGate.isKairosEnabled()
           │     └── GrowthBook 'tengu_kairos' gate (需账户资格)
           │         └── --assistant 标志可绕过（守护进程由已认证的父进程启动）
           │
           └── 通过 → setKairosActive(true)
                       opts.brief = true   ← 强制开启 BriefTool
                       initializeAssistantTeam()  ← 预建队友团队
```

### 4.1 `bootstrap/state.ts` 中的全局标志

```typescript
type State = {
  kairosActive: boolean   // KAIROS 守护进程模式是否激活
}

export function getKairosActive(): boolean  { return STATE.kairosActive }
export function setKairosActive(v: boolean) { STATE.kairosActive = v }
```

---

## 5. Cron 调度器的进程安全机制（`utils/cronScheduler.ts` + `cronTasksLock.ts`）

这是守护进程模式的核心并发控制，防止多个 Claude 进程对同一 `.claude/scheduled_tasks.json` 文件产生竞争。

### 5.1 调度器轮询架构

```typescript
const CHECK_INTERVAL_MS  = 1000   // 主检查循环（每1秒）
const LOCK_PROBE_INTERVAL_MS = 5000  // 锁竞争探测（每5秒）
const FILE_STABILITY_MS  = 300    // 文件稳定窗口（chokidar 去抖）

// 使用 chokidar 监听 .claude/scheduled_tasks.json 文件变化
// 文件变化 → 立即重载任务列表（不等下一秒 tick）
import type { FSWatcher } from 'chokidar'
let watcher: FSWatcher | null = null
```

### 5.2 文件基调度器锁（`utils/cronTasksLock.ts`）

**核心原理：O_EXCL 原子创建 + PID 存活检测**

锁文件路径：`.claude/scheduled_tasks.lock`

```typescript
type SchedulerLock = {
  sessionId: string  // 所有者标识（REPL 用 sessionId，daemon 用稳定 UUID）
  pid:       number  // 用于存活检测
  acquiredAt: number
}

// 获取锁（原子 test-and-set）
export async function tryAcquireSchedulerLock(opts?: SchedulerLockOptions): Promise<boolean> {
  const lock = { sessionId: opts?.lockIdentity ?? getSessionId(), pid: process.pid, acquiredAt: Date.now() }

  // 1. 原子创建（O_EXCL = 'wx'）：文件不存在才成功
  if (await tryCreateExclusive(lock)) {
    registerLockCleanup()  // 退出时自动释放
    return true            // 获锁成功，成为调度器主控
  }

  const existing = await readLock()

  // 2. 已是自己持有（幂等）
  if (existing?.sessionId === lock.sessionId) {
    if (existing.pid !== process.pid) await writeFile(lockPath, jsonStringify(lock))  // 更新 PID
    return true
  }

  // 3. 检查现有持有者是否存活
  if (existing && isProcessRunning(existing.pid)) {
    return false  // 另一个活跃进程持有锁 → 被动等待
  }

  // 4. 持有者已崩溃 → 回收陈旧锁
  await unlink(lockPath).catch(() => {})
  return tryCreateExclusive(lock)  // 竞争回收（race: 只有一个进程成功）
}
```

**锁竞争状态机：**
```
启动时尝试获锁
    ├── 获锁成功 → isOwner = true → 处理文件任务
    └── 获锁失败 → isOwner = false（被动）
            │
            └── lockProbeTimer（每5秒）
                    ├── 所有者存活 → 继续等待
                    └── 所有者崩溃 → 回收锁 → isOwner = true
```

### 5.3 Daemon 模式的特殊调度参数

```typescript
// 普通 REPL 模式
createCronScheduler({
  dir: undefined,              // 使用 getProjectRoot() + 全局状态
  lockIdentity: undefined,     // 使用 getSessionId()
  getJitterConfig: () => getGrowthBookJitterConfig(),  // GrowthBook 实时配置
  isKilled: () => !isKairosCronEnabled(),              // Kill switch 支持
  assistantMode: false,
})

// Daemon Worker 模式
createCronScheduler({
  dir: '/path/to/project/.claude',  // 显式目录（不读 bootstrap 全局状态）
  lockIdentity: randomUUID(),        // 稳定 UUID（进程内唯一，不用 sessionId）
  getJitterConfig: undefined,        // 使用默认（DEFAULT_CRON_JITTER_CONFIG）
  isKilled: undefined,               // Kill switch 不影响 daemon（重启处理）
  assistantMode: true,               // 绕过 isLoading 检查，自动启用调度器
  filter: (t) => t.permanent,       // 只处理 permanent 任务（非 permanent 留给 REPL）
})
```

---

## 6. CCR 传输层（云端 KAIROS Worker）

对于运行在云端容器中的 KAIROS Worker（如 Claude.ai 远程环境），使用 CCR（Claude Code Remote）传输层：

### 6.1 `CCRClient`（`cli/transports/ccrClient.ts`）

```typescript
const DEFAULT_HEARTBEAT_INTERVAL_MS  = 20_000  // 心跳间隔（20秒）
const STREAM_EVENT_FLUSH_INTERVAL_MS = 100      // 流式事件批量上传窗口（100ms）
const MAX_CONSECUTIVE_AUTH_FAILURES  = 10       // 连续 401/403 容忍次数（约200秒）

// 工作原理：
// 1. initialize() — 注册 Worker，获取 session ingress auth token
// 2. heartbeat 循环 — 每20秒 POST /heartbeat（服务器 TTL 60秒）
// 3. SerialBatchEventUploader — 将流事件序列批量上传
// 4. WorkerStateUploader — 上传 Worker 状态（busy/idle/waiting）
// 5. JWT 过期检测 — exp 已过期立即退出，未过期最多等待 10×20s
```

**容量环境配置（`cli.tsx`）：**
```typescript
// CCR 容器（16GB 内存）
if (process.env.CLAUDE_CODE_REMOTE === 'true') {
  process.env.NODE_OPTIONS = '--max-old-space-size=8192'  // 8GB 堆
}
```

### 6.2 流事件批量处理

```
Agent 产生流式事件
    │
    ▼
延迟缓冲区（最多100ms）
    │
    ├── text_delta 事件：合并为"目前为止的完整文本"快照
    │     └── 原因：客户端中途连接时能看到完整内容，非片段
    │
    └── SerialBatchEventUploader → POST /events（批量）
```

---

## 7. UDS（Unix Domain Socket）进程间通信

KAIROS 在本地运行时使用 UDS 实现进程间消息传递：

```typescript
// PID 文件中记录该进程的 UDS 路径（feature('UDS_INBOX') 时）
{
  messagingSocketPath: process.env.CLAUDE_CODE_MESSAGING_SOCKET
}

// 其他 Claude 进程通过读 ~/.claude/sessions/ 找到此 socket
// 用途：SendMessageTool（队友间通信）、权限请求委托
```

---

## 8. 助手模式与普通模式的对比

| 特性 | 普通交互模式 | KAIROS 助手模式 |
|------|------------|----------------|
| 进程类型 | `interactive` | `daemon-worker` |
| 会话生命周期 | 一次对话，结束即退出 | 持久守护进程，等待消息 |
| 用户消息来源 | 终端输入 | Claude.ai 网页/Bridge/CCR |
| 主动发消息 | 不支持 | BriefTool（SendUserMessage）|
| 调度器 assistantMode | false（等待 setEnabled 标志）| true（自动启用）|
| 调度器 lockIdentity | getSessionId() | 稳定 UUID（每进程唯一）|
| 调度器 filter | 所有任务 | 只处理 permanent 任务 |
| `SleepTool` | 禁用 | 可用（等待下一个 tick）|
| 系统提示附加 | 标准 | 追加 `assistant/systemPrompt.md` |
| BriefTool 激活 | 需手动 opt-in | 自动开启（kairosActive 绕过 opt-in）|
| Fast Mode 限制 | SDK 模式下禁用 | 豁免（kairosActive=true 跳过检查）|

---

## 9. 会话历史（`assistant/sessionHistory.ts`）

KAIROS 模式支持从 Claude.ai 服务器加载历史对话事件（用于跨重启恢复上下文）：

```typescript
// 通过 OAuth + Beta API 拉取会话事件
// GET /v1/sessions/{sessionId}/events
// Beta header: 'anthropic-beta': 'ccr-byoc-2025-07-29'
// 分页大小: 100 条/页

export async function fetchLatestEvents(ctx: HistoryAuthCtx, limit = 100) {
  // params: { limit, anchor_to_latest: true }  ← 最新100条
}

export async function fetchOlderEvents(ctx: HistoryAuthCtx, beforeId: string) {
  // params: { limit, before_id: beforeId }      ← 游标翻页
}
```

---

## 10. Proactive 模式（与 KAIROS 联动）

```typescript
// 附加到系统提示的 Proactive 段落
appendSystemPrompt = `\n# Proactive Mode\n\n` +
  `You are in proactive mode. Take initiative — explore, act, and make ` +
  `progress without waiting for instructions.\n\n` +
  `Start by briefly greeting the user.\n\n` +
  `You will receive periodic <tick> prompts. These are check-ins. Do ` +
  `whatever seems most useful, or call Sleep if there's nothing to do. ` +
  briefVisibility  // "Call SendUserMessage..." 或 "The user will see..."
```

**Tick 驱动循环：**
```
KAIROS 守护进程启动
    │
    ├── 等待用户消息（Bridge WebSocket / CCR HTTP Stream）
    │       └── 收到消息 → 处理 → SendUserMessage 回复
    │
    └── 等待 <tick> 信号（Cron 调度器注入）
            └── 主动探索/执行任务/调用 Sleep
```

---

## 11. KAIROS 相关工具清单

| 工具 | 说明 | KAIROS 启用条件 |
|------|------|----------------|
| `SendUserMessage`（BriefTool）| 主动给用户发消息 | `isBriefEnabled()` → `kairosActive \|\| userMsgOptIn` |
| `SleepTool` | 等待 N 秒/下一个 tick | 仅在 proactive 模式下启用 |
| `CronCreateTool` | 创建定时任务 | `isKairosCronEnabled()` |
| `AgentTool`（队友）| 调用命名队友 | 助手模式下预建团队 |
| `ToolSearchTool` | 搜索可用工具 | KAIROS 特性下默认开启 |

---

## 12. 技术栈总结

| 技术 | 用途 | 来源 |
|------|------|------|
| Bun/Node.js 标准进程 | Supervisor + Worker 模型 | `child_process.spawn` |
| `CLAUDE_CODE_SESSION_KIND` | 进程角色注入 | 环境变量 |
| `~/.claude/sessions/<pid>.json` | PID 注册表 | `concurrentSessions.ts` |
| `isProcessRunning(pid)` | 进程存活检测 | `genericProcessUtils.ts` |
| `registerCleanup()` | 退出时清理 PID 文件 / 锁 | `cleanupRegistry.ts` |
| **chokidar** | 文件系统监听（任务变化）| `cronScheduler.ts` |
| `setInterval(fn, 1000)` | 调度器主 tick | `cronScheduler.ts` |
| `setInterval(fn, 5000)` | 锁竞争探测 | `cronScheduler.ts` |
| `.claude/scheduled_tasks.lock` | 文件基互斥锁 | `cronTasksLock.ts` |
| **O_EXCL 原子创建**（`'wx'`）| 锁的 test-and-set 原语 | `cronTasksLock.ts` |
| **CCRClient** HTTP + JWT | 云端 Worker 心跳/事件流 | `ccrClient.ts` |
| **Unix Domain Socket** | 本地进程间消息 | `UDS_INBOX` feature |
| Anthropic Sessions API | 跨重启历史恢复 | `sessionHistory.ts` |

---

## 13. 设计亮点

### 13.1 无 daemonize()，用注册表代替
不依赖 Unix `daemon()` 或 `setsid()`，而是用 PID 文件注册表 + 存活检测，跨平台（Windows/macOS/Linux）统一工作，且与 `claude ps` 天然集成。

### 13.2 调度器锁的崩溃容忍
`tryAcquireSchedulerLock` 在持有者崩溃后 5 秒内自动恢复锁，防止定时任务因主进程崩溃而永久停止。

### 13.3 Daemon 与 REPL 共享 CronScheduler
同一套 `createCronScheduler()` API 服务两种调用场景，通过 `dir`/`lockIdentity`/`filter` 参数区分行为，而非两套独立实现。

### 13.4 信任门（Trust Gate）
KAIROS 检查 `checkHasTrustDialogAccepted()` 防止攻击者在仓库中放置恶意 `.claude/settings.json` 来非法激活助手模式。

### 13.5 双层编译+运行时授权
编译时 `feature('KAIROS')` 保证外部构建根本不包含助手模式代码；运行时 GrowthBook gate `tengu_kairos` 控制哪些用户账号有资格激活。
