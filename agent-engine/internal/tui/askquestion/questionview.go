package askquestion

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ──────────────────────────────────────────────────────────────────────────────
// QuestionView — Go port of QuestionView.tsx
// Renders a single question with options (single-select or multi-select),
// an auto-appended "Other" option with free-text input, and a footer.
// ──────────────────────────────────────────────────────────────────────────────

// QuestionViewStyles controls the appearance.
type QuestionViewStyles struct {
	QuestionText lipgloss.Style
	Pointer      lipgloss.Style
	OptionLabel  lipgloss.Style
	OptionFocus  lipgloss.Style
	OptionSelect lipgloss.Style
	Description  lipgloss.Style
	DimText      lipgloss.Style
	Divider      lipgloss.Style
	FooterItem   lipgloss.Style
	FooterFocus  lipgloss.Style
	HintText     lipgloss.Style
	CheckboxOn   lipgloss.Style
	CheckboxOff  lipgloss.Style
	TextInput    lipgloss.Style
	Placeholder  lipgloss.Style
	SuccessTick  lipgloss.Style
}

// DefaultQuestionViewStyles returns sensible defaults.
func DefaultQuestionViewStyles() QuestionViewStyles {
	return QuestionViewStyles{
		QuestionText: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		Pointer:      lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")),
		OptionLabel:  lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		OptionFocus:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")).Bold(true),
		OptionSelect: lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
		Description:  lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true),
		DimText:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true),
		Divider:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		FooterItem:   lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		FooterFocus:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")),
		HintText:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true),
		CheckboxOn:   lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
		CheckboxOff:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		TextInput:    lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		Placeholder:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
		SuccessTick:  lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
	}
}

// QuestionViewModel holds the view-model state for rendering a single question.
type QuestionViewModel struct {
	Question      Question
	FocusedIndex  int       // index within allOptions (options + Other)
	FocusArea     FocusArea // which section has focus
	FooterIndex   int       // 0 = "Chat about this", 1 = "Skip interview"
	IsMultiSelect bool
	TextInputVal  string // current value of "Other" text input
	TextCursor    int    // cursor position in text input

	// External state references
	SelectedValue  string   // single-select chosen label
	SelectedValues []string // multi-select chosen labels

	// Configuration
	ShowPlanMode  bool   // whether to show "Skip interview" footer option
	PlanFilePath  string // plan file path for display
	EditorName    string // external editor name (for ctrl+g hint)
	MultiQuestion bool   // whether there are multiple questions (for tab hint)
}

// AllOptions returns the question options plus the auto-appended "Other" option.
func (vm *QuestionViewModel) AllOptions() []QuestionOption {
	opts := make([]QuestionOption, 0, len(vm.Question.Options)+1)
	opts = append(opts, vm.Question.Options...)
	opts = append(opts, QuestionOption{
		Label:       OtherOptionLabel,
		Description: "Provide a custom response",
	})
	return opts
}

// TotalItems returns the total number of navigable items
// (options + Other + footer items).
func (vm *QuestionViewModel) TotalItems() int {
	return len(vm.Question.Options) + 1 // +1 for "Other"
}

// FooterItemCount returns how many footer items are visible.
func (vm *QuestionViewModel) FooterItemCount() int {
	n := 1 // "Chat about this" always present
	if vm.ShowPlanMode {
		n++ // "Skip interview and plan immediately"
	}
	return n
}

// ── Rendering ───────────────────────────────────────────────────────────────

// RenderQuestionView renders the full question view.
func RenderQuestionView(vm *QuestionViewModel, styles QuestionViewStyles) string {
	var sb strings.Builder

	// Question text
	sb.WriteString(styles.QuestionText.Render(vm.Question.QuestionText))
	sb.WriteString("\n\n")

	allOpts := vm.AllOptions()

	for i, opt := range allOpts {
		isOther := opt.Label == OtherOptionLabel
		isFocused := vm.FocusArea == FocusOptions && vm.FocusedIndex == i
		isSelected := vm.isSelected(opt.Label)

		// Pointer
		if isFocused {
			sb.WriteString(styles.Pointer.Render("❯ "))
		} else {
			sb.WriteString("  ")
		}

		if vm.IsMultiSelect && !isOther {
			// Checkbox mode
			if isSelected {
				sb.WriteString(styles.CheckboxOn.Render("[✓] "))
			} else {
				sb.WriteString(styles.CheckboxOff.Render("[ ] "))
			}
		}

		// Label
		labelStyle := styles.OptionLabel
		if isFocused {
			labelStyle = styles.OptionFocus
		}
		if isSelected {
			labelStyle = styles.OptionSelect
		}
		sb.WriteString(labelStyle.Render(opt.Label))

		// Tick mark for single-select
		if !vm.IsMultiSelect && isSelected {
			sb.WriteString(" " + styles.SuccessTick.Render("✓"))
		}

		// Description on the same line (dimmed)
		if opt.Description != "" && !isOther {
			sb.WriteString(" " + styles.Description.Render("— "+opt.Description))
		}
		sb.WriteString("\n")

		// "Other" text input area
		if isOther && (isFocused || vm.FocusArea == FocusOther) {
			sb.WriteString(renderTextInputArea(vm, styles))
		}
	}

	// Multi-select: show "Next" / "Submit" button after options
	if vm.IsMultiSelect {
		sb.WriteString("\n")
		nextLabel := "Next →"
		if vm.MultiQuestion {
			nextLabel = "Next question →"
		} else {
			nextLabel = "Submit"
		}
		isFocusedNext := vm.FocusArea == FocusOptions && vm.FocusedIndex == len(allOpts)
		if isFocusedNext {
			sb.WriteString(styles.Pointer.Render("❯ ") + styles.OptionFocus.Render(nextLabel))
		} else {
			sb.WriteString("  " + styles.OptionLabel.Render(nextLabel))
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("\n")
	sb.WriteString(renderFooter(vm, styles))

	// Keyboard hints
	sb.WriteString("\n")
	sb.WriteString(renderHints(vm, styles))

	return sb.String()
}

func renderTextInputArea(vm *QuestionViewModel, styles QuestionViewStyles) string {
	var sb strings.Builder
	sb.WriteString("    ")
	if vm.FocusArea == FocusOther || (vm.FocusArea == FocusOptions && vm.FocusedIndex == vm.TotalItems()-1) {
		if vm.TextInputVal == "" {
			sb.WriteString(styles.Placeholder.Render("Type something…"))
		} else {
			sb.WriteString(styles.TextInput.Render(vm.TextInputVal))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func renderFooter(vm *QuestionViewModel, styles QuestionViewStyles) string {
	var sb strings.Builder

	// Divider
	sb.WriteString(styles.Divider.Render("─────────────────────────────────────"))
	sb.WriteString("\n")

	// "Chat about this"
	isChatFocused := vm.FocusArea == FocusFooter && vm.FooterIndex == 0
	if isChatFocused {
		sb.WriteString(styles.Pointer.Render("❯ "))
	} else {
		sb.WriteString("  ")
	}
	if isChatFocused {
		sb.WriteString(styles.FooterFocus.Render("Chat about this"))
	} else {
		sb.WriteString(styles.FooterItem.Render("Chat about this"))
	}
	sb.WriteString("\n")

	// Plan mode: "Skip interview and plan immediately"
	if vm.ShowPlanMode {
		isSkipFocused := vm.FocusArea == FocusFooter && vm.FooterIndex == 1
		if isSkipFocused {
			sb.WriteString(styles.Pointer.Render("❯ "))
		} else {
			sb.WriteString("  ")
		}
		if isSkipFocused {
			sb.WriteString(styles.FooterFocus.Render("Skip interview and plan immediately"))
		} else {
			sb.WriteString(styles.FooterItem.Render("Skip interview and plan immediately"))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderHints(vm *QuestionViewModel, styles QuestionViewStyles) string {
	var parts []string

	if vm.IsMultiSelect {
		parts = append(parts, "Space to toggle")
	} else {
		parts = append(parts, "Enter to select")
	}
	parts = append(parts, "↑/↓ to navigate")

	if vm.FocusArea == FocusOther && vm.EditorName != "" {
		parts = append(parts, fmt.Sprintf("ctrl+g to edit in %s", vm.EditorName))
	}

	if vm.MultiQuestion {
		parts = append(parts, "Tab to switch questions")
	}

	parts = append(parts, "Esc to cancel")

	return styles.HintText.Render(strings.Join(parts, " · "))
}

// ── Selection helpers ───────────────────────────────────────────────────────

func (vm *QuestionViewModel) isSelected(label string) bool {
	if vm.IsMultiSelect {
		for _, v := range vm.SelectedValues {
			if v == label {
				return true
			}
		}
		return false
	}
	return vm.SelectedValue == label
}
