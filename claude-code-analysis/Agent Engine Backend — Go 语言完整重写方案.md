# Agent Engine Backend — Go 语言完整重写方案

从 claude-code-main 源码抽取核心 Agent 引擎逻辑，用 **Go 1.23+** 重新实现为可独立运行的 Agent Engine，支持 Go SDK 库嵌入和 HTTP Server 双模式，兼容 Anthropic 原生 + OpenAI 兼容接口，涵盖全部 20+ 工具、完整系统提示词迁移与 Prompt Cache 管理、记忆系统、会话持久化、命令系统、技能与插件、多 Agent 协调、定时任务、Undercover 卧底模式与 Auto Mode、守护进程模式。

---

## 一、可行性评估

### Go vs TypeScript 核心架构映射

| TypeScript 模式 | Go 等价方案 | 可行性 | 说明 |
|---|---|---|---|
| AsyncGenerator 流式链 | `<-chan StreamEvent` + goroutine | ✅ **天然优势** | Go channel 是一等公民，比 AsyncGenerator 更高效 |
| `async/await` 异步 | goroutine + channel | ✅ **天然优势** | CSP 并发模型，无回调地狱 |
| EventEmitter 状态通知 | channel / `sync.Cond` / callback | ✅ 可行 | 用 channel 或 observer pattern 替代 |
| interface + 工厂模式 | Go interface + 构造函数 | ✅ 完美映射 | Go interface 隐式实现，更灵活 |
| Zod schema 校验 | struct tags + validator | ✅ 可行 | `go-playground/validator` 或手写校验 |
| NPM 插件系统 | Go plugin / HashiCorp go-plugin / GRPC | ⚠️ 需适配 | Go 原生 plugin 仅 Linux/macOS；跨平台需 GRPC subprocess |
| Markdown frontmatter 解析 | `gohugoio/hugo` 的 frontmatter 或 `adrg/frontmatter` | ✅ 可行 | Go 生态有成熟库 |
| Shell AST 解析 | `mvdan.cc/sh/v3` | ✅ **更优** | Go 原生 Shell 解析器，比 JS 方案更成熟 |
| ripgrep 调用 | `exec.Command("rg", ...)` 或嵌入 grep 逻辑 | ✅ 可行 | 同样调用外部进程 |
| HTML→Markdown | `JohannesKaufmann/html-to-markdown` | ✅ 可行 | 成熟库 |
| SSE 流式 HTTP | `net/http` + `Flush()` | ✅ **天然优势** | Go HTTP 原生支持 SSE |
| JSONL 文件操作 | `encoding/json` + `bufio.Scanner` | ✅ 简单 | Go 标准库足够 |
| 文件监听（chokidar） | `fsnotify/fsnotify` | ✅ 可行 | 跨平台文件监听 |
| Cron 调度 | `robfig/cron/v3` | ✅ 成熟 | 比 JS cron-parser 更功能完备 |
| PID 文件锁 | `os.OpenFile` + `O_EXCL` | ✅ **更优** | Go 标准库原生支持 |
| 进程管理（spawn） | `os/exec` | ✅ **更优** | Go 进程管理比 Node.js child_process 更可控 |
| Git 操作 | `go-git/go-git` 或 `exec.Command("git", ...)` | ✅ 可行 | 纯 Go 实现或调用外部命令 |
| Base64 图片处理 | `encoding/base64` + `image` | ✅ 标准库 | 无需第三方 |
| JSON Schema 验证 | `santhosh-tekuri/jsonschema` | ✅ 可行 | 成熟库 |
| Token 估算 | `tiktoken-go/tokenizer` 或字符比 | ✅ 可行 | 有 Go 绑定 |
| 通配符匹配（minimatch） | `filepath.Match` + `doublestar` | ✅ 可行 | `bmatcuk/doublestar` 支持 `**` |

### Go 优势总结

1. **并发模型天然契合**：goroutine + channel 完美替代 AsyncGenerator 流式链，更轻量（goroutine 2KB vs Node.js Promise chain）
2. **单二进制部署**：编译为单个可执行文件，无需 Node.js 运行时、无 node_modules
3. **内存效率**：Go GC 更可控，长运行守护进程场景（KAIROS）内存表现优于 Node.js
4. **进程管理更强**：`os/exec`、信号处理、PID 管理等系统编程能力远超 Node.js
5. **类型安全**：编译时类型检查，无需 Zod 运行时校验
6. **HTTP 性能**：`net/http` + SSE 原生支持，无需 Express 等框架

### Go 挑战与应对

| 挑战 | 应对方案 |
|---|---|
| 无 Anthropic 官方 Go SDK | 使用社区 `anthropics/anthropic-sdk-go`（官方已发布） 或直接 HTTP 调用 |
| 插件系统跨平台 | 用 HashiCorp `go-plugin`（GRPC subprocess），或简化为 JSON-RPC subprocess |
| JSON 动态结构处理 | `encoding/json.RawMessage` + 类型断言 |
| Markdown 解析不如 JS 丰富 | `yuin/goldmark` + `adrg/frontmatter` 组合 |
| 无 npm 包管理 | Go modules，插件改为独立二进制或共享库 |

### 结论：**完全可行，多数模块 Go 实现更优**

---

## 二、设计原则

- **剥离 UI 层**：纯引擎逻辑，无 CLI/TUI 代码
- **多 LLM 适配**：Provider interface 同时支持 Anthropic 原生 + OpenAI 兼容接口（Ollama/vLLM/GPT）
- **双模式暴露**：Go SDK 库（`import "agent-engine/engine"`）+ HTTP Server（REST + SSE）
- **CSP 并发模型**：goroutine + channel 替代 AsyncGenerator，流式事件通过 `<-chan StreamEvent` 传递
- **接口驱动设计**：所有核心模块通过 Go interface 定义契约，支持 mock 测试和替换
- **完整系统提示迁移**：6层系统提示构建 + Prompt Cache 最大化策略
- **模式系统**：Undercover 卧底模式 + Auto Mode（LLM 分类器）+ Fast Mode
- **单二进制部署**：`go build` 产出单个可执行文件，零外部依赖
- **完整功能子系统**：工具(20+)、提示词+缓存、记忆、会话、命令、技能/插件、多Agent、定时任务、守护进程、模式系统、伴侣

---

## 三、技术栈

### 核心依赖

| 类别 | 库 | 版本 | 用途 |
|---|---|---|---|
| **HTTP 框架** | `net/http` + `chi` | v5 | 路由 + 中间件（轻量级，比 Gin 更 idiomatic） |
| **Anthropic API** | `anthropics/anthropic-sdk-go` | latest | Anthropic Claude 原生 SDK（官方 Go 版） |
| **OpenAI 兼容** | `sashabaranov/go-openai` | v1 | OpenAI/Ollama/vLLM 兼容接口 |
| **JSON Schema** | `santhosh-tekuri/jsonschema/v6` | v6 | 工具输入 Schema 校验 |
| **结构体校验** | `go-playground/validator/v10` | v10 | 配置和输入校验 |
| **Shell 解析** | `mvdan.cc/sh/v3` | v3 | Shell AST 分析（BashTool 安全检查） |
| **HTML→Markdown** | `JohannesKaufmann/html-to-markdown/v2` | v2 | WebFetchTool |
| **Markdown 解析** | `yuin/goldmark` | v1 | 技能 Markdown 解析 |
| **Frontmatter** | `adrg/frontmatter` | v0 | 技能 YAML frontmatter 解析 |
| **Glob 匹配** | `bmatcuk/doublestar/v4` | v4 | 文件路径匹配（支持 `**`） |
| **Cron 调度** | `robfig/cron/v3` | v3 | 定时任务调度器 |
| **文件监听** | `fsnotify/fsnotify` | v1 | 文件变更监听（守护进程） |
| **UUID** | `google/uuid` | v1 | 会话/任务 ID 生成 |
| **日志** | `log/slog`（标准库） | - | 结构化日志（Go 1.21+） |
| **配置** | `spf13/viper` | v1 | 配置加载（环境变量 > JSON > 默认值） |
| **Git 操作** | `go-git/go-git/v5` | v5 | 纯 Go Git 实现（仓库分类、worktree） |
| **Token 估算** | `tiktoken-go/tokenizer` | latest | Token 计数（Claude/GPT 分词器） |
| **测试** | `stretchr/testify` | v1 | 断言 + mock |
| **插件系统** | `hashicorp/go-plugin` | v1 | GRPC subprocess 插件（跨平台） |
| **PDF 解析** | `ledongthuc/pdf` | v0 | WebFetchTool PDF→Text |
| **PRNG** | 标准库 `math/rand` | - | Mulberry32 PRNG（伴侣系统） |

### go.mod 示例

```go
module github.com/wall-ai/agent-engine

go 1.23

require (
    github.com/go-chi/chi/v5          v5.1.0
    github.com/anthropics/anthropic-sdk-go  v0.2.0-beta.3
    github.com/sashabaranov/go-openai  v1.36.0
    github.com/santhosh-tekuri/jsonschema/v6  v6.0.1
    github.com/go-playground/validator/v10  v10.22.0
    mvdan.cc/sh/v3                     v3.9.0
    github.com/JohannesKaufmann/html-to-markdown/v2  v2.2.1
    github.com/yuin/goldmark           v1.7.8
    github.com/adrg/frontmatter        v0.2.0
    github.com/bmatcuk/doublestar/v4   v4.7.1
    github.com/robfig/cron/v3          v3.0.1
    github.com/fsnotify/fsnotify       v1.8.0
    github.com/google/uuid             v1.6.0
    github.com/spf13/viper             v1.19.0
    github.com/go-git/go-git/v5        v5.12.0
    github.com/tiktoken-go/tokenizer   v0.2.1
    github.com/stretchr/testify        v1.9.0
    github.com/hashicorp/go-plugin     v1.6.2
    github.com/ledongthuc/pdf          v0.0.0-20240201131950-da5b75280b06
)
```

---

## 四、目录结构

```
agent-engine/
├── go.mod
├── go.sum
├── Makefile                          # build/test/lint/run 快捷命令
├── README.md
├── cmd/
│   └── agent-engine/
│       └── main.go                   # HTTP Server 入口（可选 daemon 模式）
│
├── pkg/                              # 公共 SDK API（对外导出）
│   └── sdk/
│       ├── engine.go                 # SDK 入口（Engine 构造 + Options）
│       ├── types.go                  # 公共类型导出
│       └── options.go                # 配置选项（functional options 模式）
│
├── internal/                         # 内部实现（不对外导出）
│   │
│   ├── engine/                       # 核心引擎层
│   │   ├── engine.go                 # QueryEngine（会话管理，SubmitMessage → <-chan StreamEvent）
│   │   ├── queryloop.go              # 核心查询循环（for-select 状态机）
│   │   ├── types.go                  # Message、State、Config、StreamEvent
│   │   └── context_pipeline.go       # 上下文压缩管道
│   │
│   ├── provider/                     # LLM Provider 适配层
│   │   ├── provider.go               # Provider interface 定义
│   │   ├── anthropic.go              # Anthropic Claude（官方 SDK + 流式）
│   │   ├── openai_compat.go          # OpenAI 兼容接口（Ollama/vLLM/GPT）
│   │   ├── factory.go                # Provider 工厂（按配置创建）
│   │   └── message_convert.go        # 消息格式双向转换
│   │
│   ├── tool/                         # 工具系统
│   │   ├── tool.go                   # Tool interface 定义
│   │   ├── registry.go               # 工具注册表（排序稳定性 + deny 过滤）
│   │   ├── orchestration.go          # 工具编排（goroutine 并发分组 + 权限检查）
│   │   ├── bash/                     # BashTool
│   │   │   ├── bash.go               # 执行逻辑（exec.Command + 超时 + context.Cancel）
│   │   │   ├── security.go           # Shell AST 安全检查（mvdan.cc/sh 解析）
│   │   │   └── permissions.go        # Bash 权限规则
│   │   ├── fileread/                 # FileReadTool
│   │   │   └── fileread.go           # 缓存 + 行号范围 + 编码检测 + 图片 base64
│   │   ├── fileedit/                 # FileEditTool
│   │   │   └── fileedit.go           # 精确替换 + 唯一性校验 + 并发检测
│   │   ├── filewrite/                # FileWriteTool
│   │   │   └── filewrite.go          # 创建/覆盖 + 目录自动创建
│   │   ├── grep/                     # GrepTool
│   │   │   └── grep.go               # ripgrep 包装 + 大结果磁盘溢出
│   │   ├── glob/                     # GlobTool
│   │   │   └── glob.go               # doublestar 文件匹配
│   │   ├── webfetch/                 # WebFetchTool
│   │   │   └── webfetch.go           # HTML→MD + PDF→Text + Image→Base64
│   │   ├── websearch/                # WebSearchTool
│   │   │   └── websearch.go
│   │   ├── agent/                    # AgentTool
│   │   │   ├── agent.go              # 子 Agent 主逻辑
│   │   │   ├── runagent.go           # Agent 执行核心（创建子 Engine）
│   │   │   ├── taskmanager.go        # 后台任务管理
│   │   │   └── memorysnapshot.go     # 记忆快照
│   │   ├── askuser/                  # AskUserQuestionTool
│   │   │   └── askuser.go
│   │   ├── todo/                     # TodoWriteTool
│   │   │   └── todo.go
│   │   ├── sendmessage/              # SendMessageTool
│   │   │   └── sendmessage.go
│   │   ├── sleep/                    # SleepTool
│   │   │   └── sleep.go
│   │   ├── taskstop/                 # TaskStopTool
│   │   │   └── taskstop.go
│   │   ├── skill/                    # SkillTool
│   │   │   └── skill.go
│   │   ├── notebookedit/             # NotebookEditTool
│   │   │   └── notebookedit.go
│   │   ├── planmode/                 # EnterPlanMode + ExitPlanMode
│   │   │   └── planmode.go
│   │   ├── cron/                     # 定时任务工具
│   │   │   ├── create.go             # CronCreateTool
│   │   │   ├── delete.go             # CronDeleteTool
│   │   │   ├── list.go               # CronListTool
│   │   │   └── tasks.go              # 任务存储（内存 + 磁盘双存储）
│   │   ├── brief/                    # BriefTool
│   │   │   ├── brief.go              # 主动推送消息
│   │   │   └── attachments.go        # 附件验证
│   │   ├── teamcreate/               # TeamCreateTool
│   │   │   └── teamcreate.go
│   │   ├── teamdelete/               # TeamDeleteTool
│   │   │   └── teamdelete.go
│   │   └── listpeers/                # ListPeersTool
│   │       └── listpeers.go
│   │
│   ├── skill/                        # 技能系统
│   │   ├── types.go                  # SkillFrontmatter、Skill
│   │   ├── loader.go                 # .claude/commands/*.md → PromptCommand
│   │   ├── bundled.go                # 内置打包技能（go:embed）
│   │   ├── discovery.go              # 技能目录自动发现
│   │   ├── conditional.go            # 条件激活（filePattern 匹配）
│   │   └── search.go                 # 技能搜索
│   │
│   ├── plugin/                       # 插件系统
│   │   ├── types.go                  # PluginManifest、PluginHooks
│   │   ├── loader.go                 # 插件加载（go-plugin GRPC subprocess）
│   │   ├── registry.go               # 插件注册表 CRUD
│   │   ├── hooks.go                  # Hooks 执行引擎（6种 Hook）
│   │   └── builtin.go                # 内置插件（code-review、security-review）
│   │
│   ├── buddy/                        # 伴侣系统
│   │   ├── types.go                  # Species、Rarity、CompanionBones/Soul
│   │   ├── companion.go              # 伴侣生成（Mulberry32 PRNG）
│   │   ├── hatch.go                  # LLM 孵化
│   │   └── storage.go                # 伴侣持久化（JSON）
│   │
│   ├── command/                      # 命令系统
│   │   ├── types.go                  # Command interface（LocalCommand / PromptCommand）
│   │   ├── registry.go               # 命令注册表（4源合并 + sync.Once 缓存）
│   │   ├── executor.go               # 命令执行器
│   │   ├── builtin/                  # 内置命令
│   │   │   ├── compact.go            # /compact
│   │   │   ├── clear.go              # /clear
│   │   │   ├── memory.go             # /memory
│   │   │   ├── resume.go             # /resume
│   │   │   ├── session.go            # /session
│   │   │   ├── status.go             # /status
│   │   │   ├── cost.go               # /cost
│   │   │   ├── model.go              # /model
│   │   │   ├── permissions.go        # /permissions
│   │   │   ├── help.go               # /help
│   │   │   ├── plugincmd.go          # /plugin
│   │   │   ├── skills.go             # /skills
│   │   │   ├── hatch.go              # /hatch
│   │   │   └── automode.go           # /auto-mode
│   │   └── custom/
│   │       └── skilldir_loader.go    # 从 .claude/commands/*.md 加载
│   │
│   ├── memory/                       # 记忆系统
│   │   ├── types.go                  # Memory 类型
│   │   ├── claudemd.go               # CLAUDE.md 多层级读取
│   │   ├── nested.go                 # 嵌套记忆展开（@include 递归）
│   │   ├── session_memory.go         # 会话记忆提取（LLM）
│   │   ├── relevance.go              # 相关性排序
│   │   └── inject.go                 # 记忆注入系统提示
│   │
│   ├── session/                      # 会话存储
│   │   ├── types.go                  # SessionMetadata、TranscriptEntry
│   │   ├── storage.go                # JSONL 会话录制
│   │   ├── writequeue.go             # 异步写队列（channel + goroutine）
│   │   ├── resume.go                 # 会话恢复
│   │   ├── metadata.go               # 元数据索引
│   │   ├── history.go                # 输入历史
│   │   └── export.go                 # Markdown 导出
│   │
│   ├── agent/                        # 多 Agent 协调
│   │   ├── types.go                  # AgentDefinition、AgentTask、AgentMessage
│   │   ├── coordinator.go            # 协调器
│   │   ├── taskmanager.go            # 任务生命周期
│   │   ├── worktree.go               # Git worktree 隔离
│   │   ├── messaging.go              # Agent 间消息（channel 队列）
│   │   └── color.go                  # 颜色分配
│   │
│   ├── daemon/                       # 守护进程
│   │   ├── types.go                  # DaemonConfig、ScheduledTask
│   │   ├── supervisor.go             # 主管进程
│   │   ├── worker.go                 # 工作者进程
│   │   ├── pidregistry.go            # PID 文件注册表
│   │   ├── scheduler.go              # Cron 调度器（robfig/cron）
│   │   ├── lock.go                   # 调度器文件锁（O_EXCL）
│   │   └── proactive.go              # 主动模式
│   │
│   ├── permission/                   # 权限系统
│   │   ├── types.go                  # PermissionMode、PermissionResult
│   │   ├── checker.go                # 权限检查链（deny→allow→ask + Auto Mode）
│   │   ├── rules.go                  # 规则匹配
│   │   └── filesystem.go             # 文件系统权限
│   │
│   ├── state/                        # 状态管理
│   │   ├── store.go                  # 内存 Store（sync.RWMutex + callback 通知）
│   │   ├── types.go                  # AppState
│   │   └── session.go                # 进程级会话状态
│   │
│   ├── service/                      # 服务层
│   │   ├── compact/
│   │   │   ├── auto.go               # LLM 摘要压缩
│   │   │   ├── micro.go              # 本地微压缩
│   │   │   └── collapse.go           # 上下文折叠
│   │   ├── token.go                  # Token 估算
│   │   └── cost.go                   # 成本追踪
│   │
│   ├── prompt/                       # 提示词系统（完整迁移）
│   │   ├── system.go                 # 6层系统提示构建器
│   │   ├── templates.go              # 核心行为规范（go:embed 嵌入）
│   │   ├── templates/                # 提示词模板文件（嵌入资源）
│   │   │   ├── base_prompt.txt       # 角色定义 + 工具规则 + 安全限制
│   │   │   └── undercover.txt        # 卧底模式指令模板
│   │   ├── toolprompts.go            # 工具描述聚合
│   │   ├── envcontext.go             # 环境上下文
│   │   ├── usercontext.go            # <user_context> XML 注入
│   │   ├── processinput.go           # 用户输入处理
│   │   ├── filementions.go           # @文件展开
│   │   ├── imageattach.go            # 图片附件 base64
│   │   ├── toolresult_budget.go      # 工具结果预算截断
│   │   ├── token_warning.go          # Token 限制分级
│   │   ├── serializer.go             # 消息序列化
│   │   ├── thinking.go               # Thinking 消息规则
│   │   └── cache.go                  # Prompt Cache 管理
│   │
│   ├── mode/                         # 模式系统
│   │   ├── types.go                  # AutoModeRules、ClassifierResult
│   │   ├── undercover.go             # Undercover 卧底模式
│   │   ├── automode.go               # Auto Mode YoloClassifier
│   │   ├── automode_rules.go         # 默认规则集
│   │   ├── automode_state.go         # 决策追踪统计
│   │   ├── fastmode.go               # Fast Mode
│   │   └── sidequery.go              # 侧路查询
│   │
│   ├── api/                          # HTTP API 层
│   │   ├── router.go                 # chi 路由定义
│   │   ├── middleware/
│   │   │   ├── auth.go               # API Key 认证
│   │   │   └── error.go              # 错误处理 + recovery
│   │   └── handler/
│   │       ├── chat.go               # POST /api/chat（SSE 流式）
│   │       ├── sessions.go           # 会话管理
│   │       ├── tools.go              # 工具管理
│   │       ├── memory.go             # 记忆管理
│   │       ├── commands.go           # 命令执行
│   │       ├── agents.go             # Agent 管理
│   │       ├── daemon.go             # 守护进程管理
│   │       ├── skills.go             # 技能管理
│   │       ├── plugins.go            # 插件管理
│   │       ├── buddy.go              # 伴侣管理
│   │       └── modes.go              # 模式管理
│   │
│   └── util/                         # 工具函数
│       ├── cancel.go                 # context.Context 管理（替代 AbortController）
│       ├── logger.go                 # slog 封装
│       ├── config.go                 # Viper 配置加载
│       ├── message.go                # 消息格式转换
│       ├── pathsec.go                # 路径安全检查
│       ├── cleanup.go                # 进程退出清理（signal handler）
│       └── process.go                # 进程工具（PID 检测、exec）
│
├── embed/                            # go:embed 嵌入资源
│   ├── prompts/                      # 系统提示词模板
│   │   ├── base_prompt.txt
│   │   └── undercover_instructions.txt
│   └── skills/                       # 内置打包技能
│       ├── release-notes.md
│       └── summarize-codebase.md
│
└── test/                             # 集成测试
    ├── engine_test.go
    ├── provider_test.go
    ├── tool_test.go
    └── api_test.go
```

---

## 五、核心架构映射（TypeScript → Go）

### 流式链：AsyncGenerator → Channel

```
TypeScript:  Provider.callModel() → AsyncGenerator<StreamEvent> → queryLoop → QueryEngine.submitMessage()
Go:          Provider.CallModel() → <-chan StreamEvent → queryLoop (for-select) → Engine.SubmitMessage() → <-chan StreamEvent
```

```go
// TypeScript AsyncGenerator 等价的 Go 模式
type StreamEvent struct {
    Type    EventType
    Content interface{} // TextDelta / ToolUse / ToolResult / Error / ...
}

// Provider interface
type Provider interface {
    CallModel(ctx context.Context, params CallParams) (<-chan StreamEvent, error)
}

// Engine 公共 API
func (e *Engine) SubmitMessage(ctx context.Context, msg string) <-chan StreamEvent {
    ch := make(chan StreamEvent, 64)
    go func() {
        defer close(ch)
        e.queryLoop(ctx, msg, ch)
    }()
    return ch
}
```

### 状态管理：EventEmitter → sync.RWMutex + Callbacks

```go
type Store struct {
    mu        sync.RWMutex
    state     AppState
    listeners []func(AppState)
}

func (s *Store) Update(fn func(*AppState)) {
    s.mu.Lock()
    fn(&s.state)
    snapshot := s.state // copy
    s.mu.Unlock()
    for _, l := range s.listeners {
        l(snapshot)
    }
}
```

### 工具并发：Promise.all → errgroup

```go
func (o *Orchestrator) RunTools(ctx context.Context, calls []ToolCall) []ToolResult {
    // 分组：并发安全的工具并行，非安全的串行
    groups := o.groupByConcurrency(calls)
    var results []ToolResult
    for _, group := range groups {
        g, gCtx := errgroup.WithContext(ctx)
        groupResults := make([]ToolResult, len(group))
        for i, call := range group {
            g.Go(func() error {
                groupResults[i] = o.executeTool(gCtx, call)
                return nil
            })
        }
        g.Wait()
        results = append(results, groupResults...)
    }
    return results
}
```

### 异步写队列：FIFO Queue → Channel Worker

```go
type WriteQueue struct {
    ch chan writeRequest
}

func NewWriteQueue() *WriteQueue {
    wq := &WriteQueue{ch: make(chan writeRequest, 256)}
    go wq.worker() // 单 goroutine 串行写
    return wq
}

func (wq *WriteQueue) worker() {
    for req := range wq.ch {
        req.writeFn()
    }
}
```

---

## 六、分阶段实施计划（20 Phase / 128 步）

### Phase 1：项目脚手架 + 类型系统
1. 创建 `agent-engine/` 目录，`go mod init github.com/wall-ai/agent-engine`
2. 创建 `Makefile`（build/test/lint/run/fmt）
3. 安装核心依赖（`go get` 所有库）
4. 定义核心类型 `internal/engine/types.go`：`Message`、`StreamEvent`、`ToolCall`、`ToolResult`、`EngineConfig`
5. 定义 `internal/tool/tool.go`：Tool interface（Name/InputSchema/Call/CheckPermissions/Prompt/IsEnabled/IsReadOnly/IsConcurrencySafe）
6. 定义 `internal/state/types.go`：`AppState`、`SessionState`
7. 定义 `internal/command/types.go`：`Command` interface（LocalCommand / PromptCommand）
8. 定义 `internal/memory/types.go`：`ExtractedMemory`、`ClaudeMdContent`
9. 定义 `internal/session/types.go`：`SessionMetadata`、`TranscriptEntry`

### Phase 2：LLM Provider 适配层
10. 实现 `provider/provider.go` — Provider interface（`CallModel(ctx, params) (<-chan StreamEvent, error)`）
11. 实现 `provider/anthropic.go` — Anthropic Claude 流式调用（`anthropic-sdk-go` + 重试 + thinking 支持 + context 取消）
12. 实现 `provider/openai_compat.go` — OpenAI 兼容接口（`go-openai` + stream + tool_call 映射 + Ollama/vLLM 适配）
13. 实现 `provider/factory.go` — Provider 工厂（按配置字符串创建 Provider 实例）
14. 实现 `provider/message_convert.go` — 内部 Message ⇆ Anthropic/OpenAI 消息格式双向转换

### Phase 3：核心引擎 + 状态管理
15. 实现 `engine/engine.go` — QueryEngine（`SubmitMessage(ctx, msg) <-chan StreamEvent`，会话管理，goroutine 启动 queryLoop）
16. 实现 `engine/queryloop.go` — 核心查询循环（for-select 状态机：callModel → runTools → continue/stop，context 取消支持）
17. 实现 `state/store.go` — 内存 Store（`sync.RWMutex` + callback 通知，不可变快照）
18. 实现 `state/session.go` — 进程级会话状态（cwd/sessionId/totalCost，`atomic` 安全）

### Phase 4：系统提示词完整迁移 + Prompt Cache 管理
19. 实现 `prompt/templates.go` — **完整迁移** 核心行为规范（`go:embed` 嵌入 base_prompt.txt，角色定义 + 工具规则 + 安全限制 + 输出格式约束）
20. 创建 `embed/prompts/base_prompt.txt` — 系统提示词原文（可运行时覆盖）
21. 实现 `prompt/toolprompts.go` — 工具描述聚合（遍历 Tool.Prompt() → 按排序拼接，保持缓存稳定）
22. 实现 `prompt/envcontext.go` — 环境上下文（`runtime.GOOS`/`runtime.GOARCH`/`os.Getwd()`/时间/Git 信息）
23. 实现 `prompt/system.go` — 6层系统提示构建器 `BuildEffectiveSystemPrompt()`
    - [1] base_prompt（go:embed，最稳定）
    - [2] tool_descriptions（工具变更时才变）
    - [3] memories（CLAUDE.md 内容）
    - [4] environment（动态，放最后）
    - [5] customSystemPrompt（用户自定义）
    - [6] appendSystemPrompt（追加内容）
24. 实现 `prompt/cache.go` — Prompt Cache 管理
    - cache_control 注入策略（system prompt 多段 TextBlockParam）
    - CACHE_FRIENDLY_ORDER 常量
    - 工具列表排序稳定性（`sort.SliceStable`，内置前、MCP 后）
    - skipCacheWrite 参数
    - cache_creation/read tokens 成本追踪
25. 实现 `prompt/usercontext.go` — `<user_context>` XML 注入
26. 实现 `prompt/processinput.go` — 用户输入处理主函数
27. 实现 `prompt/filementions.go` — @文件展开（正则 + 文件读取 + 懒加载）
28. 实现 `prompt/imageattach.go` — 图片附件 base64（`encoding/base64` + `image` 标准库）
29. 实现 `prompt/toolresult_budget.go` — 工具结果预算截断
30. 实现 `prompt/token_warning.go` — Token 限制分级（WARNING=0.85 / BLOCKING=0.95）
31. 实现 `prompt/serializer.go` — 消息序列化（Message → API MessageParam）
32. 实现 `prompt/thinking.go` — Thinking 消息规则

### Phase 5：工具系统
33. 实现 `tool/tool.go` — Tool interface 完整定义 + `BuildTool()` 构造辅助
34. 实现 `tool/registry.go` — 工具注册表（`GetAllBaseTools()` → `GetTools()` → `AssembleToolPool()`，`sort.SliceStable` 排序）
35. 实现 `tool/orchestration.go` — 工具编排（`errgroup` 并发分组 + 权限检查 + `IsConcurrencySafe` 分组）
36. 实现 `permission/checker.go` — 权限检查链（deny→allow→ask + 文件系统安全 + Auto Mode 集成）

### Phase 6：核心工具实现（文件+搜索+Shell）
37. `tool/bash/` — BashTool（`exec.CommandContext` + `mvdan.cc/sh` AST 安全检查 + 超时 + 后台 goroutine + 进度回调）
38. `tool/fileread/` — FileReadTool（`sync.Map` 缓存 + 行号范围 + `chardet` 编码检测 + token 限制 + 图片 base64）
39. `tool/fileedit/` — FileEditTool（精确替换 + `strings.Count` 唯一性校验 + `os.Stat` 并发检测 + 行结尾保留）
40. `tool/filewrite/` — FileWriteTool（`os.MkdirAll` 自动创建 + 原子写入 `os.Rename`）
41. `tool/grep/` — GrepTool（`exec.Command("rg", ...)` + 大结果 `os.TempFile` 磁盘溢出）
42. `tool/glob/` — GlobTool（`doublestar.Glob` 文件匹配）
43. `tool/notebookedit/` — NotebookEditTool（`encoding/json` 解析 .ipynb）

### Phase 7：扩展工具实现（Web+Agent+流程控制）
44. `tool/webfetch/` — WebFetchTool（`net/http` + `html-to-markdown` + `ledongthuc/pdf` + base64 + robots.txt）
45. `tool/websearch/` — WebSearchTool
46. `tool/agent/` — AgentTool（子 Engine goroutine + 工具继承过滤 + 记忆快照）
47. `tool/askuser/` — AskUserQuestionTool（channel 阻塞等待用户回复）
48. `tool/todo/` — TodoWriteTool
49. `tool/sendmessage/` — SendMessageTool（Agent 间 channel 消息）
50. `tool/sleep/` — SleepTool（`time.After` + context 取消）
51. `tool/taskstop/` — TaskStopTool（设 taskStopped=true，context cancel）
52. `tool/skill/` — SkillTool（contextModifier 注入）
53. `tool/planmode/` — EnterPlanMode + ExitPlanMode
54. `tool/teamcreate/` — TeamCreateTool
55. `tool/teamdelete/` — TeamDeleteTool
56. `tool/listpeers/` — ListPeersTool

### Phase 8：定时任务 + 主动通知工具
57. `tool/cron/tasks.go` — 任务存储（`sync.Map` 内存 + JSON 磁盘双存储 + 90天过期）
58. `tool/cron/create.go` — CronCreateTool（`robfig/cron` 表达式校验 + prompt + recurring + durable）
59. `tool/cron/delete.go` — CronDeleteTool
60. `tool/cron/list.go` — CronListTool（合并内存+磁盘）
61. `tool/brief/` — BriefTool（主动推送 + 附件）

### Phase 9：模式系统（Undercover + Auto Mode + Fast Mode）
62. 实现 `mode/types.go` — AutoModeRules、ClassifierResult、AutoModeState、RepoClass
63. 实现 `mode/undercover.go` — Undercover 卧底模式
    - `IsUndercover()` — 环境变量 + 仓库分类
    - `ClassifyRepo()` — `go-git` 读 remote URL 分类（internal/external/none）
    - `GetUndercoverInstructions()` — `go:embed` 加载指令模板
    - 可配置 allowlist
64. 实现 `mode/automode.go` — Auto Mode YoloClassifier
    - `RunYoloClassifier()` — 侧路查询（Haiku），返回 allow/soft_deny/nil
    - `BuildYoloSystemPrompt()` — 默认规则 + 用户自定义规则合并
    - `BuildClassifierMessages()` — 截断上下文
    - `ParseClassifierResponse()` — 结构化解析
    - Prompt Cache（cache_control: ephemeral）
65. 实现 `mode/automode_rules.go` — 默认规则集（allow/soft_deny/environment）
66. 实现 `mode/automode_state.go` — 决策追踪（`atomic.Int64` 统计）
67. 实现 `mode/fastmode.go` — Fast Mode（可用性检查 + 不可用原因 + 配置驱动 kill switch）
68. 实现 `mode/sidequery.go` — 侧路查询（独立 goroutine API 调用，独立 context）

### Phase 10：技能系统
69. 实现 `skill/types.go` — SkillFrontmatter struct（`adrg/frontmatter` 标签）
70. 实现 `skill/loader.go` — Markdown 解析（`goldmark` + `frontmatter`）→ PromptCommand
71. 实现 `skill/bundled.go` — 内置技能（`go:embed embed/skills/*.md`）
72. 实现 `skill/discovery.go` — 技能目录发现（`filepath.WalkDir`）
73. 实现 `skill/conditional.go` — 条件激活（`doublestar.Match` filePattern）
74. 实现 `skill/search.go` — 技能搜索（关键词匹配 + 排序）

### Phase 11：插件系统
75. 实现 `plugin/types.go` — PluginManifest（JSON Schema）
76. 实现 `plugin/loader.go` — 插件加载（`hashicorp/go-plugin` GRPC subprocess）
77. 实现 `plugin/registry.go` — 插件注册表 CRUD
78. 实现 `plugin/hooks.go` — Hooks 执行引擎（PreToolUse/PostToolUse/Stop/UserPromptSubmit/SessionStart/Notification）
79. 实现 `plugin/builtin.go` — 内置插件技能（`go:embed` 提示词）

### Phase 12：伴侣系统
80. 实现 `buddy/types.go` — Species(18种)、Rarity(5级)、CompanionBones/Soul
81. 实现 `buddy/companion.go` — 伴侣生成（Mulberry32 PRNG，Go 手写实现）
82. 实现 `buddy/hatch.go` — LLM 孵化（sideQuery）
83. 实现 `buddy/storage.go` — JSON 持久化（骨骼重算，灵魂存 config.json）

### Phase 13：记忆系统
84. 实现 `memory/claudemd.go` — CLAUDE.md 多层级读取（全局→项目→本地）
85. 实现 `memory/nested.go` — 嵌套记忆展开（@include 递归，`filepath.Clean` 防遍历）
86. 实现 `memory/session_memory.go` — 会话记忆提取（LLM sideQuery）
87. 实现 `memory/relevance.go` — 相关性排序（关键词 + 时间衰减）
88. 实现 `memory/inject.go` — 记忆注入系统提示（`errgroup` 并行预取）

### Phase 14：会话存储与历史
89. 实现 `session/writequeue.go` — 异步写队列（channel + worker goroutine）
90. 实现 `session/storage.go` — JSONL 会话录制（`bufio.Writer` + 增量追加）
91. 实现 `session/metadata.go` — 元数据索引（列表/搜索/统计）
92. 实现 `session/resume.go` — 会话恢复（`bufio.Scanner` JSONL + compact_boundary）
93. 实现 `session/history.go` — 输入历史（最近 1000 条，环形缓冲区）
94. 实现 `session/export.go` — Markdown 导出

### Phase 15：命令系统
95. 实现 `command/types.go` — Command interface
96. 实现 `command/registry.go` — 命令注册表（`sync.Once` 缓存，4源合并）
97. 实现 `command/executor.go` — 命令执行器（slash 命令分发）
98. 实现内置命令（14个）：compact/clear/memory/resume/session/status/cost/model/permissions/help/plugin/skills/hatch/auto-mode
99. 实现 `command/custom/skilldir_loader.go` — 从 `.claude/commands/*.md` 加载

### Phase 16：多 Agent 协调系统
100. 实现 `agent/types.go` — Agent 类型
101. 实现 `agent/taskmanager.go` — 任务生命周期（`sync.Map` + 状态机）
102. 实现 `agent/coordinator.go` — 协调器（权限委托、分类器决策）
103. 实现 `agent/worktree.go` — Git worktree（`go-git` 或 `exec.Command("git")`）
104. 实现 `agent/messaging.go` — Agent 间消息（buffered channel 队列）
105. 实现 `agent/color.go` — 颜色分配

### Phase 17：守护进程模式
106. 实现 `daemon/types.go` — DaemonConfig、ScheduledTask
107. 实现 `daemon/pidregistry.go` — PID 文件注册表（`os.OpenFile` + O_EXCL）
108. 实现 `daemon/lock.go` — 调度器文件锁（原子互斥 + 崩溃恢复）
109. 实现 `daemon/scheduler.go` — Cron 调度器（`robfig/cron` + `fsnotify` 文件监听）
110. 实现 `daemon/supervisor.go` — 主管进程（`exec.Command` spawn + 健康检查 goroutine）
111. 实现 `daemon/worker.go` — 工作者进程（Engine 执行 + 心跳 ticker）
112. 实现 `daemon/proactive.go` — 主动模式（`time.Ticker` 驱动）

### Phase 18：上下文压缩 + Token 管理
113. 实现 `service/compact/auto.go` — LLM 摘要压缩（80% 阈值）
114. 实现 `service/compact/micro.go` — 本地微压缩
115. 实现 `service/compact/collapse.go` — 上下文折叠
116. 实现 `service/token.go` — Token 估算（`tiktoken-go` + 字符比备用）
117. 实现 `service/cost.go` — 成本追踪（`atomic` 累计 + 预算限制）
118. 实现 `engine/context_pipeline.go` — 压缩流水线

### Phase 19：SDK 导出 + HTTP Server
119. 实现 `pkg/sdk/engine.go` — SDK 公共 API（functional options 配置）
120. 实现 `api/router.go` — chi 路由定义
121. 实现 `api/handler/chat.go` — SSE 流式（`http.Flusher` + `text/event-stream`）
122. 实现 `api/handler/sessions.go` — 会话管理
123. 实现 `api/handler/memory.go` — 记忆管理
124. 实现 `api/handler/commands.go` — 命令执行
125. 实现 `api/handler/agents.go` — Agent 管理
126. 实现 `api/handler/daemon.go` — 守护进程管理
127. 实现 `api/handler/skills.go` — 技能管理
128. 实现 `api/handler/plugins.go` — 插件管理
129. 实现 `api/handler/buddy.go` — 伴侣管理
130. 实现 `api/handler/modes.go` — 模式管理
131. 实现 `api/handler/tools.go` — 工具管理
132. 实现 `api/middleware/auth.go` — API Key 认证中间件
133. 实现 `api/middleware/error.go` — 错误处理 + panic recovery
134. 实现 `cmd/agent-engine/main.go` — HTTP Server 启动入口（flag 解析 + graceful shutdown）

### Phase 20：配置 + 测试 + 文档
135. 实现 `util/config.go` — Viper 配置（环境变量 > JSON > 默认值）
136. 实现 `util/cleanup.go` — 进程退出清理（`os/signal` + cleanup 注册表）
137. 实现 `util/pathsec.go` — 路径安全检查
138. 编写单元测试（engine、provider、tool、prompt、mode 各模块）
139. 编写集成测试（`test/` 目录）
140. 编写 `README.md`（SDK 用法 + HTTP API + 工具列表 + 配置 + 构建指南）
141. 编写 `Makefile`（build/test/lint/docker/release）

---

## 七、核心架构映射表（claude-code → Go backend）

| claude-code 原模块 | Go 新模块 | 变更说明 |
|---|---|---|
| `QueryEngine.ts` | `engine/engine.go` | AsyncGenerator → `<-chan StreamEvent` |
| `query.ts` | `engine/queryloop.go` | while-true → `for-select` 状态机 |
| `Tool.ts` | `tool/tool.go` | TypeScript interface → Go interface |
| `tools.ts` | `tool/registry.go` | `sort.SliceStable` 排序 |
| `services/api/claude.ts` | `provider/anthropic.go` | `anthropic-sdk-go` 官方 SDK |
| — | `provider/openai_compat.go` | `go-openai` 库 |
| `toolOrchestration.ts` | `tool/orchestration.go` | `errgroup` 并发 |
| `AppStateStore.ts` | `state/store.go` | `sync.RWMutex` + callback |
| `useCanUseTool.tsx` | `permission/checker.go` | 纯函数 |
| `constants/prompts.ts` | `prompt/templates.go` + `embed/prompts/` | `go:embed` 嵌入 |
| `utils/systemPrompt.ts` | `prompt/system.go` + `prompt/cache.go` | 6层构建 + Cache |
| `utils/messages.ts` | `prompt/serializer.go` + `prompt/thinking.go` | 序列化 + Thinking 规则 |
| `utils/undercover.ts` | `mode/undercover.go` | `go-git` 仓库分类 |
| `yoloClassifier.ts` | `mode/automode.go` + `mode/automode_rules.go` | 侧路 goroutine 查询 |
| `autoModeState.ts` | `mode/automode_state.go` | `atomic.Int64` 统计 |
| `utils/fastMode.ts` | `mode/fastmode.go` | Viper 配置驱动 |
| `utils/sideQuery.ts` | `mode/sidequery.go` | 独立 goroutine + context |
| `utils/claudeMd.ts` | `memory/claudemd.go` | 多层级读取 |
| `sessionStorage.ts` | `session/storage.go` + `session/writequeue.go` | channel worker |
| `commands.ts` + `commands/` | `command/` | `sync.Once` 缓存 |
| `coordinator/` + `tasks/` | `agent/` | channel 消息传递 |
| `skills/` | `skill/` | `go:embed` 内置技能 |
| `utils/plugins/` | `plugin/` | `hashicorp/go-plugin` GRPC |
| `buddy/` | `buddy/` | 手写 Mulberry32 PRNG |
| `tools/ScheduleCronTool/` | `tool/cron/` | `robfig/cron` |
| `tools/BriefTool/` | `tool/brief/` | 保留逻辑 |
| KAIROS (`daemon/`) | `daemon/` | 原生 `os/exec` + `os/signal` |
| Express + SSE | `chi` + `net/http` SSE | `http.Flusher` |

---

## 八、关键设计决策

1. **Channel 流式链**：Provider → queryLoop → Engine → SDK/HTTP 全链路 `<-chan StreamEvent`，goroutine 生产，消费者 range 读取
2. **Provider interface**：`CallModel(ctx, params) (<-chan StreamEvent, error)`，Anthropic 和 OpenAI 各自实现
3. **context.Context 传播**：所有异步操作通过 ctx 传播取消信号，替代 TypeScript AbortController
4. **errgroup 并发工具执行**：`isConcurrencySafe` 分组后 `errgroup.Go()` 并行，超时通过 `context.WithTimeout`
5. **`go:embed` 嵌入提示词**：base_prompt.txt、undercover_instructions.txt、内置技能 Markdown 编译时嵌入，运行时可覆盖
6. **`sync.RWMutex` 状态管理**：替代 EventEmitter，Update 加写锁后通知 listeners
7. **channel 异步写队列**：buffered channel + 单 goroutine worker，FIFO 串行写 JSONL
8. **`hashicorp/go-plugin` 插件**：GRPC subprocess 跨平台（Windows/macOS/Linux），每个插件是独立二进制
9. **Viper 配置**：环境变量 > JSON 配置文件 > 默认值，替代 TypeScript 手写配置加载
10. **`go-git` 仓库操作**：纯 Go Git 实现，用于 Undercover 仓库分类和 worktree 隔离
11. **`mvdan.cc/sh` Shell 解析**：Go 原生 Shell AST 解析器，比 TypeScript 方案更成熟可靠
12. **`robfig/cron` 调度器**：功能完备的 cron 库，支持秒级精度，替代 JS cron-parser
13. **`atomic.Int64` 统计**：Auto Mode 决策追踪使用原子操作，无锁高性能
14. **`sort.SliceStable` 工具排序**：确保工具列表排序稳定性，保证 Prompt Cache 命中
15. **单二进制部署**：`go build -o agent-engine`，无外部运行时依赖
16. **Graceful shutdown**：`os/signal.NotifyContext` 捕获 SIGTERM/SIGINT，优雅关闭所有 goroutine
17. **Prompt Cache 最大化**：与 TypeScript 版策略一致（stable→dynamic 排序），cache_control 通过 Anthropic SDK 设置
18. **Undercover 模式 go:embed**：卧底指令模板编译时嵌入，allowlist 通过 Viper 配置
19. **Auto Mode 侧路查询**：独立 goroutine + 独立 context，不阻塞主 queryLoop
20. **系统提示完整迁移**：6层构建与 TypeScript 版完全一致，`go:embed` 替代 `constants/prompts.ts`
21. **GrowthBook 去除**：kill switches 全部改 Viper 配置文件驱动
22. **伴侣 Mulberry32**：Go 手写 Mulberry32 PRNG（约 20 行），确保与 TypeScript 版确定性一致
23. **消息序列化完整保留**：7种 Message 类型 → API MessageParam 完整映射
24. **Thinking 规则保留**：thinking blocks 完整性规则在 Go 中通过 slice 操作实现

---

## 九、构建与部署

```bash
# 开发
make dev          # go run ./cmd/agent-engine -port 8080

# 构建
make build        # go build -o bin/agent-engine ./cmd/agent-engine

# 跨平台构建
make release      # GOOS=linux/darwin/windows 三平台交叉编译

# 测试
make test         # go test ./...
make test-cover   # go test -coverprofile=cover.out ./...

# Lint
make lint         # golangci-lint run

# Docker
make docker       # docker build -t agent-engine .
```

### Dockerfile（多阶段构建）

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /agent-engine ./cmd/agent-engine

FROM alpine:3.20
RUN apk add --no-cache git ripgrep
COPY --from=builder /agent-engine /usr/local/bin/
ENTRYPOINT ["agent-engine"]
```

---

## 十、SDK 使用示例

```go
package main

import (
    "context"
    "fmt"
    sdk "github.com/wall-ai/agent-engine/pkg/sdk"
)

func main() {
    engine, err := sdk.NewEngine(
        sdk.WithProvider("anthropic"),
        sdk.WithAPIKey("sk-ant-..."),
        sdk.WithModel("claude-sonnet-4-20250514"),
        sdk.WithWorkDir("/path/to/project"),
        sdk.WithAutoMode(true),
    )
    if err != nil {
        panic(err)
    }
    defer engine.Close()

    ctx := context.Background()
    events := engine.SubmitMessage(ctx, "请帮我重构 main.go 中的错误处理")

    for event := range events {
        switch event.Type {
        case sdk.EventTextDelta:
            fmt.Print(event.Text)
        case sdk.EventToolUse:
            fmt.Printf("\n[Tool: %s]\n", event.ToolName)
        case sdk.EventToolResult:
            fmt.Printf("[Result: %s]\n", event.Result[:100])
        case sdk.EventDone:
            fmt.Println("\n--- Done ---")
        }
    }
}
```

---

## 十一、核心依赖版本汇总

```
go 1.23
chi v5.1.0
anthropic-sdk-go v0.2.0-beta.3
go-openai v1.36.0
jsonschema/v6 v6.0.1
validator/v10 v10.22.0
sh/v3 v3.9.0
html-to-markdown/v2 v2.2.1
goldmark v1.7.8
frontmatter v0.2.0
doublestar/v4 v4.7.1
cron/v3 v3.0.1
fsnotify v1.8.0
uuid v1.6.0
slog (stdlib)
viper v1.19.0
go-git/v5 v5.12.0
tiktoken-go/tokenizer v0.2.1
testify v1.9.0
go-plugin v1.6.2
```
