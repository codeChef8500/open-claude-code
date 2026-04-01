# Agent Engine Backend — 从 Claude Code 抽取核心引擎

从 claude-code-main 源码中抽取核心 Agent 引擎逻辑，在 `backend/` 目录下用 TypeScript + Node.js 重新实现为可独立运行的 Agent Engine，支持 SDK 嵌入和 HTTP Server 双模式，兼容 Anthropic 原生 + OpenAI 兼容接口，涵盖完整的工具集（20+）、完整系统提示词迁移与 Prompt 缓存管理、记忆系统、会话持久化、命令系统、技能与插件、多Agent协调、定时任务、主动通知、伴侣系统、Undercover 卧底模式与 Auto Mode 自动权限分类器、守护进程模式。

---

## 设计原则

- **剥离 UI 层**：去掉 React/Ink、CLI、Bridge 等 UI 相关代码，只保留纯引擎逻辑
- **多 LLM 适配**：通过 Provider 适配器同时支持 Anthropic 原生 API 和 OpenAI 兼容接口（Ollama、vLLM 等）
- **双模式暴露**：既是 npm SDK 库，也可独立启动为 HTTP Server（REST + SSE 流式）
- **保留核心架构**：保留 QueryEngine → queryLoop → Tool 的分层设计和 AsyncGenerator 流式链
- **简化状态管理**：用轻量 EventEmitter + 内存 Store 替代 React AppState
- **完整功能子系统**：工具(20+)、提示词+缓存、记忆、会话、命令、技能/插件、多Agent、定时任务、守护进程、模式系统、伴侣
- **完整系统提示迁移**：6层系统提示构建（base→tools→memories→env→custom→append），Prompt Cache 最大化策略
- **模式系统**：Undercover 卧底模式（信息泄露防护）+ Auto Mode（LLM 分类器自动权限审批）+ Fast Mode（模型加速）

---

## 目录结构

```
backend/
├── package.json
├── tsconfig.json
├── README.md
├── src/
│   ├── index.ts                    # SDK 入口（导出所有公共 API）
│   ├── server.ts                   # HTTP Server 入口（Express）
│   │
│   ├── engine/                     # 核心引擎层（对应 QueryEngine.ts + query.ts）
│   │   ├── QueryEngine.ts          # 会话管理器（AsyncGenerator 流式输出）
│   │   ├── queryLoop.ts            # 核心查询循环（while-true 状态机）
│   │   ├── types.ts                # 引擎类型定义（Message、State、Config）
│   │   └── contextPipeline.ts      # 上下文压缩管道（toolResultBudget→micro→auto）
│   │
│   ├── providers/                  # LLM Provider 适配层
│   │   ├── base.ts                 # Provider 抽象接口
│   │   ├── anthropic.ts            # Anthropic Claude 原生 SDK
│   │   ├── openai-compat.ts        # OpenAI 兼容接口（适配 Ollama/vLLM/GPT 等）
│   │   └── index.ts                # Provider 注册与工厂
│   │
│   ├── tools/                      # 工具实现层（20+ 工具，对应 tools/）
│   │   ├── base.ts                 # Tool 接口定义（对应 Tool.ts）
│   │   ├── registry.ts             # 工具注册表（对应 tools.ts，排序稳定性+过滤）
│   │   ├── orchestration.ts        # 工具执行编排（并发分组+权限检查+contextModifier）
│   │   ├── bash/                   # BashTool — Shell 命令执行（最复杂工具之一）
│   │   │   ├── index.ts            # 执行逻辑（超时+后台任务+进度回调+图片检测）
│   │   │   ├── security.ts         # 命令安全检查（Shell AST 分析+危险模式检测）
│   │   │   └── permissions.ts      # Bash 权限规则（通配符匹配+命令拆分）
│   │   ├── file-read/              # FileReadTool（缓存+行号范围+编码检测+token限制+图片base64）
│   │   ├── file-edit/              # FileEditTool（精确替换+唯一性校验+并发检测+行结尾保留）
│   │   ├── file-write/             # FileWriteTool（创建/覆盖+目录自动创建）
│   │   ├── grep/                   # GrepTool（ripgrep 包装+大结果磁盘溢出）
│   │   ├── glob/                   # GlobTool（文件路径匹配）
│   │   ├── web-fetch/              # WebFetchTool（HTML→Markdown+PDF→Text+Image→Base64）
│   │   ├── web-search/             # WebSearchTool
│   │   ├── agent/                  # AgentTool — 子 Agent（前台/后台+worktree隔离）
│   │   │   ├── index.ts            # AgentTool 主逻辑（输入Schema含model/isolation/team）
│   │   │   ├── runAgent.ts         # Agent 执行核心（创建子 QueryEngine+工具继承过滤）
│   │   │   ├── taskManager.ts      # 后台任务管理（LocalAgentTask 生命周期）
│   │   │   └── memorySnapshot.ts   # Agent 记忆快照（父→子上下文继承）
│   │   ├── ask-user/               # AskUserQuestionTool（向用户提问）
│   │   ├── todo/                   # TodoWriteTool（Todo 列表写入）
│   │   ├── send-message/           # SendMessageTool（Agent 间消息传递）
│   │   ├── sleep/                  # SleepTool（守护进程模式下等待 tick）
│   │   ├── task-stop/              # TaskStopTool（停止当前任务，触发 queryLoop 退出）
│   │   ├── skill/                  # SkillTool（技能执行，contextModifier 注入行为）
│   │   ├── notebook-edit/          # NotebookEditTool（Jupyter Notebook 编辑）
│   │   ├── plan-mode/              # EnterPlanModeTool + ExitPlanModeTool（计划模式切换）
│   │   ├── cron/                   # 定时任务工具（CronCreate/Delete/List）
│   │   │   ├── create.ts           # CronCreateTool（cron表达式+prompt+recurring+durable）
│   │   │   ├── delete.ts           # CronDeleteTool（按 ID 删除）
│   │   │   ├── list.ts             # CronListTool（列出所有任务）
│   │   │   └── tasks.ts            # 任务存储（内存 session-only + 磁盘 durable 双存储）
│   │   ├── brief/                  # BriefTool/SendUserMessage（主动向用户推送消息）
│   │   │   ├── index.ts            # 发送逻辑（normal/proactive 状态+附件支持）
│   │   │   └── attachments.ts      # 附件验证与解析
│   │   ├── team-create/            # TeamCreateTool（创建 Agent 团队）
│   │   ├── team-delete/            # TeamDeleteTool（解散团队）
│   │   └── list-peers/             # ListPeersTool（列出对等 Agent）
│   │
│   ├── skills/                     # 技能系统（对应 skills/）
│   │   ├── types.ts                # SkillFrontmatter、Skill 类型
│   │   ├── loader.ts               # 技能加载（.claude/commands/*.md → PromptCommand）
│   │   ├── bundled.ts              # 内置打包技能（release-notes、summarize 等）
│   │   ├── discovery.ts            # 技能目录发现（文件编辑时触发）
│   │   ├── conditional.ts          # 条件激活（activateWhen.filePattern 匹配）
│   │   └── search.ts               # 技能搜索（关键词匹配，大量技能时用）
│   │
│   ├── plugins/                    # 插件系统（对应 utils/plugins/ + services/plugins/）
│   │   ├── types.ts                # PluginManifest、PluginHooks 类型
│   │   ├── loader.ts               # 插件加载（NPM 包 → tools/commands/hooks）
│   │   ├── registry.ts             # 插件注册表（安装/启用/禁用/卸载）
│   │   ├── hooks.ts                # Hooks 系统（PreToolUse/PostToolUse/Stop/SessionStart）
│   │   └── builtinPlugins.ts       # 内置插件技能（code-review、security-review）
│   │
│   ├── buddy/                      # 伴侣系统（对应 buddy/，后端逻辑精简版）
│   │   ├── types.ts                # Species、Rarity、CompanionBones、CompanionSoul
│   │   ├── companion.ts            # 伴侣生成（Mulberry32 PRNG + userId hash → 确定性骨骼）
│   │   ├── hatch.ts                # 孵化系统（LLM 生成 name + personality）
│   │   └── storage.ts              # 伴侣持久化（灵魂存config.json，骨骼每次重算）
│   │
│   ├── commands/                   # 命令系统（对应 commands.ts + commands/）
│   │   ├── types.ts                # Command 类型（LocalCommand / PromptCommand）
│   │   ├── registry.ts             # 命令注册表（内置+技能+插件+自定义，memoize缓存）
│   │   ├── executor.ts             # 命令执行器（slash 命令分发）
│   │   ├── builtin/                # 内置命令
│   │   │   ├── compact.ts          # /compact — 上下文压缩
│   │   │   ├── clear.ts            # /clear — 清空会话
│   │   │   ├── memory.ts           # /memory — 记忆管理（add/show/edit）
│   │   │   ├── resume.ts           # /resume — 会话恢复
│   │   │   ├── session.ts          # /session — 会话列表
│   │   │   ├── status.ts           # /status — 状态信息
│   │   │   ├── cost.ts             # /cost — 费用统计
│   │   │   ├── model.ts            # /model — 模型切换
│   │   │   ├── permissions.ts      # /permissions — 权限管理
│   │   │   ├── help.ts             # /help — 帮助信息
│   │   │   ├── plugin.ts           # /plugin — 插件管理（install/list/enable/disable）
│   │   │   ├── skills.ts           # /skills — 技能列表
│   │   │   └── hatch.ts            # /hatch — 伴侣孵化
│   │   └── custom/                 # 自定义命令加载器
│   │       └── skillDirLoader.ts   # 从 .claude/commands/*.md 加载 PromptCommand
│   │
│   ├── memory/                     # 记忆系统（对应 memdir/ + services/extractMemories/）
│   │   ├── types.ts                # Memory 类型（ExtractedMemory、ClaudeMdContent）
│   │   ├── claudeMd.ts             # CLAUDE.md 读取（多层级：全局→项目→本地）
│   │   ├── nestedMemory.ts         # 嵌套记忆（@include 展开，递归解析）
│   │   ├── sessionMemory.ts        # 会话记忆提取（LLM 自动提取关键事实）
│   │   ├── relevance.ts            # 记忆相关性排序（关键词匹配+时间衰减）
│   │   └── inject.ts               # 记忆注入系统提示（全局→项目→会话三层叠加）
│   │
│   ├── session/                    # 会话存储与历史（对应 utils/sessionStorage.ts）
│   │   ├── types.ts                # SessionMetadata、TranscriptEntry 类型
│   │   ├── storage.ts              # JSONL 会话录制（增量追加+写队列）
│   │   ├── writeQueue.ts           # 异步写队列（FIFO，并发安全）
│   │   ├── resume.ts               # 会话恢复（compact_boundary 处理）
│   │   ├── metadata.ts             # 会话元数据索引（列表/搜索/统计）
│   │   ├── history.ts              # 输入历史管理（最近1000条）
│   │   └── export.ts               # 会话导出（Markdown 格式）
│   │
│   ├── agents/                     # 多 Agent 协调系统（对应 coordinator/ + tasks/）
│   │   ├── types.ts                # AgentDefinition、AgentTask、AgentMessage
│   │   ├── coordinator.ts          # 多 Agent 协调器（权限委托、任务分配）
│   │   ├── taskManager.ts          # 任务生命周期管理（running/completed/failed）
│   │   ├── worktree.ts             # Git worktree 隔离（为子Agent创建独立工作区）
│   │   ├── messaging.ts            # Agent 间消息传递（内存队列，替代 UDS）
│   │   └── colorManager.ts         # Agent 颜色分配（UI 区分多 Agent 输出）
│   │
│   ├── daemon/                     # 守护进程模式（对应 KAIROS 助手模式）
│   │   ├── types.ts                # DaemonConfig、SessionKind、ScheduledTask
│   │   ├── supervisor.ts           # 主管进程（Supervisor，管理 Worker 生命周期）
│   │   ├── worker.ts               # 工作者进程（Worker，执行实际任务）
│   │   ├── pidRegistry.ts          # PID 文件注册表（进程发现+存活检测）
│   │   ├── cronScheduler.ts        # Cron 调度器（定时任务+文件监听）
│   │   ├── schedulerLock.ts        # 调度器文件锁（O_EXCL 原子互斥+崩溃恢复）
│   │   └── proactive.ts            # 主动模式（tick 驱动循环+自主决策）
│   │
│   ├── permissions/                # 权限系统（对应 permissions/）
│   │   ├── types.ts                # PermissionMode、PermissionResult、Rules
│   │   ├── checker.ts              # 权限检查主逻辑（deny→allow→ask 责任链）
│   │   ├── rules.ts                # 规则匹配（通配符、路径、MCP）
│   │   └── filesystem.ts           # 文件系统权限（路径安全+敏感路径检测）
│   │
│   ├── state/                      # 状态管理（轻量化，替代 React AppState）
│   │   ├── store.ts                # 内存 Store（EventEmitter 驱动，不可变更新）
│   │   ├── types.ts                # AppState 类型（精简版，含工具权限+模型+MCP+任务）
│   │   └── session.ts              # 进程级会话状态（cwd/sessionId/cost 等）
│   │
│   ├── services/                   # 服务层
│   │   ├── compact/                # 上下文压缩
│   │   │   ├── autoCompact.ts      # LLM 摘要压缩（80% token 阈值触发）
│   │   │   ├── microcompact.ts     # 本地微压缩（折叠重复读取/搜索）
│   │   │   └── contextCollapse.ts  # 上下文折叠（纯本地，零 API 成本）
│   │   ├── token-estimation.ts     # Token 估算（字符比 + API usage 精确值）
│   │   └── cost-tracker.ts         # Token 成本追踪（累计费用+预算限制）
│   │
│   ├── prompt/                     # 查询管道与提示词管理（完整迁移）
│   │   ├── system.ts               # 系统提示构建（6层拼接+缓存友好排序）
│   │   ├── templates.ts            # 核心行为规范提示词（角色定义+工具规则+安全限制）
│   │   ├── toolPrompts.ts          # 工具描述聚合（Tool.prompt()→单一文本块）
│   │   ├── envContext.ts           # 环境上下文注入（OS/Shell/时间/Git/IDE信息）
│   │   ├── userContext.ts          # 用户上下文注入（<user_context> XML 标签）
│   │   ├── processInput.ts         # 用户输入处理（slash命令检测+@文件展开+图片附件）
│   │   ├── fileMentions.ts         # @文件提及展开（路径解析+内容读取+懒加载）
│   │   ├── imageAttachments.ts     # 图片附件处理（base64 编码+尺寸限制）
│   │   ├── toolResultBudget.ts     # 工具结果预算（maxResultSizeChars 截断+磁盘溢出）
│   │   ├── tokenWarning.ts         # Token 限制检查（warning 85%/blocking 95% 分级）
│   │   ├── messageSerializer.ts    # 消息序列化（内部Message→API MessageParam）
│   │   ├── thinkingRules.ts        # Thinking 消息规则（过滤/完整性保证/redacted处理）
│   │   └── cache.ts                # Prompt Cache 管理（cache_control 注入+断点策略）
│   │
│   ├── modes/                      # 模式系统（Undercover + Auto + Fast）
│   │   ├── types.ts                # AutoModeRules、YoloClassifierResult、AutoModeState
│   │   ├── undercover.ts           # Undercover 卧底模式（仓库分类+指令注入+信息屏蔽）
│   │   ├── autoMode.ts             # Auto Mode 自动权限分类器（YoloClassifier）
│   │   ├── autoModeRules.ts        # 默认规则集（allow/soft_deny/environment）
│   │   ├── autoModeState.ts        # 分类器决策追踪（统计+调试）
│   │   ├── fastMode.ts             # Fast Mode 快速模式（模型加速+不可用原因检测）
│   │   └── sideQuery.ts            # 侧路查询（独立API调用，不影响主对话上下文）
│   │
│   ├── api/                        # HTTP API 层
│   │   ├── router.ts               # 路由定义
│   │   ├── handlers/
│   │   │   ├── chat.ts             # POST /api/chat — 对话（SSE 流式）
│   │   │   ├── sessions.ts         # 会话管理 CRUD + /resume
│   │   │   ├── tools.ts            # 工具管理（列表/启用/禁用）
│   │   │   ├── memory.ts           # 记忆管理（CRUD + 查询）
│   │   │   ├── commands.ts         # 命令执行（slash 命令 API）
│   │   │   ├── agents.ts           # Agent 管理（创建/查询/终止）
│   │   │   └── daemon.ts           # 守护进程管理（启动/停止/状态）
│   │   └── middleware/
│   │       ├── auth.ts             # API Key 认证
│   │       └── error.ts            # 错误处理
│   │
│   └── utils/                      # 工具函数
│       ├── abort.ts                # AbortController 管理
│       ├── logger.ts               # 日志（可插拔后端）
│       ├── config.ts               # 配置加载（环境变量 > JSON > 默认值）
│       ├── messages.ts             # 消息格式转换（内部⇆API⇆SDK）
│       ├── path.ts                 # 路径安全检查（遍历防护+敏感路径）
│       ├── cleanup.ts              # 进程退出清理注册表
│       └── processUtils.ts         # 进程工具（PID 存活检测、spawn）
```

---

## 分阶段实施计划

### Phase 1：项目脚手架 + 类型系统
1. 创建 `backend/` 目录，初始化 `package.json`（Node.js + TypeScript）
2. 配置 `tsconfig.json`（strict 模式、ESM）
3. 安装核心依赖
4. 定义核心类型：`Message`、`Tool`、`ToolUseContext`、`ToolResult`、`AppState`、`Command`、`Memory`、`Session`

### Phase 2：LLM Provider 适配层
5. 实现 `providers/base.ts` — Provider 抽象接口（`callModel` AsyncGenerator）
6. 实现 `providers/anthropic.ts` — Anthropic 流式调用（含重试、thinking 支持）
7. 实现 `providers/openai-compat.ts` — OpenAI 兼容接口（stream、tool_call 映射）
8. 实现 Provider 工厂 + 统一消息格式转换

### Phase 3：核心引擎 + 状态管理
9. 实现 `engine/QueryEngine.ts` — 会话管理（submitMessage AsyncGenerator）
10. 实现 `engine/queryLoop.ts` — 核心查询循环（callModel → runTools → continue/stop）
11. 实现 `state/store.ts` — 轻量状态 Store（EventEmitter，不可变更新）
12. 实现 `state/session.ts` — 进程级会话状态（cwd/sessionId/totalCost）

### Phase 4：系统提示词完整迁移 + Prompt Cache 管理
13. 实现 `prompt/templates.ts` — **完整迁移** constants/prompts.ts 核心行为规范（角色定义「你是 Claude Code...」+ 工具使用规则 + 安全限制 + 输出格式约束）
14. 实现 `prompt/toolPrompts.ts` — 工具描述聚合（遍历所有 Tool.prompt() → 按排序拼接为单一文本块，保持缓存稳定）
15. 实现 `prompt/envContext.ts` — 环境上下文构建（cwd/projectRoot/platform/shell/nodeVersion/currentTime/gitBranch/gitRemote/IDE信息）
16. 实现 `prompt/system.ts` — 系统提示 6 层构建器（buildEffectiveSystemPrompt）
    - [1] base_prompt（核心行为规范，最稳定）
    - [2] tool_descriptions（工具描述，工具变更时才变）
    - [3] memories（CLAUDE.md 内容，文件变更时才变）
    - [4] environment（平台/时间/Git，每次动态，放最后）
    - [5] customSystemPrompt（用户自定义 --system-prompt / SDK 设置）
    - [6] appendSystemPrompt（追加内容，不影响缓存断点）
17. 实现 `prompt/cache.ts` — Prompt Cache 管理
    - cache_control 注入策略（system prompt 多段 TextBlockParam，每段可设 cache_control: 'ephemeral'）
    - 缓存友好排序 CACHE_FRIENDLY_ORDER = [base_prompt, tool_descriptions, memories, environment]
    - 工具列表排序稳定性（内置工具按名称排序为前缀，MCP 工具追加在后，避免插入破坏缓存）
    - skipCacheWrite 参数支持（sideQuery 等不需要写缓存的场景）
    - cache_creation_input_tokens / cache_read_input_tokens 成本追踪
18. 实现 `prompt/userContext.ts` — 用户上下文注入（buildUserContextXml → `<user_context>` XML 标签，注入到第一条 user 消息前）
19. 实现 `prompt/processInput.ts` — 用户输入处理主函数（isSlashCommand → handleSlashCommand → expandFileMentions → processImageAttachments → createUserMessage）
20. 实现 `prompt/fileMentions.ts` — @文件提及展开（正则匹配 @path → 文件读取 → FileAttachment，懒加载）
21. 实现 `prompt/imageAttachments.ts` — 图片附件处理（base64 编码 + 尺寸限制 + 格式检测）
22. 实现 `prompt/toolResultBudget.ts` — 工具结果预算（按 maxResultSizeChars 截断 + ContentReplacementState 防重复截断）
23. 实现 `prompt/tokenWarning.ts` — Token 限制检查（TOKEN_WARNING_RATIO=0.85 / TOKEN_BLOCKING_RATIO=0.95 分级）
24. 实现 `prompt/messageSerializer.ts` — 消息序列化（内部 Message[] → API MessageParam[]，处理 user/assistant/system/tombstone 类型）
25. 实现 `prompt/thinkingRules.ts` — Thinking 消息规则（thinkingEnabled 时保留 thinking blocks；禁用时移除；确保 thinking 后必有 text/tool_use）

### Phase 5：工具系统
26. 实现 `tools/base.ts` — Tool 接口 + `buildTool()` 工厂函数
27. 实现 `tools/registry.ts` — 工具注册表（getAllBaseTools → getTools → assembleToolPool，内置+MCP 合并、排序稳定性、deny 过滤）
28. 实现 `tools/orchestration.ts` — 工具编排（并发分组、权限检查、validateInput、contextModifier、isConcurrencySafe 并行）
29. 实现 `permissions/checker.ts` — 权限检查链（deny→allow→ask + filesystem 安全 + Auto Mode 集成）

### Phase 6：核心工具实现（文件+搜索+Shell）
30. `tools/bash/` — BashTool（命令执行 + Shell AST 安全检查 + 超时 + 后台任务 + 进度回调 + 图片检测）
31. `tools/file-read/` — FileReadTool（带缓存、行号范围、编码检测、token 限制、图片→base64）
32. `tools/file-edit/` — FileEditTool（精确替换 + 唯一性校验 + 并发修改检测 + 行结尾保留 + 技能目录发现）
33. `tools/file-write/` — FileWriteTool（创建/覆盖 + 目录自动创建）
34. `tools/grep/` — GrepTool（ripgrep 包装 + 大结果磁盘溢出, maxResultSizeChars=100K）
35. `tools/glob/` — GlobTool（文件路径匹配）
36. `tools/notebook-edit/` — NotebookEditTool（Jupyter Notebook 单元格编辑）

### Phase 7：扩展工具实现（Web+Agent+流程控制）
37. `tools/web-fetch/` — WebFetchTool（HTML→Markdown、PDF→Text、Image→Base64、robots.txt 检查）
38. `tools/web-search/` — WebSearchTool
39. `tools/agent/` — AgentTool（子 Agent 前台/后台模式 + 工具继承过滤 + 记忆快照）
40. `tools/ask-user/` — AskUserQuestionTool（向用户提问，暂停 query loop）
41. `tools/todo/` — TodoWriteTool（Todo 列表写入）
42. `tools/send-message/` — SendMessageTool（Agent 间消息传递）
43. `tools/sleep/` — SleepTool（守护进程模式下等待下一个 tick）
44. `tools/task-stop/` — TaskStopTool（停止当前任务，设 taskStopped=true 触发 queryLoop 退出）
45. `tools/skill/` — SkillTool（技能执行，通过 contextModifier 注入行为到上下文）
46. `tools/plan-mode/` — EnterPlanModeTool + ExitPlanModeTool（计划模式切换，修改 permissionMode）
47. `tools/team-create/` — TeamCreateTool（创建命名 Agent 团队）
48. `tools/team-delete/` — TeamDeleteTool（解散团队并终止所有成员）
49. `tools/list-peers/` — ListPeersTool（列出 Agent 对等体）

### Phase 8：定时任务 + 主动通知工具
50. `tools/cron/tasks.ts` — 任务存储（内存 session-only + 磁盘 durable 双存储 + 90天自动过期）
51. `tools/cron/create.ts` — CronCreateTool（5字段 cron 表达式 + prompt + recurring + durable）
52. `tools/cron/delete.ts` — CronDeleteTool（按 ID 删除）
53. `tools/cron/list.ts` — CronListTool（合并内存+磁盘任务列表）
54. `tools/brief/` — BriefTool/SendUserMessage（主动推送消息 + 附件支持 + normal/proactive 状态）

### Phase 9：模式系统（Undercover + Auto Mode + Fast Mode）
55. 实现 `modes/types.ts` — AutoModeRules（allow/soft_deny/environment）、YoloClassifierResult、AutoModeState、RepoClass
56. 实现 `modes/undercover.ts` — Undercover 卧底模式
    - `isUndercover()` — 根据环境变量 + 仓库分类判断是否激活
    - `classifyRepo()` — 从 git remote URL 分类仓库（internal/external/none）
    - `getUndercoverInstructions()` — 生成注入系统提示的卧底指令（屏蔽内部信息、模型代号、归因信息）
    - 可配置 allowlist 替代 Anthropic 内部 INTERNAL_MODEL_REPOS
57. 实现 `modes/autoMode.ts` — Auto Mode 自动权限分类器（YoloClassifier）
    - `runYoloClassifier()` — 侧路查询轻量模型（Haiku），返回 allow/soft_deny/null
    - `buildYoloSystemPrompt()` — 构建分类器系统提示（默认规则 + 用户自定义规则合并）
    - `buildClassifierMessages()` — 构建分类器输入（工具调用 + 截断的对话上下文）
    - `parseClassifierResponse()` — 解析结构化输出
    - 分类器系统提示开启 prompt cache（cache_control: ephemeral）
58. 实现 `modes/autoModeRules.ts` — 默认规则集
    - allow: 读文件、运行测试、Git 只读操作等
    - soft_deny: 删除项目外文件、安装全局包、访问凭证等
    - environment: 上下文描述
    - 用户自定义规则按 section 级别替换默认规则
59. 实现 `modes/autoModeState.ts` — 分类器决策追踪（totalCalls/allowed/denied/null/errors 统计）
60. 实现 `modes/fastMode.ts` — Fast Mode 模型加速
    - `isFastModeAvailable()` — 可用性检查
    - `getFastModeUnavailableReason()` — 不可用原因检测（账户级别/网络/配置）
    - 可配置 kill switch（不依赖 GrowthBook，改为配置文件驱动）
61. 实现 `modes/sideQuery.ts` — 侧路查询（独立 API 调用，不影响主对话上下文，用于分类器/记忆提取等）

### Phase 10：技能系统
62. 实现 `skills/types.ts` — SkillFrontmatter（description/allowed-tools/activateWhen/model）
63. 实现 `skills/loader.ts` — 从 `.claude/commands/*.md` 解析 frontmatter + body → PromptCommand
64. 实现 `skills/bundled.ts` — 内置打包技能（write-release-notes、summarize-codebase 等）
65. 实现 `skills/discovery.ts` — 技能目录自动发现（文件编辑时触发 `discoverSkillDirsForPaths`）
66. 实现 `skills/conditional.ts` — 条件激活（activateWhen.filePattern 匹配时注入技能记忆）
67. 实现 `skills/search.ts` — 技能搜索（关键词匹配 + 相关性排序，大量技能时使用）

### Phase 11：插件系统
68. 实现 `plugins/types.ts` — PluginManifest（claudePlugin.contributes: tools/commands/hooks）
69. 实现 `plugins/loader.ts` — 插件加载（从 NPM 包 import → 注册 tools/commands/hooks）
70. 实现 `plugins/registry.ts` — 插件注册表（install/list/enable/disable/remove CRUD）
71. 实现 `plugins/hooks.ts` — Hooks 执行引擎（PreToolUse/PostToolUse/Stop/UserPromptSubmit/SessionStart）
72. 实现 `plugins/builtinPlugins.ts` — 内置插件技能（code-review、security-review 提示词）

### Phase 12：伴侣系统
73. 实现 `buddy/types.ts` — Species(18种)、Rarity(5级)、Hat、Eye、Stats、CompanionBones/Soul
74. 实现 `buddy/companion.ts` — 伴侣生成（Mulberry32 PRNG + userId+SALT hash → 确定性骨骼，1%闪光概率）
75. 实现 `buddy/hatch.ts` — 孵化系统（LLM 生成 name+personality，inspirationSeed 确保一致性）
76. 实现 `buddy/storage.ts` — 伴侣持久化（灵魂存 config.json，骨骼每次从 userId 重算，防伪造）

### Phase 13：记忆系统
77. 实现 `memory/claudeMd.ts` — CLAUDE.md 多层级读取（全局→项目→本地，优先级叠加）
78. 实现 `memory/nestedMemory.ts` — 嵌套记忆展开（`@include` 递归解析）
79. 实现 `memory/sessionMemory.ts` — 会话记忆提取（LLM 自动提取 preference/fact/decision/rule/correction）
80. 实现 `memory/relevance.ts` — 记忆相关性排序（关键词匹配 + 时间衰减 + 数量限制）
81. 实现 `memory/inject.ts` — 记忆注入到系统提示（并行预取 + 条件技能激活联动）

### Phase 14：会话存储与历史
82. 实现 `session/writeQueue.ts` — 异步写队列（FIFO 串行，防并发写冲突）
83. 实现 `session/storage.ts` — JSONL 会话录制（增量追加 + recordTranscript 差量更新）
84. 实现 `session/metadata.ts` — 会话元数据索引（列表/搜索/统计 + Agent 元数据记录）
85. 实现 `session/resume.ts` — 会话恢复（加载 JSONL + compact_boundary 处理）
86. 实现 `session/history.ts` — 输入历史管理（readline 风格，最近 1000 条）
87. 实现 `session/export.ts` — 会话导出为 Markdown

### Phase 15：命令系统
88. 实现 `commands/types.ts` — Command 类型体系（LocalCommand / PromptCommand，去除 JSX）
89. 实现 `commands/registry.ts` — 命令注册表（内置+技能+插件+自定义，4源合并 + memoize 缓存）
90. 实现 `commands/executor.ts` — 命令执行器（isSlashCommand → 类型分发 → call/getPrompt）
91. 实现内置命令：`/compact`、`/clear`、`/memory`、`/resume`、`/session`、`/status`、`/cost`、`/model`、`/permissions`、`/help`、`/plugin`、`/skills`、`/hatch`、`/auto-mode`
92. 实现 `commands/custom/skillDirLoader.ts` — 从 `.claude/commands/*.md` 加载自定义 PromptCommand

### Phase 16：多 Agent 协调系统
93. 实现 `agents/types.ts` — Agent 类型定义（AgentDefinition、AgentTask、AgentMessage）
94. 实现 `agents/taskManager.ts` — 任务生命周期管理（注册/进度/完成/失败/取消）
95. 实现 `agents/coordinator.ts` — 多 Agent 协调器（权限委托、分类器决策）
96. 实现 `agents/worktree.ts` — Git worktree 隔离（创建/变更检测/移除）
97. 实现 `agents/messaging.ts` — Agent 间消息传递（内存队列+轮询，替代 UDS）
98. 实现 `agents/colorManager.ts` — Agent 颜色分配（调色板循环）

### Phase 17：守护进程模式（KAIROS 精简版）
99. 实现 `daemon/types.ts` — DaemonConfig、SessionKind、ScheduledTask 类型
100. 实现 `daemon/pidRegistry.ts` — PID 文件注册表（注册/发现/存活检测/清理）
101. 实现 `daemon/schedulerLock.ts` — 文件基调度器锁（O_EXCL 原子互斥 + 崩溃自动恢复）
102. 实现 `daemon/cronScheduler.ts` — Cron 调度器（定时任务轮询 + chokidar 文件监听 + 抖动配置）
103. 实现 `daemon/supervisor.ts` — 主管进程（spawn Worker + 健康检查 + 自动重启）
104. 实现 `daemon/worker.ts` — 工作者进程（接收任务 + QueryEngine 执行 + 心跳）
105. 实现 `daemon/proactive.ts` — 主动模式（tick 驱动 + 自主决策 + Sleep 退出）

### Phase 18：上下文压缩 + Token 管理
106. 实现 `services/compact/autoCompact.ts` — LLM 摘要压缩（80% 阈值 + preserved segment）
107. 实现 `services/compact/microcompact.ts` — 本地微压缩（重复读取/搜索折叠）
108. 实现 `services/compact/contextCollapse.ts` — 上下文折叠（纯本地，零 API 成本）
109. 实现 `services/token-estimation.ts` — Token 估算（字符比 + 精确 API usage）
110. 实现 `services/cost-tracker.ts` — 成本追踪（累计费用 + 预算限制 + 告警）
111. 实现 `engine/contextPipeline.ts` — 压缩流水线串联（toolResultBudget → micro → collapse → auto）

### Phase 19：SDK 导出 + HTTP Server
112. 实现 `index.ts` — SDK 公共 API 导出（Engine/Tool/Command/Memory/Session/Skill/Plugin/Daemon/Buddy）
113. 实现 `api/router.ts` — HTTP 路由定义
114. 实现 `api/handlers/chat.ts` — SSE 流式对话端点
115. 实现 `api/handlers/sessions.ts` — 会话管理（列表/恢复/导出）
116. 实现 `api/handlers/memory.ts` — 记忆管理（CRUD + 查询）
117. 实现 `api/handlers/commands.ts` — 命令执行（slash 命令 API 化）
118. 实现 `api/handlers/agents.ts` — Agent 管理（创建/查询/消息/终止）
119. 实现 `api/handlers/daemon.ts` — 守护进程管理（启动/停止/状态/调度任务）
120. 实现 `api/handlers/skills.ts` — 技能管理（列表/搜索/条件激活）
121. 实现 `api/handlers/plugins.ts` — 插件管理（安装/列表/启用/禁用）
122. 实现 `api/handlers/buddy.ts` — 伴侣管理（查看/孵化/状态）
123. 实现 `api/handlers/modes.ts` — 模式管理（Auto Mode 规则 CRUD + 状态查询 + 卧底模式状态）
124. 实现 `server.ts` — Express Server 启动入口
125. 实现中间件（auth、error handling）

### Phase 20：配置 + 文档
126. 实现 `utils/config.ts` — 配置文件加载（环境变量 > JSON > 默认值）
127. 实现 `utils/cleanup.ts` — 进程退出清理（PID 文件、锁文件、临时 worktree）
128. 编写 `README.md` 使用文档（SDK 用法 + HTTP API 文档 + 工具列表 + 配置说明）

---

## 核心架构映射（claude-code → backend）

| claude-code 原模块 | backend 新模块 | 变更说明 |
|---|---|---|
| `QueryEngine.ts` | `engine/QueryEngine.ts` | 去除 React 依赖，保留 AsyncGenerator |
| `query.ts` | `engine/queryLoop.ts` | 简化状态机，去除 feature flags |
| `Tool.ts` | `tools/base.ts` | 去除 UI 相关方法（setToolJSX） |
| `tools.ts` | `tools/registry.ts` | 去除 Bun feature()、简化过滤 |
| `services/api/claude.ts` | `providers/anthropic.ts` | 保留流式 + 重试 |
| — | `providers/openai-compat.ts` | **新增** OpenAI 兼容层 |
| `services/tools/toolOrchestration.ts` | `tools/orchestration.ts` | 保留并发分组 |
| `bootstrap/state.ts` + `state/AppStateStore.ts` | `state/store.ts` + `state/session.ts` | EventEmitter 替代 React |
| `hooks/useCanUseTool.tsx` | `permissions/checker.ts` | 纯函数，去除 React hook |
| `services/compact/` | `services/compact/` | 保留三层压缩算法 |
| `utils/processUserInput/` | `prompt/processInput.ts` | 保留 slash 命令 + @文件展开 |
| `constants/prompts.ts` | `prompt/templates.ts` | **完整迁移**核心行为规范（角色+工具+安全） |
| `utils/systemPrompt.ts` | `prompt/system.ts` + `prompt/cache.ts` | 6层构建+Prompt Cache管理 |
| `utils/messages.ts` | `prompt/messageSerializer.ts` + `prompt/thinkingRules.ts` | 消息序列化+Thinking规则 |
| `utils/undercover.ts` + `utils/commitAttribution.ts` | `modes/undercover.ts` | 保留仓库分类+指令注入，可配置allowlist |
| `utils/permissions/yoloClassifier.ts` | `modes/autoMode.ts` + `modes/autoModeRules.ts` | 保留LLM分类器，去除GrowthBook |
| `utils/permissions/autoModeState.ts` | `modes/autoModeState.ts` | 保留决策追踪统计 |
| `utils/fastMode.ts` | `modes/fastMode.ts` | 保留可用性检查，kill switch改配置文件 |
| `utils/sideQuery.ts` | `modes/sideQuery.ts` | 保留独立API调用，用于分类器/记忆提取 |
| `utils/claudeMd.ts` + `utils/nestedMemory.ts` | `memory/claudeMd.ts` + `memory/nestedMemory.ts` | 完整保留多层级记忆 |
| `services/extractMemories/` + `services/SessionMemory/` | `memory/sessionMemory.ts` + `memory/relevance.ts` | 保留 LLM 提取 + 相关性排序 |
| `utils/sessionStorage.ts` + `utils/transcript.ts` | `session/storage.ts` + `session/writeQueue.ts` | 保留 JSONL + 写队列 |
| `commands.ts` + `commands/` | `commands/registry.ts` + `commands/builtin/` | 去除 JSX 命令，保留 Local + Prompt |
| `coordinator/` + `tasks/` | `agents/coordinator.ts` + `agents/taskManager.ts` | 保留核心协调逻辑 |
| `tools/AgentTool/` | `tools/agent/` + `agents/` | 拆分为工具层 + 协调层 |
| `skills/loadSkillsDir.ts` + `skills/bundledSkills.ts` | `skills/loader.ts` + `skills/bundled.ts` | 保留 Markdown→PromptCommand + 条件激活 |
| `utils/plugins/` + `services/plugins/` | `plugins/loader.ts` + `plugins/registry.ts` | 保留 NPM 包 → tools/commands/hooks |
| `commands/hooks/` + `query/stopHooks.ts` | `plugins/hooks.ts` | 统一 Hooks 执行引擎（6种 Hook 类型） |
| `buddy/companion.ts` + `buddy/types.ts` | `buddy/companion.ts` + `buddy/types.ts` | 保留生成算法+持久化，去除 ASCII sprite 渲染 |
| `tools/ScheduleCronTool/` + `utils/cronTasks.ts` | `tools/cron/` | 保留 Create/Delete/List + 双存储 |
| `tools/BriefTool/` | `tools/brief/` | 保留发送逻辑+附件，去除 Bridge 上传+UI |
| KAIROS (`assistant/` + `daemon/`) | `daemon/` | 精简版守护进程，去除 CCR/GrowthBook |
| `utils/cronScheduler.ts` + `cronTasksLock.ts` | `daemon/cronScheduler.ts` + `daemon/schedulerLock.ts` | 完整保留文件锁+调度器+抖动 |
| — | `api/` + `server.ts` | **新增** HTTP Server 层 |

---

## 关键设计决策

1. **AsyncGenerator 链式保留**：Provider → queryLoop → QueryEngine → SDK/HTTP 保持流式传递
2. **Provider 适配器模式**：统一 `LLMProvider` 接口，`callModel()` 返回 `AsyncGenerator<StreamEvent>`
3. **工具消息格式标准化**：内部统一使用 Anthropic 的 `tool_use`/`tool_result` 格式，OpenAI 的 `function_call` 在 Provider 层双向转换
4. **权限系统可插拔**：默认提供 `auto-allow` / `ask-callback` / `rule-based` 三种模式
5. **HTTP SSE 流式**：`POST /api/chat` 返回 SSE 流，每个 event 对应一个 `StreamEvent`
6. **配置优先级**：环境变量 > 配置文件 > 代码默认值
7. **记忆三层叠加**：全局 CLAUDE.md → 项目 CLAUDE.md → 会话记忆，系统提示中按缓存友好顺序排列
8. **命令去 JSX 化**：Command 系统保留 `LocalCommand`（纯函数）和 `PromptCommand`（生成提示词），去除 `LocalJSXCommand`（React UI），结果通过 JSON 返回
9. **Agent 消息内存队列**：多 Agent 间通信用进程内内存队列替代 UDS，SDK 模式无需文件系统；HTTP Server 模式可选持久化
10. **守护进程跨平台**：PID 注册表 + 文件锁替代 Unix daemon()，Windows/macOS/Linux 统一工作
11. **会话持久化可选**：SDK 模式默认内存会话，可配置开启 JSONL 持久化；HTTP Server 模式默认持久化
12. **技能渐进式扩展**：Markdown 文件（技能）→ NPM 包（插件）→ 内置代码（命令），三级扩展机制，约定优于配置
13. **Hooks 生命周期**：6 种 Hook 类型（PreToolUse/PostToolUse/Stop/UserPromptSubmit/SessionStart/Notification），插件和用户脚本均可注册
14. **伴侣骨骼灵魂分离**：骨骼（物种/稀有度）从 `hash(userId+SALT)` 确定性重算永不持久化；灵魂（名字/性格）由 LLM 孵化后存 config.json，防伪造
15. **定时任务双存储**：session-only（内存，零副作用）+ durable（磁盘 `.claude/scheduled_tasks.json`，90天过期），kill switch 可强制降级
16. **BriefTool 结果精简**：tool_result 只返回 "Message delivered"，不将完整消息回传 LLM，避免上下文膨胀
17. **工具完整性**：保留 claude-code 全部 20+ 工具，包括 SkillTool（contextModifier 注入）、NotebookEditTool、PlanMode 工具、Cron 三件套、BriefTool、Team 工具
18. **系统提示完整迁移**：6层系统提示全部迁移（base→tools→memories→env→custom→append），保留缓存友好排序；templates.ts 完整复刻角色定义、工具规则、安全限制
19. **Prompt Cache 最大化**：固定部分（base、tools）在前，动态部分（env、time）在后；工具列表按名称排序确保前缀字节一致；cache_control: ephemeral 标注；追踪 cache_creation/read tokens
20. **消息序列化完整保留**：内部 Message（7种类型：user/assistant/toolResult/system/progress/attachment/tombstone）→ API MessageParam 完整映射；Thinking blocks 完整性规则保留
21. **Undercover 卧底模式**：仓库分类（internal/external/none）+ 系统提示指令注入（屏蔽模型代号、归因信息）；安全默认=开启；allowlist 可配置替代硬编码
22. **Auto Mode 分类器**：LLM 侧路查询（Haiku 轻量模型，maxOutputTokens=100）自动审批工具调用；规则三类（allow/soft_deny/environment）；用户可按 section 替换默认规则
23. **sideQuery 复用**：侧路查询作为通用基础设施，用于 Auto Mode 分类器、记忆提取、伴侣孵化等场景，不影响主对话上下文
24. **GrowthBook 去除**：所有 feature gates / kill switches 改为配置文件驱动（`settings.json` + 环境变量），去除对 Anthropic 内部 GrowthBook/Statsig 的依赖

---

## 完整工具清单（20+ 工具）

| 工具名 | 模块 | 说明 |
|--------|------|------|
| BashTool | `tools/bash/` | Shell 命令执行（AST 安全检查+超时+进度） |
| FileReadTool | `tools/file-read/` | 文件读取（缓存+行号+图片base64） |
| FileEditTool | `tools/file-edit/` | 精确替换（唯一性校验+并发检测） |
| FileWriteTool | `tools/file-write/` | 文件创建/覆盖 |
| GrepTool | `tools/grep/` | ripgrep 搜索（大结果磁盘溢出） |
| GlobTool | `tools/glob/` | 文件路径匹配 |
| NotebookEditTool | `tools/notebook-edit/` | Jupyter Notebook 编辑 |
| WebFetchTool | `tools/web-fetch/` | URL→Markdown/Text/Base64 |
| WebSearchTool | `tools/web-search/` | 网络搜索 |
| AgentTool | `tools/agent/` | 子 Agent（前台/后台+worktree隔离） |
| AskUserQuestionTool | `tools/ask-user/` | 向用户提问 |
| TodoWriteTool | `tools/todo/` | Todo 列表管理 |
| SendMessageTool | `tools/send-message/` | Agent 间消息 |
| SleepTool | `tools/sleep/` | 守护进程 tick 等待 |
| TaskStopTool | `tools/task-stop/` | 停止当前任务 |
| SkillTool | `tools/skill/` | 技能执行（contextModifier） |
| EnterPlanModeTool | `tools/plan-mode/` | 进入计划模式 |
| ExitPlanModeTool | `tools/plan-mode/` | 退出计划模式 |
| CronCreateTool | `tools/cron/` | 创建定时任务 |
| CronDeleteTool | `tools/cron/` | 删除定时任务 |
| CronListTool | `tools/cron/` | 列出定时任务 |
| BriefTool (SendUserMessage) | `tools/brief/` | 主动推送消息+附件 |
| TeamCreateTool | `tools/team-create/` | 创建 Agent 团队 |
| TeamDeleteTool | `tools/team-delete/` | 解散团队 |
| ListPeersTool | `tools/list-peers/` | 列出对等 Agent |

---

## 核心依赖

```json
{
  "@anthropic-ai/sdk": "^0.39.0",
  "openai": "^4.73.0",
  "zod": "^3.24.0",
  "express": "^4.21.0",
  "uuid": "^10.0.0",
  "eventsource-parser": "^3.0.0",
  "glob": "^11.0.0",
  "turndown": "^7.2.0",
  "chokidar": "^4.0.0",
  "gray-matter": "^4.0.3",
  "minimatch": "^10.0.0",
  "cron-parser": "^4.9.0",
  "typescript": "^5.7.0",
  "tsx": "^4.19.0"
}
```
