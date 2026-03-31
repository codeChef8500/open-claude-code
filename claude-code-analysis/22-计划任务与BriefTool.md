# Claude Code 源码深度解读 — 22 计划任务与 BriefTool

> 覆盖文件：`tools/ScheduleCronTool/`（CronCreateTool.ts、CronDeleteTool.ts、CronListTool.ts、prompt.ts）、`utils/cronTasks.ts`、`utils/cronScheduler.ts`、`utils/cronJitterConfig.ts`、`hooks/useScheduledTasks.ts`、`tools/BriefTool/`（BriefTool.ts、UI.tsx 14KB、attachments.ts、prompt.ts、upload.ts）

---

## 1. ScheduleCronTool — 定时任务系统

### 1.1 模块职责

ScheduleCronTool 允许 Claude 创建定时/周期性任务，这些任务会在指定时间自动向 Claude 注入提示词并执行。适用场景：

- **定期提醒**："每天上午9点提醒我检查 CI"
- **周期监控**："每15分钟检查一次测试是否通过"
- **一次性任务**："在明天下午2点提醒我"

此工具仅在 `isKairosCronEnabled()`（KAIROS 特性下）时可用。

---

### 1.2 三个工具子命令

| 工具名 | 作用 |
|-------|------|
| `CronCreateTool` | 创建新定时任务 |
| `CronDeleteTool` | 删除指定任务 |
| `CronListTool` | 列出所有当前任务 |

---

### 1.3 `CronCreateTool` — 任务创建

**输入 Schema：**

```typescript
z.strictObject({
  cron: z.string()
    // 标准 5 字段 cron 表达式，本地时间
    // "*/5 * * * *" = 每5分钟
    // "30 14 28 2 *" = 2月28日下午2:30（一次）

  prompt: z.string()
    // 触发时注入的提示词

  recurring: z.boolean().optional()
    // true（默认）= 持续循环直到删除或90天后过期
    // false = 只触发一次后自动删除

  durable: z.boolean().optional()
    // true = 持久化到 .claude/scheduled_tasks.json（重启后存活）
    // false（默认）= 内存中，Claude 退出即消失
})
```

**执行逻辑：**

```typescript
async call({ cron, prompt, recurring = true, durable = false }) {
  // Kill switch 覆盖（即使用户请求 durable，也可被 GB gate 强制关闭）
  const effectiveDurable = durable && isDurableCronEnabled()

  const id = await addCronTask(cron, prompt, recurring, effectiveDurable,
    getTeammateContext()?.agentId,  // 队友 Agent 也可创建 cron
  )

  // 启动调度器轮询（下一个 React tick 生效）
  setScheduledTasksEnabled(true)

  return { id, humanSchedule: cronToHuman(cron), recurring, durable: effectiveDurable }
}
```

**返回给 API 的 tool_result 消息示例：**
```
Scheduled recurring job j_abc123 (every 5 minutes).
Persisted to .claude/scheduled_tasks.json. Auto-expires after 90 days.
Use CronDelete to cancel sooner.
```

---

### 1.4 任务存储（`utils/cronTasks.ts`）

```typescript
type CronTask = {
  id: string               // 唯一任务 ID（j_xxxxx）
  cron: string             // cron 表达式
  prompt: string           // 触发提示词
  recurring: boolean
  durable: boolean
  createdAt: number        // Unix ms 时间戳
  agentId?: string         // 关联的队友 Agent ID（可选）
  lastRunAt?: number
  nextRunAt?: number
}

// 双存储：内存（session-only）+ 磁盘（durable）
const SESSION_TASKS: Map<string, CronTask> = new Map()
const DURABLE_TASKS_PATH = '.claude/scheduled_tasks.json'

// 合并两个存储的任务列表
export async function listAllCronTasks(): Promise<CronTask[]> {
  const durableTasks = await loadDurableTasks()
  return [...SESSION_TASKS.values(), ...durableTasks]
}
```

---

### 1.5 调度器（`utils/cronScheduler.ts` + `hooks/useScheduledTasks.ts`）

```typescript
// React Hook — 在 REPL.tsx 中注册，主线程轮询
function useScheduledTasks({ onTaskFire, ... }) {
  useEffect(() => {
    if (!isScheduledTasksEnabled()) return

    // 每分钟检查一次是否有任务到期
    const interval = setInterval(async () => {
      const tasks = await listAllCronTasks()
      const now = Date.now()

      for (const task of tasks) {
        if (shouldFireNow(task.cron, now)) {
          onTaskFire(task)          // 注入 task.prompt 到对话
          await markTaskFired(task) // 更新 lastRunAt
          if (!task.recurring) {
            await deleteCronTask(task.id)  // 一次性任务自动删除
          }
        }
      }
    }, CHECK_INTERVAL_MS)  // 每60秒检查

    return () => clearInterval(interval)
  }, [])
}
```

### 1.6 抖动配置（`utils/cronJitterConfig.ts`）

防止多个 KAIROS 实例同时触发造成 API 洪峰：

```typescript
// 每个任务在触发时随机延迟 0~jitterMs 毫秒
// jitterMs 通过 GrowthBook 配置，默认 30_000（30 秒）
export function getCronJitterMs(): number {
  return getFeatureValue_CACHED_MAY_BE_STALE('tengu_cron_jitter_ms', 30_000)
}
```

---

### 1.7 队友限制

`durable: true` 对队友 Agent 不可用，因为队友在会话结束后不持久存在：

```typescript
if (input.durable && getTeammateContext()) {
  return {
    result: false,
    message: 'durable crons are not supported for teammates (teammates do not persist across sessions)',
  }
}
```

---

## 2. BriefTool（SendUserMessage）

### 2.1 模块职责

`BriefTool`（工具名：`SendUserMessage`）是 KAIROS/Brief 模式专用的"主动发消息"工具。正常情况下 Claude 只能通过 `assistant` 消息角色回复，而 BriefTool 允许 Claude 在**工具执行过程中**主动推送消息给用户，如进度汇报、阶段性结果、发现的问题等。

---

### 2.2 激活条件（双层门控）

```
isBriefEntitled()         ← 权利检查（是否有资格使用）
    ├── getKairosActive()              → KAIROS 守护进程模式
    ├── process.env.CLAUDE_CODE_BRIEF  → 开发/测试环境变量
    └── GrowthBook 'tengu_kairos_brief' gate → 普通用户 rollout
          └── 每5分钟刷新一次（kill-switch 场景）

isBriefEnabled()          ← 激活检查（是否已 opt-in）
    ├── getKairosActive()  → KAIROS 模式下无需 opt-in，直接激活
    └── getUserMsgOptIn()  → 用户已通过以下途径 opt-in：
            ├── --brief CLI 标志
            ├── settings.json defaultView: 'chat'
            ├── /brief slash 命令
            ├── /config 界面选择
            └── --tools SendUserMessage（SDK 工具列表明确包含）
```

---

### 2.3 输入 Schema

```typescript
z.strictObject({
  message: z.string()
    // 支持 Markdown 格式的消息内容

  attachments: z.array(z.string()).optional()
    // 可附带文件路径（图片、截图、diff、日志等）

  status: z.enum(['normal', 'proactive'])
    // 'normal'    → 回复用户刚发的消息
    // 'proactive' → 主动汇报（用户不在线/未请求时触发的更新）
})
```

---

### 2.4 附件系统（`attachments.ts` + `upload.ts`）

BriefTool 支持将文件附件随消息一起发送：

```typescript
// attachments.ts — 验证和解析附件路径
async function validateAttachmentPaths(paths: string[]): Promise<ValidationResult> {
  for (const p of paths) {
    const fullPath = resolve(getCwd(), p)
    if (!await exists(fullPath)) {
      return { result: false, message: `附件文件不存在: ${p}` }
    }
    if (await isDirectory(fullPath)) {
      return { result: false, message: `附件不能是目录: ${p}` }
    }
  }
  return { result: true }
}

// upload.ts — 通过 Bridge 将本地文件上传到 Claude.ai
// 返回 file_uuid，供前端渲染使用
async function uploadAttachmentViaBridge(
  filePath: string,
  bridgeEnabled: boolean,
): Promise<UploadedFile> {
  if (!bridgeEnabled) {
    // 非 Bridge 模式：返回本地文件元数据（不上传）
    return { path: filePath, size: stat.size, isImage: isImageFile(filePath) }
  }
  // Bridge 模式：通过 Bridge WebSocket 上传文件
  const uuid = await uploadFileViaBridge(filePath)
  return { path: filePath, size: stat.size, isImage: isImageFile(filePath), file_uuid: uuid }
}
```

---

### 2.5 `call()` 执行流程

```typescript
async call({ message, attachments, status }, context) {
  const sentAt = new Date().toISOString()

  logEvent('tengu_brief_send', {
    proactive: status === 'proactive',
    attachment_count: attachments?.length ?? 0,
  })

  // 无附件：直接返回
  if (!attachments || attachments.length === 0) {
    return { data: { message, sentAt } }
  }

  // 有附件：解析附件元数据（可能上传）
  const appState = context.getAppState()
  const resolved = await resolveAttachments(attachments, {
    replBridgeEnabled: appState.replBridgeEnabled,
    signal: context.abortController.signal,
  })

  return { data: { message, attachments: resolved, sentAt } }
}
```

`tool_result` 始终返回：`"Message delivered to user."` 或 `"Message delivered to user. (2 attachments included)"`，让 LLM 知道消息已送达。

---

### 2.6 UI 渲染（`UI.tsx` 14KB）

```tsx
// 工具调用展示：显示消息和附件预览
function renderToolUseMessage({ message, attachments, status }: Output) {
  return (
    <Box flexDirection="column">
      {/* 状态标签（proactive 显示特殊图标）*/}
      {status === 'proactive' && <Text color="blue">◉ Proactive</Text>}

      {/* 消息内容（Markdown 渲染）*/}
      <MarkdownRenderer text={message} />

      {/* 附件列表 */}
      {attachments?.map(att => (
        att.isImage
          ? <ImagePreview key={att.path} uuid={att.file_uuid} path={att.path} />
          : <FileLink key={att.path} path={att.path} size={att.size} />
      ))}
    </Box>
  )
}
```

---

### 2.7 BriefTool 与常规工具的关键区别

| 特性 | 普通工具 | BriefTool |
|------|---------|---------|
| 权限要求 | 标准权限 | 无（isReadOnly: true）|
| 并发安全 | 视工具而定 | 始终安全（isConcurrencySafe: true）|
| 用户可见性 | 通过 tool_result | 直接渲染为消息气泡 |
| 用途 | 执行任务 | 向用户推送通知 |
| 结果回传 LLM | 完整输出 | 仅 "Message delivered" |
| 支持附件 | 视工具 | 支持图片/文件 |

---

## 3. 定时任务 + BriefTool 的联动场景

```
用户："每小时检查一次 CI 状态，失败时通知我"
    │
    ▼
Claude 调用 CronCreateTool:
  cron: "0 * * * *"  (每小时整点)
  prompt: "检查 CI 状态。如果有失败的构建，用 SendUserMessage 通知用户"
  recurring: true
  durable: true
    │
    ▼
调度器在整点触发 → 注入 prompt 到新对话
    │
    ▼
Claude 执行 BashTool("gh run list --limit 5")
    │
    ├── 构建通过 → 调用 SleepTool 等待下一次 tick
    └── 构建失败 → 调用 SendUserMessage:
          message: "⚠️ CI 失败！workflow 'build' 在 main 分支上失败"
          attachments: ["./build-log.txt"]
          status: "proactive"
              │
              ▼
        用户在 Claude.ai 收到通知消息 + 日志附件
```

---

## 4. 设计亮点

### 4.1 双存储（内存 + 磁盘）
Session-only 任务不写磁盘（零副作用），Durable 任务持久化到 `.claude/scheduled_tasks.json`，重启后自动恢复。

### 4.2 Kill Switch 强制降级
GrowthBook 可以随时将 `isDurableCronEnabled()` 切换为 false，所有新任务自动降级为 session-only，即使用户请求了 `durable: true`。Schema 保持不变，模型不会看到验证错误。

### 4.3 BriefTool 结果精简
`mapToolResultToToolResultBlockParam` 只返回 "Message delivered" 而非完整消息内容，避免 LLM 上下文中出现重复内容（消息已在 UI 渲染，不需要再次填入对话历史）。

### 4.4 `sentAt` 字段的向后兼容
BriefTool 的 `sentAt` 字段是可选的，恢复旧会话时可以重放不含该字段的工具输出，不会导致 UI 渲染崩溃。
