# open-claude-code
open-claude-code
# Claude Code 源码深度解读 — 00 项目总览与技术架构

> 本文件是 Claude Code 泄露源码（2026-03-31）深度解读系列的第一篇，覆盖项目背景、整体技术架构、分层设计及各模块关系。

---

## 1. 项目背景

**Claude Code** 是 Anthropic 官方推出的 CLI 工具，允许开发者通过终端直接与 Claude 模型交互，执行软件工程任务：编辑文件、运行命令、搜索代码库、管理 Git 工作流等。

2026 年 3 月 31 日，该项目的完整 TypeScript 源码因 npm registry 中存放了 `.map` 文件而意外泄露，源码压缩包可从 Anthropic 的 R2 存储桶直接下载。

**基本规模**：
- 约 1,900 个文件，512,000+ 行代码
- 纯 TypeScript 实现（strict 模式）
- 运行时：**Bun**（不是 Node.js）
- 终端 UI：**React + Ink**

---

## 2. 技术栈详解

| 层次 | 技术 | 版本/说明 |
|------|------|----------|
| 运行时 | [Bun](https://bun.sh) | 替代 Node.js，速度更快，内置 TypeScript 支持 |
| 语言 | TypeScript (strict) | 全量类型覆盖 |
| 终端 UI | React + [Ink](https://github.com/vadimdemedes/ink) | 在终端中渲染 React 组件树 |
| CLI 解析 | [Commander.js](https://github.com/tj/commander.js) + extra-typings | 类型安全的命令行参数解析 |
| Schema 验证 | [Zod v4](https://zod.dev) | 运行时类型校验 |
| 代码搜索 | [ripgrep](https://github.com/BurntSushi/ripgrep) | 通过 GrepTool 调用 |
| 协议 | [MCP SDK](https://modelcontextprotocol.io)、LSP | 工具调用协议扩展 |
| API | [Anthropic SDK](https://docs.anthropic.com) | `@anthropic-ai/sdk` |
| 遥测 | OpenTelemetry + gRPC | 懒加载（~400KB + ~700KB） |
| Feature Flags | GrowthBook | A/B 测试与功能门控 |
| 认证 | OAuth 2.0、JWT、macOS Keychain | 多平台安全存储 |
| 构建打包 | Bun bundle + `bun:bundle` Feature Flags | 编译时死代码消除 |

---

## 3. 整体架构分层

```
┌─────────────────────────────────────────────────────────────────┐
│                          用户层 (CLI/IDE)                        │
│  entrypoints/cli.tsx  ──→  main.tsx (Commander.js)              │
└──────────────────────────────┬──────────────────────────────────┘
                               │ 启动 & 路由
┌──────────────────────────────▼──────────────────────────────────┐
│                       UI 渲染层 (React + Ink)                    │
│  screens/REPL.tsx  ──→  components/  ──→  hooks/                │
│  (主交互界面)          (140+ 组件)      (80+ hooks)              │
└──────────────────────────────┬──────────────────────────────────┘
                               │ 用户输入/事件
┌──────────────────────────────▼──────────────────────────────────┐
│                    核心引擎层 (查询 & 工具)                       │
│  QueryEngine.ts  ──→  query.ts  ──→  Tool.ts                    │
│  (会话管理)          (LLM调用循环)   (工具接口定义)               │
└──────────────────────────────┬──────────────────────────────────┘
                               │ 工具调用
┌──────────────────────────────▼──────────────────────────────────┐
│                       工具实现层 (tools/)                         │
│  BashTool  FileEditTool  FileReadTool  AgentTool  ...           │
│  GrepTool  WebSearchTool  MCPTool  SkillTool  (40+)             │
└──────────────────────────────┬──────────────────────────────────┘
                               │ 服务调用
┌──────────────────────────────▼──────────────────────────────────┐
│                        服务与集成层 (services/)                   │
│  api/  mcp/  oauth/  lsp/  compact/  analytics/                 │
└──────────────────────────────┬──────────────────────────────────┘
                               │ 基础设施
┌──────────────────────────────▼──────────────────────────────────┐
│                   基础设施层 (utils/ + bootstrap/)               │
│  bootstrap/state.ts  utils/config.ts  utils/sessionStorage.ts   │
│  (全局单例状态)       (配置管理)        (会话持久化)               │
└─────────────────────────────────────────────────────────────────┘
```

---

## 4. 目录结构总览

```
src/
├── main.tsx                # 🏠 CLI 主入口（Commander.js + 并行预加载）
├── QueryEngine.ts          # ⚙️  查询引擎（会话 + LLM 调用生命周期）
├── query.ts                # 🔄 核心查询循环（流式 tool-call 循环）
├── Tool.ts                 # 📦 工具类型接口定义
├── tools.ts                # 📋 工具注册表 + 组装逻辑
├── commands.ts             # 📋 命令注册表
├── context.ts              # 🌍 系统/用户上下文收集
├── cost-tracker.ts         # 💰 Token 成本追踪
├── history.ts              # 📜 输入历史管理
├── setup.ts                # 🔧 初始化设置
│
├── entrypoints/            # 🚀 启动入口（cli.tsx, init.ts, mcp.ts, sdk/）
├── bootstrap/              # 🌱 全局引导状态（单例，无 React 依赖）
├── state/                  # 🗃️  React 状态管理（AppState, Store）
│
├── screens/                # 🖥️  全屏 UI（REPL.tsx 主界面, Doctor, Resume）
├── components/             # 🧩 UI 组件（140+）
├── hooks/                  # 🪝 React hooks（80+）
├── context/                # 🎯 React Context providers
├── ink/                    # 🎨 Ink 渲染器包装
│
├── tools/                  # 🔨 工具实现（40+ 工具目录）
├── commands/               # 💬 Slash 命令实现（80+）
│
├── services/               # 🌐 外部服务集成
│   ├── api/                #   Anthropic API 客户端
│   ├── mcp/                #   MCP 协议客户端
│   ├── oauth/              #   OAuth 2.0 流程
│   ├── lsp/                #   LSP 语言服务器协议
│   ├── compact/            #   上下文压缩
│   └── analytics/          #   GrowthBook 特征标志
│
├── bridge/                 # 🌉 IDE 集成桥接（VS Code、JetBrains）
├── coordinator/            # 🎯 多 Agent 协调器
├── skills/                 # 🎓 技能系统
├── plugins/                # 🔌 插件系统
│
├── memdir/                 # 🧠 记忆目录（持久化记忆）
├── utils/                  # 🛠️  工具函数库（极大，300+ 文件）
├── types/                  # 📐 TypeScript 类型定义
├── schemas/                # ✅ Zod 配置 Schema
├── migrations/             # 🔄 配置迁移脚本
│
├── query/                  # 🔍 查询管道辅助（stopHooks, tokenBudget 等）
├── vim/                    # ⌨️  Vim 模式支持
├── voice/                  # 🎤 语音输入（ant-only）
├── remote/                 # 🌐 远程会话管理
├── server/                 # 🖧  服务器模式（Direct Connect）
├── tasks/                  # 📋 任务管理系统
├── keybindings/            # ⌨️  快捷键配置
├── outputStyles/           # 🎨 输出样式
├── buddy/                  # 🐱 彩蛋精灵
└── native-ts/              # ⚡ 原生 TypeScript 工具
```

---

## 5. 核心设计决策

### 5.1 Bun 作为运行时

选择 Bun 而非 Node.js 的核心原因：
- 内置 TypeScript 执行（无需 tsc 编译步骤）
- `bun:bundle` 特性支持**编译时死代码消除（DCE）**
- 原生支持 `feature()` 调用，编译时去除非活跃特性代码
- 更快的启动速度（对 CLI 工具至关重要）

```typescript
// bun:bundle feature flag 示例 — 编译时移除非活跃分支
const voiceCommand = feature('VOICE_MODE')
  ? require('./commands/voice/index.js').default
  : null
```

### 5.2 React + Ink 终端 UI

将 React 组件树渲染到终端（不是浏览器 DOM），带来了：
- **组件化复用**：UI 组件与普通 React 组件无异
- **虚拟列表**（`VirtualMessageList.tsx`）：处理长对话时的性能优化
- **流式更新**：消息流式输出通过 React 状态更新即时渲染
- 代价：Ink 渲染有约 100ms 的 TTFR（首次渲染时间）

### 5.3 并行预加载优化启动速度

`main.tsx` 顶部以**副作用**形式并行启动三个耗时操作：

```typescript
// 1. 性能打点（main.tsx 导入前）
profileCheckpoint('main_tsx_entry');

// 2. MDM 子进程（macOS plutil / Windows reg query）
startMdmRawRead();  // 并行启动

// 3. macOS Keychain 预读（OAuth token + API key）
startKeychainPrefetch();  // 并行启动
// 以上三步在剩余 ~135ms 的模块加载期间并行执行
```

### 5.4 懒加载重型依赖

OpenTelemetry（~400KB）和 gRPC（~700KB）通过动态 `import()` 延迟加载：

```typescript
// 只在实际需要遥测时才加载
const telemetry = await import('./utils/telemetry/index.js')
```

### 5.5 Feature Flags 驱动的功能门控

多种特性通过 `feature()` 控制，编译时 DCE 消除：

| Flag | 功能 |
|------|------|
| `VOICE_MODE` | 语音输入 |
| `BRIDGE_MODE` | IDE 桥接（VS Code/JetBrains） |
| `COORDINATOR_MODE` | 多 Agent 协调 |
| `KAIROS` | Assistant 模式 |
| `PROACTIVE` | 主动模式 |
| `DAEMON` | 守护进程模式 |
| `AGENT_TRIGGERS` | Agent 触发器（Cron） |
| `MONITOR_TOOL` | 监控工具 |
| `HISTORY_SNIP` | 历史压缩 |
| `TOKEN_BUDGET` | Token 预算管理 |
| `TRANSCRIPT_CLASSIFIER` | 自动模式分类器 |

---

## 6. 端到端请求流程（鸟瞰）

```
用户在终端输入提示词
        │
        ▼
screens/REPL.tsx (主界面)
  └─ useTextInput / useVimInput (输入捕获)
  └─ hooks/useCanUseTool (权限上下文)
        │
        ▼
utils/processUserInput/processUserInput.ts
  └─ 解析 slash 命令（/commit, /compact 等）
  └─ 处理附件（图片、文件引用）
  └─ 构建消息对象
        │
        ▼
query.ts → queryLoop()
  └─ 1. 预处理：microcompact / snip / autocompact / contextCollapse
  └─ 2. 构建 systemPrompt + userContext
  └─ 3. 调用 services/api/claude.ts → Anthropic API（流式）
        │
        ▼
流式响应处理（AsyncGenerator）
  └─ text delta → 渲染到 UI
  └─ thinking block → 渲染折叠块
  └─ tool_use block → 触发工具执行
        │ (有 tool_use)
        ▼
services/tools/toolOrchestration.ts → runTools()
  └─ canUseTool() → 权限检查（可能弹出用户确认）
  └─ tool.call(input, context) → 实际执行
  └─ 返回 tool_result → 追加到消息列表
        │
        ▼
继续 queryLoop（多轮工具调用直到 stop_reason = end_turn）
        │
        ▼
最终响应渲染到终端 UI
```

---

## 7. 关键类型定义概览

### `QueryEngineConfig`（QueryEngine.ts:130）

```typescript
type QueryEngineConfig = {
  cwd: string               // 工作目录
  tools: Tools              // 可用工具列表
  commands: Command[]       // 可用命令列表
  mcpClients: MCPServerConnection[]
  agents: AgentDefinition[]
  canUseTool: CanUseToolFn  // 权限检查函数
  getAppState: () => AppState
  setAppState: (f: (prev: AppState) => AppState) => void
  initialMessages?: Message[]
  thinkingConfig?: ThinkingConfig
  maxTurns?: number
  maxBudgetUsd?: number
  // ...更多配置
}
```

### `AppState`（state/AppStateStore.ts:89）

全局 UI 状态（深度不可变类型 `DeepImmutable`），包含：
- 权限上下文（`toolPermissionContext`）
- 模型设置（`mainLoopModel`）
- 任务状态（`tasks`）
- MCP 连接（`mcp`）
- 思考模式配置（`thinkingConfig`）
- Bridge 连接状态（`replBridgeConnected` 等）
- 推测执行状态（`speculationState`）

---

## 8. 模块间依赖关系

```
main.tsx
  ├── bootstrap/state.ts      [全局单例，无 React 依赖]
  ├── entrypoints/init.ts     [初始化 + 遥测]
  ├── state/AppStateStore.ts  [React 状态]
  ├── QueryEngine.ts
  │     ├── query.ts
  │     │     ├── services/api/claude.ts   [Anthropic API]
  │     │     ├── services/tools/toolOrchestration.ts
  │     │     └── services/compact/       [上下文压缩]
  │     ├── Tool.ts              [工具接口]
  │     └── tools.ts             [工具注册]
  ├── commands.ts               [命令注册]
  ├── services/mcp/client.ts    [MCP 客户端]
  └── screens/REPL.tsx          [主 UI]
        ├── components/         [UI 组件]
        ├── hooks/              [React hooks]
        └── context/            [Context providers]
```

---

## 9. 编译时与运行时的双重特性门控

Claude Code 实现了两种层次的特性门控：

**编译时（build-time）**：通过 `bun:bundle` 的 `feature()` 调用：
```typescript
// 此代码在 feature('VOICE_MODE') = false 的构建中完全被删除
const voiceCommand = feature('VOICE_MODE')
  ? require('./commands/voice/index.js').default
  : null
```

**运行时（runtime）**：通过 GrowthBook 动态特性标志：
```typescript
// 运行时检查，可热更新
const isEnabled = getFeatureValue_CACHED_MAY_BE_STALE('my-feature', false)
```

这种双层机制使得：
- 内部构建（`ant`）包含所有实验特性
- 外部发布构建剥离不稳定代码，减小包体积

---

## 10. 系列解读索引

| 文档编号 | 文档名称 | 核心内容 |
|---------|---------|---------|
| 00 | **本文** | 项目总览与技术架构 |
| 01 | 入口与启动流程 | main.tsx, entrypoints/, bootstrap/ |
| 02 | QueryEngine 核心引擎 | QueryEngine.ts, query.ts |
| 03 | Tool 类型系统与注册表 | Tool.ts, tools.ts |
| 04 | 工具实现深度解读 | tools/ 目录 40+ 工具 |
| 05 | Command 命令系统 | commands.ts, commands/ |
| 06 | Permission 权限系统 | hooks/toolPermission/, types/permissions.ts |
| 07 | 状态管理与 AppState | state/, bootstrap/state.ts |
| 08 | React+Ink UI 组件层 | screens/, components/ |
| 09 | Hooks 层深度解读 | hooks/ |
| 10 | Bridge IDE 集成系统 | bridge/ |
| 11 | 多 Agent 协调与 Swarm | coordinator/, AgentTool, TeamCreateTool |
| 12 | 服务层详解 | services/ |
| 13 | 会话存储与历史管理 | utils/sessionStorage.ts |
| 14 | Memory 记忆系统 | memdir/ |
| 15 | 技能与插件系统 | skills/, plugins/ |
| 16 | 查询管道与上下文构建 | query/, utils/queryContext.ts |
| 17 | 工具函数库精要 | utils/ 精选 |
| 18 | 整体数据流与架构总结 | 端到端流程图 |
