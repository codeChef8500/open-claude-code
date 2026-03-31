# Claude Code 源码深度解读 — 14 Memory 记忆系统

> 覆盖文件：`utils/memory/`、`services/extractMemories/`、`services/SessionMemory/`、`commands/memory/`、`utils/claudeMd.ts`、`utils/nestedMemory.ts`

---

## 1. 记忆系统架构

Claude Code 的记忆系统分为三个层次：

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 1：持久记忆文件（CLAUDE.md）                           │
│  存储位置：~/.claude/CLAUDE.md + 项目/.claude/CLAUDE.md      │
│  生命周期：永久（用户手动管理）                                 │
│  内容：用户偏好、项目规范、重要事实                             │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│  Layer 2：会话记忆提取（Session Memory）                      │
│  存储位置：~/.claude/memory/session/                         │
│  生命周期：跨会话（自动提取）                                   │
│  内容：AI 从对话中自动提取的关键事实                            │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────┐
│  Layer 3：上下文内记忆（In-Context）                          │
│  存储位置：当前会话消息列表                                     │
│  生命周期：当前会话（随上下文压缩而衰减）                        │
│  内容：本次对话的即时信息                                       │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. CLAUDE.md — 项目记忆文件

### 2.1 文件位置和优先级

```
优先级（从高到低）：
1. 当前工作目录 CLAUDE.md
2. 项目根目录 .claude/CLAUDE.md
3. 父目录 CLAUDE.md（递归向上，直到 ~/.claude/）
4. 用户全局 ~/.claude/CLAUDE.md
```

### 2.2 `utils/claudeMd.ts` — CLAUDE.md 读取

```typescript
// 读取所有适用的 CLAUDE.md 文件（按目录层级）
async function readClaudeMdFiles(cwd: string): Promise<ClaudeMdContent[]> {
  const paths = getClaudeMdPaths(cwd)  // 从 cwd 向上遍历到 ~/.claude/
  
  const contents: ClaudeMdContent[] = []
  
  for (const path of paths) {
    if (await exists(path)) {
      const content = await readFile(path, 'utf-8')
      contents.push({
        path,
        content,
        scope: getScopeForPath(path),  // 'global' | 'project' | 'local'
      })
    }
  }
  
  return contents
}
```

### 2.3 嵌套记忆（`utils/nestedMemory.ts`）

CLAUDE.md 支持通过 `@include` 语法引用其他文件：

```markdown
<!-- CLAUDE.md -->
# 项目规范

@include .claude/coding-standards.md
@include .claude/testing-guide.md

## 特别注意事项
- 始终使用 TypeScript 严格模式
```

```typescript
// 解析并展开 @include 指令
async function expandNestedMemory(
  content: string,
  basePath: string,
): Promise<string> {
  const includeRegex = /^@include\s+(.+)$/gm
  
  return content.replace(includeRegex, async (match, includePath) => {
    const fullPath = resolve(basePath, includePath)
    const included = await readFile(fullPath, 'utf-8')
    return expandNestedMemory(included, dirname(fullPath))  // 递归
  })
}
```

### 2.4 `/memory` 命令管理

```typescript
// /memory 命令提供 CRUD 操作
const memoryCommands = {
  // /memory add <text> — 向 CLAUDE.md 追加新条目
  add: async (text: string, context) => {
    const targetFile = getProjectClaudeMdPath(context.cwd)
    await appendToClaudeMd(targetFile, text)
    return `✓ 已添加到 ${targetFile}`
  },
  
  // /memory show — 显示当前所有记忆
  show: async (_, context) => {
    const memories = await readClaudeMdFiles(context.cwd)
    return memories.map(m => `# ${m.path}\n${m.content}`).join('\n\n')
  },
  
  // /memory edit — 打开编辑器
  edit: async (_, context) => {
    await openInEditor(getProjectClaudeMdPath(context.cwd))
  },
}
```

---

## 3. 会话记忆提取（`services/extractMemories/`）

AI 可以主动从对话中提取重要信息，保存为可复用的记忆：

### 3.1 触发条件

```typescript
// 记忆提取在以下情况触发：
// 1. 用户明确要求（/memory add）
// 2. 会话结束时自动提取（CLAUDE_CODE_AUTO_MEMORY=1）
// 3. 发现新的项目规范时
// 4. 用户纠正 AI 的错误理解后
```

### 3.2 提取流程

```typescript
// services/extractMemories/extractMemories.ts
async function extractMemoriesFromConversation(
  messages: Message[],
): Promise<ExtractedMemory[]> {
  // 构建提取提示词
  const extractionPrompt = buildExtractionPrompt(messages)
  
  // 调用 LLM 进行结构化提取
  const result = await callClaude({
    model: 'claude-haiku-4',  // 使用轻量模型
    messages: [{ role: 'user', content: extractionPrompt }],
    systemPrompt: MEMORY_EXTRACTION_SYSTEM_PROMPT,
    maxTokens: 1000,
  })
  
  // 解析提取结果
  return parseExtractedMemories(result)
}

// 提取的记忆类型
type ExtractedMemory = {
  category: 'preference' | 'fact' | 'decision' | 'rule' | 'correction'
  content: string
  confidence: number  // 0-1，高置信度才持久化
  scope: 'global' | 'project'
}
```

---

## 4. 记忆在系统提示中的注入

### 4.1 `utils/queryContext.ts` — 系统提示构建

```typescript
// 每次查询前，将记忆注入到系统提示
async function buildSystemPromptParts(
  context: QueryContext,
): Promise<SystemPromptPart[]> {
  const parts: SystemPromptPart[] = []
  
  // 1. 基础系统提示（硬编码行为规范）
  parts.push({ type: 'base', content: getBaseSystemPrompt() })
  
  // 2. 全局记忆（~/.claude/CLAUDE.md）
  const globalMemory = await readGlobalClaudeMd()
  if (globalMemory) {
    parts.push({
      type: 'memory',
      scope: 'global',
      content: globalMemory,
      header: '# 全局偏好和规则',
    })
  }
  
  // 3. 项目记忆（./CLAUDE.md + 嵌套引用）
  const projectMemory = await readProjectClaudeMd(context.cwd)
  if (projectMemory) {
    parts.push({
      type: 'memory',
      scope: 'project',
      content: await expandNestedMemory(projectMemory, context.cwd),
      header: '# 项目规范',
    })
  }
  
  // 4. 会话记忆（从上次会话提取的关键事实）
  const sessionMemories = await loadRelevantSessionMemories(context.query)
  if (sessionMemories.length > 0) {
    parts.push({
      type: 'session_memory',
      content: formatSessionMemories(sessionMemories),
      header: '# 重要背景信息',
    })
  }
  
  // 5. 用户环境上下文
  parts.push({
    type: 'environment',
    content: buildEnvironmentContext(context),
  })
  
  return parts
}
```

### 4.2 记忆预取（启动优化）

```typescript
// QueryEngine.submitMessage() 中，记忆加载是并行执行的
const [systemPromptParts, userMessage] = await Promise.all([
  fetchSystemPromptParts(context),  // 包含记忆加载
  processUserInput(prompt),
])
```

---

## 5. 嵌套记忆目录（`utils/nestedMemory.ts`）

Claude Code 支持**结构化的记忆目录**：

```
.claude/
├── CLAUDE.md                  # 主记忆文件
├── memory/
│   ├── coding-standards.md    # 编码规范
│   ├── architecture.md        # 架构决策
│   ├── testing-guide.md       # 测试指南
│   └── team-conventions.md    # 团队约定
└── commands/
    └── fix-tests.md           # 自定义命令
```

`QueryEngine` 中记录已加载的嵌套记忆路径（防止重复加载）：

```typescript
// QueryEngine.ts
private loadedNestedMemoryPaths = new Set<string>()

// 每次 submitMessage 时检查是否有新的记忆目录
async function checkForNewNestedMemories(cwd: string): Promise<void> {
  const memoryPaths = await discoverMemoryFiles(cwd)
  
  for (const path of memoryPaths) {
    if (!this.loadedNestedMemoryPaths.has(path)) {
      this.loadedNestedMemoryPaths.add(path)
      // 将新发现的记忆注入到下一次对话
      await addMemoryToContext(path)
    }
  }
}
```

---

## 6. 技能激活的条件记忆

某些技能（Skills）在特定文件被修改时自动激活，将相关记忆注入：

```typescript
// skills/loadSkillsDir.ts
// 条件激活：当 *.test.ts 文件被编辑时，激活测试技能
async function activateConditionalSkillsForPaths(
  editedPaths: string[],
): Promise<void> {
  const skills = await loadConditionalSkills()
  
  for (const skill of skills) {
    if (skill.activatePattern && pathsMatchPattern(editedPaths, skill.activatePattern)) {
      // 将技能的记忆内容注入到上下文
      await injectSkillMemory(skill)
    }
  }
}
```

---

## 7. Team Memory Sync（`services/teamMemorySync/`）

团队模式下，多个 Claude Code 实例共享同一个记忆库：

```typescript
// 团队记忆同步（通过共享存储如 S3 或 Git）
async function syncTeamMemory(): Promise<void> {
  // 1. 从共享存储下载最新记忆
  const remoteMemory = await pullTeamMemory()
  
  // 2. 与本地记忆合并（以远程为主）
  const merged = mergeMemories(localMemory, remoteMemory)
  
  // 3. 推送本地新增的记忆
  await pushTeamMemory(merged)
}
```

---

## 8. 记忆相关性排序

`services/SessionMemory/` 实现了基于语义的记忆检索：

```typescript
// 根据当前对话主题，返回最相关的记忆
async function loadRelevantSessionMemories(
  query: string,
): Promise<SessionMemory[]> {
  const allMemories = await loadAllSessionMemories()
  
  // 关键词匹配（简单版）
  const keywordMatches = allMemories.filter(m =>
    hasKeywordOverlap(m.content, query)
  )
  
  // 时间衰减：最近的记忆排在前面
  const sorted = keywordMatches.sort((a, b) => b.timestamp - a.timestamp)
  
  // 限制数量避免注入过多 token
  return sorted.slice(0, MAX_SESSION_MEMORIES)
}
```

---

## 9. 记忆系统数据流

```
FileEditTool 修改文件
    │
    ▼
discoverSkillDirsForPaths() → 检测新技能目录
    │
    ▼
activateConditionalSkillsForPaths() → 激活相关技能的记忆
    │
    ▼
记忆注入到 SystemPromptContext
    │
    ▼
下次 queryLoop 中，记忆出现在系统提示的 "# 项目规范" 部分

──────────────────────────────────────────────────────────

/memory add "使用 jest 而非 vitest" 命令
    │
    ▼
appendToClaudeMd(targetFile, text)
    │
    ▼
~/.claude/CLAUDE.md 或 .claude/CLAUDE.md 被更新
    │
    ▼
下次 buildSystemPromptParts() 时读取新内容
    │
    ▼
Claude 在后续所有对话中都记住此偏好
```

---

## 10. 设计亮点

### 10.1 层次化记忆（全局 → 项目 → 会话）
不同作用域的记忆按优先级叠加，项目级规范覆盖全局偏好，实现精细控制。

### 10.2 @include 实现模块化
CLAUDE.md 支持 `@include` 将记忆内容模块化管理，大型项目可以分文件组织不同主题的规范。

### 10.3 条件技能激活
文件编辑时触发相关技能激活，实现"上下文感知"记忆注入，避免无关记忆污染系统提示。

### 10.4 惰性记忆提取
会话记忆不是实时提取，而是在会话结束后批量处理，避免影响对话延迟。
