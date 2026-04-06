package askquestion

import "github.com/wall-ai/agent-engine/internal/tool/askuser"

// ──────────────────────────────────────────────────────────────────────────────
// Re-export tool types for convenience inside the TUI package.
// ──────────────────────────────────────────────────────────────────────────────

// Question is a single question block from the tool input.
type Question = askuser.Question

// QuestionOption is one selectable choice within a question.
type QuestionOption = askuser.QuestionOption

// Annotation is per-question metadata collected from the user.
type Annotation = askuser.Annotation

// ──────────────────────────────────────────────────────────────────────────────
// UI-specific types
// ──────────────────────────────────────────────────────────────────────────────

// PastedContent represents an image pasted by the user.
type PastedContent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`      // "image"
	Content   string `json:"content"`   // base64
	MediaType string `json:"mediaType"` // "image/png", etc.
	Filename  string `json:"filename,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

// AskQuestionResponse is the structured response from the dialog.
type AskQuestionResponse struct {
	Answers          map[string]string     `json:"answers"`
	Annotations      map[string]Annotation `json:"annotations,omitempty"`
	Cancelled        bool                  `json:"cancelled,omitempty"`
	RespondToClaude  bool                  `json:"respondToClaude,omitempty"`
	Feedback         string                `json:"feedback,omitempty"`
	PastedContents   []PastedContent       `json:"pastedContents,omitempty"`
	FinishInterview  bool                  `json:"finishInterview,omitempty"`
}

// FocusArea tracks which section of the question view has focus.
type FocusArea int

const (
	FocusOptions FocusArea = iota
	FocusOther
	FocusNotes
	FocusFooter
)

// OtherOptionLabel is the auto-appended "Other" choice label.
const OtherOptionLabel = "Other"
