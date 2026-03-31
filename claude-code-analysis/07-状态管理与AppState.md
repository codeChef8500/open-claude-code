# Claude Code 源码深度解读 — 07 状态管理与 AppState

> 覆盖文件：`state/AppStateStore.ts`（21KB，570行）、`state/AppState.tsx`（7KB）、`state/store.ts`、`bootstrap/state.ts`（56KB）

---

## 1. 状态层次结构

Claude Code 的状态分为两个截然不同的层次：

```
┌─────────────────────────────────────────────────────────┐
│  React 状态层（state/ 目录）                              │
│  AppState — 与 UI 渲染直接相关的状态                       │
│  生命周期：React 组件树挂载到卸载                           │
│  访问方式：useAppState() hook / AppStoreContext           │
└───────────────────────────┬─────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│  进程级全局状态层（bootstrap/state.ts）                    │
│  State — 与 React 无关的进程全局状态                       │
│  生命周期：进程启动到退出                                   │
│  访问方式：导出的 getter/setter 函数                        │
└─────────────────────────────────────────────────────────┘
```

---

## 2. `AppState` — React UI 状态

### 2.1 完整类型定义（精选字段）

```typescript
type AppState = DeepImmutable<{
  // ─── 显示设置 ───────────────────────────────────────
  settings: SettingsJson                // 用户配置文件内容
  verbose: boolean                      // --verbose 标志
  mainLoopModel: ModelSetting           // 当前主模型
  mainLoopModelForSession: ModelSetting // 会话固定模型
  statusLineText: string | undefined    // 状态栏文本
  
  // ─── 交互界面 ───────────────────────────────────────
  expandedView: 'none' | 'tasks' | 'teammates'  // 展开视图
  isBriefOnly: boolean                  // 简报模式
  selectedIPAgentIndex: number          // 选中的 Agent 索引
  coordinatorTaskIndex: number          // 协调器任务索引
  viewSelectionMode: 'none' | 'selecting-agent' | 'viewing-agent'
  footerSelection: FooterItem | null    // 底部导航选中项
  
  // ─── 权限 ───────────────────────────────────────────
  toolPermissionContext: ToolPermissionContext  // 工具权限配置（DeepImmutable）
  
  // ─── 模型配置 ───────────────────────────────────────
  agent: string | undefined            // --agent CLI 标志
  kairosEnabled: boolean               // Assistant 模式启用
  fastMode: ...                        // 快速模式状态（Haiku 切换）
  effortValue: EffortValue             // 推理精力级别
  advisorModel: string | undefined     // Advisor 模型
  
  // ─── Bridge/远程连接 ─────────────────────────────────
  remoteSessionUrl: string | undefined
  remoteConnectionStatus: 'connecting' | 'connected' | 'reconnecting' | 'disconnected'
  remoteBackgroundTaskCount: number
  replBridgeEnabled: boolean           // 始终开启的 Bridge
  replBridgeConnected: boolean         // Bridge 连接状态
  replBridgeSessionActive: boolean     // Bridge 会话活跃
  replBridgeSessionUrl: string | undefined
  
  // ─── MCP ────────────────────────────────────────────
  mcp: {
    clients: MCPServerConnection[]     // MCP 服务器连接列表
    tools: Tools                       // MCP 工具列表
    resources: Record<string, ServerResource[]>
  }
  
  // ─── 任务系统 ───────────────────────────────────────
  tasks: TaskState[]                   // Agent 任务列表
  todoList: TodoList | null            // Todo 列表（TodoWriteTool 写入）
  backgroundTaskCount: number          // 后台任务数量
  
  // ─── Hooks ──────────────────────────────────────────
  sessionHooksState: SessionHooksState // session hooks 状态
  replHookContext: REPLHookContext      // REPL hooks 上下文
  
  // ─── 推测执行 ────────────────────────────────────────
  speculationState: SpeculationState   // 推测执行状态（idle/active）
  
  // ─── Thinking 配置 ──────────────────────────────────
  thinkingConfig: ThinkingConfig       // Extended Thinking 配置
  
  // ─── 文件历史 ───────────────────────────────────────
  fileHistory: FileHistoryState        // 文件操作历史
  attribution: AttributionState        // 代码归因状态
  
  // ─── 提示建议 ────────────────────────────────────────
  promptSuggestion: { text: string; promptId: string } | null
  
  // ─── 通知 ───────────────────────────────────────────
  notifications: Notification[]        // 当前通知列表
  
  // ─── 插件 ───────────────────────────────────────────
  plugins: {
    enabled: LoadedPlugin[]
    errors: PluginError[]
  }
  
  // ─── 进度/中断 ───────────────────────────────────────
  inProgressToolUseIDs: Set<string>    // 正在执行的工具 ID
  responseLength: number               // 当前响应长度
  sdkStatus: SDKStatus                 // SDK 状态
  
  // ─── 权限队列 ────────────────────────────────────────
  toolUseConfirmQueue: ToolUseConfirm[]  // 待用户确认的工具调用
}>
```

### 2.2 DeepImmutable 类型

```typescript
// AppState 使用 DeepImmutable<T> 类型保证不可变性
// 防止直接修改状态（必须通过 setAppState 更新）
type DeepImmutable<T> = {
  readonly [K in keyof T]: T[K] extends Array<infer U>
    ? ReadonlyArray<DeepImmutable<U>>
    : T[K] extends Map<infer K, infer V>
    ? ReadonlyMap<DeepImmutable<K>, DeepImmutable<V>>
    : T[K] extends Set<infer U>
    ? ReadonlySet<DeepImmutable<U>>
    : T[K] extends object
    ? DeepImmutable<T[K]>
    : T[K]
}
```

---

## 3. `AppStateStore` — 状态容器

### 3.1 Store 结构

```typescript
type AppStateStore = {
  getState: () => AppState           // 读取当前状态
  setState: (f: (prev: AppState) => AppState) => void  // 更新状态
  subscribe: (listener: () => void) => () => void       // 订阅变化
}
```

### 3.2 `createStore()` 实现（`state/store.ts`）

```typescript
function createStore<T>(
  initialState: T,
  onChange?: (args: { newState: T; oldState: T }) => void
): Store<T> {
  let state = initialState
  const listeners = new Set<() => void>()
  
  return {
    getState: () => state,
    
    setState: (f) => {
      const prev = state
      const next = f(prev)
      if (next === prev) return  // 引用相等 → 跳过通知（性能优化）
      state = next
      listeners.forEach(l => l())  // 通知所有订阅者
      onChange?.({ newState: next, oldState: prev })
    },
    
    subscribe: (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)  // 返回取消订阅函数
    },
  }
}
```

### 3.3 `AppStateProvider` — React Context 集成

```tsx
// state/AppState.tsx
export function AppStateProvider({ children, initialState, onChangeAppState }) {
  // 创建 Store 实例（memoize 保证只创建一次）
  const [store] = useState(() => createStore(initialState ?? getDefaultAppState(), onChangeAppState))
  
  // 禁止嵌套 AppStateProvider
  const hasContext = useContext(HasAppStateContext)
  if (hasContext) throw new Error('AppStateProvider 不能嵌套')
  
  // 监听设置变更（settings.json / MDM 更新）
  useSettingsChange((source) => applySettingsChange(source, store.setState))
  
  return (
    <HasAppStateContext.Provider value={true}>
      <AppStoreContext.Provider value={store}>
        <MailboxProvider>
          <VoiceProvider>  {/* DCE: ant-only */}
            {children}
          </VoiceProvider>
        </MailboxProvider>
      </AppStoreContext.Provider>
    </HasAppStateContext.Provider>
  )
}
```

---

## 4. `useAppState` Hook

```typescript
// 选择器模式：只在所选字段变化时重渲染
export function useAppState<T>(selector: (state: AppState) => T): T {
  const store = useAppStore()
  return useSyncExternalStore(
    store.subscribe,
    () => selector(store.getState()),  // 当前值
    () => selector(getDefaultAppState()),  // SSR 值（不用）
  )
}

// 使用示例：
const model = useAppState(s => s.mainLoopModel)     // 只在 model 变化时重渲染
const verbose = useAppState(s => s.verbose)          // 只在 verbose 变化时重渲染

// ❌ 错误用法（每次渲染创建新对象 → 无限重渲染）：
const { model, verbose } = useAppState(s => ({ model: s.mainLoopModel, verbose: s.verbose }))
```

### 4.1 `useSetAppState` Hook

```typescript
export function useSetAppState(): (f: (prev: AppState) => AppState) => void {
  const store = useAppStore()
  return store.setState
}

// 使用示例：
const setAppState = useSetAppState()
setAppState(prev => ({
  ...prev,
  mainLoopModel: 'claude-opus-4',
}))
```

---

## 5. 默认状态（`getDefaultAppState()`）

```typescript
export function getDefaultAppState(): AppState {
  return {
    settings: getInitialSettings(),          // 从 settings.json 读取
    verbose: false,
    mainLoopModel: getInitialMainLoopModel(), // 从配置读取
    toolPermissionContext: getEmptyToolPermissionContext(),
    mcp: { clients: [], tools: [], resources: {} },
    tasks: [],
    todoList: null,
    speculationState: IDLE_SPECULATION_STATE,
    thinkingConfig: {
      enabled: shouldEnableThinkingByDefault(),
      budget: DEFAULT_THINKING_BUDGET,
    },
    plugins: { enabled: [], errors: [] },
    notifications: [],
    inProgressToolUseIDs: new Set(),
    // ... 更多默认值 ...
  }
}
```

---

## 6. 推测执行状态（`SpeculationState`）

这是 Claude Code 中一个高级优化特性，允许在用户尚未完成输入时就预测性地开始工具调用：

```typescript
type SpeculationState =
  | { status: 'idle' }
  | {
      status: 'active'
      id: string
      abort: () => void
      startTime: number
      messagesRef: { current: Message[] }      // 可变 ref（避免扩展数组）
      writtenPathsRef: { current: Set<string> } // 推测写入的文件路径（overlay）
      boundary: CompletionBoundary | null       // 推测完成的边界
      suggestionLength: number
      toolUseCount: number
      isPipelined: boolean
      // 流水线预测
      pipelinedSuggestion?: {
        text: string
        promptId: 'user_intent' | 'stated_intent'
        generationRequestId: string | null
      } | null
    }
```

**工作原理**：
1. 用户开始输入 → 触发推测执行（后台执行预测的工具调用）
2. 推测结果写入内存 overlay（不影响真实文件系统）
3. 用户确认发送 → 如果推测正确，直接采用结果（节省延迟）
4. 用户修改输入 → 推测结果丢弃

---

## 7. `bootstrap/state.ts` — 进程全局状态

这是**独立于 React 的进程级状态**，设计为 TypeScript 模块单例：

```typescript
// 关键约束：DO NOT ADD MORE STATE HERE
const state: State = {
  // 初始值在模块加载时设置
  originalCwd: realpathSync(cwd()),
  projectRoot: realpathSync(cwd()),
  cwd: realpathSync(cwd()),
  totalCostUSD: 0,
  sessionId: randomUUID() as SessionId,
  // ...
}
```

### 7.1 访问模式对比

| 状态类型 | 读取方式 | 写入方式 | 适用场景 |
|---------|---------|---------|---------|
| AppState | `useAppState(selector)` | `setAppState(f)` | React 组件、UI 状态 |
| bootstrap/state | `getCwd()`, `getSessionId()` | `setCwd()`, `setSessionId()` | 非 React 代码、服务层 |

### 7.2 关键 API

```typescript
// 工作目录
export const getCwd = () => state.cwd
export const setCwd = (p: string) => { state.cwd = expandPath(p) }

// 会话管理
export const getSessionId = () => state.sessionId
export const switchSession = (id: SessionId) => {
  state.parentSessionId = state.sessionId
  state.sessionId = id
  resetSettingsCache()
}

// 成本追踪
export const getTotalCost = () => state.totalCostUSD
export const addCost = (cost: number) => { state.totalCostUSD += cost }

// Token 预算信号（Signal 模式）
export const [getCurrentTurnTokenBudget, setCurrentTurnTokenBudget] =
  createSignal<number | undefined>(undefined)
```

---

## 8. 状态更新模式

### 8.1 函数式更新（不可变）

```typescript
// ✅ 正确：使用函数式更新，确保不可变性
setAppState(prev => ({
  ...prev,
  toolPermissionContext: {
    ...prev.toolPermissionContext,
    alwaysAllowRules: {
      ...prev.toolPermissionContext.alwaysAllowRules,
      command: [...(prev.toolPermissionContext.alwaysAllowRules.command ?? []), newRule],
    },
  },
}))

// ❌ 错误：直接修改状态（TypeScript 编译报错，DeepImmutable 保证）
appState.verbose = true  // 类型错误！
```

### 8.2 引用相等优化

```typescript
// store.ts 中的 setState 检查引用相等
if (next === prev) return  // 相同引用 → 跳过所有 re-render

// 在更新函数中利用此特性：
setAppState(prev => {
  if (prev.verbose === newVerbose) return prev  // 未变化 → 返回原对象
  return { ...prev, verbose: newVerbose }
})
```

---

## 9. 设计模式

### 9.1 Flux 单向数据流
AppState → UI 渲染 → 用户事件 → setAppState → AppState（循环）

### 9.2 useSyncExternalStore
使用 React 18 的 `useSyncExternalStore` 而非 `useState`，因为 Store 是外部状态（不属于任何组件），该 API 保证并发安全。

### 9.3 两层状态分离
- **AppState（React 层）**：与 UI 渲染强耦合，在 React 生命周期内有效
- **bootstrap/state（进程层）**：独立于 React，供服务层（工具、API 调用）使用，避免将 React 引入非 UI 代码

### 9.4 DeepImmutable 类型安全
编译时防止意外状态修改，比运行时的 `Object.freeze` 性能更好（零运行时开销）。
