# Claude Code CLI 交互样式 1:1 复制执行方案

> **目标**: 将 `claude-code-main` 的 CLI 模式交互样式完整、精确地复制到 `agent-engine` 的 Bubbletea TUI 中，达到视觉和交互上的 1:1 一致性。

---

## 一、现状差距分析

### 1.1 已完成的基础工作（agent-engine 已有）

| 模块 | 文件 | 状态 |
|------|------|------|
| 主题色彩系统 | `tui/themes/` (theme.go, definitions.go, styles.go) | ✅ 6套主题已定义，RGB/ANSI颜色值已从claude-code-main移植 |
| 颜色解析引擎 | `tui/color/color.go` | ✅ 支持 rgb(), #hex, ansi:, ansi256() 格式 |
| Clawd 吉祥物 | `tui/logo/clawd.go` | ✅ 4种姿势(default/look-left/look-right/arms-up)已实现 |
| 欢迎Banner | `tui/logo/banner.go` | ✅ 精简版和完整版Banner已实现 |
| Unicode图标 | `tui/figures/figures.go` | ✅ 所有关键图标已定义 |
| 消息类型系统 | `tui/message/types.go` | ✅ 完整的消息类型和内容块定义 |
| 消息分组 | `tui/message/grouping.go` | ✅ 工具调用分组逻辑已实现 |
| 消息渲染 | `tui/message/row.go` | ⚠️ 基础渲染已有，但样式不匹配claude-code-main |
| 状态栏 | `tui/statusline/statusline.go` | ⚠️ 基础实现已有，但缺少shimmer动画和精确布局 |
| 权限对话框 | `tui/permissionui/dialog.go` | ⚠️ 基础实现已有，但样式不匹配 |
| 输入框 | `tui/input/promptinput.go` | ⚠️ 基础实现已有，但缺少claude-code-main的边框样式 |
| Spinner | `tui/spinner.go` | ❌ 使用bubbles/spinner，不是claude-code-main的自定义动画 |

### 1.2 关键差距清单

1. **Spinner动画**: 当前使用 `bubbles/spinner.Dot`，claude-code-main 使用自定义帧序列 `·✢✳✶✻✽` + shimmer色彩渐变 + stalled检测(红色警告)
2. **消息渲染格式**: 当前使用硬编码ANSI色值和简单前缀，需要切换到主题系统并精确匹配 `●`/`⎿` 缩进格式
3. **输入框边框**: 当前使用bubbles/textarea默认样式，claude-code-main使用 `round` 边框 + 主题色 `promptBorder` + 只有顶部边框
4. **权限对话框**: 当前使用完整四边框，claude-code-main使用只有顶部的 `round` 边框 + `permission` 颜色
5. **Spinner verbs**: 缺少随机动词列表 (Thinking/Cooking/Brewing...)
6. **Shimmer/Glimmer动画**: 完全缺失 — claude-code-main的spinner文本有色彩闪烁效果
7. **Token计数器/耗时显示**: Spinner行需要显示token数和耗时
8. **新旧App整合**: `app.go` 中的 `App` 和 `model.go` 中的 `Model` 需要统一使用新的主题和组件系统

---

## 二、执行阶段

### 阶段 1: Spinner 动画引擎重写（核心视觉差异）

**目标**: 完全替换当前 `spinner.go`，实现claude-code-main的自定义Spinner动画效果。

#### 1.1 新建 `tui/spinnerv2/` 包

**文件**: `tui/spinnerv2/spinner.go`

```
功能清单:
- SpinnerModel: Bubbletea Model 实现
- 帧序列: 使用 figures.SpinnerFrames() (·✢✳✶✻✽✻✶✢·)
- 帧间隔: 120ms (与 claude-code-main 的 Math.floor(time/120) 一致)
- 颜色: 使用主题 Claude 颜色
- Stalled检测: 超过 stalledThreshold(默认5s) 后开始红色渐变
- stalledIntensity: 在 stalledDuration(5s) 内从0线性增长到1
- 颜色插值: 从 theme.Claude → theme.Error(红色) 插值
```

**参考源码**:
- `claude-code-main/src/components/Spinner/SpinnerGlyph.tsx:1-80`
  - 帧索引: `SPINNER_FRAMES[frame % SPINNER_FRAMES.length]`
  - stalled颜色: `interpolateColor(baseRGB, ERROR_RED, stalledIntensity)`
- `claude-code-main/src/components/Spinner/SpinnerAnimationRow.tsx:1-265`
  - 帧计算: `Math.floor(time / 120)`
  - stalled判断: `elapsedTimeMs > stalledThreshold`

**关键实现**:
```go
// SpinnerModel 核心字段
type SpinnerModel struct {
    frames      []string          // figures.SpinnerFrames()
    frameIdx    int
    startTime   time.Time
    visible     bool
    label       string            // 当前动词 "Thinking…"
    theme       themes.Theme
    stalledMs   int               // 默认 5000
    stalledDurMs int              // 默认 5000
    tokenCount  int
    elapsedTime time.Duration
}

// Tick 间隔 120ms
func (s SpinnerModel) tickCmd() tea.Cmd {
    return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
        return spinnerTickMsg(t)
    })
}

// stalledIntensity 计算
func (s SpinnerModel) stalledIntensity() float64 {
    elapsed := time.Since(s.startTime).Milliseconds()
    if elapsed <= int64(s.stalledMs) {
        return 0
    }
    t := float64(elapsed-int64(s.stalledMs)) / float64(s.stalledDurMs)
    if t > 1 { t = 1 }
    return t
}
```

#### 1.2 新建 `tui/spinnerv2/shimmer.go`

**功能**: 实现 GlimmerMessage 文字闪烁效果

```
- shimmer效果: 文本中的字符按位置偏移做亮度波动
- 使用 theme.ClaudeShimmer 作为亮色
- 使用 theme.Claude 作为暗色
- 波动周期: 每个字符根据 (charIndex + time/80) 计算亮度
```

**参考源码**:
- `claude-code-main/src/components/Spinner/SpinnerAnimationRow.tsx` 中的 `GlimmerMessage` 组件

#### 1.3 新建 `tui/spinnerv2/verbs.go`

**功能**: Spinner 随机动词列表

```
- 包含完整的 SPINNER_VERBS 列表 (约190个动词)
- 从 claude-code-main/src/constants/spinnerVerbs.ts 1:1 复制
- RandomVerb() 函数随机选取
- 支持自定义动词配置 (replace/append 模式)
```

#### 1.4 Spinner 行组合

**文件**: `tui/spinnerv2/row.go`

```
渲染格式 (与 claude-code-main 一致):
  ● Thinking…                          42 tokens · 3s

组成:
  [SpinnerGlyph] [GlimmerLabel] [padding] [TokenCount · ElapsedTime]

- SpinnerGlyph: 帧字符 + 颜色(含stalled插值)
- GlimmerLabel: shimmer效果的动词文本
- TokenCount: 仅在 >0 时显示
- ElapsedTime: 仅在 >500ms 后显示
```

---

### 阶段 2: 消息渲染样式对齐

**目标**: 让消息显示格式与 claude-code-main 完全一致。

#### 2.1 用户消息格式

**当前**: `❯ You: <content>`
**目标**: `❯ <content>` (无 "You:" 标签)

**参考**: claude-code-main 中用户消息使用 `❯` 前缀，不显示 "You:" 标签

**修改文件**: `tui/message/row.go` - `RenderUserMessage()`

```
修改前: "❯ You\n<content>"
修改后: "❯ <content>"
颜色: 使用 theme.Claude (橙色 rgb(215,119,87)) 渲染 ❯ 符号
```

#### 2.2 助手消息格式

**当前**: `⏺ Assistant:\n<content>`
**目标**: `● <content>` (使用 BlackCircle + 主题色，无 "Assistant:" 标签)

**参考**: `claude-code-main/src/components/MessageResponse.tsx:22`
- 响应内容使用 `⎿` 缩进连接器
- 嵌套响应不重复 `⎿`

**修改文件**: `tui/message/row.go` - `RenderAssistantMessage()`

```
格式:
● <first line of content>
  ⎿  <subsequent content lines with indentation>

- ● 使用 figures.BlackCircle() (macOS: ⏺, others: ●)
- ● 颜色: theme.Claude
- ⎿ 颜色: dimColor (faint)
- 缩进: "  ⎿  " (2空格 + ⎿ + 2空格)
```

#### 2.3 工具调用消息格式

**当前**: `⚙ <toolName>\n  ⎿ <input>`
**目标**: 与 claude-code-main 一致的分层格式

```
修改后格式:
● <ToolDisplayName>
  ⎿  <summarized input>

已完成的工具:
● <ToolDisplayName>
  ⎿  <result summary or diff preview>

- ● 颜色: theme.Claude
- ⎿ 颜色: faint/dim
- 工具名称使用人类可读名称 (Bash→"Running command", Edit→"Editing file" 等)
```

#### 2.4 工具结果消息格式

**当前**: `  ⎿ <output>`
**目标**: 精确匹配 claude-code-main 的缩进和截断格式

```
  ⎿  <output line 1>
     <output line 2>
     ... (N more lines)

- 长输出截断到 5-10 行
- 错误结果使用 theme.Error 颜色
- 成功结果使用 dim/faint 颜色
```

#### 2.5 统一使用新主题系统

**修改文件**: `tui/message/row.go`

```
- 移除所有硬编码 lipgloss.Color("12") 等 ANSI 色值
- 传入 themes.Styles 参数
- 使用 Styles.Dot / Styles.Connector / Styles.Dimmed 等
- 修改 RenderOpts 增加 Styles themes.Styles 字段
```

---

### 阶段 3: 输入框样式对齐

**目标**: 输入框的边框、提示符和占位文本与 claude-code-main 一致。

#### 3.1 输入框边框样式

**当前**: 使用 `bubbles/textarea` 默认样式
**目标**: 匹配 claude-code-main 的输入框

**参考**: claude-code-main `PromptInput.tsx` 中的输入框使用:
- `round` 边框样式
- `borderColor={promptBorder}` (灰色 rgb(136,136,136))
- 只有顶部边框（无底部、左右边框）
- 最小高度 1 行

**修改文件**: `tui/input/promptinput.go`

```go
// 在 View() 中包装 textarea 输出
borderStyle := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(color.Resolve(theme.PromptBorder)).
    BorderBottom(false).
    BorderLeft(false).
    BorderRight(false)
```

#### 3.2 提示符样式

**当前**: `"Type a message… (Enter to send, Ctrl+C to quit)"`
**目标**: 匹配 claude-code-main 的占位提示

```
占位文本: "Reply to Claude…" 或 "Type your message…"
提示符颜色: theme.Inactive (dim gray)
```

#### 3.3 Footer 快捷键提示

**当前**: 简单文本 `"?: help  ctrl+c: quit"`
**目标**: claude-code-main 风格的底部快捷键栏

```
格式: [key1: desc1]  [key2: desc2]  ...
颜色: 键名使用 theme.Text, 描述使用 theme.Inactive
位置: 输入框下方
```

---

### 阶段 4: 权限对话框样式对齐

**目标**: 权限对话框与 claude-code-main 的 PermissionDialog 视觉一致。

#### 4.1 边框样式

**参考**: `claude-code-main/src/components/permissions/PermissionDialog.tsx:62`

```
borderStyle="round"
borderColor={permission}   // theme.Permission (蓝紫色)
borderLeft={false}
borderRight={false}
borderBottom={false}
marginTop={1}
```

**修改文件**: `tui/permissionui/dialog.go` - `View()`

```go
// 当前: 完整四边框 + 黄色
// 修改后: 只有顶部round边框 + permission紫色
border := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(color.Resolve(theme.Permission)).
    BorderBottom(false).
    BorderLeft(false).
    BorderRight(false).
    MarginTop(1)
```

#### 4.2 标题样式

```
当前: "⚠  Permission Required"
目标: PermissionRequestTitle 样式
  - 标题颜色: theme.Permission (蓝紫色)
  - 副标题: 工具名称 + 描述
```

#### 4.3 按钮样式

```
当前: [y] Allow  [n] Deny
目标: 匹配 claude-code-main 的权限按钮布局
  - Allow 按钮: theme.Success 颜色
  - Deny 按钮: theme.Error 颜色
  - Always Allow / Always Deny: dim 颜色
```

---

### 阶段 5: 状态栏精确对齐

**目标**: 状态栏与 claude-code-main 视觉一致。

#### 5.1 布局格式

**参考**: claude-code-main 状态栏布局:

```
[model name] · [$cost] · [context bar]    [permission mode] · [turn N] · [/path]
```

**当前实现** (`tui/statusline/statusline.go`) 已基本正确，需要微调:

- 确保使用新主题色彩
- 上下文进度条使用 `▏▎▍▌▋▊▉█` Unicode块字符 (与claude-code-main `ProgressBar.tsx` 一致)
- 颜色阈值: <70% theme.Suggestion, 70-90% theme.Warning, >90% theme.Error

#### 5.2 接入新主题

```
- 修改 statusline.Theme → 使用 themes.BuildStatusLineStyles()
- 确保颜色值来自新主题系统
```

---

### 阶段 6: 设计系统组件

**目标**: 实现 claude-code-main 的通用 UI 组件。

#### 6.1 Divider 分割线

**新建文件**: `tui/designsystem/divider.go`

**参考**: `claude-code-main/src/components/design-system/Divider.tsx`

```go
func RenderDivider(width int, title string, color string) string
// 无标题: ────────────────────
// 有标题: ───── Title ─────
// 字符: ─ (U+2500)
// 颜色: 指定颜色或 dimColor
```

#### 6.2 StatusIcon 状态图标

**新建文件**: `tui/designsystem/statusicon.go`

**参考**: `claude-code-main/src/components/design-system/StatusIcon.tsx`

```go
type StatusIconType string
const (
    StatusSuccess StatusIconType = "success"  // ✓ green
    StatusError   StatusIconType = "error"    // ✗ red
    StatusWarning StatusIconType = "warning"  // ⚠ yellow
    StatusInfo    StatusIconType = "info"     // ℹ blue
    StatusPending StatusIconType = "pending"  // ○ dim
    StatusLoading StatusIconType = "loading"  // … dim
)
func RenderStatusIcon(status StatusIconType, theme themes.Theme) string
```

#### 6.3 ProgressBar 进度条

**新建文件**: `tui/designsystem/progressbar.go`

**参考**: `claude-code-main/src/components/design-system/ProgressBar.tsx`

```go
// BLOCKS: [' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█']
func RenderProgressBar(ratio float64, width int, fillColor, emptyColor string) string
```

---

### 阶段 7: App 整合与统一

**目标**: 将所有新组件整合到 `App` 中，替换旧组件。

#### 7.1 替换 App.spinner

```go
// 当前: spinner SpinnerModel (基于 bubbles/spinner)
// 替换: spinner *spinnerv2.SpinnerModel (自定义帧动画)
```

#### 7.2 替换消息渲染

```go
// 当前: App.renderMessages() 使用硬编码样式
// 替换: 使用 message.RenderMessageRow() + themes.Styles
```

#### 7.3 替换输入框渲染

```go
// 当前: App.renderInput() 直接返回 textarea.View()
// 替换: 包装 round 边框 + 仅顶部边框 + promptBorder 颜色
```

#### 7.4 替换权限对话框

```go
// 当前: App.permission PermissionModel (旧简单版)
// 替换: 使用 permissionui.PermissionDialog (新增强版) + 新样式
```

#### 7.5 统一主题传递

```go
// AppConfig 增加 ThemeName 字段
type AppConfig struct {
    ThemeName      themes.ThemeName
    // ...
}

// NewApp 中:
theme := themes.GetTheme(cfg.ThemeName)
styles := themes.BuildStyles(theme)
// 传递 theme 和 styles 到所有子组件
```

#### 7.6 Model (model.go) 清理

```
- 移除 model.go 中的硬编码样式变量 (userStyle, assistantStyle 等)
- Model.View() 改为使用新主题系统
- 或者将 Model 合并到 App 中 (Model 是早期简化版)
```

---

### 阶段 8: Logo/Banner 细节对齐

**目标**: 确保启动Banner与claude-code-main完全一致。

#### 8.1 CondensedLogo 精确布局

**参考**: `claude-code-main/src/components/LogoV2/CondensedLogo.tsx`

```
╭──────────────────────────────────────────────╮
│  [Clawd]  Claude Code v1.0.0                 │
│            sonnet-4 · API                     │
│            /home/user/project                 │
╰──────────────────────────────────────────────╯

- borderStyle: round
- borderColor: theme.Claude (橙色)
- 内容: Clawd mascot + 版本 + 模型 + 计费 + CWD
```

**修改文件**: `tui/logo/banner.go`

```go
// RenderCondensedBanner 增加 round 边框
// 使用 lipgloss.RoundedBorder()
// borderColor 使用 theme.Claude
```

#### 8.2 欢迎消息

```
目标格式:
  "Welcome back, <username>!" 或 "Welcome to Claude Code!"
参考: claude-code-main/src/utils/logoV2Utils.ts - getWelcomeMessage()
```

---

## 三、文件变更总览

### 新建文件

| 文件路径 | 说明 |
|----------|------|
| `tui/spinnerv2/spinner.go` | 自定义帧动画Spinner核心 |
| `tui/spinnerv2/shimmer.go` | GlimmerMessage文字闪烁效果 |
| `tui/spinnerv2/verbs.go` | Spinner随机动词列表(190个) |
| `tui/spinnerv2/row.go` | Spinner行组合渲染 |
| `tui/designsystem/divider.go` | 水平分割线组件 |
| `tui/designsystem/statusicon.go` | 状态图标组件 |
| `tui/designsystem/progressbar.go` | Unicode进度条组件 |

### 修改文件

| 文件路径 | 修改内容 |
|----------|----------|
| `tui/message/row.go` | 消息渲染格式对齐 + 使用新主题 |
| `tui/input/promptinput.go` | 输入框边框样式 + 占位文本 |
| `tui/permissionui/dialog.go` | 权限对话框边框/颜色样式 |
| `tui/statusline/statusline.go` | 接入新主题 + 进度条字符 |
| `tui/logo/banner.go` | 添加round边框 + 欢迎消息 |
| `tui/app.go` | 整合新组件 + 统一主题 |
| `tui/model.go` | 移除硬编码样式 / 可能合并到app |
| `tui/spinner.go` | 标记为deprecated或删除 |

---

## 四、视觉对照检查清单

### 启动界面
- [ ] Clawd mascot 使用 theme.ClawdBody/ClawdBackground 颜色
- [ ] Round边框使用 theme.Claude 颜色
- [ ] 版本号 dim 颜色
- [ ] 模型名 · 计费类型 dim 颜色
- [ ] CWD 路径 dim 颜色 + 超长截断

### 用户输入
- [ ] 输入框仅顶部round边框
- [ ] 边框颜色 theme.PromptBorder (灰色)
- [ ] ❯ 提示符颜色
- [ ] 占位文本 dim 颜色

### 助手消息
- [ ] ● 前缀使用 figures.BlackCircle() + theme.Claude 颜色
- [ ] 无 "Assistant:" 标签
- [ ] 响应内容使用 `  ⎿  ` 缩进连接器
- [ ] 连接器 dim 颜色

### 工具调用
- [ ] ● 前缀 + 工具显示名称
- [ ] `  ⎿  ` 缩进显示输入摘要
- [ ] 结果显示使用 dim 颜色
- [ ] 错误结果使用 theme.Error 颜色
- [ ] 长输出截断

### Spinner
- [ ] 帧序列: ·✢✳✶✻✽ (forward+reverse)
- [ ] 帧间隔: 120ms
- [ ] 颜色: theme.Claude
- [ ] Stalled检测: 5s后开始红色渐变
- [ ] 随机动词显示
- [ ] Shimmer文字效果
- [ ] Token计数 + 耗时显示 (>500ms)

### 权限对话框
- [ ] 仅顶部round边框
- [ ] 边框颜色 theme.Permission (蓝紫色)
- [ ] Allow = theme.Success, Deny = theme.Error
- [ ] marginTop=1

### 状态栏
- [ ] 模型名 bold
- [ ] 费用 theme.Success 颜色
- [ ] 上下文进度条使用Unicode块字符
- [ ] 路径 dim 颜色

---

## 五、执行顺序和估时

| 序号 | 阶段 | 预计工作量 | 依赖 |
|------|------|-----------|------|
| 1 | Spinner动画引擎重写 | 大 | 无 |
| 2 | 消息渲染样式对齐 | 中 | 阶段1(spinner行) |
| 3 | 输入框样式对齐 | 小 | 无 |
| 4 | 权限对话框样式对齐 | 小 | 无 |
| 5 | 状态栏精确对齐 | 小 | 无 |
| 6 | 设计系统组件 | 中 | 无 |
| 7 | App整合与统一 | 大 | 阶段1-6全部 |
| 8 | Logo/Banner细节对齐 | 小 | 无 |

**建议执行路径**: 3→4→5→8 (小改动先行) → 1→6 (核心组件) → 2 (消息渲染) → 7 (最终整合)

---

## 六、关键源码参考索引

| claude-code-main 源文件 | 对应功能 | agent-engine 目标文件 |
|------------------------|----------|---------------------|
| `src/utils/theme.ts` | 主题色彩定义 | `tui/themes/definitions.go` ✅ |
| `src/constants/figures.ts` | Unicode图标 | `tui/figures/figures.go` ✅ |
| `src/constants/spinnerVerbs.ts` | Spinner动词 | `tui/spinnerv2/verbs.go` 🆕 |
| `src/components/Spinner.tsx` | Spinner主组件 | `tui/spinnerv2/spinner.go` 🆕 |
| `src/components/Spinner/SpinnerGlyph.tsx` | 帧渲染+stalled | `tui/spinnerv2/spinner.go` 🆕 |
| `src/components/Spinner/SpinnerAnimationRow.tsx` | 行组合+shimmer | `tui/spinnerv2/row.go` 🆕 |
| `src/components/MessageResponse.tsx` | ⎿缩进连接器 | `tui/message/row.go` 📝 |
| `src/components/PromptInput/PromptInput.tsx` | 输入框 | `tui/input/promptinput.go` 📝 |
| `src/components/permissions/PermissionDialog.tsx` | 权限对话框 | `tui/permissionui/dialog.go` 📝 |
| `src/components/LogoV2/CondensedLogo.tsx` | 精简Banner | `tui/logo/banner.go` 📝 |
| `src/components/LogoV2/Clawd.tsx` | Clawd吉祥物 | `tui/logo/clawd.go` ✅ |
| `src/components/design-system/Divider.tsx` | 分割线 | `tui/designsystem/divider.go` 🆕 |
| `src/components/design-system/StatusIcon.tsx` | 状态图标 | `tui/designsystem/statusicon.go` 🆕 |
| `src/components/design-system/ProgressBar.tsx` | 进度条 | `tui/designsystem/progressbar.go` 🆕 |
| `src/components/design-system/ThemedBox.tsx` | 主题Box | `tui/themes/styles.go` ✅ (已通过Styles实现) |

---

## 七、注意事项

1. **不改变功能逻辑** — 本计划只涉及视觉/交互样式，不改变引擎、工具执行、权限等业务逻辑
2. **保持Go风格** — 虽然参考React/TypeScript源码，但实现必须符合Go惯用法和Bubbletea模式
3. **渐进式替换** — 每个阶段独立可测试，不破坏现有功能
4. **终端兼容性** — 考虑 Windows Terminal / CMD / PowerShell 的Unicode和颜色支持差异
5. **Reduced Motion** — 可选支持减少动画模式（静态●代替动画帧）
