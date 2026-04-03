# Claude-Code-Main CLI 视觉交互 1:1 复刻到 Agent-Engine 执行方案

将 claude-code-main 的完整 CLI 交互视觉系统（Logo、主题色、动画、消息样式、输入框、权限对话框、状态栏等）精确复刻到 agent-engine 的 Bubbletea TUI 中。

---

## 现状差距分析

| 视觉元素 | claude-code-main | agent-engine 当前 |
|---|---|---|
| **Logo** | Clawd 吉祥物 (▐▛███▜▌) + "Claude Code" + version/model/cwd | 纯文本 banner |
| **主题** | 6套完整主题 (dark/light/ansi/daltonized), 70+ RGB 色值 | 硬编码 ANSI 色号 (12,10,11,9) |
| **消息前缀** | ● 彩色圆点 + ⎿ 缩进连接符 | "You:" / "Assistant:" 文本 |
| **Spinner** | ·✢✳✶✻✽ 动画 + glimmer 闪烁 + stalled 变红 | 无动画, 纯 "Thinking…" |
| **输入框** | ❯ 提示符 + round 上边框 + 主题色边框 | textarea 默认样式 |
| **权限框** | round 上边框 + permission 蓝紫色 + 工具专用 UI | 简单对话框 |
| **状态栏** | model · cost · context% · vim mode · session | 纯文本 status |
| **Diff 颜色** | 绿/红 RGB 精确色 (added/removed/word-level) | 无 diff 着色 |
| **设计系统** | ThemedBox, StatusIcon, Divider, ProgressBar | 无 |

---

## Phase 1: 主题系统重写 — `internal/tui/themes/`

**目标**: 替换 `themes/manager.go`，完整复刻 claude-code-main 的 6 套主题。

**文件**: `internal/tui/themes/theme.go` (新建，替代 manager.go)

**关键数据结构**:
```go
type Theme struct {
    // 核心色
    Claude           string // rgb(215,119,87)
    ClaudeShimmer    string // 更亮的 claude 色
    Permission       string // rgb(177,185,249) dark / rgb(87,105,247) light
    PermissionShimmer string
    PromptBorder     string
    PromptBorderShimmer string
    Text             string
    InverseText      string
    Inactive         string
    Subtle           string
    Suggestion       string
    
    // 语义色
    Success    string
    Error      string
    Warning    string
    
    // Diff 色
    DiffAdded        string
    DiffRemoved      string
    DiffAddedDimmed  string
    DiffRemovedDimmed string
    DiffAddedWord    string
    DiffRemovedWord  string
    
    // TUI V2 色
    ClawdBody        string
    ClawdBackground  string
    UserMsgBg        string
    BashMsgBg        string
    MemoryBg         string
    
    // Bash / Plan 模式
    BashBorder   string
    PlanMode     string
    AutoAccept   string
    FastMode     string
    
    // 子代理色 (8色)
    AgentRed, AgentBlue, AgentGreen, AgentYellow string
    AgentPurple, AgentOrange, AgentPink, AgentCyan string
    
    // 彩虹色 (ultrathink)
    RainbowRed, RainbowOrange, RainbowYellow string
    RainbowGreen, RainbowBlue, RainbowIndigo, RainbowViolet string
    // + shimmer 变体
}

type ThemeName string
const (
    ThemeDark            ThemeName = "dark"
    ThemeLight           ThemeName = "light"
    ThemeDarkAnsi        ThemeName = "dark-ansi"
    ThemeLightAnsi       ThemeName = "light-ansi"
    ThemeDarkDaltonized  ThemeName = "dark-daltonized"
    ThemeLightDaltonized ThemeName = "light-daltonized"
)
```

**操作**:
1. 新建 `theme.go`，定义 `Theme` 结构体 (70+ 字段)
2. 实现 6 套完整主题常量，色值 1:1 抄自 `claude-code-main/src/utils/theme.ts`
3. 添加 `GetTheme(name ThemeName) Theme`
4. 添加 `ResolveThemeSetting(setting string, systemDark bool) ThemeName` (auto 解析)
5. 添加 `ThemeToLipgloss(theme Theme)` 将 `rgb(r,g,b)` / `ansi:xxx` 转为 `lipgloss.Color`
6. 删除旧 `manager.go`
7. `go build ./...` 验证

---

## Phase 2: 图标/符号常量 — `internal/tui/figures/`

**目标**: 复刻 `constants/figures.ts` 的所有 Unicode 图标。

**文件**: `internal/tui/figures/figures.go` (新建)

**内容**:
```go
package figures

import "runtime"

// 消息圆点 (macOS 用 ⏺, 其他用 ●)
func BlackCircle() string {
    if runtime.GOOS == "darwin" { return "⏺" }
    return "●"
}

const (
    BulletOperator   = "∙"
    TeardropAsterisk = "✻"
    Pointer          = "❯"       // 输入提示符
    ResponseIndent   = "  ⎿  "   // 助手响应缩进
    BlockquoteBar    = "▎"
    HeavyHorizontal  = "━"
    
    // 状态图标
    Tick    = "✓"
    Cross   = "✗"
    Warning = "⚠"
    Info    = "ℹ"
    Circle  = "○"
    
    // 箭头
    ArrowUp   = "↑"
    ArrowDown = "↓"
    Lightning = "↯"
    
    // 努力等级
    EffortLow    = "○"
    EffortMedium = "◐"
    EffortHigh   = "●"
    EffortMax    = "◉"
)

// Spinner 字符序列 (平台相关)
func SpinnerChars() []string {
    if runtime.GOOS == "darwin" {
        return []string{"·", "✢", "✳", "✶", "✻", "✽"}
    }
    return []string{"·", "✢", "*", "✶", "✻", "✽"}
}
```

**操作**:
1. 新建 `figures.go`
2. `go build ./...` 验证

---

## Phase 3: Clawd Logo & Welcome Banner — `internal/tui/logo/`

**目标**: 复刻 Clawd 吉祥物 ASCII art + CondensedLogo 格式的欢迎信息。

**文件**: `internal/tui/logo/clawd.go`, `internal/tui/logo/banner.go`

### clawd.go
```
Clawd 默认造型 (3行, 9列宽):
   ▐▛███▜▌    (row1: arm + eyes + arm)
   ▝▜█████▛▘  (row2: arm + body + arm)
     ▘▘ ▝▝    (row3: feet)

颜色: clawd_body 主题色, clawd_background 背景色
```

**结构**:
```go
type ClawdPose string
const (
    PoseDefault   ClawdPose = "default"
    PoseLookLeft  ClawdPose = "look-left"
    PoseLookRight ClawdPose = "look-right"
    PoseArmsUp    ClawdPose = "arms-up"
)

func RenderClawd(pose ClawdPose, theme Theme) string
```

### banner.go (CondensedLogo 格式)
```
[Clawd]  Claude Code v1.0.0
         sonnet-4 · API
         /path/to/cwd
```

**结构**:
```go
type BannerData struct {
    Version string
    Model   string
    Billing string  // "API" / "Pro" / etc
    CWD     string
    Agent   string  // optional agent name
}

func RenderCondensedBanner(data BannerData, theme Theme, width int) string
func RenderFullBanner(data BannerData, theme Theme, width int) string
```

**操作**:
1. 新建 `clawd.go` — 4 种造型的 block char 渲染 + lipgloss 着色
2. 新建 `banner.go` — Condensed 和 Full 两种 banner 格式
3. 在 `app.go` 的 Init 中渲染 banner 并写入 viewport
4. `go build ./...` 验证

---

## Phase 4: Spinner 动画系统 — `internal/tui/spinner/`

**目标**: 复刻 claude-code-main 的完整 spinner 动画系统。

**文件**: 重写 `internal/tui/spinner/` 包

### spinner.go (主模型)
```go
type SpinnerMode string
const (
    ModeRequesting SpinnerMode = "requesting"
    ModeResponding SpinnerMode = "responding"
    ModeThinking   SpinnerMode = "thinking"
    ModeToolUse    SpinnerMode = "tool-use"
    ModeToolInput  SpinnerMode = "tool-input"
)

type SpinnerModel struct {
    mode           SpinnerMode
    frame          int
    startTime      time.Time
    responseLength int
    stalledFrames  int
    thinkingStatus string // "thinking" | "thought for Xs" | ""
    theme          themes.Theme
    verbose        bool
    width          int
}
```

### glyph.go (SpinnerGlyph)
- 字符序列: `['·', '✢', '✳', '✶', '✻', '✽']` → forward + reverse
- 帧间隔: 120ms
- 颜色: theme.Claude (正常), 渐变到 rgb(171,43,63) (stalled)
- Stalled 检测: 3s 无新内容 → 渐变变红
- Reduced motion: 固定 ● 加 dim 闪烁

### glimmer.go (GlimmerMessage)
- 逐字符着色，shimmer 光带从左到右/右到左移动
- 光带宽度 ~10 字符，速度随 mode 变化
- requesting: 快速 (50ms/step), 从左到右
- responding: 慢速 (200ms/step), 从右到左
- 支持 stalled → 红色渐变

### animation.go (动画行)
- 组合: `[SpinnerGlyph] [GlimmerMessage] (thinking · 5s · ↓ 1,234 tokens)`
- 渐进式显示: 宽度不够时先隐藏 tokens, 再隐藏 timer, 再隐藏 thinking
- Token counter 平滑递增动画
- Elapsed time 格式: formatDuration

**操作**:
1. 重写 `spinner.go` — SpinnerModel 作为 Bubbletea 子模型，50ms tick
2. 新建 `glyph.go` — 字符帧 + stalled 颜色插值
3. 新建 `glimmer.go` — 逐字符 shimmer 着色
4. 新建 `animation.go` — 组装完整的动画行 + 状态部分
5. 添加 RGB 颜色插值工具函数
6. `go build ./...` 验证

---

## Phase 5: 消息渲染系统重写 — `internal/tui/message/`

**目标**: 消息显示从 "You:" / "Assistant:" 改为 ● 圆点 + ⎿ 缩进格式。

### 消息布局 (claude-code-main 风格):

**用户消息**:
```
● Your message here
```
(● 用 theme.Claude 色, 可选 userMessageBackground 背景)

**助手文本**:
```
● Claude response here
  ⎿  Markdown rendered content
      continues with indent...
```
(● 用 theme.Claude 色, ⎿ dimColor)

**工具调用**:
```
● Read file.go
  ⎿  (file contents, collapsed)
```
(● 用 theme.Claude 色)

**系统消息**:
```
● System notification
```
(● 用 theme.Suggestion 色)

**错误消息**:
```
● Error: something went wrong
```
(● 用 theme.Error 色)

### 文件修改:

**`message/row.go`** — 重写 `RenderMessageRow`:
```go
func RenderMessageRow(msg RenderableMessage, theme themes.Theme, width int) string {
    dot := figures.BlackCircle()
    switch msg.Role {
    case "user":
        // ● [claude色] + 用户文本
    case "assistant":
        // ● [claude色] + 第一行
        // ⎿  [dim] + 后续行 (markdown 渲染)
    case "tool_use":
        // ● [claude色] + 工具名
        // ⎿  [dim] + 工具详情
    case "system":
        // ● [suggestion色] + 系统消息
    case "error":
        // ● [error色] + 错误信息
    }
}
```

**`message/response.go`** (新建) — MessageResponse 缩进:
```go
// IndentResponse wraps content with the ⎿ connector prefix
func IndentResponse(content string) string {
    lines := strings.Split(content, "\n")
    var sb strings.Builder
    for i, line := range lines {
        if i == 0 {
            sb.WriteString("  ⎿  " + line)
        } else {
            sb.WriteString("     " + line)  // 5 chars indent
        }
        sb.WriteString("\n")
    }
    return sb.String()
}
```

**操作**:
1. 新建 `message/response.go` — ⎿ 缩进连接符
2. 重写 `message/row.go` — ● 圆点 + 主题色 + ⎿ 缩进
3. 更新 `message/types.go` — 添加 dot/connector 相关字段
4. 更新 `model.go` 中的 `renderMessages()` 使用新格式
5. `go build ./...` 验证

---

## Phase 6: 输入框样式 — `internal/tui/input/`

**目标**: 复刻 ❯ 提示符 + round 上边框 + 主题色边框的输入框。

### 输入框布局 (claude-code-main 风格):
```
╭─────────────────────────────────────╮
│ ❯ Type your message here...         │
╰─────────────────────────────────────╯
  ! for bash · /help · esc to undo
```
- 边框: round 样式, 仅顶部 (borderLeft=false, borderRight=false, borderBottom=false 在 claude 中)
  - agent-engine 用完整 round border 更合理
- 边框色: theme.PromptBorder (bash 模式: theme.BashBorder)
- ❯ 提示符: figures.Pointer, dimColor when loading
- 底部: 快捷键提示 (dim)

### 文件修改:

**`input/promptinput.go`** — 更新渲染:
```go
func (p *PromptInput) View() string {
    // 1. 构建提示符 "❯ "
    pointer := lipgloss.NewStyle().
        Foreground(resolveColor(theme.PromptBorder)).
        Render(figures.Pointer + " ")
    if p.isLoading {
        pointer = lipgloss.NewStyle().Faint(true).Render(figures.Pointer + " ")
    }
    
    // 2. 边框
    borderColor := theme.PromptBorder
    if p.mode == "bash" { borderColor = theme.BashBorder }
    
    border := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(resolveColor(borderColor)).
        Width(p.width)
    
    // 3. 底部提示
    footer := lipgloss.NewStyle().Faint(true).
        Render("  /help · esc to interrupt")
}
```

**操作**:
1. 更新 `input/promptinput.go` — ❯ 提示符 + round border + 主题色
2. 在 `app.go` 的 View 中集成新输入框样式
3. `go build ./...` 验证

---

## Phase 7: 权限对话框样式 — `internal/tui/permissionui/`

**目标**: 复刻 round 上边框 + permission 主题色的权限对话框。

### 权限对话框布局:
```
╭─ Agent Engine wants to run a command ────────╮
│  Tool: Bash                                   │
│  Command: ls -la                              │
│                                               │
│  [y] Allow  [n] Deny  [a] Always allow        │
╰───────────────────────────────────────────────╯
```
- 边框: round, 仅顶部线, borderColor = theme.Permission
- 标题: bold, permission 色
- 操作按钮: 高亮当前选中

**文件修改**: `permissionui/dialog.go`
- 使用 theme.Permission 色
- Round border (top only 模拟: 使用 lipgloss Border 但只绘制顶部)
- 工具详情区域
- 按钮行

**操作**:
1. 重写 `permissionui/dialog.go` — round border + permission 色
2. `go build ./...` 验证

---

## Phase 8: 状态栏重写 — `internal/tui/statusline/`

**目标**: 复刻 claude-code-main 的状态栏格式。

### 状态栏布局:
```
 sonnet-4 · default · $0.05 · 45% context · [VIM: NORMAL]
```

**格式详情**:
- model name (dimColor)
- permission mode
- cost: `$X.XX` 格式
- context: `XX%` + 可选 mini progress bar
- vim mode (如启用)
- session name (如有)

**文件修改**: `statusline/statusline.go`
```go
func Render(data StatusData, theme themes.Theme, width int) string {
    parts := []string{
        renderModel(data.Model),
        renderPermMode(data.PermissionMode),
        renderCost(data.CostUSD),
        renderContext(data.ContextPct, theme),
    }
    if data.VimMode != "" {
        parts = append(parts, renderVimMode(data.VimMode))
    }
    return joinParts(parts, " · ", width)
}
```

**操作**:
1. 重写 `statusline/statusline.go` — 精确匹配 claude 格式
2. `go build ./...` 验证

---

## Phase 9: 设计系统组件 — `internal/tui/designsystem/`

**目标**: 复刻 claude-code-main 的基础 UI 组件。

**文件**: `internal/tui/designsystem/` (新建目录)

### components.go
```go
// StatusIcon 渲染状态图标
func StatusIcon(status string) string {
    // success: ✓ (green), error: ✗ (red), warning: ⚠ (yellow)
    // info: ℹ (blue), pending: ○ (dim), loading: … (dim)
}

// Divider 渲染分隔线
func Divider(width int, theme Theme) string {
    return lipgloss.NewStyle().
        Foreground(resolveColor(theme.Subtle)).
        Render(strings.Repeat("─", width))
}

// ProgressBar 渲染进度条
func ProgressBar(pct float64, width int, fillColor, emptyColor string) string

// ThemedBox 渲染主题感知的框
func ThemedBox(content string, borderColor string, theme Theme, width int) string
```

### diff.go
```go
// RenderDiff 渲染带颜色的 diff
func RenderDiff(added, removed string, theme Theme) string
// RenderWordDiff 渲染单词级别的 diff 高亮
func RenderWordDiff(oldLine, newLine string, theme Theme) string
```

**操作**:
1. 新建 `designsystem/components.go` — StatusIcon, Divider, ProgressBar, ThemedBox
2. 新建 `designsystem/diff.go` — Diff 颜色渲染
3. `go build ./...` 验证

---

## Phase 10: 全局集成 & 打磨

**目标**: 将所有新组件集成到 `app.go`，确保完整的视觉效果。

### app.go 改动清单:
1. **Init**: 渲染 Clawd banner → 写入 viewport
2. **Update**: 使用新 SpinnerModel 的 Tick 消息
3. **View**: 
   - Header: (无, 全屏模式由 banner 在消息历史中)
   - Body: 消息用新的 ● + ⎿ 格式渲染
   - Spinner: 新的动画 spinner 行
   - Input: ❯ + round border
   - Permission: 新的 permission dialog
   - StatusLine: 新格式
4. **Theme**: 从 AppConfig 接收 ThemeName, 全局传递

### model.go 改动:
- 删除旧的硬编码 `userStyle`, `assistantStyle` 等
- 全部使用 `themes.Theme` 对象动态生成样式
- `renderMessages()` 使用 `message.RenderMessageRow()`

### 颜色工具 (`internal/tui/color/`):
```go
// ParseRGB 解析 "rgb(r,g,b)" 为 lipgloss.Color
func ParseRGB(s string) lipgloss.Color

// ParseAnsi 解析 "ansi:colorName" 为 lipgloss.Color
func ParseAnsi(s string) lipgloss.Color

// ResolveColor 通用解析 (rgb / ansi / hex)
func ResolveColor(s string) lipgloss.Color

// InterpolateRGB 两色插值
func InterpolateRGB(c1, c2 [3]int, t float64) lipgloss.Color
```

**操作**:
1. 新建 `internal/tui/color/color.go` — RGB/ANSI/Hex 解析 + 插值
2. 更新 `app.go` — 集成所有新组件
3. 更新 `model.go` — 删除旧样式, 使用 Theme 对象
4. 更新 `interactive.go` — 传递 ThemeName 给 App
5. 全面 `go build ./...` + `go vet ./...` 验证
6. 手动运行测试视觉效果

---

## 文件变动汇总

| 操作 | 文件路径 |
|------|----------|
| **新建** | `internal/tui/themes/theme.go` |
| **新建** | `internal/tui/figures/figures.go` |
| **新建** | `internal/tui/logo/clawd.go` |
| **新建** | `internal/tui/logo/banner.go` |
| **新建** | `internal/tui/spinner/glyph.go` |
| **新建** | `internal/tui/spinner/glimmer.go` |
| **新建** | `internal/tui/spinner/animation.go` |
| **新建** | `internal/tui/message/response.go` |
| **新建** | `internal/tui/color/color.go` |
| **新建** | `internal/tui/designsystem/components.go` |
| **新建** | `internal/tui/designsystem/diff.go` |
| **重写** | `internal/tui/spinner/spinner.go` (现有) |
| **重写** | `internal/tui/message/row.go` (现有) |
| **重写** | `internal/tui/statusline/statusline.go` (现有) |
| **重写** | `internal/tui/permissionui/dialog.go` (现有) |
| **修改** | `internal/tui/input/promptinput.go` |
| **修改** | `internal/tui/app.go` |
| **修改** | `internal/tui/model.go` |
| **删除** | `internal/tui/themes/manager.go` (被 theme.go 替代) |

---

## 执行顺序 & 依赖关系

```
Phase 1 (Theme) ─┐
Phase 2 (Figures)─┤
                  ├─→ Phase 3 (Logo) ──┐
                  ├─→ Phase 4 (Spinner)─┤
                  ├─→ Phase 5 (Message)─┤
                  ├─→ Phase 6 (Input) ──┤─→ Phase 10 (Integration)
                  ├─→ Phase 7 (Perm) ───┤
                  ├─→ Phase 8 (Status) ─┤
                  └─→ Phase 9 (Design) ─┘
```

Phase 1+2 是基础，可并行。Phase 3-9 依赖 Phase 1+2，互相独立可并行。Phase 10 是最终集成。

**每个 Phase 完成后必须 `go build ./...` 通过。**

---

## 关键色值参考 (Dark Theme)

| 用途 | RGB 值 |
|------|--------|
| claude (品牌橙) | `rgb(215,119,87)` |
| permission (蓝紫) | `rgb(177,185,249)` |
| promptBorder (灰) | `rgb(136,136,136)` |
| text (白) | `rgb(255,255,255)` |
| error (红) | `rgb(255,107,128)` |
| success (绿) | `rgb(78,186,101)` |
| warning (琥珀) | `rgb(255,193,7)` |
| diffAdded (暗绿) | `rgb(34,92,43)` |
| diffRemoved (暗红) | `rgb(122,41,54)` |
| bashBorder (粉) | `rgb(253,93,177)` |
| clawd_body (橙) | `rgb(215,119,87)` |
| subtle (深灰) | `rgb(80,80,80)` |
| inactive (浅灰) | `rgb(153,153,153)` |
