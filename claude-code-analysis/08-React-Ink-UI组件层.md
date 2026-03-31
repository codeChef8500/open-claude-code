# Claude Code 源码深度解读 — 08 React + Ink UI 组件层

> 覆盖文件：`screens/REPL.tsx`（895KB，最大文件）、`screens/Doctor.tsx`、`screens/ResumeConversation.tsx`、`components/`（140+ 组件）、`ink.ts`

---

## 1. React + Ink 架构概述

Claude Code 使用 **React + Ink** 在终端中渲染 UI，与普通 Web React 的区别：

| 特性 | 普通 React (DOM) | Claude Code (Ink) |
|------|----------------|-----------------|
| 渲染目标 | 浏览器 DOM | 终端 stdout (ANSI 转义码) |
| 布局引擎 | CSS Flexbox / Grid | Yoga (CSS Flexbox 子集) |
| 文本渲染 | HTML elements | Ink `<Text>` 组件 |
| 事件系统 | DOM 事件 | Node.js readline / stdin |
| 样式 | CSS classes | Ink 内联 props |

---

## 2. 主界面 `REPL.tsx` — 核心结构

`REPL.tsx` 是 Claude Code 最大的单个文件（895KB），整合了几乎所有 UI 功能。

### 2.1 组件层次

```
REPL
├── AppStateProvider          ← 全局状态 Context
│   ├── ThemeProvider          ← 主题 Context
│   ├── MailboxProvider        ← Agent 消息收件箱
│   └── VoiceProvider          ← 语音输入（ant-only）
│
├── [主界面层]
│   ├── StatusBar              ← 顶部状态栏（模型/成本/连接）
│   │
│   ├── VirtualMessageList     ← 消息列表（虚拟滚动）
│   │   ├── UserMessage        ← 用户消息气泡
│   │   └── AssistantMessage   ← AI 响应（流式渲染）
│   │       ├── ThinkingBlock  ← Extended Thinking 折叠块
│   │       ├── TextBlock      ← Markdown 渲染
│   │       └── ToolUse        ← 工具调用展示
│   │           ├── BashToolUI         ← 命令执行 UI
│   │           ├── FileEditToolUI     ← 文件编辑 diff
│   │           ├── AgentToolUI        ← 子 Agent 进度
│   │           └── [其他工具 UI]
│   │
│   ├── PromptInput            ← 底部输入框
│   │   ├── TextInput          ← 文本输入（支持多行）
│   │   ├── CommandAutocomplete ← Slash 命令自动补全
│   │   ├── FileAutoComplete   ← 文件路径自动补全
│   │   └── PromptInputFooter  ← 底部导航栏
│   │       ├── TasksPill      ← Agent 任务胶囊
│   │       ├── BridgePill     ← Bridge 连接状态
│   │       └── SpinnerStatus  ← 当前操作状态
│   │
│   └── PermissionRequestUI    ← 权限确认弹框
│       └── ToolUseConfirmQueue ← 待确认工具调用队列
```

### 2.2 REPL.tsx 的关键 Hooks

```typescript
function REPL(props: REPLProps) {
  // ─── 核心状态 ────────────────────────────────────
  const [messages, setMessages] = useState<Message[]>(initialMessages)
  const [isLoading, setIsLoading] = useState(false)
  
  // ─── 权限队列 ────────────────────────────────────
  const [toolUseConfirmQueue, setToolUseConfirmQueue] = useState<ToolUseConfirm[]>([])
  
  // ─── 工具 + 命令（合并内置 + MCP + 插件）─────────
  const mergedTools = useMergedTools()      // hook: 组合工具池
  const mergedCommands = useMergedCommands() // hook: 组合命令列表
  
  // ─── 输入处理 ────────────────────────────────────
  const [inputValue, setInputValue] = useState('')
  const vimInput = useVimInput({ ... })      // Vim 模式输入
  const textInput = useTextInput({ ... })    // 普通文本输入
  
  // ─── 快捷键 ──────────────────────────────────────
  useGlobalKeybindings(...)                  // 全局快捷键处理
  useCommandKeybindings(...)                 // 命令特定快捷键
  
  // ─── REPL Bridge ─────────────────────────────────
  useReplBridge(...)    // IDE 集成桥接（115KB hook）
  
  // ─── 推测执行 ────────────────────────────────────
  useSpeculativeExecution(...)
  
  // ─── 会话 ────────────────────────────────────────
  useLogMessages(messages)   // 录制消息到 transcript
  
  // ─── 其他 20+ hooks ──────────────────────────────
}
```

---

## 3. 虚拟消息列表（`VirtualMessageList`）

对话消息列表使用**虚拟滚动**避免长对话的性能问题：

```typescript
// hooks/useVirtualScroll.ts（35KB）
function useVirtualScroll(messages: Message[], viewportHeight: number) {
  // 只渲染视口内的消息
  const visibleRange = calculateVisibleRange(
    messages,
    scrollOffset,
    viewportHeight,
    messageHeights,
  )
  
  return {
    visibleMessages: messages.slice(visibleRange.start, visibleRange.end),
    totalHeight,
    scrollOffset,
    onScroll,
  }
}
```

### 3.1 消息高度估算

```typescript
// 消息高度估算（渲染前）
function estimateMessageHeight(message: Message): number {
  if (message.type === 'user') return 3   // 用户消息：约 3 行
  if (message.type === 'assistant') {
    const textLength = getTextLength(message)
    return Math.ceil(textLength / TERMINAL_WIDTH) + 2
  }
  // ...
}
```

---

## 4. 核心 UI 组件

### 4.1 `AssistantMessage` — AI 响应渲染

```tsx
// components/AssistantMessage.tsx
function AssistantMessage({ message, isStreaming }: Props) {
  return (
    <Box flexDirection="column">
      {message.message.content.map((block, i) => {
        switch (block.type) {
          case 'text':
            return <MarkdownRenderer key={i} text={block.text} isStreaming={isStreaming} />
          case 'thinking':
            return <ThinkingBlock key={i} thinking={block.thinking} />
          case 'redacted_thinking':
            return <RedactedThinkingBlock key={i} />
          case 'tool_use':
            return <ToolUseComponent key={i} toolUse={block} />
        }
      })}
    </Box>
  )
}
```

### 4.2 `MarkdownRenderer` — Markdown 转终端文本

```typescript
// Markdown 元素的终端渲染：
// ## 标题     → 粗体 + 下划线
// **粗体**    → 粗体文本
// `代码`      → 高亮背景
// ```代码块``` → 语法高亮（通过 chalk）
// - 列表项    → ● 前缀
// | 表格 |    → ASCII 表格
```

### 4.3 `BashToolUI` — 命令执行 UI

```tsx
// tools/BashTool/UI.tsx
function BashToolResultMessage({ message }) {
  const [isExpanded, setIsExpanded] = useState(false)
  
  return (
    <Box flexDirection="column">
      {/* 命令头：> bash 命令摘要 */}
      <Text dimColor>❯ {summarizeCommand(message.command)}</Text>
      
      {/* 折叠/展开输出 */}
      {isExpanded ? (
        <Text>{message.output}</Text>
      ) : (
        <Text dimColor>{truncate(message.output, 3)} ...</Text>
      )}
      
      {/* 退出码 */}
      {message.exitCode !== 0 && (
        <Text color="error">Exit {message.exitCode}</Text>
      )}
    </Box>
  )
}
```

### 4.4 `AgentToolUI` — 子 Agent 进度

```tsx
function AgentProgress({ agentId, status }) {
  const task = useTask(agentId)
  
  return (
    <Box>
      <Spinner spinning={task.status === 'running'} />
      <Text>{task.description}</Text>
      {task.status === 'completed' && <Text color="success"> ✓</Text>}
      {task.status === 'failed' && <Text color="error"> ✗</Text>}
    </Box>
  )
}
```

### 4.5 `PermissionRequest` — 权限确认 UI

```tsx
function PermissionRequest({ confirm, onDecide }) {
  useInput((input) => {
    switch (input.toLowerCase()) {
      case 'y': onDecide({ behavior: 'allow' }); break
      case 'n': onDecide({ behavior: 'deny' }); break
      case 'a': onDecide({ behavior: 'allow', alwaysAllow: true }); break
      case 'x': onDecide({ behavior: 'deny', alwaysDeny: true }); break
    }
  })
  
  return (
    <Box borderStyle="round" borderColor="yellow">
      <Text>⚠️  {confirm.description}</Text>
      <Text dimColor>[y] 允许  [n] 拒绝  [a] 总是允许  [x] 总是拒绝</Text>
    </Box>
  )
}
```

---

## 5. 输入系统

### 5.1 多模式输入

Claude Code 支持三种输入模式：

```typescript
// 1. 普通文本模式（hooks/useTextInput.ts，17KB）
function useTextInput({ onSubmit, ... }) {
  useInput((input, key) => {
    if (key.return) onSubmit(currentValue)
    if (key.ctrl && input === 'c') onInterrupt()
    // 多行输入（Shift+Enter）
    if (key.shift && key.return) insertNewline()
    // 历史导航（↑ ↓）
    if (key.upArrow) navigateHistory('up')
    if (key.downArrow) navigateHistory('down')
  })
}

// 2. Vim 模式（hooks/useVimInput.ts，9KB）
function useVimInput({ ... }) {
  // Normal / Insert / Visual 模式切换
  // h/j/k/l 移动，i/a 进入 Insert 模式
  // :w 保存（提交），Esc 回到 Normal 模式
}

// 3. 语音输入（hooks/useVoice.ts，45KB，ant-only）
function useVoice({ onTranscript }) {
  // 集成 Whisper 或平台语音识别
  // 实时转录并注入到文本输入
}
```

### 5.2 自动补全系统（`useTypeahead.tsx`，212KB — 最大 hook）

```typescript
// 处理 @ 文件提及、/ 命令补全、Tab 文件路径补全
function useTypeahead({ value, onChange, ... }) {
  // 1. 检测补全触发字符（/ 或 @）
  const trigger = detectTrigger(value)
  
  // 2. 根据触发类型获取候选
  if (trigger === '/') {
    candidates = await getCommandCandidates(partial)
  } else if (trigger === '@') {
    candidates = await getFileCandidates(partial)
  }
  
  // 3. 渲染候选列表
  return <TypeaheadList items={candidates} onSelect={handleSelect} />
}
```

---

## 6. 主题系统

```typescript
// 主题定义（utils/theme.ts）
type Theme = {
  primary: string       // 主色（通常蓝色）
  secondary: string     // 次色
  success: string       // 成功（绿色）
  error: string         // 错误（红色）
  warning: string       // 警告（黄色）
  text: string          // 正文
  dimText: string       // 暗化文字
  background: string    // 背景
  // ...
}

// 内置主题：
// dark（默认暗色）、light（亮色）
// system（跟随系统）
// colorblind（无障碍）
// custom（用户自定义）
```

---

## 7. `Doctor.tsx` — 诊断界面

```
/doctor 命令打开诊断界面，检查：
├── API 连接状态
├── 认证状态（OAuth / API Key）
├── 模型可用性
├── MCP 服务器状态
├── 系统依赖（ripgrep, git 等）
├── 权限配置
└── 版本信息
```

---

## 8. `ResumeConversation.tsx` — 会话恢复界面

```typescript
// 列出最近的会话供用户选择恢复
function ResumeConversation({ onSelect, onCancel }) {
  const sessions = useRecentSessions()  // 读取 ~/.claude/sessions/
  
  return (
    <Box flexDirection="column">
      <Text bold>选择要恢复的会话：</Text>
      {sessions.map(session => (
        <SessionItem
          key={session.id}
          session={session}
          onSelect={onSelect}
        />
      ))}
    </Box>
  )
}
```

---

## 9. Ink 渲染器包装（`ink.ts`）

```typescript
// Claude Code 封装的 Ink 导出
export { Box, Text, Static, Spacer, useInput, useApp, useFocus } from 'ink'

// 扩展的 renderToString（用于测试和快照）
export function renderToString(component: React.ReactNode): string {
  return inkRenderToString(component, { columns: 80 })
}
```

---

## 10. 关键 UI 特性

### 10.1 流式渲染（Streaming）
LLM 响应流式输出时，UI 实时更新：
- text block：字符级追加
- thinking block：折叠显示，streaming 中可折叠
- tool_use block：开始时显示 spinner，结束时显示结果

### 10.2 背景提示（Background Hint）
长时间运行的 Bash 命令 (>2s) 显示 "运行中，可继续输入" 提示：

```tsx
function BackgroundHint() {
  return (
    <Text dimColor>
      ↑ 命令仍在运行，可以输入新消息（将在命令完成后发送）
    </Text>
  )
}
```

### 10.3 Spinner 状态指示器

```typescript
type SpinnerMode =
  | 'thinking'        // Claude 正在思考
  | 'tool_executing'  // 工具执行中
  | 'compacting'      // 上下文压缩中
  | 'idle'            // 空闲
```

---

## 11. 组件目录结构精选

```
src/components/
├── Message.tsx              # 消息渲染路由
├── AssistantMessage.tsx     # AI 响应
├── UserMessage.tsx          # 用户输入气泡
├── MarkdownRenderer.tsx     # Markdown → 终端文本
├── ThinkingBlock.tsx        # Extended Thinking 块
├── StatusBar.tsx            # 顶部状态栏
├── PromptInput.tsx          # 底部输入框（大组件）
├── PromptInputFooter.tsx    # 底部导航栏
├── TasksPill.tsx            # Agent 任务胶囊
├── VirtualMessageList.tsx   # 虚拟滚动消息列表
├── permissions/
│   ├── PermissionRequest.tsx   # 权限确认弹框
│   └── ToolUseConfirmQueue.tsx # 权限队列管理
├── mcp/
│   └── McpConnectionStatus.tsx # MCP 连接状态
└── ... (140+ 组件)
```
