# Claude Code 源码深度解读 — 06 Permission 权限系统

> 覆盖文件：`hooks/useCanUseTool.tsx`（40KB）、`utils/permissions/`（20+ 文件）、`hooks/toolPermission/`、`types/permissions.ts`

---

## 1. 模块职责概述

权限系统控制**什么工具在什么条件下可以被调用**，是 Claude Code 安全机制的核心。它在用户体验和安全性之间取得平衡：
- **太严格**：频繁弹出确认框，打断工作流
- **太宽松**：AI 可能执行危险操作（删除文件、运行恶意命令）

---

## 2. 权限模式（`PermissionMode`）

```typescript
type PermissionMode =
  | 'default'             // 正常模式：危险操作需要用户确认
  | 'plan'                // 计划模式：所有工具调用都需确认
  | 'bypassPermissions'   // 旁路模式：跳过所有权限检查（需明确启用）
  | 'auto'               // 自动模式：AI 分类器决定是否允许（实验性）
```

模式转换规则（`getNextPermissionMode.ts`）：

```
default ─→ plan          (/plan 命令或 EnterPlanModeTool)
plan    ─→ default        (/plan 退出 或 ExitPlanModeV2Tool)
default ─→ bypassPerms    (用户明确请求 --bypass-permissions)
```

---

## 3. 权限规则系统

### 3.1 规则结构

```typescript
type ToolPermissionRulesBySource = {
  command?: string[]    // Bash 命令规则（如 "git commit*"）
  file?: string[]       // 文件路径规则（如 "src/**"）
  tool?: string[]       // 工具名规则（如 "BashTool"）
  mcp?: string[]        // MCP 工具规则（如 "mcp__server__tool"）
}

type ToolPermissionContext = {
  mode: PermissionMode
  alwaysAllowRules: ToolPermissionRulesBySource  // 永远允许
  alwaysDenyRules: ToolPermissionRulesBySource   // 永远拒绝
  alwaysAskRules: ToolPermissionRulesBySource    // 永远询问
}
```

### 3.2 规则优先级（`permissions.ts`）

```
权限检查流程：
1. alwaysDenyRules 拒绝？     → 立即拒绝（最高优先级）
2. alwaysAllowRules 允许？    → 立即允许
3. alwaysAskRules 匹配？      → 强制弹出确认框
4. 工具自身 checkPermissions() → 工具特定逻辑
5. 计划模式？                  → 弹出确认框
6. 否则                        → 允许
```

---

## 4. `hasPermissionsToUseTool()` — 权限核查主函数

```typescript
// utils/permissions/permissions.ts
async function hasPermissionsToUseTool(
  tool: Tool,
  input: unknown,
  toolUseContext: ToolUseContext,
  assistantMessage: AssistantMessage,
  toolUseId: string,
): Promise<PermissionResult> {
  
  const { toolPermissionContext } = toolUseContext.getAppState()
  
  // Step 1: bypassPermissions 模式 → 直接允许
  if (toolPermissionContext.mode === 'bypassPermissions') {
    return { behavior: 'allow' }
  }
  
  // Step 2: 检查 always-deny 规则
  const denyRule = getDenyRuleForTool(toolPermissionContext, tool, input)
  if (denyRule) {
    return { behavior: 'deny', message: `工具被规则拒绝: ${denyRule}` }
  }
  
  // Step 3: 检查 always-allow 规则
  const allowRule = getAllowRuleForTool(toolPermissionContext, tool, input)
  if (allowRule) {
    return { behavior: 'allow', decisionReason: { type: 'rule', rule: allowRule } }
  }
  
  // Step 4: 工具特定权限检查
  const toolPermission = await tool.checkPermissions(input, toolUseContext)
  if (toolPermission.behavior === 'deny') return toolPermission
  
  // Step 5: plan 模式 → 所有工具都需确认
  if (toolPermissionContext.mode === 'plan') {
    return { behavior: 'ask', reason: '计划模式下需要确认' }
  }
  
  // Step 6: 工具要求用户确认
  if (toolPermission.behavior === 'ask') return toolPermission
  
  return { behavior: 'allow' }
}
```

---

## 5. `useCanUseTool` React Hook — 权限 UI 集成

这是权限系统与 React UI 的桥梁，是整个系统中最复杂的 hook 之一（40KB）：

```typescript
function useCanUseTool(
  setToolUseConfirmQueue: ...,
  setToolPermissionContext: ...,
): CanUseToolFn
```

### 5.1 决策流程

```
canUseTool(tool, input, toolUseContext)
    │
    ├── hasPermissionsToUseTool()  ← 规则检查（纯逻辑，无 UI）
    │
    ├── result.behavior === 'allow'  → 直接允许，记录日志
    │
    ├── result.behavior === 'deny'   → 拒绝，记录日志
    │     └── auto-mode 时发出通知（"xxx denied by auto mode"）
    │
    └── result.behavior === 'ask'    → 需要用户决策
          │
          ├── awaitAutomatedChecksBeforeDialog?
          │     └── handleCoordinatorPermission()
          │           ← 等待自动分类器结果
          │
          ├── swarm worker 模式?
          │     └── handleSwarmWorkerPermission()
          │           ← 通知协调者决策
          │
          └── 交互模式（默认）
                └── handleInteractivePermission()
                      ← 向 setToolUseConfirmQueue 推入确认请求
                      ← 等待用户点击 Yes/No/Always
```

### 5.2 权限队列机制

```typescript
// 工具调用被排入确认队列
setToolUseConfirmQueue(prev => [...prev, {
  id: toolUseId,
  tool,
  input,
  description,
  resolve,  // 用户决策后调用此函数
}])

// 用户在 UI 中点击 Yes/No/Always
// → resolve({ behavior: 'allow' | 'deny', ... })
// → Promise 完成，工具执行继续
```

---

## 6. 权限决策结果类型

```typescript
type PermissionDecision<Input> =
  | {
      behavior: 'allow'
      updatedInput?: Input                // 钩子可能修改输入（如扩展文件路径）
      decisionReason?: DecisionReason     // 决策来源
    }
  | {
      behavior: 'deny'
      message: string
      suggestBypassPermissions?: boolean
    }

type DecisionReason =
  | { type: 'rule'; rule: string }          // 匹配了规则
  | { type: 'classifier'; classifier: 'auto-mode'; reason: string }  // AI 分类器
  | { type: 'user'; decision: 'yes' | 'no' | 'always' | 'never' }  // 用户手动决策
```

---

## 7. 文件系统权限（`filesystem.ts`，62KB）

最复杂的权限模块，处理文件路径相关的权限：

```typescript
async function checkWritePermissionForTool(
  filePath: string,
  context: ToolUseContext,
): Promise<PermissionResult> {
  
  // 1. 路径必须在项目根目录或 additionalWorkingDirectories 内
  if (!isPathUnderTrustedDirectory(filePath, context)) {
    return { behavior: 'ask', reason: `路径 ${filePath} 在项目外` }
  }
  
  // 2. 检查敏感路径（shell 配置、SSH 密钥、git 配置等）
  if (isSensitivePath(filePath)) {
    return { behavior: 'ask', reason: '敏感文件' }
  }
  
  // 3. 检查文件权限规则（alwaysAllow/alwaysDeny）
  const rule = matchingRuleForInput(filePath, context.getAppState().toolPermissionContext)
  // ...
}
```

### 7.1 受保护目录列表

```typescript
const SENSITIVE_PATHS = [
  '~/.ssh/',
  '~/.gnupg/',
  '~/.aws/',
  '~/.config/claude/',  // Claude Code 自身配置
  '/etc/',
  '/usr/',
  '/bin/',
  // ...
]
```

### 7.2 路径验证（`pathValidation.ts`）

```typescript
// 防止路径遍历攻击
function isPathSafe(path: string, cwd: string): boolean {
  const resolved = resolve(cwd, path)
  
  // 必须在工作目录或可信目录内
  return trustedDirectories.some(dir => resolved.startsWith(dir))
}
```

---

## 8. 自动模式（`yoloClassifier.ts`，52KB）

自动模式（`mode: 'auto'`）使用 LLM 分类器自动审批/拒绝工具调用：

```typescript
// auto 模式的决策流程：
// 1. 将工具调用发送给轻量级分类器模型
// 2. 分类器返回 allow/deny 决策 + 理由
// 3. 写入分类器批准缓存（classifierApprovals）

async function runYoloClassifier(
  tool: Tool,
  input: unknown,
  context: string,
): Promise<ClassifierDecision> {
  // 调用 Claude Haiku（轻量）进行快速分类
  const result = await callClassifier({
    tool: tool.name,
    input: JSON.stringify(input),
    conversationContext: context,
  })
  
  return {
    decision: result.decision,  // 'allow' | 'deny'
    reason: result.reason,
    confidence: result.confidence,
  }
}
```

---

## 9. 拒绝追踪（`denialTracking.ts`）

追踪连续拒绝次数，防止 AI 无限重试被拒绝的操作：

```typescript
type DenialTrackingState = {
  consecutiveDenials: number  // 连续拒绝次数
  lastDeniedTool: string      // 最后被拒绝的工具名
  lastDeniedAt: number        // 时间戳
}

// 当连续拒绝次数超过阈值时，回退到提示用户
const DENIAL_THRESHOLD = 3
if (denialCount >= DENIAL_THRESHOLD) {
  // 从 auto-deny 回退到手动确认
}
```

---

## 10. 权限规则持久化（`permissions.ts` + settings）

权限规则存储在三个地方：

| 来源 | 存储位置 | 作用域 |
|------|---------|-------|
| 全局规则 | `~/.config/claude/settings.json` | 所有项目 |
| 项目规则 | `.claude/settings.json`（git 根目录） | 单个项目 |
| 本地规则 | `.claude/settings.local.json`（不提交 git）| 本地临时 |
| 运行时规则 | AppState.toolPermissionContext | 当前会话 |

---

## 11. 权限 UI（`components/permissions/`）

```tsx
// 权限确认弹框（在 REPL 中渲染）
function PermissionRequest({ confirm, onDecide }) {
  return (
    <Box flexDirection="column" borderStyle="round">
      <Text>{confirm.description}</Text>
      <Text>是否允许执行 {confirm.tool.userFacingName(confirm.input)}？</Text>
      <Text>[y] 允许  [n] 拒绝  [a] 总是允许  [x] 总是拒绝</Text>
    </Box>
  )
}
```

用户的决策被记录：
- `y`（单次允许）：当前调用允许，下次再问
- `a`（总是允许）：添加 `alwaysAllowRules` 规则，写入 settings.json
- `n`（单次拒绝）：当前调用拒绝
- `x`（总是拒绝）：添加 `alwaysDenyRules` 规则

---

## 12. 设计模式

### 12.1 责任链（Chain of Responsibility）
权限检查按 deny → allow → ask → 工具逻辑的顺序依次检查，每一层可以短路退出。

### 12.2 策略注入（`CanUseToolFn`）
`useCanUseTool` 返回一个函数，被注入到 `ToolUseContext`，让工具在递归调用时也能进行权限检查。

### 12.3 乐观预批准（Speculative Execution）
推测执行时，权限检查结果被缓存，主线程接手时重用缓存的决策而不是重复弹框。
