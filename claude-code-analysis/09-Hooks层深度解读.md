# Claude Code 源码深度解读 — 09 Hooks 层深度解读

> 覆盖目录：`hooks/`（80+ hook 文件）

---

## 1. Hooks 层概述

`hooks/` 目录包含 80+ 个自定义 React hook，是 REPL.tsx 的核心业务逻辑所在。这些 hook 按职能分为几大类：

| 类别 | 典型 hook | 说明 |
|------|---------|------|
| 输入处理 | `useTextInput`, `useVimInput`, `useTypeahead` | 用户输入 |
| 权限 | `useCanUseTool`, `toolPermission/` | 工具权限决策 |
| 网络/集成 | `useReplBridge`, `useIDEIntegration`, `useRemoteSession` | IDE/Bridge |
| 数据持久 | `useLogMessages`, `useAssistantHistory` | 消息录制 |
| UI 辅助 | `useBlink`, `useElapsedTime`, `useVirtualScroll` | 视觉效果 |
| 任务管理 | `useTasksV2`, `useInboxPoller`, `useScheduledTasks` | Agent 任务 |
| 会话 | `useCancelRequest`, `useAwaySummary` | 会话生命周期 |

---

## 2. 最重要的 Hook 深度解读

### 2.1 `useReplBridge` — IDE 集成核心（115KB，最大 hook）

`useReplBridge` 是 Claude Code 与 Claude.ai 网页端集成的核心，实现了双向通信：

```typescript
function useReplBridge({
  messages, setMessages, tools, commands,
  setIsLoading, onSubmit, ...
}) {
  // 1. 初始化 WebSocket/SSE 连接
  const { bridge, bridgeStatus } = useBridgeConnection()
  
  // 2. 接收来自 IDE 的指令（新消息、权限决策等）
  useEffect(() => {
    bridge.on('inbound_message', async (msg) => {
      switch (msg.type) {
        case 'user_message':
          await onSubmit(msg.content)  // 代理用户发送消息
          break
        case 'permission_decision':
          resolvePermission(msg.toolUseId, msg.decision)
          break
        case 'interrupt':
          abortController.abort()
          break
      }
    })
  }, [bridge])
  
  // 3. 发送消息/状态到 IDE
  useEffect(() => {
    if (newMessages.length > 0) {
      bridge.send({ type: 'messages_update', messages: newMessages })
    }
  }, [messages])
  
  // 4. 权限确认通过 Bridge 处理（IDE 决策）
  // 5. 保活心跳
}
```

Bridge 状态机：

```
disconnected → connecting → connected
                                │
                            reconnecting ←─ 网络中断
                                │
                             disconnected ← 用户关闭
```

### 2.2 `useCanUseTool` — 权限决策 hook（40KB）

已在第 06 章详解，关键点：
- 返回 `CanUseToolFn` 函数注入到 `ToolUseContext`
- 整合规则检查 + UI 确认弹框
- 支持 Coordinator/Swarm Worker/交互式三种处理路径

### 2.3 `useTypeahead` — 智能补全（212KB，最大 hook 文件）

这个 hook 处理 REPL 输入框的所有自动补全逻辑：

```typescript
function useTypeahead({ value, onChange, commands, ... }) {
  // ─── Slash 命令补全（/compact, /memory 等）──────────
  const commandSuggestions = useCommandSuggestions(value, commands)
  
  // ─── @文件提及补全（@src/main.tsx）─────────────────
  const fileSuggestions = useFileSuggestions(value)
  
  // ─── 剪贴板图片提示（粘贴图片时）─────────────────────
  const clipboardHint = useClipboardImageHint(value)
  
  // ─── 工具搜索提示（工具延迟加载时）─────────────────────
  const toolSearchHint = useToolSearchHint(value)
  
  // 合并所有建议
  const suggestions = merge([
    commandSuggestions,
    fileSuggestions,
    toolSearchHint,
  ])
  
  return { suggestions, onSelect, renderDropdown }
}
```

文件提及系统（`fileSuggestions.ts`，27KB）：

```typescript
// @mention 触发文件路径补全
// 支持：
// @src/         → 列出目录下文件
// @main         → 模糊搜索 main.tsx, main.ts 等
// @*.tsx        → glob 匹配
// 近期编辑文件优先排序
```

### 2.4 `useTextInput` — 多行文本输入（17KB）

```typescript
function useTextInput({
  onSubmit,
  onInterrupt,
  isDisabled,
  vimEnabled,
  ...
}) {
  const [value, setValue] = useState('')
  const [cursorPos, setCursorPos] = useState(0)
  const [history, setHistory] = useState<string[]>([])
  const [historyIndex, setHistoryIndex] = useState(-1)
  
  useInput((char, key) => {
    // Enter → 提交（含空行检查）
    if (key.return && !key.shift) {
      if (value.trim()) {
        onSubmit(value)
        setHistory(prev => [value, ...prev])
        setValue('')
      }
      return
    }
    
    // Shift+Enter → 多行输入（插入换行符）
    if (key.return && key.shift) {
      setValue(prev => insertAt(prev, cursorPos, '\n'))
      return
    }
    
    // Ctrl+C → 中断当前请求
    if (key.ctrl && char === 'c') {
      onInterrupt()
      return
    }
    
    // ↑/↓ → 历史导航
    if (key.upArrow) navigateHistory('up')
    if (key.downArrow) navigateHistory('down')
    
    // Tab → 触发自动补全
    if (key.tab) triggerTypeahead()
    
    // 普通字符输入
    setValue(prev => insertAt(prev, cursorPos, char))
    setCursorPos(prev => prev + char.length)
  })
  
  return { value, cursorPos, ... }
}
```

### 2.5 `useGlobalKeybindings` — 全局快捷键（31KB）

```typescript
// 全局快捷键映射
const GLOBAL_KEYBINDINGS = {
  // 会话管理
  'Ctrl+L': clearScreen,
  'Ctrl+R': searchHistory,
  'Escape': cancelCurrentRequest,
  
  // 视图切换
  'Alt+T': toggleTasksView,
  'Alt+A': toggleTeammatesView,
  
  // Bridge
  'Alt+O': openInBrowser,
  
  // 导航
  'PageUp': scrollUp,
  'PageDown': scrollDown,
}
```

### 2.6 `useLogMessages` — 消息录制（5KB）

```typescript
function useLogMessages(messages: Message[]) {
  const prevLengthRef = useRef(0)
  
  useEffect(() => {
    // 只记录新增的消息（差量更新）
    const newMessages = messages.slice(prevLengthRef.current)
    
    if (newMessages.length > 0) {
      // fire-and-forget：异步录制，不阻塞 UI
      void recordTranscript(messages)
      prevLengthRef.current = messages.length
    }
  }, [messages])
}
```

### 2.7 `useRemoteSession` — 远程会话（23KB）

处理 Claude.ai web 端通过 Bridge 远程控制的完整流程：

```typescript
function useRemoteSession({ onRemoteMessage, ... }) {
  const [connectionStatus, setConnectionStatus] = useState('disconnected')
  
  // 建立 SSE 长连接
  useEffect(() => {
    const eventSource = new EventSource(remoteSessionUrl)
    
    eventSource.on('message', (event) => {
      const msg = JSON.parse(event.data)
      switch (msg.type) {
        case 'user_message': onRemoteMessage(msg); break
        case 'interrupt': abortRef.current.abort(); break
        case 'reconnect': reconnect(msg.newUrl); break
      }
    })
    
    return () => eventSource.close()
  }, [remoteSessionUrl])
}
```

### 2.8 `useInboxPoller` — Agent 消息轮询（34KB）

用于 Swarm/Team 模式下轮询 UDS (Unix Domain Socket) 收件箱：

```typescript
function useInboxPoller({ agentId, onMessage }) {
  useEffect(() => {
    const poll = async () => {
      const messages = await pollInbox(agentId)
      for (const msg of messages) {
        onMessage(msg)
      }
    }
    
    const interval = setInterval(poll, POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [agentId])
}
```

### 2.9 `useArrowKeyHistory` — 历史搜索（34KB）

```typescript
// 类似 shell 的历史搜索（Ctrl+R）
function useArrowKeyHistory({ onSelect, ... }) {
  const [query, setQuery] = useState('')
  const history = useAssistantHistory()  // 从 ~/.claude/history 读取
  
  const filtered = history.filter(h =>
    h.prompt.toLowerCase().includes(query.toLowerCase())
  )
  
  return { filtered, onSelect, query, setQuery }
}
```

### 2.10 `useVoice` — 语音输入（45KB，ant-only）

```typescript
function useVoice({ onTranscript, isEnabled }) {
  // 集成平台语音识别（macOS SFSpeechRecognizer / Whisper API）
  // 实时转录后注入到文本输入框
  
  const [isListening, setIsListening] = useState(false)
  
  const startListening = async () => {
    setIsListening(true)
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    
    // 使用 Web Speech API 或 Whisper API 转录
    const transcript = await transcribe(stream)
    onTranscript(transcript)
    setIsListening(false)
  }
}
```

---

## 3. Hooks 依赖图（核心部分）

```
REPL.tsx
    │
    ├── useTextInput          ← 基础文本编辑
    │   └── useArrowKeyHistory ← 历史导航
    │
    ├── useTypeahead          ← 智能补全
    │   ├── fileSuggestions   ← 文件路径候选
    │   └── commandSuggestions ← 命令候选
    │
    ├── useGlobalKeybindings  ← 全局快捷键
    │   └── useCommandKeybindings ← 命令专用快捷键
    │
    ├── useCanUseTool         ← 权限核心
    │   └── toolPermission/handlers/ ← 处理器策略
    │
    ├── useReplBridge         ← IDE 集成（最重）
    │   ├── useDirectConnect  ← WebSocket 直连
    │   └── useMailboxBridge  ← 消息收发
    │
    ├── useMergedTools        ← 合并内置+MCP 工具
    │   └── useMergedClients  ← MCP 客户端列表
    │
    ├── useLogMessages        ← 消息持久化
    │
    └── useIDEIntegration     ← VS Code/JetBrains 集成
```

---

## 4. Hook 设计原则

### 4.1 关注点分离
每个 hook 只处理一个具体的业务逻辑，如 `useElapsedTime`（计时器）、`useBlink`（闪烁效果）各只有几十行。

### 4.2 稳定引用（`useCallback` + `useEffectEvent`）
关键回调函数使用 `useEffectEvent`（React 18 实验性）确保稳定引用，避免因 hook 闭包导致的陈旧状态：

```typescript
// hooks/useReplBridge.tsx 中大量使用
const handleMessage = useEffectEvent((msg) => {
  // 总能访问到最新的 messages、tools 等状态
  // 无论何时调用都不会 stale closure
})
```

### 4.3 清理机制
每个 hook 的 `useEffect` 都返回清理函数，防止内存泄漏：

```typescript
useEffect(() => {
  const subscription = subscribe(handler)
  return () => subscription.unsubscribe()  // 组件卸载时清理
}, [])
```

### 4.4 条件 hook（Feature Flags）
某些 hook 内部使用 `feature()` 检查，但 hook 本身始终被调用（React 规则禁止条件调用 hook）：

```typescript
function useSpeculativeExecution(...) {
  if (!feature('SPECULATION')) {
    // 特性关闭时，hook 存在但什么都不做
    return { speculationState: IDLE_STATE }
  }
  // ...实际逻辑
}
```
