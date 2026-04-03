package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wall-ai/agent-engine/internal/tui/search"
	sess "github.com/wall-ai/agent-engine/internal/tui/session"
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
	theme      Theme
	keymap     KeyMap
	showHelp   bool
	isLoading  bool
	screenMode ScreenMode

	// Advanced sub-models
	vimState   *vim.VimState
	searchBar  *search.Overlay
	toolTrack  *ToolUseTracker
	transcript *sess.TranscriptView
	sessStore  *sess.SessionStore

	// Status line data
	model       string
	costUSD     float64
	contextPct  float64
	permMode    string
	cwd         string
	turnCount   int
	inputTokens int

	// Timing
	loadingStart time.Time

	// SubmitFn is called when the user sends a message.
	SubmitFn func(text string)
}

// AppConfig holds configuration for creating a new App.
type AppConfig struct {
	Dark           bool
	Model          string
	PermissionMode string
	WorkDir        string
	SubmitFn       func(text string)
}

// NewApp creates a fully initialised full-screen App.
func NewApp(cfg AppConfig) (*App, error) {
	theme := DefaultDarkTheme()
	if !cfg.Dark {
		theme = DefaultLightTheme()
	}
	km := DefaultKeyMap()

	ta := textarea.New()
	ta.Placeholder = "Type a message… (Enter to send, Ctrl+C to quit)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // unlimited

	vp := viewport.New(80, 20)
	vp.SetContent("")

	mdRenderer, err := NewMarkdownRenderer(76, cfg.Dark)
	if err != nil {
		return nil, err
	}

	return &App{
		screen:     NewScreenManager(),
		layout:     NewLayout(80, 24),
		viewport:   vp,
		textarea:   ta,
		spinner:    NewSpinner(theme),
		permission: NewPermissionModel(theme, km),
		md:         mdRenderer,
		vimState:   vim.New(),
		searchBar:  search.NewOverlay(80),
		toolTrack:  NewToolUseTracker(theme),
		transcript: sess.NewTranscriptView(80, 20),
		sessStore:  sess.NewSessionStore(""),
		status:     "Ready",
		theme:      theme,
		keymap:     km,
		screenMode: ScreenPrompt,
		model:      cfg.Model,
		permMode:   cfg.PermissionMode,
		cwd:        cfg.WorkDir,
		SubmitFn:   cfg.SubmitFn,
	}, nil
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		textarea.Blink,
		a.spinner.Init(),
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
			a.messages = append(a.messages, ChatMessage{Role: "user", Content: text})
			a.textarea.Reset()
			a.status = "Thinking…"
			a.isLoading = true
			a.loadingStart = time.Now()
			a.spinner.Show("thinking…")
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
		a.spinner.Hide()

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
			Role:    "tool_use",
			Content: msg.ToolName + " " + msg.Input,
		})
		a.toolTrack.StartTool(msg.ToolID, msg.ToolName, msg.Input)
		a.spinner.Show(msg.ToolName + "…")
		a.transcript.Append(sess.TranscriptEntry{
			Timestamp: time.Now(), Role: "tool_use",
			ToolName: msg.ToolName, Content: msg.Input,
		})
		a.refreshViewport()
		a.viewport.GotoBottom()

	case ToolDoneMsg:
		prefix := "  ⎿ "
		if msg.IsError {
			prefix = "  ✗ "
		}
		a.messages = append(a.messages, ChatMessage{
			Role:    "tool_result",
			Content: prefix + truncateOutput(msg.Output, 200),
		})
		a.toolTrack.FinishTool(msg.ToolID, msg.Output, msg.IsError)
		a.spinner.Show("thinking…")
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

	case PermissionAnswerMsg:
		// Answered — engine handles this via callback.

	case CostUpdateMsg:
		a.costUSD = msg.CostUSD
		a.inputTokens = msg.InputTokens
		a.turnCount = msg.TurnCount
	}

	a.textarea, taCmd = a.textarea.Update(msg)
	a.viewport, vpCmd = a.viewport.Update(msg)
	a.spinner, spCmd = a.spinner.Update(msg)
	cmds = append(cmds, taCmd, vpCmd, spCmd)
	return a, tea.Batch(cmds...)
}

// AskPermission activates the permission dialog (called from the engine bridge).
func (a *App) AskPermission(toolName, desc string) {
	a.permission.Ask(toolName, desc)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (a *App) View() string {
	w := a.layout.Width()
	if w == 0 {
		return "Initializing..."
	}

	header := a.renderStatusLine()
	body := a.viewport.View()
	input := a.renderInput()
	footer := a.renderFooter()

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

	// Left side: model + cost
	left := a.theme.Highlight.Render(a.model)
	if a.costUSD > 0 {
		left += a.theme.Dimmed.Render(fmt.Sprintf(" · $%.2f", a.costUSD))
	}

	// Right side: permission mode + cwd
	right := a.theme.Dimmed.Render(a.permMode)
	if a.cwd != "" {
		short := shortenPath(a.cwd, 30)
		right += a.theme.Dimmed.Render(" · " + short)
	}

	// Pad to full width
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := w - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	return a.theme.StatusBar.Width(w).Render(left + strings.Repeat(" ", gap) + right)
}

func (a *App) renderInput() string {
	return a.textarea.View()
}

func (a *App) renderFooter() string {
	w := a.layout.Width()

	var parts []string

	// Spinner or status
	if a.spinner.IsVisible() {
		elapsed := time.Since(a.loadingStart).Truncate(time.Second)
		parts = append(parts, a.spinner.View()+a.theme.Dimmed.Render(fmt.Sprintf(" (%s)", elapsed)))
	} else {
		parts = append(parts, a.theme.Dimmed.Render(a.status))
	}

	// Help hint
	if a.showHelp {
		helpLines := []string{}
		for _, row := range a.keymap.FullHelp() {
			var rowParts []string
			for _, b := range row {
				rowParts = append(rowParts, b.Help().Key+": "+b.Help().Desc)
			}
			helpLines = append(helpLines, strings.Join(rowParts, "  "))
		}
		parts = append(parts, a.theme.Dimmed.Render(strings.Join(helpLines, " | ")))
	} else {
		parts = append(parts, a.theme.Dimmed.Render("?: help  ctrl+c: quit  ctrl+k: compact  ctrl+o: transcript"))
	}

	line := strings.Join(parts, "  ")

	return a.theme.StatusBar.Width(w).Render(line)
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
	a.textarea.SetWidth(w)
	_ = a.md.Resize(w - 4)
	a.refreshViewport()
}

func (a *App) refreshViewport() {
	a.viewport.SetContent(a.renderMessages())
}

func (a *App) renderMessages() string {
	var sb strings.Builder
	for _, m := range a.messages {
		var line string
		switch m.Role {
		case "user":
			line = a.theme.User.Render("❯ You: ") + m.Content
		case "assistant":
			rendered := a.md.Render(m.Content)
			line = a.theme.Assistant.Render("⏺ Assistant:") + "\n" + rendered
		case "system":
			line = a.theme.System.Render("▶ " + m.Content)
		case "error":
			line = a.theme.Error.Render("⚠ " + m.Content)
		case "tool_use":
			line = a.theme.ToolUse.Render("⚙ " + m.Content)
		case "tool_result":
			line = a.theme.ToolResult.Render("  ⎿ " + m.Content)
		default:
			line = m.Content
		}
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}
	return strings.TrimRight(sb.String(), "\n")
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

// shortenPath truncates a path for display.
func shortenPath(p string, maxLen int) string {
	if len(p) <= maxLen {
		return p
	}
	return "…" + p[len(p)-maxLen+1:]
}
