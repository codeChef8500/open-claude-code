# Claude Code 源码深度解读 — 03 Tool 类型系统与注册表

> 覆盖文件：`Tool.ts`（29KB，793行）、`tools.ts`（17KB，390行）

---

## 1. 模块职责概述

`Tool.ts` 定义了整个工具系统的**类型契约**，是所有工具必须遵守的接口规范。  
`tools.ts` 是工具的**注册表**，负责按环境和权限组装可用工具列表。

---

## 2. `Tool` 接口 — 完整字段解读

`Tool<Input, Output, P>` 是一个泛型接口，三个类型参数：
- `Input`: Zod Schema（工具输入的运行时验证规则）
- `Output`: 工具调用的返回数据类型
- `P`: 进度数据类型（工具执行中的流式进度）

### 2.1 必须实现的方法

```typescript
type Tool = {
  // ① 工具名称（全局唯一标识符）
  readonly name: string

  // ② 工具输入的 Zod Schema（决定 Claude 如何填参数）
  readonly inputSchema: Input

  // ③ 核心执行方法（异步）
  call(
    args: z.infer<Input>,        // 已验证的输入
    context: ToolUseContext,     // 工具执行上下文（工作目录、AppState 等）
    canUseTool: CanUseToolFn,    // 权限检查函数（供递归调用）
    parentMessage: AssistantMessage, // 触发此工具调用的 AI 消息
    onProgress?: ToolCallProgress<P>  // 流式进度回调
  ): Promise<ToolResult<Output>>

  // ④ 权限检查（调用前调用）
  checkPermissions(
    input: z.infer<Input>,
    context: ToolUseContext,
  ): Promise<PermissionResult>

  // ⑤ 返回给 Claude 的系统提示描述
  prompt(options: {
    getToolPermissionContext: () => Promise<ToolPermissionContext>
    tools: Tools
    agents: AgentDefinition[]
    allowedAgentTypes?: string[]
  }): Promise<string>

  // ⑥ 是否启用（运行时动态判断）
  isEnabled(): boolean

  // ⑦ 是否只读（影响权限检查逻辑）
  isReadOnly(input: z.infer<Input>): boolean

  // ⑧ 是否并发安全（影响工具并行执行策略）
  isConcurrencySafe(input: z.infer<Input>): boolean

  // ⑨ 工具结果最大尺寸（超限则写磁盘）
  maxResultSizeChars: number

  // ⑩ 生成用户可读的描述字符串（用于权限弹框、UI 显示）
  description(
    input: z.infer<Input>,
    options: { isNonInteractiveSession: boolean; ... }
  ): Promise<string>

  // ⑪ 生成用户可见的名称（包含操作对象信息）
  userFacingName(input: Partial<z.infer<Input>> | undefined): string
}
```

### 2.2 可选方法（扩展能力）

```typescript
// 输入验证（额外的语义验证，在权限检查前）
validateInput?(input, context): Promise<ValidationResult>

// 兼容旧工具名
aliases?: string[]

// 是否破坏性操作（删除/覆盖/发送等不可逆操作）
isDestructive?(input): boolean

// 中断行为（用户发新消息时：cancel 停止本工具 / block 继续运行）
interruptBehavior?(): 'cancel' | 'block'

// 是否为搜索/读取操作（用于 UI 折叠显示）
isSearchOrReadCommand?(input): { isSearch: boolean; isRead: boolean; isList?: boolean }

// 权限 pattern 匹配器（如 BashTool 的 "git *" 规则）
preparePermissionMatcher?(input): Promise<(pattern: string) => boolean>

// 获取操作的文件路径（用于路径相关权限检查）
getPath?(input): string

// 填充可观察字段（向 SDK 流/transcript 追加 legacy 字段）
backfillObservableInput?(input: Record<string, unknown>): void

// 是否为延迟工具（需要先用 ToolSearch 发现）
readonly shouldDefer?: boolean

// 是否始终加载（不受 ToolSearch 延迟影响）
readonly alwaysLoad?: boolean

// MCP 工具元数据
mcpInfo?: { serverName: string; toolName: string }

// 工具概要描述（用于 ToolSearch 关键词匹配）
searchHint?: string

// 获取活动描述（用于 spinner 显示，如 "Reading src/foo.ts"）
getActivityDescription?(input): string | null

// 获取工具使用摘要（用于折叠视图）
getToolUseSummary?(input): string | null

// 是否为透明包装器（如 REPLTool，渲染委托给内部工具）
isTransparentWrapper?(): boolean

// 是否为严格模式（API 更严格遵循参数 Schema）
readonly strict?: boolean
```

---

## 3. 关键辅助类型

### 3.1 `ToolUseContext` — 工具执行上下文

这是工具调用时的"运行时环境"，包含几乎所有需要的依赖：

```typescript
type ToolUseContext = {
  options: {
    commands: Command[]
    mainLoopModel: string
    tools: Tools
    thinkingConfig: ThinkingConfig
    mcpClients: MCPServerConnection[]
    // ...
  }
  abortController: AbortController   // 取消信号
  readFileState: FileStateCache       // 文件读取缓存
  getAppState(): AppState             // 读取全局 UI 状态
  setAppState(f: ...): void           // 写入全局 UI 状态
  setToolJSX?: SetToolJSXFn          // 渲染工具 UI 到 REPL
  addNotification?: (notif) => void   // 添加 OS 通知
  appendSystemMessage?: (msg) => void // 追加系统消息到 REPL
  sendOSNotification?: (opts) => void // 发送 OS 级通知
  agentId?: AgentId                   // 子 Agent ID（主线程为空）
  agentType?: string                  // Agent 类型名
  messages: Message[]                 // 当前消息历史
  queryTracking?: QueryChainTracking  // 查询链追踪
  requestPrompt?: (name, summary?) => (req) => Promise<PromptResponse>
  // 文件操作限制
  fileReadingLimits?: { maxTokens?: number; maxSizeBytes?: number }
  // 工具决策追踪（用于分析）
  toolDecisions?: Map<string, { source: string; decision: string; ... }>
  // 内容替换状态（工具结果预算）
  contentReplacementState?: ContentReplacementState
  // ...更多字段
}
```

### 3.2 `ToolResult<T>` — 工具返回值

```typescript
type ToolResult<T> = {
  data: T                // 工具执行结果数据
  newMessages?: (        // 可选：追加到消息历史的新消息
    | UserMessage
    | AssistantMessage
    | AttachmentMessage
    | SystemMessage
  )[]
  contextModifier?: (context: ToolUseContext) => ToolUseContext  // 修改上下文
  mcpMeta?: {            // MCP 协议元数据
    _meta?: Record<string, unknown>
    structuredContent?: Record<string, unknown>
  }
}
```

### 3.3 `ToolPermissionContext` — 权限配置

```typescript
type ToolPermissionContext = DeepImmutable<{
  mode: PermissionMode    // 'default' | 'plan' | 'bypassPermissions' | 'auto'
  additionalWorkingDirectories: Map<string, AdditionalWorkingDirectory>
  alwaysAllowRules: ToolPermissionRulesBySource  // 永远允许的规则
  alwaysDenyRules: ToolPermissionRulesBySource   // 永远拒绝的规则
  alwaysAskRules: ToolPermissionRulesBySource    // 永远询问的规则
  isBypassPermissionsModeAvailable: boolean
  isAutoModeAvailable?: boolean
  shouldAvoidPermissionPrompts?: boolean  // 后台 Agent 自动拒绝权限
  awaitAutomatedChecksBeforeDialog?: boolean  // 等待自动检查后再弹框
}>
```

### 3.4 `ValidationResult` / `PermissionResult`

```typescript
// 输入验证结果
type ValidationResult =
  | { result: true }
  | { result: false; message: string; errorCode: number }

// 权限检查结果（来自 types/permissions.ts）
type PermissionResult =
  | { behavior: 'allow' }
  | { behavior: 'deny'; message: string }
  | { behavior: 'ask'; reason?: string; details?: string }
```

---

## 4. `tools.ts` 注册表详解

### 4.1 `getAllBaseTools()` — 基础工具集

这是所有内置工具的**权威来源**，顺序决定 prompt cache 稳定性：

```typescript
export function getAllBaseTools(): Tools {
  return [
    AgentTool,           // 子 Agent 生成
    TaskOutputTool,      // 获取任务输出
    BashTool,            // Shell 命令执行
    // 嵌入式搜索工具时跳过 Glob/Grep
    ...(hasEmbeddedSearchTools() ? [] : [GlobTool, GrepTool]),
    ExitPlanModeV2Tool,  // 退出计划模式
    FileReadTool,        // 读取文件
    FileEditTool,        // 编辑文件
    FileWriteTool,       // 写入文件
    NotebookEditTool,    // 编辑 Jupyter Notebook
    WebFetchTool,        // 获取 URL 内容
    TodoWriteTool,       // 写入 Todo 列表
    WebSearchTool,       // Web 搜索
    TaskStopTool,        // 停止任务
    AskUserQuestionTool, // 向用户提问
    SkillTool,           // 执行技能
    EnterPlanModeTool,   // 进入计划模式
    // ant-only 工具
    ...(process.env.USER_TYPE === 'ant' ? [ConfigTool, TungstenTool] : []),
    // Feature-gated 工具
    ...(WebBrowserTool ? [WebBrowserTool] : []),     // WEB_BROWSER_TOOL
    ...(SleepTool ? [SleepTool] : []),               // PROACTIVE / KAIROS
    ...cronTools,                                     // AGENT_TRIGGERS
    ...(isAgentSwarmsEnabled() ? [
      getTeamCreateTool(), getTeamDeleteTool()        // 多 Agent 协作
    ] : []),
    SendMessageTool,     // Agent 间消息传递
    // ... 更多工具
  ]
}
```

**注意**：注释明确要求该函数与 Statsig 上的系统提示缓存配置保持同步，否则会破坏 prompt cache。

### 4.2 `getTools()` — 运行时工具过滤

```typescript
export function getTools(permissionContext: ToolPermissionContext): Tools {
  // 简单模式（CLAUDE_CODE_SIMPLE）：只有 Bash/Read/Edit
  if (isEnvTruthy(process.env.CLAUDE_CODE_SIMPLE)) {
    return filterToolsByDenyRules([BashTool, FileReadTool, FileEditTool], permissionContext)
  }

  // 常规模式：获取所有基础工具
  const tools = getAllBaseTools().filter(tool => !specialTools.has(tool.name))

  // 1. 过滤掉被 deny rules 拒绝的工具
  let allowedTools = filterToolsByDenyRules(tools, permissionContext)

  // 2. REPL 模式：隐藏原始工具（它们在 REPLTool VM 内部可用）
  if (isReplModeEnabled()) {
    allowedTools = allowedTools.filter(tool => !REPL_ONLY_TOOLS.has(tool.name))
  }

  // 3. 过滤掉 isEnabled() 返回 false 的工具
  const isEnabled = allowedTools.map(_ => _.isEnabled())
  return allowedTools.filter((_, i) => isEnabled[i])
}
```

### 4.3 `assembleToolPool()` — 组合内置 + MCP 工具

```typescript
export function assembleToolPool(
  permissionContext: ToolPermissionContext,
  mcpTools: Tools,
): Tools {
  const builtInTools = getTools(permissionContext)
  const allowedMcpTools = filterToolsByDenyRules(mcpTools, permissionContext)

  // 关键：按名称排序后合并，保持 prompt cache 稳定性
  // 内置工具作为前缀（服务器在最后一个内置工具后设置缓存断点）
  const byName = (a: Tool, b: Tool) => a.name.localeCompare(b.name)
  return uniqBy(
    [...builtInTools].sort(byName).concat(allowedMcpTools.sort(byName)),
    'name',
  )
}
```

**设计原因**：工具列表的排序影响 prompt cache（prompt caching 要求前缀字节完全一致）。内置工具排序稳定，MCP 工具追加在后面，避免 MCP 工具插入内置工具之间破坏缓存。

### 4.4 `filterToolsByDenyRules()` — 权限过滤

```typescript
export function filterToolsByDenyRules<T extends { name: string; mcpInfo?: ... }>(
  tools: readonly T[],
  permissionContext: ToolPermissionContext
): T[] {
  // 使用 getDenyRuleForTool 检查每个工具是否有匹配的全局拒绝规则
  // MCP 服务器级别的规则 (mcp__server) 可以一次性禁用整个 MCP 服务器的所有工具
  return tools.filter(tool => !getDenyRuleForTool(permissionContext, tool))
}
```

---

## 5. `buildTool()` 工具构建辅助函数

实际工具定义通常使用 `buildTool()` 辅助函数：

```typescript
// AgentTool.tsx 示例
import { buildTool } from 'src/Tool.js'

export const AgentTool = buildTool({
  name: AGENT_TOOL_NAME,  // 'Task'
  inputSchema: fullInputSchema,
  isEnabled: () => true,
  isConcurrencySafe: (input) => !input.run_in_background,
  isReadOnly: () => false,
  maxResultSizeChars: Infinity,  // Agent 结果不受大小限制
  async call(args, context, canUseTool, parentMessage, onProgress) {
    // ... 执行逻辑
  },
  // ...
})
```

---

## 6. 工具进度类型体系

各工具有不同的进度数据类型（定义在 `types/tools.ts`）：

```typescript
// Agent 工具进度
type AgentToolProgress = {
  type: 'agent_tool'
  status: 'running' | 'completed' | 'failed'
  agentId: string
  output?: string
  // ...
}

// Bash 工具进度
type BashProgress = {
  type: 'bash'
  output: string          // 实时输出（stdout/stderr）
  isPartialOutput: boolean
  // ...
}

// Web 搜索进度
type WebSearchProgress = {
  type: 'web_search'
  status: 'searching' | 'complete'
  query: string
  results?: SearchResult[]
}
```

---

## 7. 工具注册数据流

```
tools.ts::assembleToolPool(permissionContext, mcpTools)
    │
    ├── getTools(permissionContext)
    │     ├── getAllBaseTools()           ← 静态列表（编译时确定）
    │     ├── filterToolsByDenyRules()    ← 按用户权限配置过滤
    │     ├── REPL 模式过滤
    │     └── isEnabled() 动态过滤
    │
    ├── filterToolsByDenyRules(mcpTools)  ← MCP 工具过滤
    │
    └── uniqBy([...builtIn.sort(), ...mcp.sort()], 'name')
          └── 最终工具池（传入 QueryEngine）
```

---

## 8. 设计模式

### 8.1 接口契约模式（Interface Contract）
`Tool` 接口定义了清晰的行为契约，所有工具必须实现相同的接口，使 `query.ts` 中的工具调用逻辑通用化。

### 8.2 策略模式（Strategy Pattern）
`checkPermissions()`、`isConcurrencySafe()`、`isReadOnly()` 等方法让每个工具独立决定自己的权限和并发策略。

### 8.3 工厂模式（Factory with `buildTool`）
`buildTool()` 辅助函数提供默认实现，减少每个工具的样板代码。

### 8.4 排序稳定性优化（Prompt Cache Optimization）
工具注册表中刻意保持工具列表排序稳定，以最大化 prompt cache 命中率（Anthropic API 的 cache_creation_input_tokens 昂贵）。

---

## 9. 与其他模块的关联

| 模块 | 关联方式 |
|------|---------|
| `query.ts` | 遍历 `tools` 执行工具调用 |
| `hooks/useCanUseTool.tsx` | 实现 `CanUseToolFn`，集成权限 UI |
| `services/tools/toolOrchestration.ts` | 调用 `tool.call()` 并管理并发 |
| `services/api/claude.ts` | 将 `tools` 序列化为 API 请求的 `tools` 数组 |
| `screens/REPL.tsx` | 通过 `useMergedTools` hook 获取工具池 |
| `components/Message.tsx` | 根据工具类型渲染工具调用 UI |

---

## 10. 设计亮点与潜在问题

### 亮点
- **Zod Schema 双用**：既作为运行时类型验证，也作为生成 JSON Schema 传给 API
- **`maxResultSizeChars` 内存保护**：防止大型工具结果撑爆内存（结果写磁盘）
- **`isConcurrencySafe` 并行优化**：允许多个只读工具并行执行
- **工具排序 == 缓存稳定**：精心设计的排序逻辑最大化 prompt cache 命中

### 潜在问题
- `ToolUseContext` 字段过多（30+），成为"上帝对象"，任何工具都依赖它
- `maxResultSizeChars: Infinity` 对 AgentTool 可能在极端情况下造成内存问题
- `buildTool()` 没有强制所有字段，部分工具（如 TungstenTool）缺少 `outputSchema`
