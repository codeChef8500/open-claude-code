package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// App is the top-level Bubbletea model.  It composes:
//   - viewport  (message history)
//   - textarea  (multi-line input)
//   - SpinnerModel (thinking indicator)
//   - PermissionModel (permission confirmation dialog)
//   - MarkdownRenderer (for assistant messages)
type App struct {
	viewport   viewport.Model
	textarea   textarea.Model
	spinner    SpinnerModel
	permission PermissionModel
	md         *MarkdownRenderer

	messages []ChatMessage
	status   string
	width    int
	height   int
	theme    Theme
	keymap   KeyMap
	showHelp bool

	// SubmitFn is called when the user sends a message.
	SubmitFn func(text string)
}

// NewApp creates a fully initialised App.
func NewApp(dark bool, submitFn func(string)) (*App, error) {
	theme := DefaultDarkTheme()
	if !dark {
		theme = DefaultLightTheme()
	}
	km := DefaultKeyMap()

	ta := textarea.New()
	ta.Placeholder = "Type a message… (Enter to send)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("")

	mdRenderer, err := NewMarkdownRenderer(76, dark)
	if err != nil {
		return nil, err
	}

	return &App{
		viewport:   vp,
		textarea:   ta,
		spinner:    NewSpinner(theme),
		permission: NewPermissionModel(theme, km),
		md:         mdRenderer,
		status:     "Ready",
		width:      80,
		height:     26,
		theme:      theme,
		keymap:     km,
		SubmitFn:   submitFn,
	}, nil
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (a *App) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, a.spinner.Init())
}

// ── Update ────────────────────────────────────────────────────────────────────

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd  tea.Cmd
		vpCmd  tea.Cmd
		spCmd  tea.Cmd
		cmds   []tea.Cmd
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
		switch {
		case msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc:
			return a, tea.Quit

		case msg.Type == tea.KeyEnter && !msg.Alt:
			text := strings.TrimSpace(a.textarea.Value())
			if text == "" {
				return a, nil
			}
			a.messages = append(a.messages, ChatMessage{Role: "user", Content: text})
			a.textarea.Reset()
			a.status = "Thinking…"
			a.spinner.Show("thinking…")
			a.refreshViewport()
			a.viewport.GotoBottom()
			if a.SubmitFn != nil {
				a.SubmitFn(text)
			}
			return a, a.spinner.Init()

		case msg.String() == "?":
			a.showHelp = !a.showHelp

		case msg.Type == tea.KeyCtrlK:
			a.messages = append(a.messages, ChatMessage{Role: "system", Content: "Compacting context…"})
			a.refreshViewport()
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
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
		a.spinner.Hide()

	case StreamErrorMsg:
		a.messages = append(a.messages, ChatMessage{
			Role:    "error",
			Content: msg.Err.Error(),
		})
		a.status = "Error"
		a.spinner.Hide()
		a.refreshViewport()
		a.viewport.GotoBottom()

	case PermissionAnswerMsg:
		// Answered — engine handles this via SubmitFn callback.
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
	vpBlock := a.theme.Border.
		Width(a.width - 2).
		Render(a.viewport.View())

	inputBlock := a.theme.Border.
		Width(a.width - 2).
		Render(a.textarea.View())

	statusText := a.status
	if a.spinner.IsVisible() {
		statusText = a.spinner.View()
	}
	statusBar := a.theme.StatusBar.
		Width(a.width).
		Render(" " + statusText)

	layers := []string{vpBlock, inputBlock, statusBar}

	if a.permission.IsVisible() {
		overlay := a.permission.View()
		layers = append(layers, overlay)
	}

	if a.showHelp {
		helpLines := []string{}
		for _, row := range a.keymap.FullHelp() {
			var parts []string
			for _, b := range row {
				parts = append(parts, b.Help().Key+": "+b.Help().Desc)
			}
			helpLines = append(helpLines, strings.Join(parts, "   "))
		}
		helpBlock := a.theme.Dimmed.Render(strings.Join(helpLines, "\n"))
		layers = append(layers, helpBlock)
	}

	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (a *App) reflow() {
	inputH := 5
	statusH := 1
	vpH := a.height - inputH - statusH - 4
	if vpH < 5 {
		vpH = 5
	}
	a.viewport.Width = a.width - 4
	a.viewport.Height = vpH
	a.textarea.SetWidth(a.width - 4)
	_ = a.md.Resize(a.width - 8)
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
			line = a.theme.User.Render("You: ") + m.Content
		case "assistant":
			rendered := a.md.Render(m.Content)
			line = a.theme.Assistant.Render("Assistant:") + "\n" + rendered
		case "system":
			line = a.theme.System.Render("▶ " + m.Content)
		case "error":
			line = a.theme.Error.Render("⚠ " + m.Content)
		default:
			line = m.Content
		}
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}
