# Claude Code 源码深度解读 — 05 Command 命令系统

> 覆盖文件：`commands.ts`（28KB，755行）、`types/command.ts`、`commands/` 目录（80+ 命令）

---

## 1. 模块职责概述

Command（命令）系统是 Claude Code 的 **Slash 命令层**，提供 `/compact`、`/clear`、`/memory`、`/commit` 等内置命令。与 Tool 系统（供 Claude AI 调用）不同，Command 系统**由用户发起调用**。

---

## 2. Command 类型体系（`types/command.ts`）

### 2.1 Command 联合类型

```typescript
type Command = LocalCommand | LocalJSXCommand | PromptCommand
```

三种命令类型：

#### PromptCommand（提示型命令）
将命令转化为发送给 Claude 的提示词，Claude 完成实际工作：

```typescript
type PromptCommand = CommandBase & {
  type: 'prompt'
  getPromptForCommand(
    args: string,
    context: LocalJSXCommandContext,
  ): Promise<string>  // 返回要发送给 Claude 的提示词
  contentLength?: number  // 提示词 token 数（用于预算）
  progressMessage?: string  // 执行中显示的消息
}
```

示例：`/commit` 命令返回一条让 Claude 生成 git commit 消息的提示词。

#### LocalCommand（本地执行型命令）
在客户端本地执行，不调用 Claude API：

```typescript
type LocalCommand = CommandBase & {
  type: 'local'
  call(
    args: string,
    context: LocalJSXCommandContext,
  ): Promise<LocalCommandResult>
}

type LocalCommandResult = {
  type: 'result'
  resultMarkdown?: string  // Markdown 格式的结果
  data?: unknown
}
```

示例：`/clear` 命令清空当前会话消息，无需调用 Claude。

#### LocalJSXCommand（本地 JSX 型命令）
执行后返回 React 组件渲染到 REPL：

```typescript
type LocalJSXCommand = CommandBase & {
  type: 'local'
  call(args, context): Promise<LocalJSXCommandResult>
}

type LocalJSXCommandResult = {
  type: 'jsx'
  jsx: React.ReactNode  // 要渲染的 React 组件
  shouldHidePromptInput?: boolean
}
```

示例：`/help` 命令渲染一个帮助 UI 组件。

### 2.2 CommandBase（所有命令的公共字段）

```typescript
type CommandBase = {
  name: string           // 命令名（如 'compact'，不含前缀 /）
  description: string    // 用户可见的描述
  source: 'builtin' | 'custom' | 'sdk'  // 来源类型
  isEnabled?: (context: LocalJSXCommandContext) => boolean  // 动态启用判断
  aliases?: string[]     // 别名（如 'c' 是 'clear' 的别名）
  argSpec?: ArgSpec      // 参数规格（用于自动补全）
  hidden?: boolean       // 是否在帮助列表中隐藏
  userFacingName?: string  // 用户可见名称（可能与 name 不同）
}
```

---

## 3. 命令注册表 (`commands.ts`)

### 3.1 命令分类

**通用命令（所有用户可见）**：
```
/add-dir      /agents       /branch       /clear        /color
/compact      /config       /context      /copy         /cost
/diff         /doctor       /effort       /fast         /files
/help         /hooks        /ide          /init         /keybindings
/mcp          /memory       /model        /permissions  /plan
/plugin       /pr_comments  /release-notes /rename      /resume
/review       /session      /skills       /status       /tasks
/theme        /vim          /usage        /upgrade      /version
```

**内部命令（ant-only，生产用户不可见）**：
```
/backfill-sessions  /break-cache  /bughunter   /commit      /commit-push-pr
/ctx_viz            /good-claude  /init-verifiers /issue    /mock-limits
/onboarding         /share        /summary      /teleport   /ant-trace
/perf-issue         /env          /debug-tool-call
```

**Feature-gated 命令**：

| 命令 | Feature Flag |
|-----|-------------|
| `/proactive` | `PROACTIVE` / `KAIROS` |
| `/brief` | `KAIROS` / `KAIROS_BRIEF` |
| `/assistant` | `KAIROS` |
| `/bridge` | `BRIDGE_MODE` |
| `/remote-control-server` | `DAEMON` + `BRIDGE_MODE` |
| `/voice` | `VOICE_MODE` |
| `/force-snip` | `HISTORY_SNIP` |
| `/workflows` | `WORKFLOW_SCRIPTS` |
| `/fork` | `FORK_SUBAGENT` |

### 3.2 动态命令来源

除内置命令外，`getCommands()` 还合并四类动态命令：

```typescript
export async function getCommands(context?: ...): Promise<Command[]> {
  // 1. 内置命令（COMMANDS 数组）
  const builtin = COMMANDS()
  
  // 2. 技能目录命令（.claude/commands/ 目录下的 .md 文件）
  const skillDirCmds = await getSkillDirCommands(getCwd())
  
  // 3. 内置打包技能（bundled skills，编译进二进制）
  const bundledSkills = getBundledSkills()
  
  // 4. 插件命令（~/.config/claude/plugins/ 目录）
  const pluginCmds = await getPluginCommands()
  
  return [...builtin, ...skillDirCmds, ...bundledSkills, ...pluginCmds]
}
```

---

## 4. 核心命令深度解读

### 4.1 `/compact` — 上下文压缩

```typescript
// commands/compact/index.ts
const compact: LocalCommand = {
  type: 'local',
  name: 'compact',
  description: '压缩当前对话历史，保留重要信息',
  async call(args, context) {
    const { messages, setMessages, ... } = context
    
    // 调用 LLM 对整个历史进行摘要
    const summary = await compactMessages(messages, ...)
    
    // 替换消息列表为摘要 + compact_boundary
    const newMessages = buildPostCompactMessages({ summary, ... })
    setMessages(newMessages)
    
    return { type: 'result', resultMarkdown: '✓ 历史已压缩' }
  }
}
```

### 4.2 `/memory` — 记忆管理

```typescript
// commands/memory/index.ts
const memory: LocalJSXCommand = {
  type: 'local',
  name: 'memory',
  description: '管理 Claude 的记忆文件（CLAUDE.md）',
  async call(args, context) {
    // 打开记忆文件编辑界面
    return {
      type: 'jsx',
      jsx: <MemoryManager context={context} />,
    }
  }
}
```

### 4.3 `/init` — 项目初始化

```typescript
// commands/init.ts（20KB）
// 在当前目录生成 CLAUDE.md 项目描述文件
// 使用 Claude 分析项目结构，自动填充：
// - 项目描述
// - 代码规范
// - 关键文件路径
// - 构建/测试命令
```

### 4.4 `/mcp` — MCP 管理

```typescript
// commands/mcp/index.ts
// 提供子命令：
// /mcp add <server>      — 添加 MCP 服务器
// /mcp remove <server>   — 删除 MCP 服务器
// /mcp list             — 列出所有 MCP 服务器
// /mcp get-prompt <server> <prompt>  — 获取 MCP 提示词
```

### 4.5 `/resume` — 会话恢复

```typescript
// commands/resume/index.ts
// 列出最近的会话，允许用户选择一个继续
// 读取 ~/.claude/sessions/*.jsonl 文件
// 恢复时重放历史消息到 QueryEngine
```

### 4.6 `/compact` 快捷触发

用户也可以在提示词中使用特殊格式触发 compact：
```
当前上下文太长时，Claude 会自动建议 /compact
```

---

## 5. 命令执行流程

```
用户输入 /compact [args]
    │
    ▼
utils/processUserInput/processUserInput.ts
    │
    ├── isSlashCommand('/compact')  → true
    ├── 查找命令：getCommandName(cmd) → 'compact'
    │     ├── 内置命令列表
    │     ├── 技能目录命令
    │     └── 插件命令
    │
    ├── isCommandEnabled(cmd, context) → true/false
    │
    ├── type === 'local' / 'local_jsx'
    │     └── cmd.call(args, context)
    │           → { type: 'result' | 'jsx' | ... }
    │
    └── type === 'prompt'
          └── cmd.getPromptForCommand(args, context)
                → 发送给 Claude 的提示词字符串
                → 进入正常 LLM 查询流程
```

---

## 6. 命令执行上下文（`LocalJSXCommandContext`）

```typescript
type LocalJSXCommandContext = {
  // 消息操作
  messages: Message[]
  setMessages: (msgs: Message[]) => void
  
  // UI 控制
  setToolJSX: SetToolJSXFn
  
  // 状态访问
  getAppState: () => AppState
  setAppState: (f: ...) => void
  
  // 工具/命令列表
  tools: Tools
  commands: Command[]
  
  // 会话信息
  sessionId: string
  cwd: string
  
  // 网络/服务
  mcpClients: MCPServerConnection[]
  
  // 用户认证
  apiKey?: string
}
```

---

## 7. 自定义命令（用户技能）

用户可以在 `.claude/commands/` 目录下创建 Markdown 文件定义自定义命令：

```markdown
<!-- .claude/commands/fix-tests.md -->
---
description: 修复所有失败的测试
---

请分析当前目录下所有失败的测试，理解失败原因并修复它们。
修复完成后，运行测试确认全部通过。
```

这种命令被解析为 `PromptCommand`，触发时会把 Markdown 内容作为提示词发给 Claude。

---

## 8. 命令参数自动补全（`argSpec`）

```typescript
// 命令可声明参数规格，支持 Tab 自动补全
const mcp: Command = {
  name: 'mcp',
  argSpec: {
    type: 'subcommand',
    subcommands: ['add', 'remove', 'list', 'get-prompt'],
  },
  // ...
}
```

---

## 9. 命令来源标记（`source`）

```typescript
type CommandSource = 'builtin' | 'custom' | 'sdk'
```

- `builtin`：内置命令（代码库中定义）
- `custom`：用户自定义命令（技能目录中的 `.md` 文件）
- `sdk`：SDK 调用者通过 API 注入的命令

---

## 10. 设计模式

### 10.1 策略模式（`type` 字段驱动）
通过 `type: 'local' | 'prompt'` 决定执行策略，`processUserInput` 根据类型分发。

### 10.2 延迟加载（`insights.ts`）
`/insights` 命令使用懒加载模式，只在真正调用时才导入 113KB 的模块：

```typescript
const usageReport: Command = {
  type: 'prompt',
  name: 'insights',
  async getPromptForCommand(args, context) {
    const real = (await import('./commands/insights.js')).default
    return real.getPromptForCommand(args, context)
  },
}
```

### 10.3 Memoize 缓存（`COMMANDS()` 函数）
命令注册表使用 `memoize` 缓存，避免每次调用重新构建：

```typescript
const COMMANDS = memoize((): Command[] => [...所有内置命令...])
```

---

## 11. 与其他模块关联

| 模块 | 关联方式 |
|------|---------|
| `utils/processUserInput/` | 解析 slash 命令，调用 `cmd.call()` 或 `getPromptForCommand()` |
| `hooks/useTypeahead.tsx` | 命令名自动补全（Tab 键） |
| `skills/loadSkillsDir.ts` | 提供技能目录命令 |
| `plugins/loadPluginCommands.ts` | 提供插件命令 |
| `QueryEngine.ts` | 将命令传入 `processUserInputContext` |
| `screens/REPL.tsx` | 通过 `useMergedCommands` hook 获取命令列表 |
