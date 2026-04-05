package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d77757")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d77757"))

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6495ed")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff6b80")).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#666666"))

	connectorStyle  = lipgloss.NewStyle().Faint(true)
	toolResultStyle = lipgloss.NewStyle().Faint(true)
)

// ── Message types ─────────────────────────────────────────────────────────────

// ChatMessage is a single entry in the displayed conversation.
type ChatMessage struct {
	Role     string // "user" | "assistant" | "system" | "error" | "tool_use" | "tool_result" | "banner"
	Content  string
	ToolName string // for tool_use / tool_result messages
	IsError  bool   // for tool_result errors
}

// ── Bubbletea messages ────────────────────────────────────────────────────────

// StreamTextMsg carries a streaming text delta from the engine.
type StreamTextMsg struct{ Text string }

// StreamDoneMsg signals that the current engine turn has finished.
type StreamDoneMsg struct{}

// StreamErrorMsg carries an error from the engine.
type StreamErrorMsg struct{ Err error }

// CostUpdateMsg carries updated cost/token info for the status bar.
type CostUpdateMsg struct {
	CostUSD     float64
	InputTokens int
	TurnCount   int
}

// ToolStartMsg signals that a tool call has started.
type ToolStartMsg struct {
	ToolID   string
	ToolName string
	Input    string
}

// ToolDoneMsg signals that a tool call has completed.
type ToolDoneMsg struct {
	ToolID  string
	Output  string
	IsError bool
}

// SystemMsg carries a system-level message for display.
type SystemMsg struct {
	Text string
}

// ── Companion Bubbletea messages ─────────────────────────────────────────────

// CompanionLoadMsg signals that a companion has been loaded/hatched.
type CompanionLoadMsg struct {
	Companion interface{} // *buddy.Companion — interface to avoid import cycle
}

// CompanionReactionMsg sets the companion speech bubble text.
type CompanionReactionMsg struct {
	Text string
}

// CompanionPetMsg triggers the pet heart animation.
type CompanionPetMsg struct {
	Timestamp int64 // Unix milliseconds
}

// CompanionMuteMsg sets the companion muted state.
type CompanionMuteMsg struct {
	Muted bool
}

// TeaserExpiredMsg signals the teaser notification should be hidden.
type TeaserExpiredMsg struct{}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the top-level Bubbletea model for the agent TUI.
type Model struct {
	// viewport displays the message history.
	viewport viewport.Model
	// textarea is the multi-line input area.
	textarea textarea.Model

	messages  []ChatMessage
	status    string
	width     int
	height    int
	streaming bool

	// SubmitFn is called when the user submits a message.
	// It should start a goroutine and send StreamTextMsg / StreamDoneMsg /
	// StreamErrorMsg back to the program via program.Send.
	SubmitFn func(text string)
}

// New creates a new TUI Model with default dimensions.
func New(submitFn func(string)) Model {
	ta := textarea.New()
	ta.Placeholder = "Reply to Claude…"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("")

	return Model{
		viewport: vp,
		textarea: ta,
		status:   "Ready",
		width:    80,
		height:   26,
		SubmitFn: submitFn,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		taCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if msg.Alt {
				// Shift/Alt+Enter → newline in textarea.
				m.textarea, taCmd = m.textarea.Update(msg)
				return m, taCmd
			}
			// Plain Enter → submit.
			text := strings.TrimSpace(m.textarea.Value())
			if text == "" {
				return m, nil
			}
			m.messages = append(m.messages, ChatMessage{Role: "user", Content: text})
			m.textarea.Reset()
			m.status = "Thinking…"
			m.streaming = true
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			if m.SubmitFn != nil {
				m.SubmitFn(text)
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		inputH := 5
		statusH := 1
		vpH := m.height - inputH - statusH - 4 // borders
		if vpH < 5 {
			vpH = 5
		}
		m.viewport.Width = m.width - 4
		m.viewport.Height = vpH
		m.textarea.SetWidth(m.width - 4)
		m.viewport.SetContent(m.renderMessages())

	// ── Streaming engine events ───────────────────────────────────────────

	case StreamTextMsg:
		// Append the delta to the last assistant message, or start a new one.
		if len(m.messages) == 0 || m.messages[len(m.messages)-1].Role != "assistant" {
			m.messages = append(m.messages, ChatMessage{Role: "assistant"})
		}
		m.messages[len(m.messages)-1].Content += msg.Text
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case StreamDoneMsg:
		m.status = "Ready"
		m.streaming = false

	case StreamErrorMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:    "error",
			Content: "Error: " + msg.Err.Error(),
		})
		m.status = "Error"
		m.streaming = false
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	m.textarea, taCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, tea.Batch(taCmd, vpCmd)
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	vpBlock := borderStyle.
		Width(m.width - 2).
		Render(m.viewport.View())

	// Input: top-only round border (claude-code-main style)
	inputBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#666666")).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		Width(m.width - 2)
	inputBlock := inputBorder.Render(m.textarea.View())

	statusBar := statusStyle.
		Width(m.width).
		Render(" " + m.status)

	return lipgloss.JoinVertical(lipgloss.Left,
		vpBlock,
		inputBlock,
		statusBar,
	)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *Model) renderMessages() string {
	dot := assistantStyle.Render("●")
	connector := connectorStyle.Render("  ⎿  ")

	var sb strings.Builder
	for _, msg := range m.messages {
		var line string
		switch msg.Role {
		case "user":
			line = userStyle.Render("❯") + " " + msg.Content
		case "assistant":
			// First line gets ●, subsequent lines get ⎿ connector
			parts := strings.SplitN(msg.Content, "\n", 2)
			if len(parts) > 1 {
				indented := modelIndentConnector(parts[1], connector)
				line = dot + " " + parts[0] + "\n" + indented
			} else {
				line = dot + " " + msg.Content
			}
		case "system":
			line = systemStyle.Render("▶ " + msg.Content)
		case "error":
			line = errorStyle.Render("⚠ " + msg.Content)
		case "tool_use":
			line = dot + " " + assistantStyle.Render(msg.Content)
		case "tool_result":
			line = connector + toolResultStyle.Render(msg.Content)
		default:
			line = msg.Content
		}
		sb.WriteString(line)
		sb.WriteString("\n\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// modelIndentConnector prepends the connector prefix to each line.
func modelIndentConnector(text, connector string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = connector + l
	}
	return strings.Join(lines, "\n")
}

// AddSystemMessage appends a system-level notification to the message list.
func (m *Model) AddSystemMessage(text string) {
	m.messages = append(m.messages, ChatMessage{Role: "system", Content: text})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
}
