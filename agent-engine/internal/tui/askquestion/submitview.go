package askquestion

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ──────────────────────────────────────────────────────────────────────────────
// SubmitQuestionsView — Go port of SubmitQuestionsView.tsx
// Shows a summary of all answers and lets the user submit or cancel.
// ──────────────────────────────────────────────────────────────────────────────

// SubmitViewStyles controls the appearance.
type SubmitViewStyles struct {
	Title       lipgloss.Style
	Answered    lipgloss.Style
	Unanswered  lipgloss.Style
	QuestionLbl lipgloss.Style
	AnswerLbl   lipgloss.Style
	Warning     lipgloss.Style
	Pointer     lipgloss.Style
	ActionFocus lipgloss.Style
	ActionNorm  lipgloss.Style
	DimText     lipgloss.Style
}

// DefaultSubmitViewStyles returns sensible defaults.
func DefaultSubmitViewStyles() SubmitViewStyles {
	return SubmitViewStyles{
		Title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		Answered:    lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
		Unanswered:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b80")),
		QuestionLbl: lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		AnswerLbl:   lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
		Warning:     lipgloss.NewStyle().Foreground(lipgloss.Color("#ffb347")).Bold(true),
		Pointer:     lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")),
		ActionFocus: lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")).Bold(true),
		ActionNorm:  lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		DimText:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true),
	}
}

// SubmitViewModel holds the view state for the submit page.
type SubmitViewModel struct {
	Questions    []Question
	Answers      map[string]string
	Annotations  map[string]Annotation
	FocusedIndex int  // 0 = Submit, 1 = Cancel
	AllAnswered  bool
}

// RenderSubmitView renders the submit/review page.
func RenderSubmitView(vm *SubmitViewModel, styles SubmitViewStyles) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Review your answers"))
	sb.WriteString("\n\n")

	// Warning if not all answered
	if !vm.AllAnswered {
		sb.WriteString(styles.Warning.Render("⚠  Not all questions have been answered."))
		sb.WriteString("\n\n")
	}

	// Question → Answer list
	for i, q := range vm.Questions {
		answer, answered := vm.Answers[q.QuestionText]

		// Number + question
		numStr := fmt.Sprintf("%d. ", i+1)
		sb.WriteString(styles.QuestionLbl.Render(numStr + q.QuestionText))
		sb.WriteString("\n")

		// Answer
		if answered {
			sb.WriteString("   " + styles.Answered.Render("→ ") + styles.AnswerLbl.Render(answer))

			// Show notes if present
			if ann, ok := vm.Annotations[q.QuestionText]; ok && ann.Notes != "" {
				sb.WriteString(" " + styles.DimText.Render(fmt.Sprintf("(notes: %s)", ann.Notes)))
			}
		} else {
			sb.WriteString("   " + styles.Unanswered.Render("✗ Not answered"))
		}
		sb.WriteString("\n")

		if i < len(vm.Questions)-1 {
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	// Action buttons
	actions := []string{"Submit answers", "Cancel"}
	for i, action := range actions {
		isFocused := vm.FocusedIndex == i
		if isFocused {
			sb.WriteString(styles.Pointer.Render("❯ "))
			sb.WriteString(styles.ActionFocus.Render(action))
		} else {
			sb.WriteString("  ")
			sb.WriteString(styles.ActionNorm.Render(action))
		}
		sb.WriteString("\n")
	}

	// Hints
	sb.WriteString("\n")
	sb.WriteString(styles.DimText.Render("Enter to confirm · ↑/↓ to navigate · Tab to go back to questions · Esc to cancel"))

	return sb.String()
}
