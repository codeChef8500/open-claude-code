# Claude Code 源码深度解读 — 21 Undercover 卧底模式与 Auto Mode

> 覆盖文件：`utils/undercover.ts`、`utils/permissions/yoloClassifier.ts`（63KB）、`cli/handlers/autoMode.ts`、`utils/fastMode.ts`、`utils/permissions/autoModeState.ts`、`utils/permissions/bypassPermissionsKillswitch.ts`

---

## 1. 卧底模式（Undercover Mode）

### 1.1 什么是卧底模式

**Undercover Mode** 是 Anthropic 内部（ant-only）的安全特性，专为 Anthropic 员工在**公开/开源仓库**工作时设计。当激活时，Claude Code 会：

- 屏蔽所有内部信息出现在 commit 消息、PR 标题和 PR 描述中
- 不告知模型自己是哪个版本/代号
- 移除所有 Git 提交中的 AI 归因信息（`Co-Authored-By` 等）

> ⚠️ 此模块在外部构建中被 **完全 DCE 消除**（`USER_TYPE` 是构建时 `--define`），外部用户不受此影响。

### 1.2 自动激活逻辑

```typescript
// utils/undercover.ts
export function isUndercover(): boolean {
  if (process.env.USER_TYPE === 'ant') {
    // 强制开启
    if (isEnvTruthy(process.env.CLAUDE_CODE_UNDERCOVER)) return true

    // 自动检测：未明确确认为内部仓库 → 开启
    // 'external'、'none' 或 null（尚未检测）→ ON
    // 'internal'（仅内部 allowlist 仓库）→ OFF
    return getRepoClassCached() !== 'internal'
  }
  return false  // 外部构建：永远返回 false
}
```

**设计原则：安全默认 = 开启**
- 没有"强制关闭"机制
- 即使工作目录不是 git 仓库（如 `/tmp` 崩溃复现），也保持 ON
- 只有明确确认是内部 allowlist 仓库才关闭

### 1.3 仓库分类（`utils/commitAttribution.ts`）

```typescript
type RepoClass = 'internal' | 'external' | 'none'

// 从 git remote URL 分类仓库
function classifyRepo(remoteUrl: string): RepoClass {
  // INTERNAL_MODEL_REPOS 是内部 allowlist（不公开）
  if (INTERNAL_MODEL_REPOS.some(r => remoteUrl.includes(r))) {
    return 'internal'
  }
  if (remoteUrl.length > 0) return 'external'
  return 'none'  // 非 git 仓库或无 remote
}
```

### 1.4 卧底模式指令（注入系统提示）

```typescript
export function getUndercoverInstructions(): string {
  // 注入到 BashTool / commit / PR 命令的系统提示段
  return `## UNDERCOVER MODE — CRITICAL

  你正在公开/开源仓库中 UNDERCOVER 工作。你的 commit 消息、
  PR 标题和 PR 正文绝对不能包含任何 Anthropic 内部信息。

  永远不要包含：
  - 内部模型代号（动物名称：Capybara、Tengu 等）
  - 未发布的模型版本号（如 opus-4-7、sonnet-4-8）
  - 内部仓库或项目名（如 claude-cli-internal）
  - 内部工具、Slack 频道或短链（如 go/cc）
  - "Claude Code" 短语或任何 AI 身份暗示
  - Co-Authored-By 行或任何其他归因

  像人类开发者一样写 commit 消息 — 只描述代码变更本身。`
}
```

### 1.5 一次性提示对话框

```typescript
// 自动检测到卧底模式时，显示一次性提示（不要重复烦扰）
export function shouldShowUndercoverAutoNotice(): boolean {
  if (!isUndercover()) return false
  if (isEnvTruthy(process.env.CLAUDE_CODE_UNDERCOVER)) return false  // 强制开启，用户已知
  if (getGlobalConfig().hasSeenUndercoverAutoNotice) return false     // 已看过
  return true
}
```

---

## 2. Auto Mode（自动权限分类器）

### 2.1 什么是 Auto Mode

Auto Mode 是 `TRANSCRIPT_CLASSIFIER` 特性门控的**基于 LLM 的工具调用自动审批系统**。它用一个轻量级 AI 分类器实时判断每个工具调用是否应该自动批准，代替用户手动逐一确认。

```
工具调用触发
    │
    ├── [alwaysAllow 规则匹配] → 直接允许（无分类器）
    ├── [alwaysDeny 规则匹配]  → 直接拒绝
    └── [Auto Mode 开启] → 调用 YoloClassifier
            │
            ├── 'allow'     → 自动批准
            ├── 'soft_deny' → 阻止但不报错（提示模型换方式）
            └── null        → 回退到用户确认弹框
```

### 2.2 Auto Mode 规则系统（`yoloClassifier.ts`）

规则分三类，存储在 `settings.json` 的 `autoMode` 字段：

```typescript
type AutoModeRules = {
  allow:      string[]   // 自动批准的描述
  soft_deny:  string[]   // 自动阻止的描述
  environment: string[]  // 给分类器的环境上下文
}

// 默认外部规则示例（permissions_external.txt）
const defaults = {
  allow: [
    "Reading files in the project directory",
    "Running tests via npm/yarn/bun test",
    "Git operations: status, log, diff",
    // ...
  ],
  soft_deny: [
    "Deleting files outside the project directory",
    "Installing global packages",
    "Accessing credentials or secrets",
    // ...
  ],
  environment: [
    "This is a software development context",
    // ...
  ],
}
```

### 2.3 分类器调用流程

```typescript
// yoloClassifier.ts 中的核心函数
async function runYoloClassifier(
  toolCall: { tool: string; input: unknown },
  conversationContext: Message[],
  permissionContext: ToolPermissionContext,
): Promise<YoloClassifierResult> {

  // 1. 构建分类器系统提示（模板 + 用户自定义规则）
  const systemPrompt = buildYoloSystemPrompt(
    getAutoModeConfig(),   // 用户自定义规则（替换对应 section）
    getDefaultRules(),     // 默认规则（用户未覆盖的 section）
  )

  // 2. 侧路查询（sideQuery）— 不影响主对话的独立 API 调用
  const result = await sideQuery({
    messages: buildClassifierMessages(toolCall, conversationContext),
    systemPrompt,
    model: resolveAntModel('claude-haiku-4'),  // 使用轻量模型
    maxOutputTokens: 100,  // 分类器只需要短回答
    signal: abortController.signal,
  })

  // 3. 解析结构化输出
  return parseClassifierResponse(result)  // 返回 'allow' | 'soft_deny' | null
}
```

### 2.4 Prompt Cache 优化

```typescript
// 分类器系统提示不会改变（规则固定）→ 开启 prompt cache
const systemPromptWithCache: Anthropic.TextBlockParam = {
  type: 'text',
  text: systemPrompt,
  cache_control: getCacheControl('ephemeral'),  // 短期缓存
}

// 对话上下文截断（防止分类器输入超过限制）
const truncatedContext = tokenCountWithEstimation(context) > MAX_CLASSIFIER_CONTEXT_TOKENS
  ? context.slice(-CLASSIFIER_CONTEXT_TURNS)
  : context
```

### 2.5 `claude auto-mode` 子命令

```bash
# 查看当前有效规则（用户自定义 + 默认合并）
claude auto-mode config

# 查看纯默认规则（不含用户覆盖）
claude auto-mode defaults

# 让 Claude 评审你的自定义规则
claude auto-mode critique --file my-rules.json
```

```typescript
// cli/handlers/autoMode.ts 实现

// defaults：直接输出 getDefaultExternalAutoModeRules()
export function autoModeDefaultsHandler(): void {
  writeRules(getDefaultExternalAutoModeRules())
}

// config：合并（用户规则优先，section 级别替换）
export function autoModeConfigHandler(): void {
  const config = getAutoModeConfig()
  const defaults = getDefaultExternalAutoModeRules()
  writeRules({
    allow:       config?.allow?.length       ? config.allow       : defaults.allow,
    soft_deny:   config?.soft_deny?.length   ? config.soft_deny   : defaults.soft_deny,
    environment: config?.environment?.length ? config.environment : defaults.environment,
  })
}

// critique：调用 LLM 对用户规则进行评审
export async function autoModeCritiqueHandler(rulesFile: string): Promise<void> {
  const rules = JSON.parse(await readFile(rulesFile, 'utf-8'))
  const critique = await sideQuery({
    messages: [{ role: 'user', content: jsonStringify(rules) }],
    systemPrompt: CRITIQUE_SYSTEM_PROMPT,
    model: getMainLoopModel(),
  })
  process.stdout.write(critique + '\n')
}
```

### 2.6 `autoModeState.ts` — 状态追踪

```typescript
// 追踪当前 Auto Mode 会话中的分类器决策历史
// 用于遥测和调试

type AutoModeState = {
  totalClassifierCalls: number
  allowedCount: number
  deniedCount: number
  nullCount: number        // 分类器返回 null（回退到用户确认）
  classifierErrors: number
}
```

---

## 3. Fast Mode（快速模式）

### 3.1 什么是 Fast Mode

Fast Mode 是 Claude Code 的付费加速特性，使用更快的模型响应。

```typescript
// utils/fastMode.ts
export function isFastModeAvailable(): boolean {
  if (!isFastModeEnabled()) return false  // CLAUDE_CODE_DISABLE_FAST_MODE=1 禁用
  return getFastModeUnavailableReason() === null
}

type FastModeDisabledReason =
  | 'free'               // 免费账户，需付费订阅
  | 'preference'         // 组织策略禁用
  | 'extra_usage_disabled' // OAuth 用户未启用额外用量计费
  | 'network_error'      // 网络连接问题
  | 'unknown'            // 其他未知原因
```

### 3.2 不可用原因检测

```typescript
export function getFastModeUnavailableReason(): string | null {
  // 1. GrowthBook killswitch（'tengu_penguins_off' 非 null → 返回原因字符串）
  const statsigReason = getFeatureValue_CACHED_MAY_BE_STALE('tengu_penguins_off', null)
  if (statsigReason !== null) return statsigReason

  // 2. bundled 模式检查（老版本限制，现已可选）
  if (!isInBundledMode() && getFeatureValue_CACHED_MAY_BE_STALE('tengu_marble_sandcastle', false)) {
    return 'Fast mode requires the native binary'
  }

  // 3. Agent SDK 非交互模式限制（KAIROS 豁免）
  if (getIsNonInteractiveSession() && preferThirdPartyAuthentication() && !getKairosActive()) {
    const flagFastMode = getSettingsForSource('flagSettings')?.fastMode
    if (!flagFastMode) return 'Fast mode is not available in the Agent SDK'
  }

  // 4. 账户级别检查（通过 API 查询）
  const accountReason = getFastModeAccountDisabledReason()
  if (accountReason) return accountReason

  return null  // 可用
}
```

### 3.3 三者关系总结

```
权限处理层
    │
    ├── [规则层]    alwaysAllow / alwaysDeny → 秒决
    ├── [Auto Mode] YoloClassifier → AI 自动审批（allow/soft_deny）
    └── [交互层]    PermissionRequest UI → 用户手动确认
    
模型速度层
    └── Fast Mode → 使用更快响应模型（与权限无关）

安全层（ant-only）
    └── Undercover Mode → 屏蔽内部信息泄露
```

---

## 4. BypassPermissions Killswitch（`bypassPermissionsKillswitch.ts`）

Auto Mode 的安全阀，可以在紧急情况下关闭所有自动权限绕过：

```typescript
// 检查 bypassPermissions 是否被 killswitch 阻止
export function isBypassPermissionsKillswitched(): boolean {
  // GrowthBook killswitch — 运行时可动态关闭
  return getFeatureValue_CACHED_MAY_BE_STALE(
    'tengu_bypass_permissions_killswitch',
    false,
  )
}

// 在 Auto Mode 判断前检查
if (isBypassPermissionsKillswitched()) {
  // 强制要求用户手动确认所有权限，不管任何规则
  return requestUserConfirmation(toolCall)
}
```
