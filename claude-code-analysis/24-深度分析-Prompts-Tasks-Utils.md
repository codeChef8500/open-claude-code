# Claude Code 深度分析报告：Prompts · Tasks · Utils

> 基于 `claude-code-main/src/` 实际源码的逐文件深度分析，覆盖全部 Prompt 来源、Tasks 子系统、Utils 目录（564 文件 / 23 子目录）。

---

## 一、全部 Prompt 来源（38 个文件）

### 1.1 核心系统 Prompt

| 文件 | 作用 |
|------|------|
| `constants/prompts.ts` (915行) | **主入口**：组装完整 system prompt，含静态/动态分区、`SYSTEM_PROMPT_DYNAMIC_BOUNDARY` 缓存边界 |
| `constants/systemPromptSections.ts` | Section 管理：`systemPromptSection()` 带 memoize 缓存、`DANGEROUS_uncachedSystemPromptSection()` 每轮重算 |
| `constants/cyberRiskInstruction.ts` | 安全指令 `CYBER_RISK_INSTRUCTION`，不可修改，需 safeguards 团队审批 |
| `constants/outputStyles.ts` | 输出风格（Explanatory / Learning / 自定义），通过 memoize 和 plugin 加载 |
| `buddy/prompt.ts` | Companion 系统的 prompt 注入（角色介绍文本） |

### 1.2 工具级 Prompt（29 个 prompt.ts）

每个工具目录下的 `prompt.ts` 定义该工具的 LLM 指令：

**文件操作工具：**
- `tools/FileReadTool/prompt.ts` — 文件读取指令
- `tools/FileEditTool/prompt.ts`（含 `constants.ts`） — 文件编辑指令
- `tools/FileWriteTool/prompt.ts` — 文件写入指令
- `tools/NotebookEditTool/prompt.ts` — Jupyter 编辑指令

**Shell 工具：**
- `tools/BashTool/prompt.ts` — Bash 执行指令（含 Git 操作、undercover 模式、background task）
- `tools/PowerShellTool/prompt.ts` — PowerShell 指令

**搜索工具：**
- `tools/GrepTool/prompt.ts` — 正则搜索指令
- `tools/GlobTool/prompt.ts` — 文件名搜索指令
- `tools/ToolSearchTool/prompt.ts` — 工具搜索指令（deferred tool 发现）

**网络工具：**
- `tools/WebFetchTool/prompt.ts` — Web 抓取指令
- `tools/WebSearchTool/prompt.ts` — Web 搜索指令

**MCP 工具：**
- `tools/MCPTool/prompt.ts` — MCP 工具调用指令
- `tools/ListMcpResourcesTool/prompt.ts` — MCP 资源列表指令
- `tools/ReadMcpResourceTool/prompt.ts` — MCP 资源读取指令

**任务管理工具：**
- `tools/TaskCreateTool/prompt.ts` — 创建后台任务
- `tools/TaskGetTool/prompt.ts` — 获取任务状态
- `tools/TaskListTool/prompt.ts` — 列出所有任务
- `tools/TaskStopTool/prompt.ts` — 停止任务
- `tools/TaskUpdateTool/prompt.ts` — 更新任务

**Agent / Swarm 工具：**
- `tools/SendMessageTool/prompt.ts` — 发送消息给 agent
- `tools/TeamCreateTool/prompt.ts` — 创建团队
- `tools/TeamDeleteTool/prompt.ts` — 删除团队
- `tools/RemoteTriggerTool/prompt.ts` — 远程触发

**其他工具：**
- `tools/SkillTool/prompt.ts` — Skill 系统指令
- `tools/ScheduleCronTool/prompt.ts` — Cron 调度指令
- `tools/SleepTool/prompt.ts` — 等待指令
- `tools/TodoWriteTool/prompt.ts` — TODO 写入指令
- `tools/LSPTool/prompt.ts` — LSP 工具指令

### 1.3 Prompt 相关 Utilities

| 文件 | 作用 |
|------|------|
| `utils/promptCategory.ts` | Prompt 分类逻辑 |
| `utils/promptEditor.ts` | Prompt 编辑器（用户自定义 prompt 修改） |
| `utils/promptShellExecution.ts` | Shell 执行前的 prompt 处理 |
| `utils/claudeInChrome/prompt.ts` | Chrome 扩展 prompt |
| `context/promptOverlayContext.tsx` | React context：prompt overlay UI 状态 |

### 1.4 Prompt 架构设计要点

```
┌─────────────────────────────────────────────────────┐
│                  System Prompt                       │
├─────────────────────────────────────────────────────┤
│  [STATIC - cached across turns]                     │
│  ├─ Identity & Rules                                │
│  ├─ Tool Instructions (per-tool prompt.ts)          │
│  ├─ Agent Subsystem Prompt                          │
│  ├─ Cyber Risk Instruction                          │
│  └─ Output Style                                    │
│  ── SYSTEM_PROMPT_DYNAMIC_BOUNDARY ──               │
│  [DYNAMIC - recomputed each turn]                   │
│  ├─ Session Guidance                                │
│  ├─ Memory (CLAUDE.md)                              │
│  ├─ Environment Info                                │
│  ├─ Language Preference                             │
│  ├─ MCP Instructions                                │
│  ├─ Token Budget                                    │
│  └─ Brief Instructions                              │
└─────────────────────────────────────────────────────┘
```

**关键机制：**
- **Memoize 缓存**：`systemPromptSection()` 对静态段做 memoize，跨 turn 复用
- **Volatile 段**：`DANGEROUS_uncachedSystemPromptSection()` 标记 `cacheBreak: true`，每轮重算
- **Feature Gate**：通过 `bun:bundle` 的 `feature()` 做编译时 DCE（Dead Code Elimination）
- **条件编译**：`process.env.USER_TYPE === 'ant'` 区分内部/外部版本

---

## 二、Tasks 子系统深度分析

### 2.1 目录结构

```
src/tasks/
├── types.ts                    — TaskState 联合类型定义
├── pillLabel.ts                — Footer pill 标签生成
├── stopTask.ts                 — 统一的任务停止逻辑
├── LocalMainSessionTask.ts     — 主会话后台化
├── LocalShellTask/
│   ├── LocalShellTask.tsx      — Shell 后台任务（67KB）
│   ├── guards.ts               — 类型守卫 + BashTaskKind
│   └── killShellTasks.ts       — Kill 逻辑
├── LocalAgentTask/
│   └── LocalAgentTask.tsx      — 本地 Agent 任务（84KB）
├── RemoteAgentTask/
│   └── RemoteAgentTask.tsx     — 远程 Agent 任务（127KB，最大）
├── InProcessTeammateTask/
│   └── types.ts                — Teammate 任务类型定义
├── DreamTask/
│   └── DreamTask.ts            — Dream（记忆整合）任务
└── [LocalWorkflowTask, MonitorMcpTask] — 工作流 / MCP 监控
```

### 2.2 七种任务类型

```typescript
type TaskState =
  | LocalShellTaskState      // 后台 Shell 命令
  | LocalAgentTaskState      // 本地 Sub-agent
  | RemoteAgentTaskState     // 远程云端 Agent
  | InProcessTeammateTaskState // 进程内 Teammate（Swarm）
  | LocalWorkflowTaskState   // 本地 Workflow
  | MonitorMcpTaskState      // MCP 监控
  | DreamTaskState           // 记忆整合 Agent
```

### 2.3 核心架构

#### TaskStateBase（基础状态）
所有任务共享的字段：
- `id`, `type`, `description`, `status` (running/pending/completed/failed/killed)
- `startTime`, `endTime`, `notified`, `outputOffset`, `toolUseId`

#### Task 生命周期
```
register → running → [notification] → completed/failed/killed → evict
                ↕
          backgrounded (isBackgrounded = true)
```

#### 关键设计模式

**1. Stall Watchdog（LocalShellTask）**
```
- 每 5s 检查输出文件大小
- 45s 无增长 + 输出末行匹配 PROMPT_PATTERNS → 通知用户
- 匹配模式：(y/n), Press any key, Continue?, Overwrite? 等
```

**2. Progress Tracking（LocalAgentTask）**
```typescript
type ProgressTracker = {
  toolUseCount: number
  latestInputTokens: number      // 累计（API 输入是累计的）
  cumulativeOutputTokens: number  // 累加（输出是每次的）
  recentActivities: ToolActivity[] // 最近 5 个工具活动
}
```

**3. Background Main Session（LocalMainSessionTask）**
- Ctrl+B 两次触发主会话后台化
- 复用 `LocalAgentTaskState` + `agentType: 'main-session'`
- 隔离 transcript 记录
- foreground 时恢复消息历史

**4. Dream Task（记忆整合）**
```
4 阶段：orient → gather → consolidate → prune
不做阶段检测，仅通过 Edit/Write 工具调用检测 'starting' → 'updating'
最多保留 30 个最近 turn
```

**5. InProcessTeammate**
```
- 50 条消息 UI 上限（TEAMMATE_MESSAGES_UI_CAP）
- 独立 permissionMode
- plan mode approval flow
- idle/shutdown lifecycle
- onIdleCallbacks 用于 leader 无轮询等待
```

**6. Notification System**
- XML 格式通知：`<task_notification>` 包含 `<task_id>`, `<status>`, `<summary>`, `<output_file>`
- 通过 `enqueuePendingNotification()` 放入消息队列
- SDK 事件通过 `enqueueSdkEvent()` 发射

### 2.4 utils/task/ 子目录

| 文件 | 功能 |
|------|------|
| `framework.ts` (309行) | 核心框架：`registerTask`, `updateTaskState`, `evictTerminalTask`, `pollTasks`, `generateTaskAttachments` |
| `diskOutput.ts` (452行) | 磁盘输出：`DiskTaskOutput` 类（写队列+flush）、`O_NOFOLLOW` 安全、5GB 上限 |
| `TaskOutput.ts` (391行) | 输出管理：file mode (bash) / pipe mode (hooks)、共享 poller、CircularBuffer |
| `sdkProgress.ts` | SDK 进度事件发射 |
| `outputFormatting.ts` | 输出格式化 |

**DiskTaskOutput 设计亮点：**
- 单 drain loop 处理写队列，每 chunk 写完即 GC
- 避免 Promise chain 闭包导致的内存保留
- `O_NOFOLLOW` 防止符号链接攻击（安全）
- Session ID 隔离防止并发会话冲突

---

## 三、Utils 目录深度分析（564 文件 / 23 子目录）

### 3.1 子目录总览

| 子目录 | 文件数 | 总大小 | 核心功能 |
|--------|--------|--------|----------|
| `bash/` | 15+ | ~400KB | Bash 解析器、AST、命令分析、heredoc、shell 引用 |
| `hooks/` | 17 | ~120KB | Hook 系统：事件、配置、执行器（bash/HTTP/prompt/agent）|
| `permissions/` | 24 | ~320KB | 权限系统：规则、分类器、YOLO、文件系统安全 |
| `model/` | 16 | ~90KB | 模型管理：选择、能力、别名、验证、定价 |
| `plugins/` | 45 | ~700KB | 插件系统：加载、验证、marketplace、依赖解析 |
| `swarm/` | 14+ | ~180KB | Swarm 系统：runner、backend、权限同步、团队管理 |
| `settings/` | 17+ | ~140KB | 设置系统：类型、验证、变更检测、MDM |
| `task/` | 5 | ~40KB | 任务框架（已在上节详述）|
| `shell/` | 10 | ~110KB | Shell 提供者、只读命令验证、输出限制 |
| `git/` | 3 | ~33KB | Git 文件系统操作（无 subprocess）|
| `memory/` | 2 | ~0.6KB | 内存类型定义（User/Project/Local/Managed/AutoMem/TeamMem）|
| `sandbox/` | 2 | ~37KB | 沙箱适配器（@anthropic-ai/sandbox-runtime 桥接）|

### 3.2 核心 Utils 文件详解

#### 3.2.1 消息系统

**`messages.ts` (5513行)** — 消息工厂 & 操作
- 30+ 消息类型的创建函数（UserMessage, AssistantMessage, SystemMessage 等）
- 消息规范化（`normalizeMessages`）、提取（`extractTextContent`）
- Tombstone 机制、compact boundary 标记
- 系统提醒注入（`wrapInSystemReminder`）

**`messageQueueManager.ts` (548行)** — 统一命令队列
```
commandQueue: QueuedCommand[]
优先级：'now' > 'next' > 'later'
- useSyncExternalStore 接口（React）
- 直接读取接口（非 React：print.ts streaming loop）
- Signal 订阅模式
```

#### 3.2.2 会话存储

**`sessionStorage.ts` (5106行)** — 会话持久化
- JSONL 格式的 transcript 存储
- 读取优化：`readHeadAndTail` 跳过 pre-compact 数据
- Session 切换、恢复、导出
- Agent transcript 隔离路径
- 并发会话名称管理

#### 3.2.3 Token 管理

**`tokens.ts` (262行)** — Token 计算
- `tokenCountWithEstimation()` — **权威函数**：last API usage + 新消息估算
- 处理并行 tool call 的 split assistant records（同 message.id 去重）
- `finalContextTokensFromLastResponse()` — 任务预算的 remaining 计算

**`tokenBudget.ts` (74行)** — 用户 Token 预算解析
- 支持 `+500k`, `use 2M tokens` 等格式
- 到达预算百分比时发送 continuation message

**`context.ts` (222行)** — 上下文窗口管理
- 默认 200K tokens
- 1M context 支持（`[1m]` 后缀）
- `CAPPED_DEFAULT_MAX_TOKENS = 8000`（slot 优化）
- `ESCALATED_MAX_TOKENS = 64000`（重试时升级）

#### 3.2.4 配置系统

**`config.ts` (1818行)** — 全局配置
- `ProjectConfig`: 工具白名单、MCP 服务器、上次使用统计
- `HistoryEntry`: 输入历史 + 粘贴内容
- re-entrancy guard 防止配置读取递归
- 文件监听 + lockfile 并发安全

**`claudemd.ts` (1480行)** — Memory 文件加载
```
加载顺序（优先级递增）：
1. Managed memory (/etc/claude-code/CLAUDE.md)
2. User memory (~/.claude/CLAUDE.md)
3. Project memory (CLAUDE.md, .claude/CLAUDE.md, .claude/rules/*.md)
4. Local memory (CLAUDE.local.md)

@include 指令：
- @path, @./relative, @~/home, @/absolute
- 仅在叶文本节点生效（不在代码块内）
- 循环引用保护
- 最大 40,000 字符
```

#### 3.2.5 权限系统

**`permissions/permissions.ts` (1487行)** — 权限核心
- 多来源规则合并（settings, cliArg, command, session）
- 权限决策：allow / deny / ask
- 分类器集成（bash classifier, transcript classifier）
- Denial tracking + 自动降级到 prompting

**`permissions/yoloClassifier.ts` (1496行)** — Auto Mode 分类器
- 基于 LLM 的 side-query 判断工具调用是否安全
- 3 个可配置规则段：allow / soft_deny / environment
- 内部版 vs 外部版权限模板
- 分类器失败时 fail-closed（30 分钟刷新）

**`permissions/filesystem.ts` (1778行)** — 文件系统安全
- DANGEROUS_FILES 列表（.gitconfig, .bashrc, .zshrc 等）
- DANGEROUS_DIRECTORIES 列表（.git, .vscode, .claude 等）
- Case-insensitive 路径规范化（macOS/Windows）
- Skill 目录权限隔离

**`permissions/permissionSetup.ts` (1533行)** — 权限初始化
- 危险 Bash 权限检测（`python:*`, `node:*` 等前缀规则）
- Auto mode 状态转换管理
- Plan mode 权限处理
- 工具预设解析

#### 3.2.6 Hook 系统

**`hooks.ts` (5023行)** — Hook 主入口
```
Hook 事件类型（20+）：
SessionStart, SessionEnd, Setup, Stop, StopFailure,
PreToolUse, PostToolUse, PostToolUseFailure,
Notification, SubagentStart, SubagentStop,
PermissionDenied, PreCompact, PostCompact,
TaskCreated, TaskCompleted, ConfigChange,
CwdChanged, FileChanged, InstructionsLoaded,
UserPromptSubmit, PermissionRequest,
Elicitation, ElicitationResult, TeammateIdle
```

**`hooks/` 子目录：**
| 文件 | 功能 |
|------|------|
| `hookEvents.ts` | 事件系统：started/progress/response 事件，pending 队列 |
| `hooksConfigManager.ts` (18KB) | Hook 配置管理（热重载） |
| `hooksConfigSnapshot.ts` | 配置快照（一致性读取） |
| `hooksSettings.ts` | Hook 设置验证和显示 |
| `execAgentHook.ts` (13KB) | Agent hook 执行器 |
| `execHttpHook.ts` (9KB) | HTTP hook 执行器 |
| `execPromptHook.ts` (7KB) | Prompt hook 执行器 |
| `sessionHooks.ts` (13KB) | Session lifecycle hooks |
| `AsyncHookRegistry.ts` (9KB) | 异步 hook 注册 + 清理 |
| `ssrfGuard.ts` (9KB) | SSRF 防护（HTTP hook） |
| `skillImprovement.ts` (9KB) | Skill 自动改进 hook |

#### 3.2.7 Bash 解析系统

**`bash/` 子目录（~400KB）— 完整的 Bash 解析器**

| 文件 | 大小 | 功能 |
|------|------|------|
| `bashParser.ts` | 135KB | 完整的 Bash 语法解析器 |
| `ast.ts` | 115KB | AST 节点定义和操作 |
| `commands.ts` | 52KB | 命令分析（危险命令检测、输出重定向提取）|
| `heredoc.ts` | 32KB | Heredoc 解析 |
| `ShellSnapshot.ts` | 22KB | Shell 状态快照 |
| `shellQuote.ts` | 11KB | Shell 引用处理 |
| `ParsedCommand.ts` | 10KB | 解析后的命令表示 |
| `treeSitterAnalysis.ts` | 18KB | Tree-sitter AST 分析 |

#### 3.2.8 Shell 系统

**`shell/readOnlyCommandValidation.ts` (70KB)** — 只读命令白名单
- 完整的 Git 子命令安全标志映射
- `gh` CLI 命令白名单
- 跨平台外部命令配置
- UNC 路径凭据泄露检测（`containsVulnerableUncPath`）

#### 3.2.9 模型管理

**`model/model.ts` (22KB)** — 模型选择
```
优先级：
1. /model 命令（会话内覆盖）
2. --model 标志（启动时）
3. ANTHROPIC_MODEL 环境变量
4. Settings 配置
```

**`model/modelOptions.ts` (19KB)** — 模型选项
**`model/modelCapabilities.ts`** — 模型能力查询
**`model/modelAllowlist.ts`** — 模型白名单
**`model/configs.ts`** — 模型配置（上下文窗口、价格）

**`thinking.ts` (163行)** — Extended Thinking
- ultrathink 关键字检测 + rainbow 彩色渲染
- 模型 thinking 支持检测（按 provider 区分）
- adaptive thinking 支持（4-6+ 模型）

#### 3.2.10 成本计算

**`modelCost.ts` (232行)** — 定价表
```
COST_TIER_3_15:   Sonnet ($3/$15 per Mtok)
COST_TIER_15_75:  Opus 4/4.1 ($15/$75 per Mtok)
COST_TIER_5_25:   Opus 4.5 ($5/$25 per Mtok)
COST_TIER_30_150: Opus 4.6 Fast ($30/$150 per Mtok)
COST_HAIKU_35:    Haiku 3.5 ($0.80/$4 per Mtok)
```

#### 3.2.11 Side Query

**`sideQuery.ts` (223行)** — 轻量级 API 包装器
- 用于主会话外的 LLM 调用（权限分类器、session 搜索等）
- 自动处理：fingerprint、attribution header、CLI prefix、betas、model 规范化
- 支持 thinking、structured outputs、stop sequences

#### 3.2.12 API 工具构建

**`api.ts` (719行)** — API 请求构建
- `BetaToolWithExtras`: 扩展工具定义（strict mode, defer_loading, cache_control）
- `SystemPromptBlock`: system prompt 分块（含 cache scope）
- Swarm 字段过滤（非 swarm 模式下移除相关 schema 字段）

#### 3.2.13 工具结果持久化

**`toolResultStorage.ts` (1041行)** — 大结果落盘
- 超过阈值（默认 50K 字符）的结果持久化到磁盘
- GrowthBook 可覆盖每工具阈值
- `Infinity` 阈值 = 硬禁用（避免循环读取）
- 100K token / 200K chars per message 聚合上限

#### 3.2.14 Swarm 系统

**`swarm/inProcessRunner.ts` (55KB)** — 进程内 Teammate 运行器
- AsyncLocalStorage 上下文隔离
- Progress tracking + AppState 更新
- Plan mode approval flow
- 清理机制

**`swarm/permissionSync.ts` (27KB)** — 权限同步
**`swarm/teamHelpers.ts` (22KB)** — 团队管理辅助
**`swarm/backends/` (9 文件)** — 多后端支持：
  - `InProcessBackend.ts` — 进程内
  - `TmuxBackend.ts` — Tmux 终端复用
  - `ITermBackend.ts` — iTerm2 原生
  - `PaneBackendExecutor.ts` — 面板后端

#### 3.2.15 Plugin 系统（45 文件，~700KB）

最大的子目录，完整的插件生态：

| 关键文件 | 大小 | 功能 |
|----------|------|------|
| `pluginLoader.ts` | 114KB | 插件加载器（核心） |
| `marketplaceManager.ts` | 96KB | Marketplace 管理 |
| `schemas.ts` | 61KB | 插件 Schema 定义 |
| `installedPluginsManager.ts` | 43KB | 已安装插件管理 |
| `loadPluginCommands.ts` | 31KB | 插件命令加载 |
| `validatePlugin.ts` | 29KB | 插件验证 |
| `pluginInstallationHelpers.ts` | 21KB | 安装辅助 |
| `mcpPluginIntegration.ts` | 21KB | MCP 插件集成 |
| `mcpbHandler.ts` | 32KB | MCPB 协议处理 |

#### 3.2.16 Settings 系统

**`settings/settings.ts` (33KB)** — 设置读写核心
**`settings/types.ts` (44KB)** — Zod schema 定义（最大的类型文件）
**`settings/changeDetector.ts` (17KB)** — 设置变更检测（热重载）
**`settings/validation.ts` (8KB)** — 设置验证

#### 3.2.17 沙箱系统

**`sandbox/sandbox-adapter.ts` (37KB)** — 沙箱桥接
- 包装 `@anthropic-ai/sandbox-runtime`
- 设置转换（Claude CLI → sandbox config）
- 文件系统/网络限制配置
- 违规事件处理

#### 3.2.18 Git 文件系统

**`git/gitFilesystem.ts` (23KB)** — 无 subprocess 的 Git 操作
- HEAD 解析（ref / raw SHA）
- packed-refs 解析
- Worktree/submodule `.git` 文件处理
- GitHeadWatcher：`fs.watchFile` 缓存 branch/SHA

#### 3.2.19 其他重要 Utils

| 文件 | 功能 |
|------|------|
| `collapseReadSearch.ts` | 读/搜操作折叠（UI 优化） |
| `worktree.ts` | Git worktree 管理（slug 验证、创建、hook） |
| `toolSearch.ts` | Deferred 工具发现（MCP 工具动态加载） |
| `stats.ts` | 使用统计（日活、streak、session stats） |
| `analyzeContext.ts` | 上下文分析（/context 命令） |
| `betas.ts` | API beta 头管理 |
| `fileStateCache.ts` | 文件状态缓存 |

---

## 四、关键设计模式总结

### 4.1 编译时优化
```typescript
// Feature gate（编译时 DCE）
import { feature } from 'bun:bundle'
if (feature('TRANSCRIPT_CLASSIFIER')) { ... }

// 环境区分
if (process.env.USER_TYPE === 'ant') { ... }
```

### 4.2 运行时特性控制
```typescript
// GrowthBook feature flags
getFeatureValue_CACHED_MAY_BE_STALE('flag_name', defaultValue)
checkStatsigFeatureGate_CACHED_MAY_BE_STALE('gate_name')
```

### 4.3 缓存策略
```
- Memoize: lodash-es/memoize（system prompt sections, config）
- LRU Cache: 自定义（文件状态、工具 schema）
- Snapshot: frozen 数组（message queue, useSyncExternalStore）
- Session-level: module-level 变量（task output dir）
```

### 4.4 并发安全
```
- Mutex: lockfile.js（配置文件）
- Atomic update: setAppState(prev => ...)（React 状态）
- Signal: createSignal()（跨系统事件）
- AbortController: 任务取消 + 资源清理
```

### 4.5 安全模式
```
- O_NOFOLLOW: 防符号链接（task output）
- Path validation: 遍历检测、UNC 路径、大小写规范化
- DANGEROUS_FILES/DIRECTORIES: 敏感文件保护
- SSRF Guard: HTTP hook 网络请求防护
- Sandbox: 文件系统 + 网络隔离
```

---

## 五、对 agent-engine Go 复刻的关键发现

### 5.1 需要新增/增强的模块

| 优先级 | 模块 | Claude Code 复杂度 | agent-engine 现状 |
|--------|------|---------------------|-------------------|
| P0 | Task Framework | 高（7 种任务 + 生命周期） | 已有基础 taskmanager |
| P0 | Prompt System | 高（38 个来源 + 缓存） | 有 SystemPromptBuilder 接口 |
| P0 | Permission System | 极高（24 文件 320KB） | 有基础权限检查 |
| P1 | Hook System | 高（17 文件 + 5 种执行器） | 有基础 hooks |
| P1 | Message Queue | 中（优先级队列） | 需新增 |
| P1 | Token Management | 中（估算 + 预算） | 有基础 token/cost |
| P2 | Bash Parser | 极高（135KB 解析器） | 可简化（Go exec） |
| P2 | Plugin System | 极高（45 文件 700KB） | 有基础 registry |
| P2 | Swarm System | 高（14 文件 + 多后端） | 有基础 agent/worktree |
| P3 | Sandbox | 高（外部包适配） | 需评估 |

### 5.2 关键差异与决策点

1. **Bash 解析**: Claude Code 有完整的 Bash AST 解析器（135KB）用于只读命令检测。Go 侧可用 `mvdan.cc/sh` 替代。

2. **Plugin 系统**: 最复杂的子系统（700KB+）。Go 复刻可先实现核心 registry + loader，marketplace 延后。

3. **Hook 执行器**: 5 种类型（bash/HTTP/prompt/agent/session）。Go 可统一为 process exec + HTTP client。

4. **Task 系统**: 需要实现完整的 7 种任务类型。`DiskTaskOutput` 的写队列模式在 Go 中可用 channel + goroutine。

5. **Permission YOLO**: 需要 side-query 基础设施（已有 `RunYoloClassifier`），但需增加：denial tracking、fail-closed、分类器缓存。

6. **Swarm Backend**: 多终端后端（tmux/iTerm2/InProcess）在 Go TUI 中可简化为 goroutine + channel。

### 5.3 建议更新执行计划

基于此分析，建议在现有 20 Phase 计划中补充：

- **Phase 4 增强**: Prompt 系统需增加 memoize 缓存和 dynamic boundary 分区
- **Phase 5 增强**: Permission 系统需增加 denial tracking、fail-closed 机制
- **Phase 6 增强**: Hook 系统需增加 HTTP/Agent 执行器和 SSRF 防护
- **Phase 16 增强**: Task 系统需实现 DiskTaskOutput 模式和 stall watchdog
- **新 Phase**: Message Queue 优先级系统（now/next/later）
- **新 Phase**: Tool Result Storage（大结果持久化）
- **新 Phase**: Context Analysis（/context 命令支持）

---

## 六、文件规模统计

| 区域 | 文件数 | 总行数（估） | 总大小 |
|------|--------|-------------|--------|
| Prompt Sources | 38 | ~3,000 | ~120KB |
| Tasks | 12 | ~4,500 | ~380KB |
| Utils (core) | ~50 | ~25,000 | ~1.2MB |
| Utils/bash | 15 | ~8,000 | ~400KB |
| Utils/permissions | 24 | ~7,000 | ~320KB |
| Utils/plugins | 45 | ~15,000 | ~700KB |
| Utils/hooks | 17 | ~3,000 | ~120KB |
| Utils/swarm | 14 | ~5,000 | ~180KB |
| Utils/settings | 17 | ~3,500 | ~140KB |
| Utils/model | 16 | ~2,000 | ~90KB |
| Utils/shell | 10 | ~3,000 | ~110KB |
| Utils/其他 | ~350+ | ~15,000 | ~600KB |
| **总计** | **~564** | **~94,000** | **~4.3MB** |
