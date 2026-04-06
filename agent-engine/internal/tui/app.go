package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wall-ai/agent-engine/internal/buddy"
	"github.com/wall-ai/agent-engine/internal/tui/color"
	"github.com/wall-ai/agent-engine/internal/tui/companion"
	"github.com/wall-ai/agent-engine/internal/tui/designsystem"
	"github.com/wall-ai/agent-engine/internal/tui/figures"
	"github.com/wall-ai/agent-engine/internal/tui/logo"
	"github.com/wall-ai/agent-engine/internal/tui/message"
	"github.com/wall-ai/agent-engine/internal/tui/search"
	sess "github.com/wall-ai/agent-engine/internal/tui/session"
	"github.com/wall-ai/agent-engine/internal/tui/spinnerv2"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
	"github.com/wall-ai/agent-engine/internal/tui/vim"
)

// App is the top-level Bubbletea model for the full-screen TUI.
// It composes a three-region layout (header/body/footer) with:
//   - viewport  (message history, scrollable)
//   - textarea  (multi-line input)
//   - SpinnerModel (thinking/tool-use indicator)
//   - PermissionModel (permission confirmation dialog)
//   - MarkdownRenderer (for assistant messages)
//   - StatusLine (model · cost · context)
type App struct {
	// Screen & layout
	screen ScreenManager
	layout Layout

	// Core sub-models
	viewport   viewport.Model
	textarea   textarea.Model
	spinner    SpinnerModel
	permission PermissionModel
	md         *MarkdownRenderer

	// State
	messages   []ChatMessage
	status     string
	themeData  themes.Theme
	styles     themes.Styles
	keymap     KeyMap
	showHelp   bool
	isLoading  bool
	screenMode ScreenMode

	// Advanced sub-models
	vimState      *vim.VimState
	searchBar     *search.Overlay
	toolTrack     *ToolUseTracker
	transcript    *sess.TranscriptView
	sessStore     *sess.SessionStore
	companionView companion.Model

	// Status line data
	model        string
	prevModel    string // for model attribution change detection
	costUSD      float64
	contextPct   float64
	permMode     string
	cwd          string
	turnCount    int
	inputTokens  int
	linesAdded   int
	linesDeleted int

	// Timing
	loadingStart time.Time

	// Autocomplete
	completer *Completer
	compState CompletionState

	// Footer navigation (P7)
	footerFocused bool // true when arrow-down enters footer mode

	// SubmitFn is called when the user sends a message.
	SubmitFn func(text string)
}

// AppConfig holds configuration for creating a new App.
type AppConfig struct {
	ThemeName      themes.ThemeName // empty defaults to ThemeDark
	Dark           bool             // deprecated: use ThemeName instead
	Model          string
	PermissionMode string
	WorkDir        string
	SubmitFn       func(text string)
}

// NewApp creates a fully initialised full-screen App.
func NewApp(cfg AppConfig) (*App, error) {
	// Resolve theme: prefer ThemeName, fall back to Dark bool.
	themeName := cfg.ThemeName
	if themeName == "" {
		if cfg.Dark {
			themeName = themes.ThemeDark
		} else {
			themeName = themes.ThemeLight
		}
	}
	themeData := themes.GetTheme(themeName)
	styles := themes.BuildStyles(themeData)
	isDark := themes.IsDarkTheme(themeName)
	km := DefaultKeyMap()

	ta := textarea.New()
	ta.Placeholder = "Reply to Claude…"
	ta.Prompt = "> " // clean prompt matching claude-code-main
	ta.Focus()
	ta.SetWidth(76) // 80 - 4 (border content area minus side borders)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // unlimited

	// Apply theme colors to textarea (matching claude-code-main)
	promptColor := color.Resolve(themeData.Claude)
	textColor := color.Resolve(themeData.Text)
	subtleColor := color.Resolve(themeData.Subtle)
	ta.FocusedStyle.Base = lipgloss.NewStyle().PaddingLeft(1)
	ta.BlurredStyle.Base = lipgloss.NewStyle().PaddingLeft(1)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(textColor)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(subtleColor)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(subtleColor).Faint(true)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(subtleColor).Faint(true)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(promptColor)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(subtleColor)
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(promptColor)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(subtleColor)
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(subtleColor)

	vp := viewport.New(80, 20)
	vp.SetContent("")

	mdRenderer, err := NewMarkdownRenderer(76, isDark)
	if err != nil {
		return nil, err
	}

	// Render startup banner as the first message
	banner := logo.RenderCondensedBanner(logo.BannerData{
		Version: "0.1.0",
		Model:   cfg.Model,
		Billing: "API",
		CWD:     cfg.WorkDir,
	}, themeData, 60)

	initialMessages := []ChatMessage{
		{Role: "banner", Content: banner},
	}

	return &App{
		screen:        NewScreenManager(),
		layout:        NewLayout(80, 24),
		viewport:      vp,
		textarea:      ta,
		spinner:       NewSpinnerWithTheme(themeData),
		permission:    NewPermissionModelWithTheme(styles, themeData, km),
		md:            mdRenderer,
		vimState:      vim.New(),
		searchBar:     search.NewOverlay(80),
		toolTrack:     NewToolUseTracker(styles),
		transcript:    sess.NewTranscriptView(80, 20),
		sessStore:     sess.NewSessionStore(""),
		messages:      initialMessages,
		status:        "Ready",
		themeData:     themeData,
		styles:        styles,
		keymap:        km,
		screenMode:    ScreenPrompt,
		model:         cfg.Model,
		permMode:      cfg.PermissionMode,
		cwd:           cfg.WorkDir,
		completer:     NewCompleter(DefaultSlashCommands(), nil),
		companionView: companion.New(),
		SubmitFn:      cfg.SubmitFn,
	}, nil
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		textarea.Blink,
		a.spinner.Init(),
		a.companionView.Init(),
	)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
		cmds  []tea.Cmd
	)

	// ── Permission modal intercepts all keys while visible ────────────────
	if a.permission.IsVisible() {
		var permCmd tea.Cmd
		a.permission, permCmd = a.permission.Update(msg)
		if permCmd != nil {
			cmds = append(cmds, permCmd)
		}
		return a, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {

	case tea.KeyMsg:
		// ── Autocomplete popup intercepts keys when active ──────────
		if a.compState.Active {
			switch msg.Type {
			case tea.KeyTab:
				if item := a.compState.SelectedItem(); item != nil {
					a.textarea.SetValue(item.Value + " ")
				}
				a.compState.Reset()
				return a, nil
			case tea.KeyUp:
				a.compState.SelectPrev()
				return a, nil
			case tea.KeyDown:
				a.compState.SelectNext()
				return a, nil
			case tea.KeyEscape:
				a.compState.Reset()
				return a, nil
			case tea.KeyEnter:
				// Enter auto-submits the selected command.
				selected := a.compState.SelectedItem()
				a.compState.Reset()
				if selected != nil {
					text := strings.TrimSpace(selected.Value)
					if text != "" {
						a.messages = append(a.messages, ChatMessage{Role: "user", Content: text})
						a.textarea.Reset()
						a.status = "Thinking\u2026"
						a.isLoading = true
						a.loadingStart = time.Now()
						a.spinner.ShowRandom()
						a.refreshViewport()
						a.viewport.GotoBottom()
						if a.SubmitFn != nil {
							a.SubmitFn(text)
						}
						return a, a.spinner.Init()
					}
				}
				return a, nil
			}
		}

		// Search overlay intercepts keys when visible
		if a.searchBar.IsVisible() {
			consumed := a.searchBar.Update(msg, func(q string) []search.Hit {
				return a.searchMessages(q)
			})
			if consumed {
				// Jump to search hit in viewport
				if hit := a.searchBar.CurrentHit(); hit != nil {
					a.viewport.GotoBottom() // simplified — full impl would scroll to line
				}
				return a, nil
			}
		}

		// Vim mode processing
		if a.vimState.IsEnabled() && !a.textarea.Focused() {
			action := a.vimState.HandleKey(msg)
			if action.Type != vim.ActionPassthrough && action.Type != vim.ActionNone {
				a.handleVimAction(action)
				return a, nil
			}
		}

		switch {
		case msg.Type == tea.KeyCtrlC:
			return a, tea.Quit

		case msg.Type == tea.KeyEsc:
			if a.searchBar.IsVisible() {
				a.searchBar.Hide()
				return a, nil
			}
			if a.isLoading {
				return a, nil
			}
			if a.vimState.IsEnabled() {
				return a, nil // vim handles esc internally
			}
			return a, tea.Quit

		case msg.Type == tea.KeyEnter && !msg.Alt:
			text := strings.TrimSpace(a.textarea.Value())
			if text == "" {
				return a, nil
			}
			a.compState.Reset()
			a.messages = append(a.messages, ChatMessage{Role: "user", Content: text})
			a.textarea.Reset()
			a.status = "Thinking…"
			a.isLoading = true
			a.loadingStart = time.Now()
			a.spinner.ShowRandom()
			a.refreshViewport()
			a.viewport.GotoBottom()
			if a.SubmitFn != nil {
				a.SubmitFn(text)
			}
			return a, a.spinner.Init()

		case msg.String() == "?":
			if !a.textarea.Focused() {
				a.showHelp = !a.showHelp
			}

		case msg.Type == tea.KeyCtrlK:
			a.messages = append(a.messages, ChatMessage{Role: "system", Content: "Compacting context…"})
			a.refreshViewport()

		case msg.Type == tea.KeyCtrlO:
			// Toggle transcript mode
			if a.screenMode == ScreenPrompt {
				a.screenMode = ScreenTranscript
				a.screen.SetMode(ScreenTranscript)
			} else {
				a.screenMode = ScreenPrompt
				a.screen.SetMode(ScreenPrompt)
			}

		case msg.Type == tea.KeyCtrlF:
			a.searchBar.Show()
			return a, nil

		case msg.Type == tea.KeyPgUp || msg.Type == tea.KeyPgDown:
			// P6: Dismiss companion speech bubble on scroll (matches claude-code-main REPL.tsx)
			a.companionView.SetReaction("")

		case msg.Type == tea.KeyDown && !a.compState.Active:
			// P7: Arrow-down from empty input or end of input → enter footer mode
			if a.companionView.IsVisible() && !a.footerFocused {
				val := a.textarea.Value()
				if val == "" || !strings.Contains(val, "\n") {
					a.footerFocused = true
					a.companionView.SetFocused(true)
					a.textarea.Blur()
					return a, nil
				}
			}

		case msg.Type == tea.KeyUp && a.footerFocused:
			// P7: Arrow-up exits footer mode
			a.footerFocused = false
			a.companionView.SetFocused(false)
			a.textarea.Focus()
			return a, nil

		case msg.Type == tea.KeyEnter && a.footerFocused:
			// P7: Enter on companion pill → submit /buddy
			a.footerFocused = false
			a.companionView.SetFocused(false)
			a.textarea.Focus()
			a.messages = append(a.messages, ChatMessage{Role: "user", Content: "/buddy"})
			a.textarea.Reset()
			a.status = "Thinking…"
			a.isLoading = true
			a.loadingStart = time.Now()
			a.spinner.ShowRandom()
			a.refreshViewport()
			a.viewport.GotoBottom()
			if a.SubmitFn != nil {
				a.SubmitFn("/buddy")
			}
			return a, a.spinner.Init()
		}

	case tea.WindowSizeMsg:
		a.screen.Resize(msg.Width, msg.Height)
		a.layout.Resize(msg.Width, msg.Height)
		a.searchBar.SetWidth(msg.Width)
		a.transcript.SetSize(msg.Width, msg.Height-4)
		a.reflow()

	// ── Streaming engine events ────────────────────────────────────────────
	case StreamTextMsg:
		if len(a.messages) == 0 || a.messages[len(a.messages)-1].Role != "assistant" {
			a.messages = append(a.messages, ChatMessage{Role: "assistant"})
		}
		a.messages[len(a.messages)-1].Content += msg.Text
		a.refreshViewport()
		a.viewport.GotoBottom()

	case StreamDoneMsg:
		a.status = "Ready"
		a.isLoading = false
		a.turnCount++
		// Show turn completion message (matching claude-code-main)
		if a.spinner.IsVisible() {
			elapsed := a.spinner.Elapsed()
			a.spinner.Hide()
			completionMsg := spinnerv2.FormatTurnCompletion(elapsed)
			a.messages = append(a.messages, ChatMessage{Role: "system", Content: completionMsg})
			a.refreshViewport()
			a.viewport.GotoBottom()
		} else {
			a.spinner.Hide()
		}

	case StreamErrorMsg:
		a.messages = append(a.messages, ChatMessage{
			Role:    "error",
			Content: msg.Err.Error(),
		})
		a.status = "Error"
		a.isLoading = false
		a.spinner.Hide()
		a.refreshViewport()
		a.viewport.GotoBottom()

	case ToolStartMsg:
		a.messages = append(a.messages, ChatMessage{
			Role:     "tool_use",
			ToolName: msg.ToolName,
			Content:  msg.Input,
		})
		a.toolTrack.StartTool(msg.ToolID, msg.ToolName, msg.Input)
		a.spinner.SetLabel(msg.ToolName + "…")
		a.spinner.SetMode(SpinnerModeToolUse)
		a.transcript.Append(sess.TranscriptEntry{
			Timestamp: time.Now(), Role: "tool_use",
			ToolName: msg.ToolName, Content: msg.Input,
		})
		a.refreshViewport()
		a.viewport.GotoBottom()

	case ToolDoneMsg:
		a.messages = append(a.messages, ChatMessage{
			Role:     "tool_result",
			ToolName: msg.ToolID,
			Content:  msg.Output,
			IsError:  msg.IsError,
		})
		a.toolTrack.FinishTool(msg.ToolID, msg.Output, msg.IsError)
		a.spinner.ShowRandom()
		a.spinner.SetMode(SpinnerModeRequesting)
		a.transcript.Append(sess.TranscriptEntry{
			Timestamp: time.Now(), Role: "tool_result",
			Content: msg.Output, IsError: msg.IsError,
		})
		a.refreshViewport()
		a.viewport.GotoBottom()

	case SystemMsg:
		a.messages = append(a.messages, ChatMessage{
			Role:    "system",
			Content: msg.Text,
		})
		a.refreshViewport()
		a.viewport.GotoBottom()

	case ClearHistoryMsg:
		// Actually clear conversation messages (keep banner if present).
		var kept []ChatMessage
		for _, m := range a.messages {
			if m.Role == "banner" {
				kept = append(kept, m)
			}
		}
		a.messages = kept
		a.messages = append(a.messages, ChatMessage{Role: "system", Content: "Conversation cleared."})
		a.refreshViewport()
		a.viewport.GotoBottom()

	case CompactHistoryMsg:
		a.messages = append(a.messages, ChatMessage{Role: "system", Content: "Context compacted."})
		a.refreshViewport()
		a.viewport.GotoBottom()

	case PermissionAnswerMsg:
		// Answered — engine handles this via callback.

	case CostUpdateMsg:
		a.costUSD = msg.CostUSD
		a.inputTokens = msg.InputTokens
		a.turnCount = msg.TurnCount

	// ── Companion events ──────────────────────────────────────────────────
	case CompanionLoadMsg:
		if c, ok := msg.Companion.(*buddy.Companion); ok && c != nil {
			a.companionView.SetCompanion(c)
		}

	case CompanionReactionMsg:
		a.companionView.SetReaction(msg.Text)

	case CompanionPetMsg:
		a.companionView.SetPetAt(msg.Timestamp)

	case CompanionMuteMsg:
		a.companionView.SetMuted(msg.Muted)
	}

	a.textarea, taCmd = a.textarea.Update(msg)
	a.viewport, vpCmd = a.viewport.Update(msg)
	a.spinner, spCmd = a.spinner.Update(msg)
	var compCmd tea.Cmd
	a.companionView, compCmd = a.companionView.Update(msg)
	cmds = append(cmds, taCmd, vpCmd, spCmd, compCmd)

	// ── Auto-trigger slash command completion ─────────────────────────
	val := a.textarea.Value()
	if a.completer != nil && strings.HasPrefix(val, "/") {
		items := a.completer.Complete(val, len(val))
		if len(items) > 0 {
			a.compState.Active = true
			a.compState.Items = items
			if a.compState.Selected >= len(items) {
				a.compState.Selected = 0
			}
			a.compState.Prefix = val
		} else {
			a.compState.Reset()
		}
	} else if a.compState.Active {
		a.compState.Reset()
	}

	return a, tea.Batch(cmds...)
}

// SetCompleter replaces the current completer (e.g. after loading dynamic commands).
func (a *App) SetCompleter(c *Completer) {
	a.completer = c
}

// Completer returns the current completer for external updates.
func (a *App) Completer() *Completer {
	return a.completer
}

// AskPermission activates the permission dialog (called from the engine bridge).
func (a *App) AskPermission(toolName, desc string) {
	a.permission.Ask(toolName, desc)
}

// UpdateDiffStats updates the lines added/deleted counters for the status bar.
func (a *App) UpdateDiffStats(added, deleted int) {
	a.linesAdded += added
	a.linesDeleted += deleted
}

// SetModel updates the current model name (triggers attribution label on change).
func (a *App) SetModel(model string) {
	if a.model != model {
		a.prevModel = a.model
		a.model = model
	}
}

// ── Companion public API ─────────────────────────────────────────────────────

// SetCompanion updates the companion data for the sprite widget.
func (a *App) SetCompanion(c *buddy.Companion) {
	a.companionView.SetCompanion(c)
}

// SetCompanionReaction sets the companion's speech bubble text.
func (a *App) SetCompanionReaction(text string) {
	a.companionView.SetReaction(text)
}

// SetCompanionPetAt triggers the petting heart animation.
func (a *App) SetCompanionPetAt(ts int64) {
	a.companionView.SetPetAt(ts)
}

// SetCompanionMuted sets the companion muted state.
func (a *App) SetCompanionMuted(muted bool) {
	a.companionView.SetMuted(muted)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (a *App) View() string {
	w := a.layout.Width()
	if w == 0 {
		return "Initializing..."
	}

	header := a.renderStatusLine()

	// Spinner renders inline at the bottom of the body (claude-code-main style)
	body := a.viewport.View()
	if a.spinner.IsVisible() {
		body += "\n" + a.spinner.View()
	}

	input := a.renderInput()

	// P10: Render floating bubble above input in fullscreen mode
	floatingBubble := a.companionView.FloatingBubbleView()
	if floatingBubble != "" {
		input = floatingBubble + "\n" + input
	}

	footer := a.renderFooter()

	// Dynamically expand input region when autocomplete popup is visible,
	// so Compose/padToHeight doesn't clip the popup lines.
	if a.compState.Active && len(a.compState.Items) > 0 {
		popupLines := len(a.compState.Items)
		if popupLines > 8 {
			popupLines = 8
		}
		// popup border adds 2 lines (top+bottom)
		extra := popupLines + 2
		a.layout.SetInputHeight(a.layout.defaultInputHeight() + extra)
		// Shrink viewport to fit
		a.viewport.Height = a.layout.BodyHeight()
	} else if a.layout.InputHeight() != a.layout.defaultInputHeight() {
		a.layout.SetInputHeight(a.layout.defaultInputHeight())
		a.viewport.Height = a.layout.BodyHeight()
	}

	view := a.layout.Compose(header, body, input, footer)

	// Overlay permission dialog if visible
	if a.permission.IsVisible() {
		view += "\n" + a.permission.View()
	}

	return view
}

// ── Region renderers ─────────────────────────────────────────────────────────

func (a *App) renderStatusLine() string {
	w := a.layout.Width()
	sep := a.styles.Dimmed.Render(" · ")

	// ── Left: model · cost · context bar ──
	var leftParts []string
	leftParts = append(leftParts, a.styles.Highlight.Render(a.model))

	if a.costUSD > 0 {
		leftParts = append(leftParts, a.styles.Success.Render(formatStatusCost(a.costUSD)))
	}

	if a.contextPct > 0 {
		// Color thresholds: <70% blue, 70-90% warning, >90% error
		fillColor := a.themeData.Suggestion
		if a.contextPct > 0.9 {
			fillColor = a.themeData.Error
		} else if a.contextPct > 0.7 {
			fillColor = a.themeData.Warning
		}
		bar := designsystem.RenderProgressBar(a.contextPct, 8, fillColor, a.themeData.Subtle)
		label := a.styles.Dimmed.Render(fmt.Sprintf(" %d%%", int(a.contextPct*100)))
		leftParts = append(leftParts, bar+label)
	}

	left := strings.Join(leftParts, sep)

	// ── Right: mode · lines +/- · vim · turn · cwd ──
	var rightParts []string
	if a.permMode != "" && a.permMode != "default" {
		rightParts = append(rightParts, a.styles.Warning.Render(a.permMode))
	}
	if a.linesAdded > 0 || a.linesDeleted > 0 {
		diffStr := fmt.Sprintf("+%d -%d", a.linesAdded, a.linesDeleted)
		rightParts = append(rightParts, a.styles.Dimmed.Render(diffStr))
	}
	if a.vimState != nil && a.vimState.Enabled {
		rightParts = append(rightParts, a.styles.Dimmed.Render("VIM"))
	}
	if a.turnCount > 0 {
		rightParts = append(rightParts, a.styles.Dimmed.Render(fmt.Sprintf("turn %d", a.turnCount)))
	}
	if a.cwd != "" {
		rightParts = append(rightParts, a.styles.Dimmed.Render(shortenPath(a.cwd, 25)))
	}

	right := strings.Join(rightParts, sep)

	// Pad to full width
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := w - leftW - rightW - 2
	if gap < 1 {
		gap = 1
	}

	return a.styles.StatusBar.Width(w).Render(" " + left + strings.Repeat(" ", gap) + right + " ")
}

// formatStatusCost formats USD cost for the status bar.
func formatStatusCost(usd float64) string {
	if usd < 0.01 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}

func (a *App) renderInput() string {
	w := a.layout.BodyWidth()

	// P14: Hide companion when permission dialog or help overlay is showing
	// (matches claude-code-main: companionVisible = !toolJSX?.shouldHidePromptInput && !focusedInputDialog)
	companionVisible := a.companionView.IsVisible() && !a.permission.IsVisible() && !a.showHelp

	// Calculate input width, reserving space for companion sprite if visible.
	inputW := w
	speaking := companionVisible && a.companionView.IsSpeaking()
	reserved := companion.CompanionReservedColumns(w, speaking, a.companionView.NameWidth(), a.companionView.IsFullscreen())
	if companionVisible && reserved > 0 {
		inputW = w - reserved
		if inputW < 40 {
			inputW = w // fall back to full width if too narrow
			reserved = 0
		}
	}

	inputView := a.textarea.View()

	// P8: Rainbow highlight /buddy in input text (matches claude-code-main PromptInput.tsx getRainbowColor)
	if strings.Contains(a.textarea.Value(), "/buddy") {
		inputView = rainbowBuddyReplace(inputView)
	}

	// Wrap in rounded border without bottom (claude-code-main PromptInput style):
	//   ╭─────────────────╮
	//   │ > input text     │
	//   │                  │
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color.Resolve(a.themeData.PromptBorder)).
		BorderBottom(false).
		Width(inputW - 2) // content width; rendered = inputW (incl side borders)

	bordered := borderStyle.Render(inputView)

	// Show autocomplete popup above the input when active.
	if a.compState.Active && len(a.compState.Items) > 0 {
		popup := a.renderCompletionPopup(inputW)
		bordered = lipgloss.JoinVertical(lipgloss.Left, popup, bordered)
	}

	// Join companion sprite to the right of the input if visible and space allows.
	if companionVisible && reserved > 0 {
		spriteView := a.companionView.View()
		if spriteView != "" {
			bordered = lipgloss.JoinHorizontal(lipgloss.Bottom, bordered, "  ", spriteView)
		}
	}

	return bordered
}

// renderCompletionPopup draws the slash-command completion menu.
func (a *App) renderCompletionPopup(w int) string {
	maxShow := 8
	items := a.compState.Items
	if len(items) > maxShow {
		items = items[:maxShow]
	}

	var lines []string
	for i, item := range items {
		label := item.Label
		if item.Description != "" {
			label += "  " + a.styles.Dimmed.Render(item.Description)
		}
		if i == a.compState.Selected {
			label = a.styles.Highlight.Render("\u25b8 " + label)
		} else {
			label = "  " + label
		}
		lines = append(lines, label)
	}

	popup := strings.Join(lines, "\n")
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color.Resolve(a.themeData.Suggestion)).
		Width(w-4).
		Padding(0, 1)
	return popupStyle.Render(popup)
}

func (a *App) renderFooter() string {
	w := a.layout.Width()

	// Shortcut hints below input (claude-code-main style)
	if a.showHelp {
		helpLines := []string{}
		for _, row := range a.keymap.FullHelp() {
			var rowParts []string
			for _, b := range row {
				rowParts = append(rowParts, b.Help().Key+": "+b.Help().Desc)
			}
			helpLines = append(helpLines, strings.Join(rowParts, "  "))
		}
		return a.styles.Dimmed.Render("  " + strings.Join(helpLines, " │ "))
	}

	hint := "  ! for bash · /help · esc to interrupt"
	if a.spinner.IsVisible() {
		hint = ""
	}

	// P9: Teaser notification — show rainbow "/buddy" in footer during teaser window
	hasCompanion := a.companionView.IsVisible()
	teaserParts := buddy.TeaserRainbowParts(hasCompanion)
	if len(teaserParts) > 0 {
		var rb strings.Builder
		for _, cp := range teaserParts {
			s := lipgloss.NewStyle().Foreground(lipgloss.Color(cp.Color)).Bold(true)
			rb.WriteString(s.Render(string(cp.Char)))
		}
		teaserStr := "  Try " + rb.String() + " ✨"
		if hint != "" {
			hint = hint + " · " + teaserStr
		} else {
			hint = teaserStr
		}
	}

	_ = w
	return a.styles.Dimmed.Render(hint)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (a *App) reflow() {
	w := a.layout.BodyWidth()
	h := a.layout.BodyHeight()
	if w < 10 {
		w = 10
	}
	if h < 3 {
		h = 3
	}
	a.viewport.Width = w
	a.viewport.Height = h
	// Textarea width must fit inside border content area:
	// border Width(w-2) = content w-2, side borders take 2 more from visual width
	taWidth := w - 4
	if taWidth < 10 {
		taWidth = 10
	}
	a.textarea.SetWidth(taWidth)
	_ = a.md.Resize(w - 4)
	a.refreshViewport()
}

func (a *App) refreshViewport() {
	a.viewport.SetContent(a.renderMessages())
}

func (a *App) renderMessages() string {
	connector := a.styles.Connector.Render("  ⎿  ")

	var sb strings.Builder
	for i, m := range a.messages {
		var line string
		switch m.Role {
		case "user":
			// User messages: no dot prefix, just ❯ prompt
			line = a.styles.DotUser.Render("❯") + " " + m.Content

		case "assistant":
			// Model attribution: show model label when it changes
			if a.model != "" {
				modelShort := message.ShortenModelName(a.model)
				modelLabel := a.styles.Dimmed.Render(modelShort)
				line = modelLabel + "\n"
			}
			// Assistant text: render as Markdown, no dot prefix
			rendered := a.md.Render(m.Content)
			line += rendered

		case "system":
			line = a.styles.System.Render(m.Content)

		case "error":
			errDot := a.styles.Error.Render(figures.BlackCircle())
			line = errDot + " " + a.styles.Error.Render(m.Content)

		case "tool_use":
			// claude-code format: ● ToolName (params)
			toolDot := a.styles.Dot.Render(figures.BlackCircle()) + " "
			toolName := toolUserFacingName(m.ToolName)
			nameStyle := a.styles.ToolUse.Bold(true).Italic(false)
			line = toolDot + nameStyle.Render(toolName)
			if m.Content != "" {
				summary := truncateOutput(m.Content, 120)
				line += " " + a.styles.Dimmed.Render("("+summary+")")
			}
			// If permission dialog is visible and this is the last tool_use,
			// show "Waiting for permission…" below it
			if a.permission.IsVisible() && isLastToolUse(a.messages, i) {
				line += "\n" + connector + a.styles.Dimmed.Render("Waiting for permission…")
			}

		case "tool_result":
			// claude-code format:   ⎿  result content
			if m.IsError {
				line = connector + a.styles.Error.Render(truncateToolOutput(m.Content, 5))
			} else {
				line = connector + a.styles.Dimmed.Render(truncateToolOutput(m.Content, 10))
			}

		case "banner":
			line = m.Content

		default:
			line = m.Content
		}
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// isLastToolUse returns true if messages[idx] is the last "tool_use" message.
func isLastToolUse(messages []ChatMessage, idx int) bool {
	for j := idx + 1; j < len(messages); j++ {
		if messages[j].Role == "tool_use" {
			return false
		}
	}
	return true
}

// toolUserFacingName returns the user-facing tool name matching claude-code-main.
// This uses the short tool name ("Bash", "Read", "Update", etc.) not a description.
func toolUserFacingName(name string) string {
	switch name {
	case "Bash", "bash":
		return "Bash"
	case "Read", "read", "FileRead", "file_read":
		return "Read"
	case "Edit", "edit", "FileEdit", "file_edit":
		return "Update"
	case "Write", "write", "FileWrite", "file_write":
		return "Write"
	case "Glob", "glob":
		return "Search"
	case "Grep", "grep":
		return "Grep"
	case "ListDir", "list_dir":
		return "LS"
	case "WebSearch", "web_search":
		return "WebSearch"
	case "WebFetch", "web_fetch":
		return "WebFetch"
	default:
		return name
	}
}

// truncateToolOutput shortens multi-line tool output to maxLines.
func truncateToolOutput(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	result := strings.Join(lines[:maxLines], "\n")
	return result + fmt.Sprintf("\n… (%d more lines)", len(lines)-maxLines)
}

// indentWithConnector prepends the connector prefix to each line.
func indentWithConnector(text, connector string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = connector + l
	}
	return strings.Join(lines, "\n")
}

// AddSystemMessage appends a system-level notification to the message list.
func (a *App) AddSystemMessage(text string) {
	a.messages = append(a.messages, ChatMessage{Role: "system", Content: text})
	a.refreshViewport()
	a.viewport.GotoBottom()
}

// truncateOutput shortens tool output for display.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// searchMessages searches the message history for a query string.
func (a *App) searchMessages(query string) []search.Hit {
	query = strings.ToLower(query)
	var hits []search.Hit
	for i, m := range a.messages {
		if strings.Contains(strings.ToLower(m.Content), query) {
			hits = append(hits, search.Hit{
				MessageIdx: i,
				Context:    truncateOutput(m.Content, 80),
			})
		}
	}
	return hits
}

// handleVimAction processes a vim action and applies it to the app state.
func (a *App) handleVimAction(action vim.Action) {
	switch action.Type {
	case vim.ActionInsertMode, vim.ActionAppendMode,
		vim.ActionInsertLineStart, vim.ActionAppendLineEnd,
		vim.ActionNewLineBelow, vim.ActionNewLineAbove:
		a.textarea.Focus()
	case vim.ActionMoveUp:
		a.viewport.LineUp(action.Count)
	case vim.ActionMoveDown:
		a.viewport.LineDown(action.Count)
	case vim.ActionMoveDocTop:
		a.viewport.GotoTop()
	case vim.ActionMoveDocBottom:
		a.viewport.GotoBottom()
	case vim.ActionSearch:
		a.searchBar.Show()
	case vim.ActionExecCommand:
		// Handle :q, :w, etc.
		switch action.Command {
		case "q", "quit":
			// Will be handled by the caller checking for quit
		case "w", "write":
			// placeholder for session save
		}
	}
}

// rainbowBuddyReplace replaces literal "/buddy" in rendered text with rainbow-colored version.
// Matches claude-code-main PromptInput.tsx getRainbowColor per-character styling.
func rainbowBuddyReplace(view string) string {
	const trigger = "/buddy"
	rainbowColors := []string{
		"#ff0000", "#ff8800", "#ffff00", "#00ff00", "#0088ff", "#8800ff", "#ff00ff",
	}
	var rainbow strings.Builder
	for i, ch := range trigger {
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(rainbowColors[i%len(rainbowColors)])).Bold(true)
		rainbow.WriteString(s.Render(string(ch)))
	}
	return strings.ReplaceAll(view, trigger, rainbow.String())
}

// shortenPath truncates a path for display.
func shortenPath(p string, maxLen int) string {
	if len(p) <= maxLen {
		return p
	}
	return "…" + p[len(p)-maxLen+1:]
}
