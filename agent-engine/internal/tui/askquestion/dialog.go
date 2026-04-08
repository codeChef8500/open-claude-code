package askquestion

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ──────────────────────────────────────────────────────────────────────────────
// AskQuestionDialog — the main Bubble Tea model that orchestrates
// the multi-question interactive dialog.
//
// It combines:
//   - MultiChoiceState  (state machine)
//   - NavBar            (tab navigation)
//   - QuestionView      (single/multi-select)
//   - PreviewQuestionView (side-by-side when preview present)
//   - SubmitQuestionsView (final review)
//
// The dialog is driven entirely by keyboard input.
// ──────────────────────────────────────────────────────────────────────────────

// DialogResult is sent via tea.Msg when the user completes or cancels the dialog.
type DialogResult struct {
	Response AskQuestionResponse
}

// AskQuestionDialog is the top-level Bubble Tea model.
type AskQuestionDialog struct {
	state *MultiChoiceState

	// Per-question UI state
	focusedIndex int       // within current question's options
	focusArea    FocusArea // which section has focus
	footerIndex  int       // footer item index
	textInput    string    // "Other" text or notes text
	notesCursor  int

	// Display dimensions
	width  int
	height int

	// Configuration
	showPlanMode bool
	planFilePath string
	editorName   string

	// Styles
	navStyles NavBarStyles
	qvStyles  QuestionViewStyles
	svStyles  SubmitViewStyles
	pvStyles  PreviewViewStyles
	pbStyles  PreviewBoxStyles

	// Submit view state
	submitFocusIdx int

	// Image paste support
	imageStore *ImagePasteStore

	// Whether the dialog is active
	visible bool

	// Result channel — closed when the dialog finishes
	resultCh chan<- AskQuestionResponse
}

// NewAskQuestionDialog creates a new dialog for the given questions.
func NewAskQuestionDialog(
	questions []Question,
	resultCh chan<- AskQuestionResponse,
) *AskQuestionDialog {
	return &AskQuestionDialog{
		state:      NewMultiChoiceState(questions),
		navStyles:  DefaultNavBarStyles(),
		qvStyles:   DefaultQuestionViewStyles(),
		svStyles:   DefaultSubmitViewStyles(),
		pvStyles:   DefaultPreviewViewStyles(),
		pbStyles:   DefaultPreviewBoxStyles(),
		imageStore: NewImagePasteStore(),
		visible:    true,
		resultCh:   resultCh,
		width:      80,
		height:     24,
	}
}

// SetDimensions updates the available terminal dimensions.
func (d *AskQuestionDialog) SetDimensions(w, h int) {
	d.width = w
	d.height = h
}

// SetPlanMode enables plan mode footer options.
func (d *AskQuestionDialog) SetPlanMode(filePath string) {
	d.showPlanMode = true
	d.planFilePath = filePath
}

// SetEditorName sets the name of the external editor for hints.
func (d *AskQuestionDialog) SetEditorName(name string) {
	d.editorName = name
}

// IsVisible reports whether the dialog is active.
func (d *AskQuestionDialog) IsVisible() bool { return d.visible }

// ── Bubble Tea interface ────────────────────────────────────────────────────

// Init returns no initial command.
func (d *AskQuestionDialog) Init() tea.Cmd { return nil }

// Update handles key events.
func (d *AskQuestionDialog) Update(msg tea.Msg) (*AskQuestionDialog, tea.Cmd) {
	if !d.visible {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		return d, nil

	case tea.KeyMsg:
		return d.handleKey(msg)
	}

	return d, nil
}

// View renders the dialog.
func (d *AskQuestionDialog) View() string {
	if !d.visible {
		return ""
	}

	var sb strings.Builder

	// Top divider
	divStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(divStyle.Render(strings.Repeat("─", min(d.width, 80))))
	sb.WriteString("\n")

	// Navigation bar
	navbar := RenderNavBar(
		d.state.Questions,
		d.state.CurrentQuestionIndex,
		d.state.Answers,
		d.state.HideSubmitTab(),
		d.width,
		d.navStyles,
	)
	if navbar != "" {
		sb.WriteString(navbar)
		sb.WriteString("\n\n")
	}

	// Plan mode indicator
	if d.showPlanMode && d.planFilePath != "" {
		planStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
		sb.WriteString(planStyle.Render("Planning: " + d.planFilePath))
		sb.WriteString("\n\n")
	}

	// Main content area
	if d.state.IsOnSubmitPage() {
		sb.WriteString(d.renderSubmitPage())
	} else {
		sb.WriteString(d.renderQuestionPage())
	}

	return sb.String()
}

// ── Key handling ────────────────────────────────────────────────────────────

func (d *AskQuestionDialog) handleKey(msg tea.KeyMsg) (*AskQuestionDialog, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case "esc":
		return d, d.finish(AskQuestionResponse{Cancelled: true})

	case "tab":
		// Move to next question / submit page
		d.state.NextQuestion()
		d.resetFocus()
		return d, nil

	case "shift+tab":
		// Move to previous question
		d.state.PrevQuestion()
		d.resetFocus()
		return d, nil
	}

	// Delegate to current page handler
	if d.state.IsOnSubmitPage() {
		return d.handleSubmitKey(msg)
	}

	// Text input mode
	if d.focusArea == FocusOther || d.focusArea == FocusNotes {
		return d.handleTextInputKey(msg)
	}

	return d.handleQuestionKey(msg)
}

func (d *AskQuestionDialog) handleQuestionKey(msg tea.KeyMsg) (*AskQuestionDialog, tea.Cmd) {
	q := d.state.CurrentQuestion()
	if q == nil {
		return d, nil
	}

	key := msg.String()
	allOptsLen := len(q.Options) + 1 // +1 for "Other"
	footerCount := d.footerItemCount()

	switch key {
	case "up", "k":
		if d.focusArea == FocusFooter {
			if d.footerIndex > 0 {
				d.footerIndex--
			} else {
				// Move back to options
				d.focusArea = FocusOptions
				if q.MultiSelect {
					d.focusedIndex = allOptsLen // the "Next"/"Submit" button
				} else {
					d.focusedIndex = allOptsLen - 1
				}
			}
		} else if d.focusArea == FocusOptions {
			if d.focusedIndex > 0 {
				d.focusedIndex--
			}
		}

	case "down", "j":
		if d.focusArea == FocusOptions {
			maxIdx := allOptsLen - 1
			if q.MultiSelect {
				maxIdx = allOptsLen // includes "Next" button
			}
			if d.focusedIndex < maxIdx {
				d.focusedIndex++
			} else {
				// Move to footer
				d.focusArea = FocusFooter
				d.footerIndex = 0
			}
		} else if d.focusArea == FocusFooter {
			if d.footerIndex < footerCount-1 {
				d.footerIndex++
			}
		}

	case "n":
		// Focus notes (preview mode only)
		if d.state.HasPreview() {
			d.focusArea = FocusNotes
			return d, nil
		}

	case " ":
		// Space: toggle multi-select or select in single-select
		if q.MultiSelect && d.focusArea == FocusOptions && d.focusedIndex < len(q.Options) {
			label := q.Options[d.focusedIndex].Label
			d.state.ToggleMultiSelect(q.QuestionText, label)
		}

	case "enter":
		return d.handleEnter()

	case "1", "2", "3", "4", "5":
		// Number shortcut to select option
		idx := int(key[0]-'0') - 1
		if idx >= 0 && idx < len(q.Options) {
			d.focusedIndex = idx
			if !q.MultiSelect {
				return d.selectCurrentOption()
			} else {
				label := q.Options[idx].Label
				d.state.ToggleMultiSelect(q.QuestionText, label)
			}
		}
	}

	return d, nil
}

func (d *AskQuestionDialog) handleEnter() (*AskQuestionDialog, tea.Cmd) {
	q := d.state.CurrentQuestion()
	if q == nil {
		return d, nil
	}

	switch d.focusArea {
	case FocusOptions:
		if q.MultiSelect {
			allOptsLen := len(q.Options) + 1
			if d.focusedIndex == allOptsLen {
				// "Next"/"Submit" button — require at least one selection
				qs := d.state.GetQuestionState(q.QuestionText)
				if len(qs.SelectedValues) == 0 {
					// No selections yet — ignore the press
					return d, nil
				}
				d.state.SetMultiSelectAnswer(q.QuestionText, qs.SelectedValues)
				if d.state.ShouldAutoSubmit() {
					return d, d.submit()
				}
				d.state.NextQuestion()
				d.resetFocus()
				return d, nil
			}
			if d.focusedIndex < len(q.Options) {
				label := q.Options[d.focusedIndex].Label
				d.state.ToggleMultiSelect(q.QuestionText, label)
			} else {
				// "Other" selected in multi-select
				d.focusArea = FocusOther
			}
		} else {
			return d.selectCurrentOption()
		}

	case FocusFooter:
		if d.footerIndex == 0 {
			// "Chat about this" — user wants to discuss rather than answer
			feedback := d.buildFeedbackSummary("The user chose to \"Respond to Claude\" instead of answering the questions.")
			return d, d.finish(AskQuestionResponse{
				RespondToClaude: true,
				Feedback:        feedback,
				Answers:         d.state.Answers,
				Annotations:     d.state.Annotations,
				PastedContents:  d.imageStore.All(),
			})
		} else if d.footerIndex == 1 && d.showPlanMode {
			// "Skip interview and plan immediately"
			feedback := d.buildFeedbackSummary("The user chose to \"Finish Plan Interview\" and skip the remaining questions. Proceed with the plan using the answers provided so far.")
			return d, d.finish(AskQuestionResponse{
				FinishInterview: true,
				Feedback:        feedback,
				Answers:         d.state.Answers,
				Annotations:     d.state.Annotations,
				PastedContents:  d.imageStore.All(),
			})
		}
	}

	return d, nil
}

func (d *AskQuestionDialog) selectCurrentOption() (*AskQuestionDialog, tea.Cmd) {
	q := d.state.CurrentQuestion()
	if q == nil {
		return d, nil
	}

	allOpts := len(q.Options)
	if d.focusedIndex < allOpts {
		// Selected a predefined option
		label := q.Options[d.focusedIndex].Label
		d.state.SetAnswer(q.QuestionText, label, false)

		// Set annotation if preview present
		if q.Options[d.focusedIndex].Preview != "" {
			qs := d.state.GetQuestionState(q.QuestionText)
			d.state.SetAnnotation(q.QuestionText, Annotation{
				Preview: q.Options[d.focusedIndex].Preview,
				Notes:   qs.TextInputValue,
			})
		}

		// Single question + non-multiSelect → auto-submit
		if d.state.ShouldAutoSubmit() {
			return d, d.submit()
		}

		d.state.NextQuestion()
		d.resetFocus()
	} else {
		// "Other" option → enter text input
		d.focusArea = FocusOther
	}

	return d, nil
}

func (d *AskQuestionDialog) handleTextInputKey(msg tea.KeyMsg) (*AskQuestionDialog, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		q := d.state.CurrentQuestion()
		if q == nil {
			return d, nil
		}
		if d.focusArea == FocusOther {
			if d.textInput != "" {
				d.state.SetAnswer(q.QuestionText, d.textInput, false)
				if d.state.ShouldAutoSubmit() {
					return d, d.submit()
				}
				d.state.NextQuestion()
				d.resetFocus()
			}
		} else if d.focusArea == FocusNotes {
			// Exit notes mode
			qs := d.state.GetQuestionState(q.QuestionText)
			qs.TextInputValue = d.textInput
			d.focusArea = FocusOptions
		}
		return d, nil

	case "esc":
		// Exit text input mode
		d.focusArea = FocusOptions
		return d, nil

	case "backspace":
		if len(d.textInput) > 0 {
			_, size := utf8.DecodeLastRuneInString(d.textInput)
			d.textInput = d.textInput[:len(d.textInput)-size]
		}
		return d, nil

	default:
		// Append printable character
		var added string
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			added = key
		} else if msg.Type == tea.KeyRunes {
			added = string(msg.Runes)
		}
		if added != "" {
			// Detect image paste (large base64 blocks)
			if img := DetectBase64Image(added); img != nil {
				if q := d.state.CurrentQuestion(); q != nil {
					d.imageStore.Add(q.QuestionText, *img)
				}
			} else {
				d.textInput += added
			}
		}
		return d, nil
	}
}

func (d *AskQuestionDialog) handleSubmitKey(msg tea.KeyMsg) (*AskQuestionDialog, tea.Cmd) {
	key := msg.String()

	switch key {
	case "up", "k":
		if d.submitFocusIdx > 0 {
			d.submitFocusIdx--
		}

	case "down", "j":
		if d.submitFocusIdx < 1 {
			d.submitFocusIdx++
		}

	case "enter":
		if d.submitFocusIdx == 0 {
			// Submit
			return d, d.submit()
		}
		// Cancel
		return d, d.finish(AskQuestionResponse{Cancelled: true})
	}

	return d, nil
}

// buildFeedbackSummary constructs a descriptive feedback string that includes
// the action context plus any partial answers the user provided.
func (d *AskQuestionDialog) buildFeedbackSummary(action string) string {
	var sb strings.Builder
	sb.WriteString(action)

	// Include any partial answers so the LLM knows what was answered.
	answeredCount := 0
	for _, q := range d.state.Questions {
		if ans, ok := d.state.Answers[q.QuestionText]; ok && ans != "" {
			answeredCount++
		}
	}
	if answeredCount > 0 {
		sb.WriteString(fmt.Sprintf("\n\nPartial answers provided (%d of %d questions):", answeredCount, len(d.state.Questions)))
		for _, q := range d.state.Questions {
			if ans, ok := d.state.Answers[q.QuestionText]; ok && ans != "" {
				sb.WriteString(fmt.Sprintf("\n- %s: %s", q.QuestionText, ans))
			}
		}
	}

	return sb.String()
}

// ── Submit / finish helpers ─────────────────────────────────────────────────

func (d *AskQuestionDialog) submit() tea.Cmd {
	return d.finish(AskQuestionResponse{
		Answers:        d.state.Answers,
		Annotations:    d.state.Annotations,
		PastedContents: d.imageStore.All(),
	})
}

func (d *AskQuestionDialog) finish(resp AskQuestionResponse) tea.Cmd {
	d.visible = false
	ch := d.resultCh
	return func() tea.Msg {
		if ch != nil {
			ch <- resp
		}
		return DialogResult{Response: resp}
	}
}

// ── Focus management ────────────────────────────────────────────────────────

func (d *AskQuestionDialog) resetFocus() {
	d.focusedIndex = 0
	d.focusArea = FocusOptions
	d.footerIndex = 0
	d.textInput = ""
	d.submitFocusIdx = 0

	// Pre-populate text input from notes if in preview mode
	if q := d.state.CurrentQuestion(); q != nil {
		qs := d.state.GetQuestionState(q.QuestionText)
		if qs.TextInputValue != "" {
			d.textInput = qs.TextInputValue
		}
	}
}

func (d *AskQuestionDialog) footerItemCount() int {
	n := 1 // "Chat about this"
	if d.showPlanMode {
		n++
	}
	return n
}

// ── Page renderers ──────────────────────────────────────────────────────────

func (d *AskQuestionDialog) renderQuestionPage() string {
	q := d.state.CurrentQuestion()
	if q == nil {
		return ""
	}

	// Choose between preview view and normal view
	if d.state.HasPreview() {
		return d.renderPreviewPage(q)
	}
	return d.renderNormalPage(q)
}

func (d *AskQuestionDialog) renderNormalPage(q *Question) string {
	qs := d.state.GetQuestionState(q.QuestionText)

	vm := &QuestionViewModel{
		Question:       *q,
		FocusedIndex:   d.focusedIndex,
		FocusArea:      d.focusArea,
		FooterIndex:    d.footerIndex,
		IsMultiSelect:  q.MultiSelect,
		TextInputVal:   d.textInput,
		SelectedValue:  qs.SelectedValue,
		SelectedValues: qs.SelectedValues,
		ShowPlanMode:   d.showPlanMode,
		PlanFilePath:   d.planFilePath,
		EditorName:     d.editorName,
		MultiQuestion:  len(d.state.Questions) > 1,
		Width:          d.width,
	}

	return RenderQuestionView(vm, d.qvStyles)
}

func (d *AskQuestionDialog) renderPreviewPage(q *Question) string {
	qs := d.state.GetQuestionState(q.QuestionText)

	vm := &PreviewViewModel{
		Question:        *q,
		FocusedIndex:    d.focusedIndex,
		FocusArea:       d.focusArea,
		FooterIndex:     d.footerIndex,
		SelectedValue:   qs.SelectedValue,
		NotesValue:      d.textInput,
		ShowPlanMode:    d.showPlanMode,
		EditorName:      d.editorName,
		MultiQuestion:   len(d.state.Questions) > 1,
		MinContentWidth: 20,
		MaxPreviewWidth: max(d.width-40, 30),
		PreviewMaxLines: max(d.height-12, 10),
		Width:           d.width,
	}

	return RenderPreviewQuestionView(vm, d.pvStyles, d.pbStyles)
}

func (d *AskQuestionDialog) renderSubmitPage() string {
	vm := &SubmitViewModel{
		Questions:    d.state.Questions,
		Answers:      d.state.Answers,
		Annotations:  d.state.Annotations,
		FocusedIndex: d.submitFocusIdx,
		AllAnswered:  d.state.AllAnswered(),
	}

	return RenderSubmitView(vm, d.svStyles)
}

// ── Serialization helpers ───────────────────────────────────────────────────

// ResponseJSON serializes the dialog result to JSON for the tool output.
func ResponseJSON(questions []Question, resp AskQuestionResponse) string {
	out := map[string]interface{}{
		"questions":   questions,
		"answers":     resp.Answers,
		"annotations": resp.Annotations,
	}
	if len(resp.PastedContents) > 0 {
		out["pastedContents"] = resp.PastedContents
	}
	if resp.Cancelled {
		out["cancelled"] = true
	}
	if resp.RespondToClaude {
		out["respondToClaude"] = true
	}
	if resp.Feedback != "" {
		out["feedback"] = resp.Feedback
	}
	if resp.FinishInterview {
		out["finishInterview"] = true
	}
	data, _ := json.Marshal(out)
	return string(data)
}
