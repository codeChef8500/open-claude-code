# Claude Code 源码深度解读 — 10 Bridge / IDE 集成系统

> 覆盖文件：`bridge/`（30 个文件，~400KB）、`hooks/useReplBridge.tsx`（115KB）、`hooks/useIDEIntegration.tsx`（10KB）

---

## 1. 模块职责概述

Bridge 系统是 Claude Code CLI 与外部环境（Claude.ai 网页端、VS Code 扩展、JetBrains 插件）双向通信的基础设施，实现：

- **远程控制**：通过 Claude.ai 在浏览器中控制终端里的 Claude Code
- **IDE 集成**：VS Code / JetBrains 扩展与 Claude Code 共享光标、文件选择、诊断信息
- **权限委托**：IDE 界面处理权限确认弹框（替代终端 UI）

---

## 2. Bridge 架构图

```
┌───────────────────────────────────────────────────────────┐
│  Claude.ai 网页端 / IDE 扩展（外部进程）                    │
│                                                           │
│  ┌─────────────┐    HTTP/WebSocket/SSE    ┌────────────┐  │
│  │ Claude.ai   │ ◄──────────────────────► │  bridge/   │  │
│  │ Web UI      │                          │  replBridge│  │
│  └─────────────┘                          └─────┬──────┘  │
│                                                │          │
│  ┌─────────────┐    MCP over stdio/WSS         │          │
│  │ VS Code     │ ◄──────────────────────────────┘          │
│  │ Extension   │                                           │
│  └─────────────┘                                           │
└───────────────────────────────────────────────────────────┘
                         ▲
                         │ 内部 IPC（useReplBridge hook）
                         ▼
┌───────────────────────────────────────────────────────────┐
│  Claude Code CLI（本地进程）                                │
│                                                           │
│  REPL.tsx ─► useReplBridge ─► replBridge.ts ─► bridgeMain │
│                ▲                                           │
│                └── 消息双向流                               │
└───────────────────────────────────────────────────────────┘
```

---

## 3. `bridgeMain.ts` — Bridge 入口（115KB）

`bridgeMain.ts` 是 Bridge 模式的完整实现，当 `claude remote-control` 被调用时执行：

```typescript
// bridge/bridgeMain.ts
export async function bridgeMain(args: string[]): Promise<void> {
  // 1. 解析 Bridge 配置
  const config = parseBridgeConfig(args)
  
  // 2. 建立与 Claude.ai 的连接
  const transport = await createBridgeTransport(config)
  
  // 3. 启动 REPL（带 Bridge 模式标志）
  const { render } = await import('../ink.js')
  render(
    <AppStateProvider>
      <REPL {...replProps} bridgeMode={true} />
    </AppStateProvider>
  )
  
  // 4. Bridge 循环（处理入站/出站消息）
  await runBridgeLoop(transport, replInterface)
}
```

---

## 4. `replBridge.ts` — REPL Bridge 核心（100KB）

这是 Bridge 系统中最复杂的模块，处理所有协议细节：

### 4.1 消息类型（`types.ts`）

```typescript
type BridgeMessage =
  // 入站（从外部到 Claude Code）
  | { type: 'user_message'; content: string | ContentBlock[]; attachments?: Attachment[] }
  | { type: 'interrupt' }
  | { type: 'permission_response'; toolUseId: string; decision: PermissionDecision }
  | { type: 'question_response'; questionId: string; answer: string }
  | { type: 'set_permission_mode'; mode: PermissionMode }
  | { type: 'inject_file'; filePath: string }
  // 出站（从 Claude Code 到外部）
  | { type: 'assistant_message'; content: ContentBlock[] }
  | { type: 'stream_delta'; delta: string; messageId: string }
  | { type: 'permission_request'; toolUseId: string; description: string; tool: string; input: unknown }
  | { type: 'tool_result'; toolUseId: string; result: unknown }
  | { type: 'status_update'; status: 'thinking' | 'executing' | 'idle'; model: string }
  | { type: 'context_info'; usedTokens: number; maxTokens: number }
  | { type: 'session_created'; sessionId: string; url: string }
```

### 4.2 会话创建（`createSession.ts`）

```typescript
// 在 Claude.ai 服务端创建一个会话，获取 WebSocket URL
async function createBridgeSession(config: BridgeConfig): Promise<SessionInfo> {
  const response = await fetch(`${config.apiUrl}/api/sessions`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${config.token}` },
    body: JSON.stringify({
      agentType: 'claude-code',
      capabilities: ['file_access', 'shell_execution'],
    }),
  })
  
  return {
    sessionId: response.sessionId,
    wsUrl: response.webSocketUrl,
    displayUrl: response.displayUrl,
  }
}
```

### 4.3 传输层（`replBridgeTransport.ts`）

```typescript
// 支持多种传输方式：
// 1. WebSocket（主要用于远程会话）
// 2. stdio（本地 MCP 服务器模式）
// 3. Unix Domain Socket（本地多 Agent 通信）

class WebSocketBridgeTransport implements BridgeTransport {
  private ws: WebSocket
  
  async send(message: BridgeMessage): Promise<void> {
    this.ws.send(JSON.stringify(message))
  }
  
  onMessage(handler: (msg: BridgeMessage) => void): void {
    this.ws.on('message', (data) => handler(JSON.parse(data.toString())))
  }
  
  // 重连逻辑（指数退避）
  private async reconnect(): Promise<void> {
    let delay = INITIAL_RECONNECT_DELAY
    while (retries < MAX_RETRIES) {
      await sleep(delay)
      delay = Math.min(delay * 2, MAX_RECONNECT_DELAY)
      retries++
      // 尝试重新连接
    }
  }
}
```

---

## 5. `initReplBridge.ts` — Bridge 初始化（23KB）

```typescript
export async function initReplBridge(config: BridgeInitConfig): Promise<ReplBridgeHandle> {
  // 1. 创建远程会话
  const session = await createBridgeSession(config)
  
  // 2. 打印会话 URL（用户可以在浏览器中打开）
  console.log(`\n🌐 在浏览器中打开: ${session.displayUrl}\n`)
  
  // 3. 建立双向通信通道
  const transport = new WebSocketBridgeTransport(session.wsUrl)
  await transport.connect()
  
  // 4. 返回 handle（供 useReplBridge 使用）
  return createReplBridgeHandle(transport, session)
}
```

---

## 6. `useIDEIntegration` — VS Code / JetBrains 集成（10KB）

```typescript
function useIDEIntegration() {
  // 1. 检测 IDE 类型（VS Code / JetBrains / 其他）
  const ideType = detectIDE()
  
  // 2. 注册文件选择监听（IDE 选中的文件自动注入到提示词）
  useEffect(() => {
    if (ideType === 'vscode') {
      const mcpClient = getVscodeMcpClient()
      mcpClient.on('fileSelected', (file) => {
        // 自动在输入框中插入 @file_path
        appendToInput(`@${file.path}`)
      })
    }
  }, [ideType])
  
  // 3. 光标位置上下文（AI 可读取当前光标位置）
  const cursorContext = useIdeSelection()
  
  // 4. 诊断信息推送（LSP 错误自动注入到上下文）
  useDiffInIDE()
}
```

### 6.1 VS Code 集成（通过 MCP over stdio）

```
VS Code Extension          Claude Code
      │                         │
      │  MCP stdio protocol     │
      │ ──────────────────────► │ vscodeSdkMcp.ts
      │                         │     ├── 文件读取（lsp资源）
      │ ◄────────────────────── │     ├── 编辑通知
      │                         │     └── 诊断信息
```

### 6.2 JetBrains 集成（通过 HTTP polling）

```typescript
// initJetBrainsDetection()
// 检测是否在 JetBrains IDE 中运行（检查 IDEA_INITIAL_DIRECTORY 等环境变量）
// 通过 HTTP 轮询获取 IDE 状态（光标位置、打开文件等）
```

---

## 7. `bridgeEnabled.ts` — Bridge 可用性检查（8KB）

```typescript
// 判断是否启用 Bridge 功能
export function isBridgeEnabled(): boolean {
  // 1. feature('BRIDGE_MODE') 必须开启
  if (!feature('BRIDGE_MODE')) return false
  
  // 2. 不能在 CI / 无头模式下启动 Bridge
  if (isCI || isNonInteractive) return false
  
  // 3. 检查配置（settings.json 中的 replBridgeEnabled）
  return getSettings().replBridgeEnabled ?? true
}

// 判断是否应该自动在启动时连接 Bridge
export function shouldAutoConnectBridge(): boolean {
  return isBridgeEnabled() && getSettings().remoteControlAtStartup ?? false
}
```

---

## 8. `remoteBridgeCore.ts` — 远程 Bridge 核心（39KB）

处理所有权限委托逻辑（终端权限 → Bridge 客户端决策）：

```typescript
// 当工具需要权限时，通过 Bridge 向 IDE/Web 端发送请求
async function requestPermissionViaBridge(
  transport: BridgeTransport,
  request: PermissionRequest,
): Promise<PermissionDecision> {
  // 1. 发送权限请求到外部客户端
  await transport.send({
    type: 'permission_request',
    toolUseId: request.toolUseId,
    description: request.description,
    tool: request.tool.name,
    input: request.input,
  })
  
  // 2. 等待外部客户端决策
  return new Promise((resolve) => {
    permissionCallbacks.set(request.toolUseId, resolve)
  })
}

// 当收到外部的权限响应时
transport.onMessage((msg) => {
  if (msg.type === 'permission_response') {
    const callback = permissionCallbacks.get(msg.toolUseId)
    callback?.({ behavior: msg.decision })
  }
})
```

---

## 9. `workSecret.ts` — 安全认证

Bridge 使用"工作密钥"（work secret）验证本地进程与 Bridge 服务的连接：

```typescript
// 生成一次性工作密钥（防止未授权的 Bridge 连接）
export function generateWorkSecret(): string {
  return randomBytes(32).toString('hex')
}

// Bridge 建立连接时验证密钥
export function validateWorkSecret(
  provided: string,
  expected: string,
): boolean {
  // 使用时间恒定比较（防止时序攻击）
  return timingSafeEqual(
    Buffer.from(provided),
    Buffer.from(expected),
  )
}
```

---

## 10. Bridge 数据流

```
用户在 Claude.ai 网页端输入消息
    │
    ▼ WebSocket 推送
bridgeMain.ts 收到 'user_message'
    │
    ▼
useReplBridge hook 处理
    │
    ├── 注入到 REPL 的 onSubmit 函数
    ▼
QueryEngine.submitMessage(content)
    │
    ▼ 流式响应
AssistantMessage 消息生成
    │
    ▼ stream_delta 事件
transport.send({ type: 'stream_delta', delta, messageId })
    │
    ▼ WebSocket 推送回 Claude.ai
网页端实时显示 AI 响应
    │
    ├── [遇到需要权限的工具]
    │   ▼ permission_request 事件
    │   网页端显示权限确认对话框
    │   用户点击 Yes → permission_response 事件
    │   Claude Code 收到并继续执行
```

---

## 11. 设计亮点

### 11.1 协议无关传输层
`BridgeTransport` 接口抽象了具体传输，使得 WebSocket、stdio、UDS 可以互换使用，便于测试和扩展。

### 11.2 权限委托
Bridge 模式下，权限确认不显示在终端，而是通过 Bridge 推送到网页端或 IDE，提供更好的 GUI 体验。

### 11.3 工作密钥安全
每次启动生成一次性密钥，防止本地恶意进程伪装成 Bridge 客户端。
