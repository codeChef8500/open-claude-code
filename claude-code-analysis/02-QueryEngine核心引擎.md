# Claude Code 源码深度解读 — 02 QueryEngine 核心引擎

> 覆盖文件：`QueryEngine.ts`（46KB）、`query.ts`（68KB）、`query/config.ts`、`query/deps.ts`、`query/stopHooks.ts`、`query/tokenBudget.ts`

---

## 1. 模块职责概述

`QueryEngine` + `query.ts` 是整个 Claude Code 的**心脏**，负责：
- 管理单次对话的完整生命周期（消息历史、成本追踪、权限）
- 驱动 LLM API 调用循环（流式响应 + 工具调用迭代）
- 上下文压缩管理（autocompact、microcompact、snip、contextCollapse）
- Token 预算控制与 max_output_tokens 恢复
- 会话持久化（录制 transcript）

两个文件的关系：
```
QueryEngine.ts         ← 会话级封装（SDK 暴露层）
    └── query.ts       ← 核心查询循环（单次 query 调用，纯函数式）
         └── services/api/claude.ts  ← Anthropic API 流式调用
         └── services/tools/toolOrchestration.ts  ← 工具执行编排
```

---

## 2. 文件结构

| 文件 | 大小 | 职责 |
|------|------|------|
| `QueryEngine.ts` | 46KB，约 1296 行 | 会话管理、SDK 接口、transcript 录制 |
| `query.ts` | 68KB，约 1730 行 | 核心查询循环，状态机实现 |
| `query/config.ts` | 1.8KB | 快照不可变环境配置（feature flags、session ID 等） |
| `query/deps.ts` | 1.4KB | 依赖注入接口（方便测试 mock） |
| `query/stopHooks.ts` | 17KB | stop_reason 处理钩子 |
| `query/tokenBudget.ts` | 2.3KB | Token 预算追踪 |

---

## 3. 核心类型定义

### 3.1 `QueryEngineConfig`（QueryEngine.ts:130-173）

```typescript
type QueryEngineConfig = {
  cwd: string                    // 工作目录
  tools: Tools                   // 工具列表
  commands: Command[]            // Slash 命令列表
  mcpClients: MCPServerConnection[]
  agents: AgentDefinition[]      // 子 Agent 定义
  canUseTool: CanUseToolFn       // 权限检查函数（注入）
  getAppState: () => AppState    // 读取全局 UI 状态
  setAppState: (f: ...) => void  // 写入全局 UI 状态
  initialMessages?: Message[]    // 会话历史（恢复时提供）
  readFileCache: FileStateCache  // 文件读取缓存
  customSystemPrompt?: string    // 自定义系统提示（SDK 调用者设置）
  appendSystemPrompt?: string    // 追加到系统提示末尾
  userSpecifiedModel?: string    // 用户指定模型
  thinkingConfig?: ThinkingConfig // Extended Thinking 配置
  maxTurns?: number              // 最大对话轮次
  maxBudgetUsd?: number          // 最大费用限制（USD）
  taskBudget?: { total: number } // API task_budget（token 预算）
  jsonSchema?: Record<string, unknown>  // 结构化输出 Schema
  snipReplay?: ...               // history snip 边界回放（feature-gated）
}
```

### 3.2 `QueryParams`（query.ts:181-198）

```typescript
type QueryParams = {
  messages: Message[]            // 当前消息历史
  systemPrompt: SystemPrompt     // 系统提示（已构建）
  userContext: { [k: string]: string }   // 用户环境上下文
  systemContext: { [k: string]: string } // 系统环境上下文
  canUseTool: CanUseToolFn
  toolUseContext: ToolUseContext  // 工具调用上下文（超级大类型）
  fallbackModel?: string         // 备选模型（模型超时时切换）
  querySource: QuerySource       // 来源标识（repl/sdk/agent:xxx）
  maxOutputTokensOverride?: number
  maxTurns?: number
  skipCacheWrite?: boolean       // 跳过 prompt cache 写入
  taskBudget?: { total: number }
  deps?: QueryDeps               // 依赖注入（测试用）
}
```

### 3.3 `State`（query.ts:204-217，查询循环内部状态）

```typescript
type State = {
  messages: Message[]                   // 当前消息列表
  toolUseContext: ToolUseContext
  autoCompactTracking: AutoCompactTrackingState | undefined
  maxOutputTokensRecoveryCount: number  // max_output_tokens 恢复次数（最多3次）
  hasAttemptedReactiveCompact: boolean
  maxOutputTokensOverride: number | undefined
  pendingToolUseSummary: Promise<...> | undefined  // 异步工具摘要
  stopHookActive: boolean | undefined
  turnCount: number
  transition: Continue | undefined      // 上一次迭代的继续原因
}
```

---

## 4. QueryEngine 类详解

### 4.1 构造函数

```typescript
class QueryEngine {
  private mutableMessages: Message[]      // 可变消息历史（跨轮次）
  private abortController: AbortController
  private permissionDenials: SDKPermissionDenial[]  // 权限拒绝记录
  private totalUsage: NonNullableUsage    // 累计 token 使用量
  private discoveredSkillNames = new Set<string>() // 本轮发现的技能
  private loadedNestedMemoryPaths = new Set<string>() // 已加载的嵌套记忆路径

  constructor(config: QueryEngineConfig) {
    this.mutableMessages = config.initialMessages ?? []
    this.abortController = config.abortController ?? createAbortController()
    // ...
  }
}
```

**关键设计**：一个 `QueryEngine` 实例对应一个**会话**（session），`submitMessage()` 每次调用对应一个**对话轮次**（turn）。

### 4.2 `submitMessage()` — 核心方法

这是一个 **AsyncGenerator** 方法，产出 `SDKMessage` 流：

```typescript
async *submitMessage(
  prompt: string | ContentBlockParam[],
  options?: { uuid?: string; isMeta?: boolean }
): AsyncGenerator<SDKMessage, void, unknown>
```

**执行流程**（简化）：

```
submitMessage(prompt)
    │
    ├── 1. 重置轮次状态（discoveredSkillNames.clear()）
    │
    ├── 2. 构建系统提示
    │     └── fetchSystemPromptParts()
    │           ├── defaultSystemPrompt（模板化）
    │           ├── userContext（环境信息）
    │           └── customSystemPrompt（SDK 调用者）
    │
    ├── 3. 处理用户输入
    │     └── processUserInput(prompt, 'prompt', ...)
    │           ├── 解析 slash 命令（/compact, /clear 等）
    │           ├── 处理文件附件
    │           └── 返回: { messages, shouldQuery, allowedTools, model }
    │
    ├── 4. 持久化用户消息（recordTranscript）
    │
    ├── 5. yield buildSystemInitMessage(...)  ← SDK 首条消息（系统初始化信息）
    │
    ├── 6. if (!shouldQuery)  ← slash 命令本地执行，不调用 LLM
    │     └── yield result message，return
    │
    └── 7. for await (message of query(...))  ← 进入核心循环
          ├── 录制 transcript
          ├── 追踪 token 使用量
          ├── yield normalizeMessage(message) ← 转换为 SDKMessage 格式
          └── 处理 max_turns_reached / compact_boundary 等特殊情况
```

---

## 5. `query.ts` — 核心查询循环

### 5.1 `query()` 函数

```typescript
export async function* query(
  params: QueryParams,
): AsyncGenerator<
  | StreamEvent | RequestStartEvent | Message | TombstoneMessage | ToolUseSummaryMessage,
  Terminal  // 返回值：终止原因
>
```

`query()` 是一个**薄包装层**，真正的逻辑在 `queryLoop()`：

```typescript
async function* query(params, consumedCommandUuids) {
  const terminal = yield* queryLoop(params, consumedCommandUuids)
  // 只有正常返回（非抛出）才执行
  for (const uuid of consumedCommandUuids) {
    notifyCommandLifecycle(uuid, 'completed')
  }
  return terminal
}
```

### 5.2 `queryLoop()` — 核心 `while(true)` 循环

```
queryLoop 状态机
    │
    ├── [初始化]
    │     ├── buildQueryConfig()         ← 快照不可变配置
    │     ├── startRelevantMemoryPrefetch() ← 异步预取相关记忆
    │     └── budgetTracker（TOKEN_BUDGET 特性）
    │
    └── while(true):
          │
          ├── [每轮迭代开始]
          │     ├── 技能发现预取（skill discovery prefetch）
          │     ├── yield { type: 'stream_request_start' }
          │     └── 更新 queryTracking（chainId + depth++）
          │
          ├── [上下文处理流水线]
          │     ├── applyToolResultBudget()  ← 压缩大型工具结果
          │     ├── snipCompactIfNeeded()    ← HISTORY_SNIP 特性
          │     ├── microcompact()           ← 微压缩（cached microcompact）
          │     ├── contextCollapse()        ← 上下文折叠（CONTEXT_COLLAPSE 特性）
          │     └── autocompact()            ← 自动压缩（token 超限时触发）
          │
          ├── [Token 限制检查]
          │     └── calculateTokenWarningState() → isAtBlockingLimit
          │           └── if true: yield error, return 'blocking_limit'
          │
          ├── [API 调用循环]
          │     └── for await (message of callModel(...)):
          │           ├── 流式接收 text / thinking / tool_use blocks
          │           ├── toolUseBlocks.push()  ← 收集 tool_use
          │           └── yield message（传递给 QueryEngine）
          │
          ├── [工具执行]
          │     └── runTools(toolUseBlocks, canUseTool, toolUseContext)
          │           ├── 并行执行工具（并发安全的工具同时跑）
          │           ├── canUseTool() ← 权限检查（可能弹 UI 确认）
          │           ├── tool.call(input, context) ← 实际执行
          │           └── yield toolResults
          │
          ├── [继续/终止判断]
          │     ├── 有 tool_use → 继续循环（continue）
          │     ├── stop_reason = 'end_turn' → 退出循环
          │     ├── stop_reason = 'max_tokens' → 尝试恢复（最多3次）
          │     └── 达到 maxTurns → yield max_turns_reached，return
          │
          └── [stop hooks 处理]
                └── handleStopHooks() ← 用户自定义 stop 钩子
```

### 5.3 上下文压缩流水线

这是 `queryLoop` 中最复杂的部分，每次迭代开始时依次执行：

```
messagesForQuery = [...getMessagesAfterCompactBoundary(messages)]
    │
    ▼
applyToolResultBudget()    ← 按 tool 配置的 maxResultSizeChars 截断大结果
    │
    ▼（HISTORY_SNIP 特性）
snipCompactIfNeeded()      ← 移除过时的工具调用历史（保留最近 N 轮）
    │
    ▼
microcompact()             ← 缓存微压缩：压缩重复的 read/search 调用
    │
    ▼（CONTEXT_COLLAPSE 特性）
contextCollapse()          ← 将多轮交互折叠成摘要（不走 API，纯本地）
    │
    ▼
autocompact()              ← 当 token 超过阈值时，调用 LLM 压缩整个历史
```

### 5.4 Thinking 规则（重要！）

源码中有一段著名的注释（query.ts:151-163）：

```typescript
/**
 * The rules of thinking are lengthy and fortuitous. They require plenty of
 * thinking of most long duration and deep meditation...
 *
 * 规则：
 * 1. 含有 thinking/redacted_thinking block 的消息必须在 max_thinking_length > 0 的查询中
 * 2. thinking block 不能是消息的最后一个 block
 * 3. thinking blocks 必须在整个 assistant trajectory 中保留
 *    （单轮，或包含 tool_use 的轮次及其后续 tool_result 和下一条 assistant 消息）
 */
```

这三条规则决定了含 thinking 的消息如何在 API 调用中传递，违反任何一条都会导致 API 错误。

### 5.5 max_output_tokens 恢复机制

```typescript
const MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3  // 最多恢复3次

// 当检测到 max_output_tokens 错误时：
if (message.apiError === 'max_output_tokens') {
  // 扣除缓冲量，继续下一轮（让 LLM 继续生成）
  state = {
    ...state,
    maxOutputTokensRecoveryCount: maxOutputTokensRecoveryCount + 1,
    transition: { reason: 'max_output_tokens_recovery' },
  }
  continue  // 继续 while(true) 循环
}
```

---

## 6. `query/config.ts` — 快照配置

在每次 `queryLoop` 入口调用 `buildQueryConfig()`，快照当前环境状态：

```typescript
// 用途：将 feature() / GrowthBook 特性标志和会话 ID 在入口处读取一次，
// 避免循环体中多次调用造成的不一致（GrowthBook 中途可能更新）
function buildQueryConfig(): QueryConfig {
  return {
    sessionId: getSessionId(),
    gates: {
      isAnt: process.env.USER_TYPE === 'ant',
      fastModeEnabled: getFeatureValue_CACHED_MAY_BE_STALE('fast_mode', false),
      streamingToolExecution: isStreamingToolExecutionEnabled(),
      // ...
    }
  }
}
```

### 6.1 依赖注入 (`query/deps.ts`)

```typescript
// 生产实现
export function productionDeps(): QueryDeps {
  return {
    callModel: (params) => callClaude(params),      // 真实 API 调用
    microcompact: (msgs, ctx, src) => runMicrocompact(msgs, ctx, src),
    autocompact: (msgs, ...) => runAutocompact(msgs, ...),
    uuid: () => randomUUID(),
  }
}

// 测试中可注入 mock：
const testDeps: QueryDeps = {
  callModel: async function* () { yield mockMessage },
  // ...
}
```

---

## 7. 数据流图

```
用户输入 prompt
    │
    ▼
QueryEngine.submitMessage(prompt)
    │
    ├── fetchSystemPromptParts()     → systemPrompt（含记忆、CLAUDE.md等）
    ├── processUserInput(prompt)     → { messages, shouldQuery }
    ├── recordTranscript(messages)   → 写入 ~/.claude/sessions/xxx.jsonl
    ├── yield systemInitMessage      → SDK 消费者收到会话初始化信息
    │
    └── for await of query(params):
          │
          └── queryLoop:
                │
                ├── [上下文压缩流水线]
                │     snip → microcompact → collapse → autocompact
                │
                ├── callModel(messages, systemPrompt, tools)
                │     └── Anthropic API (/v1/messages, stream=true)
                │           stream:
                │           ├── message_start   → 初始化 usage
                │           ├── content_block_start → 开始新 block
                │           ├── content_block_delta → text/thinking 增量
                │           ├── content_block_stop  → block 结束（tool_use 完成）
                │           ├── message_delta   → stop_reason, 最终 usage
                │           └── message_stop    → 消息结束
                │
                ├── runTools(toolUseBlocks)
                │     ├── canUseTool() → 权限检查
                │     ├── tool.call()  → 实际执行
                │     └── yield toolResults（作为 user 消息追加）
                │
                └── 判断：有 tool_use → continue，否则 return Terminal
```

---

## 8. 关键设计模式

### 8.1 AsyncGenerator 链式传递

整个响应流从 `callModel` → `queryLoop` → `query` → `QueryEngine.submitMessage` 形成一条 AsyncGenerator 链，每层只负责自己的转换：

```typescript
// 内层：Anthropic SDK 流式响应
for await (const event of anthropicStream) {
  yield transformEvent(event)
}

// 中层：queryLoop 添加工具执行
for await (const message of callModel(...)) {
  yield message  // 直接传递
  if (isToolUse(message)) {
    const results = await runTools(toolUseBlocks)
    for (const result of results) yield result
  }
}

// 外层：QueryEngine 添加 transcript 录制
for await (const message of query(...)) {
  await recordTranscript(messages)
  yield* normalizeMessage(message)  // 转为 SDK 格式
}
```

### 8.2 Tombstone 机制

当流式传输中途发生 fallback（切换模型重试）时，已发出的 assistant 消息会被"墓碑化"：

```typescript
// 通知 UI 移除这些消息（它们含无效的 thinking 签名）
for (const msg of assistantMessages) {
  yield { type: 'tombstone', message: msg }
}
assistantMessages.length = 0  // 清空，重新开始
```

### 8.3 工具结果预算（Tool Result Budget）

每个工具可设置 `maxResultSizeChars`，防止超大工具结果撑爆上下文：

```typescript
// 工具定义时设置：
const GrepTool = {
  maxResultSizeChars: 100_000,  // 100KB
  // ...
}

// query.ts 中压缩超限结果：
messagesForQuery = await applyToolResultBudget(
  messagesForQuery,
  contentReplacementState,
  // ...
)
```

---

## 9. 关键函数分析

### `normalizeMessage()` (utils/queryHelpers.ts)

将 `query.ts` 内部消息格式转换为 SDK 消费者需要的格式，同时处理 `thinking` blocks 的 redaction。

### `handleOrphanedPermission()` (utils/queryHelpers.ts)

在 SDK 模式下，当之前的会话有"孤立"（未决）的权限请求时，在第一次 `submitMessage` 调用时处理它。

### `buildSystemInitMessage()` (utils/messages/systemInit.ts)

构建 `SDKMessage` 中的第一条系统消息，包含：
- 工具列表（名称、描述、Schema）
- 模型信息
- 权限模式
- 命令列表
- Agent 列表
- 技能列表

---

## 10. 与其他模块的依赖关系

```
QueryEngine.ts
    ├── 依赖：query.ts（核心循环）
    ├── 依赖：utils/processUserInput/processUserInput.ts（输入解析）
    ├── 依赖：utils/queryContext.ts（系统提示构建）
    ├── 依赖：utils/sessionStorage.ts（transcript 录制）
    ├── 依赖：services/compact/*.ts（上下文压缩）
    ├── 依赖：bootstrap/state.ts（全局状态读取）
    └── 被依赖：main.tsx（-p 模式）、entrypoints/sdk/（SDK 入口）

query.ts
    ├── 依赖：services/api/claude.ts（Anthropic API 调用）
    ├── 依赖：services/tools/toolOrchestration.ts（工具执行）
    ├── 依赖：services/compact/autoCompact.ts（自动压缩）
    ├── 依赖：services/compact/microcompact.ts（微压缩）
    ├── 依赖：query/stopHooks.ts（stop 钩子）
    └── 被依赖：QueryEngine.ts（唯一调用者）
```

---

## 11. 设计亮点与潜在问题

### 亮点

1. **清晰的分层**：`QueryEngine`（会话级）和 `query`（请求级）职责分明
2. **依赖注入**：`QueryDeps` 使核心循环可测试（mock callModel）
3. **Tombstone 机制**：优雅处理流式 fallback，避免 UI 脏状态
4. **多层上下文压缩**：snip → microcompact → collapse → autocompact 形成递进式压缩策略
5. **task_budget 追踪**：跨 compact 边界追踪 token 预算（server 无法感知 compact 前的历史）

### 潜在问题

1. **`query.ts` 函数体过长**（约 1730 行），包含大量注释说明各种边界情况，可读性挑战大
2. **`while(true)` 无限循环**需依赖多个正确的退出条件，调试困难
3. **特性标志分散**：`feature()` 调用散布在循环体各处，代码路径矩阵复杂
4. **递归压缩调用**：compact agent 本身也会触发 `query()`（`querySource: 'compact'`），形成递归，需特别处理（`querySource !== 'compact'` 的跳过逻辑）
