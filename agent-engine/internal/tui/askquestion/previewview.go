package askquestion

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ──────────────────────────────────────────────────────────────────────────────
// PreviewQuestionView — Go port of PreviewQuestionView.tsx
// Side-by-side layout: options on the left, preview + notes on the right.
// Used when any option in the question has a `preview` field.
// ──────────────────────────────────────────────────────────────────────────────

// PreviewViewStyles controls the appearance.
type PreviewViewStyles struct {
	QuestionText lipgloss.Style
	Pointer      lipgloss.Style
	OptionLabel  lipgloss.Style
	OptionFocus  lipgloss.Style
	OptionSelect lipgloss.Style
	DimText      lipgloss.Style
	NumberLabel  lipgloss.Style
	SuccessTick  lipgloss.Style
	NotesLabel   lipgloss.Style
	NotesInput   lipgloss.Style
	NotesPlaceh  lipgloss.Style
	FooterItem   lipgloss.Style
	FooterFocus  lipgloss.Style
	HintText     lipgloss.Style
	Divider      lipgloss.Style
}

// DefaultPreviewViewStyles returns sensible defaults.
func DefaultPreviewViewStyles() PreviewViewStyles {
	return PreviewViewStyles{
		QuestionText: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		Pointer:      lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")),
		OptionLabel:  lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		OptionFocus:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")).Bold(true),
		OptionSelect: lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
		DimText:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true),
		NumberLabel:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true),
		SuccessTick:  lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
		NotesLabel:   lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")),
		NotesInput:   lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		NotesPlaceh:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true).Faint(true),
		FooterItem:   lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		FooterFocus:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")),
		HintText:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true),
		Divider:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

// PreviewViewModel holds the view-model state for a preview question.
type PreviewViewModel struct {
	Question     Question
	FocusedIndex int       // index within allOptions (options + Other)
	FocusArea    FocusArea // Options, Notes, or Footer
	FooterIndex  int

	SelectedValue string // single-select chosen label
	NotesValue    string // notes text input
	NotesCursor   int

	// Configuration
	ShowPlanMode    bool
	EditorName      string
	MultiQuestion   bool
	MinContentWidth int
	MaxPreviewWidth int
	PreviewMaxLines int
	Width           int // available render width (0 = use default 37)
}

// AllOptions returns question options + "Other".
func (vm *PreviewViewModel) AllOptions() []QuestionOption {
	opts := make([]QuestionOption, 0, len(vm.Question.Options)+1)
	opts = append(opts, vm.Question.Options...)
	opts = append(opts, QuestionOption{
		Label:       OtherOptionLabel,
		Description: "Provide a custom response",
	})
	return opts
}

// FocusedPreview returns the preview content for the currently focused option.
func (vm *PreviewViewModel) FocusedPreview() string {
	if vm.FocusedIndex < 0 || vm.FocusedIndex >= len(vm.Question.Options) {
		return "No preview available"
	}
	p := vm.Question.Options[vm.FocusedIndex].Preview
	if p == "" {
		return "No preview available"
	}
	return p
}

// RenderPreviewQuestionView renders the side-by-side preview layout.
func RenderPreviewQuestionView(vm *PreviewViewModel, pvStyles PreviewViewStyles, pbStyles PreviewBoxStyles) string {
	var sb strings.Builder

	// Question text
	sb.WriteString(pvStyles.QuestionText.Render(vm.Question.QuestionText))
	sb.WriteString("\n\n")

	// Side-by-side: options (left) | preview+notes (right)
	leftPanel := renderLeftPanel(vm, pvStyles)
	rightPanel := renderRightPanel(vm, pvStyles, pbStyles)

	// Join panels horizontally
	leftLines := strings.Split(leftPanel, "\n")
	rightLines := strings.Split(rightPanel, "\n")

	// Determine left panel width based on longest option label.
	// Minimum 20 chars, capped to avoid squeezing the preview panel.
	leftWidth := 20
	for _, opt := range vm.AllOptions() {
		// Account for pointer(2) + number(4) + space(1) + label
		w := 7 + len(opt.Label)
		if w > leftWidth {
			leftWidth = w
		}
	}
	if leftWidth > 40 {
		leftWidth = 40
	}
	gap := 4

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	for i := 0; i < maxLines; i++ {
		var left, right string
		if i < len(leftLines) {
			left = leftLines[i]
		}
		if i < len(rightLines) {
			right = rightLines[i]
		}

		// Pad left to fixed width
		leftPW := printableLen(left)
		if leftPW < leftWidth {
			left += strings.Repeat(" ", leftWidth-leftPW)
		}

		sb.WriteString(left + strings.Repeat(" ", gap) + right)
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("\n")
	sb.WriteString(renderPreviewFooter(vm, pvStyles))

	// Hints
	sb.WriteString("\n")
	sb.WriteString(renderPreviewHints(vm, pvStyles))

	return sb.String()
}

func renderLeftPanel(vm *PreviewViewModel, styles PreviewViewStyles) string {
	var sb strings.Builder
	allOpts := vm.AllOptions()

	for i, opt := range allOpts {
		isFocused := vm.FocusArea == FocusOptions && vm.FocusedIndex == i
		isSelected := vm.SelectedValue == opt.Label

		// Pointer
		if isFocused {
			sb.WriteString(styles.Pointer.Render("❯"))
		} else {
			sb.WriteString(" ")
		}

		// Number
		sb.WriteString(styles.NumberLabel.Render(fmt.Sprintf(" %d.", i+1)))

		// Label
		labelStyle := styles.OptionLabel
		if isFocused {
			labelStyle = styles.OptionFocus
		}
		if isSelected {
			labelStyle = styles.OptionSelect
		}
		sb.WriteString(" " + labelStyle.Render(opt.Label))

		// Tick
		if isSelected {
			sb.WriteString(" " + styles.SuccessTick.Render("✓"))
		}

		if i < len(allOpts)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func renderRightPanel(vm *PreviewViewModel, pvStyles PreviewViewStyles, pbStyles PreviewBoxStyles) string {
	var sb strings.Builder

	// Preview box
	previewContent := vm.FocusedPreview()
	maxWidth := vm.MaxPreviewWidth
	if maxWidth < 20 {
		maxWidth = 60
	}
	maxLines := vm.PreviewMaxLines
	if maxLines <= 0 {
		maxLines = 20
	}

	box := RenderPreviewBox(PreviewBoxOpts{
		Content:   previewContent,
		MaxLines:  maxLines,
		MinHeight: 3,
		MinWidth:  vm.MinContentWidth,
		MaxWidth:  maxWidth,
	}, pbStyles)
	sb.WriteString(box)
	sb.WriteString("\n")

	// Notes
	sb.WriteString(pvStyles.NotesLabel.Render("Notes: "))
	if vm.FocusArea == FocusNotes {
		if vm.NotesValue == "" {
			sb.WriteString(pvStyles.NotesPlaceh.Render("Add notes on this design…"))
		} else {
			sb.WriteString(pvStyles.NotesInput.Render(vm.NotesValue))
		}
	} else {
		if vm.NotesValue != "" {
			sb.WriteString(pvStyles.DimText.Render(vm.NotesValue))
		} else {
			sb.WriteString(pvStyles.DimText.Render("press n to add notes"))
		}
	}

	return sb.String()
}

func renderPreviewFooter(vm *PreviewViewModel, styles PreviewViewStyles) string {
	var sb strings.Builder

	divWidth := vm.Width - 4
	if divWidth < 20 {
		divWidth = 37
	}
	sb.WriteString(styles.Divider.Render(strings.Repeat("─", divWidth)))
	sb.WriteString("\n")

	// "Chat about this"
	isChatFocused := vm.FocusArea == FocusFooter && vm.FooterIndex == 0
	if isChatFocused {
		sb.WriteString(styles.Pointer.Render("❯ ") + styles.FooterFocus.Render("Chat about this"))
	} else {
		sb.WriteString("  " + styles.FooterItem.Render("Chat about this"))
	}
	sb.WriteString("\n")

	// Plan mode
	if vm.ShowPlanMode {
		isSkipFocused := vm.FocusArea == FocusFooter && vm.FooterIndex == 1
		if isSkipFocused {
			sb.WriteString(styles.Pointer.Render("❯ ") + styles.FooterFocus.Render("Skip interview and plan immediately"))
		} else {
			sb.WriteString("  " + styles.FooterItem.Render("Skip interview and plan immediately"))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderPreviewHints(vm *PreviewViewModel, styles PreviewViewStyles) string {
	var parts []string
	parts = append(parts, "Enter to select")
	parts = append(parts, "↑/↓ to navigate")
	parts = append(parts, "n to add notes")

	if vm.MultiQuestion {
		parts = append(parts, "Tab to switch questions")
	}

	if vm.FocusArea == FocusNotes && vm.EditorName != "" {
		parts = append(parts, fmt.Sprintf("ctrl+g to edit in %s", vm.EditorName))
	}

	parts = append(parts, "Esc to cancel")

	return styles.HintText.Render(strings.Join(parts, " · "))
}
