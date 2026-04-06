package askquestion

import "strings"

// ──────────────────────────────────────────────────────────────────────────────
// Multi-question state machine — Go port of use-multiple-choice-state.ts
// ──────────────────────────────────────────────────────────────────────────────

// QuestionState tracks the UI state for a single question.
type QuestionState struct {
	SelectedValue  string   // single-select: the chosen label; multi-select: comma-joined labels
	SelectedValues []string // multi-select: individual chosen labels
	TextInputValue string   // "Other" free-text or notes
	IsInTextInput  bool     // whether the text input is focused
}

// MultiChoiceState manages the state for the entire multi-question dialog.
type MultiChoiceState struct {
	Questions            []Question
	CurrentQuestionIndex int
	Answers              map[string]string      // question text → answer string
	Annotations          map[string]Annotation  // question text → annotation
	QuestionStates       map[string]*QuestionState // question text → UI state
}

// NewMultiChoiceState creates a fresh state from a set of questions.
func NewMultiChoiceState(questions []Question) *MultiChoiceState {
	qs := make(map[string]*QuestionState, len(questions))
	for _, q := range questions {
		qs[q.QuestionText] = &QuestionState{}
	}
	return &MultiChoiceState{
		Questions:            questions,
		CurrentQuestionIndex: 0,
		Answers:              make(map[string]string),
		Annotations:          make(map[string]Annotation),
		QuestionStates:       qs,
	}
}

// CurrentQuestion returns the currently active question.
func (s *MultiChoiceState) CurrentQuestion() *Question {
	if s.CurrentQuestionIndex < 0 || s.CurrentQuestionIndex >= len(s.Questions) {
		return nil
	}
	return &s.Questions[s.CurrentQuestionIndex]
}

// IsOnSubmitPage returns true when the user is on the final submit page.
func (s *MultiChoiceState) IsOnSubmitPage() bool {
	return s.CurrentQuestionIndex >= len(s.Questions)
}

// AllAnswered returns true when every question has an answer.
func (s *MultiChoiceState) AllAnswered() bool {
	for _, q := range s.Questions {
		if _, ok := s.Answers[q.QuestionText]; !ok {
			return false
		}
	}
	return true
}

// ── Actions ──────────────────────────────────────────────────────────────────

// NextQuestion advances to the next question or the submit page.
func (s *MultiChoiceState) NextQuestion() {
	if s.CurrentQuestionIndex <= len(s.Questions) {
		s.CurrentQuestionIndex++
	}
}

// PrevQuestion goes back to the previous question.
func (s *MultiChoiceState) PrevQuestion() {
	if s.CurrentQuestionIndex > 0 {
		s.CurrentQuestionIndex--
	}
}

// GoToQuestion jumps to a specific question index.
func (s *MultiChoiceState) GoToQuestion(idx int) {
	if idx >= 0 && idx <= len(s.Questions) {
		s.CurrentQuestionIndex = idx
	}
}

// SetAnswer records an answer for a question and optionally advances.
func (s *MultiChoiceState) SetAnswer(questionText, answer string, advance bool) {
	s.Answers[questionText] = answer

	// For single-question + non-multiSelect, auto-submit semantics:
	// the caller decides whether to advance.
	if advance {
		s.NextQuestion()
	}
}

// SetMultiSelectAnswer records a multi-select answer (comma-joined).
func (s *MultiChoiceState) SetMultiSelectAnswer(questionText string, labels []string) {
	if qs, ok := s.QuestionStates[questionText]; ok {
		qs.SelectedValues = labels
		qs.SelectedValue = strings.Join(labels, ", ")
	}
	s.Answers[questionText] = strings.Join(labels, ", ")
}

// UpdateQuestionState updates the UI state for a specific question.
func (s *MultiChoiceState) UpdateQuestionState(questionText string, selected string, textInput string, isTextInput bool) {
	qs, ok := s.QuestionStates[questionText]
	if !ok {
		qs = &QuestionState{}
		s.QuestionStates[questionText] = qs
	}
	if selected != "" {
		qs.SelectedValue = selected
	}
	qs.TextInputValue = textInput
	qs.IsInTextInput = isTextInput
}

// SetAnnotation sets annotation data for a question.
func (s *MultiChoiceState) SetAnnotation(questionText string, ann Annotation) {
	s.Annotations[questionText] = ann
}

// GetQuestionState returns the UI state for a question, creating one if needed.
func (s *MultiChoiceState) GetQuestionState(questionText string) *QuestionState {
	qs, ok := s.QuestionStates[questionText]
	if !ok {
		qs = &QuestionState{}
		s.QuestionStates[questionText] = qs
	}
	return qs
}

// ShouldAutoSubmit returns true when there is exactly one question,
// it is not multi-select, and the user selected a predefined option (not "Other").
func (s *MultiChoiceState) ShouldAutoSubmit() bool {
	if len(s.Questions) != 1 {
		return false
	}
	q := s.Questions[0]
	if q.MultiSelect {
		return false
	}
	ans, ok := s.Answers[q.QuestionText]
	if !ok || ans == "" {
		return false
	}
	return ans != OtherOptionLabel
}

// HideSubmitTab returns true when the submit navigation tab should be hidden.
// Hidden when there is exactly one non-multiSelect question.
func (s *MultiChoiceState) HideSubmitTab() bool {
	if len(s.Questions) != 1 {
		return false
	}
	return !s.Questions[0].MultiSelect
}

// HasPreview returns true if the current question has any option with preview content.
func (s *MultiChoiceState) HasPreview() bool {
	q := s.CurrentQuestion()
	if q == nil {
		return false
	}
	for _, opt := range q.Options {
		if opt.Preview != "" {
			return true
		}
	}
	return false
}

// ToggleMultiSelect toggles a label in the multi-select set for the current question.
func (s *MultiChoiceState) ToggleMultiSelect(questionText, label string) {
	qs := s.GetQuestionState(questionText)
	found := false
	newVals := make([]string, 0, len(qs.SelectedValues))
	for _, v := range qs.SelectedValues {
		if v == label {
			found = true
			continue
		}
		newVals = append(newVals, v)
	}
	if !found {
		newVals = append(newVals, label)
	}
	qs.SelectedValues = newVals
	qs.SelectedValue = strings.Join(newVals, ", ")
}

// IsMultiSelected checks if a label is currently selected in multi-select mode.
func (s *MultiChoiceState) IsMultiSelected(questionText, label string) bool {
	qs := s.GetQuestionState(questionText)
	for _, v := range qs.SelectedValues {
		if v == label {
			return true
		}
	}
	return false
}
