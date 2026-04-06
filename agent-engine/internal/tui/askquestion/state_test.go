package askquestion

import (
	"testing"
)

func TestNewMultiChoiceState(t *testing.T) {
	questions := []Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A", Description: "desc A"},
			{Label: "B", Description: "desc B"},
		}},
		{QuestionText: "Q2", Header: "H2", Options: []QuestionOption{
			{Label: "C", Description: "desc C"},
			{Label: "D", Description: "desc D"},
		}},
	}

	s := NewMultiChoiceState(questions)

	if len(s.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(s.Questions))
	}
	if s.CurrentQuestionIndex != 0 {
		t.Fatalf("expected index 0, got %d", s.CurrentQuestionIndex)
	}
	if len(s.QuestionStates) != 2 {
		t.Fatalf("expected 2 question states, got %d", len(s.QuestionStates))
	}
	if s.AllAnswered() {
		t.Fatal("should not be all answered initially")
	}
}

func TestCurrentQuestion(t *testing.T) {
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1"},
		{QuestionText: "Q2", Header: "H2"},
	})

	q := s.CurrentQuestion()
	if q == nil || q.QuestionText != "Q1" {
		t.Fatalf("expected Q1, got %v", q)
	}

	s.NextQuestion()
	q = s.CurrentQuestion()
	if q == nil || q.QuestionText != "Q2" {
		t.Fatalf("expected Q2, got %v", q)
	}
}

func TestIsOnSubmitPage(t *testing.T) {
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1"},
	})

	if s.IsOnSubmitPage() {
		t.Fatal("should not be on submit page initially")
	}
	s.NextQuestion()
	if !s.IsOnSubmitPage() {
		t.Fatal("should be on submit page after advancing past last question")
	}
}

func TestSetAnswer(t *testing.T) {
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A"}, {Label: "B"},
		}},
		{QuestionText: "Q2", Header: "H2", Options: []QuestionOption{
			{Label: "C"}, {Label: "D"},
		}},
	})

	s.SetAnswer("Q1", "A", true)
	if s.Answers["Q1"] != "A" {
		t.Fatalf("expected answer A, got %s", s.Answers["Q1"])
	}
	if s.CurrentQuestionIndex != 1 {
		t.Fatalf("expected advance to index 1, got %d", s.CurrentQuestionIndex)
	}

	s.SetAnswer("Q2", "D", false)
	if s.CurrentQuestionIndex != 1 {
		t.Fatalf("expected no advance, got %d", s.CurrentQuestionIndex)
	}

	if !s.AllAnswered() {
		t.Fatal("should be all answered")
	}
}

func TestMultiSelect(t *testing.T) {
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", MultiSelect: true, Options: []QuestionOption{
			{Label: "A"}, {Label: "B"}, {Label: "C"},
		}},
	})

	s.ToggleMultiSelect("Q1", "A")
	s.ToggleMultiSelect("Q1", "C")

	if !s.IsMultiSelected("Q1", "A") {
		t.Fatal("A should be selected")
	}
	if s.IsMultiSelected("Q1", "B") {
		t.Fatal("B should not be selected")
	}
	if !s.IsMultiSelected("Q1", "C") {
		t.Fatal("C should be selected")
	}

	// Toggle off
	s.ToggleMultiSelect("Q1", "A")
	if s.IsMultiSelected("Q1", "A") {
		t.Fatal("A should be deselected after toggle")
	}
}

func TestSetMultiSelectAnswer(t *testing.T) {
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", MultiSelect: true, Options: []QuestionOption{
			{Label: "A"}, {Label: "B"},
		}},
	})

	s.SetMultiSelectAnswer("Q1", []string{"A", "B"})
	if s.Answers["Q1"] != "A, B" {
		t.Fatalf("expected 'A, B', got '%s'", s.Answers["Q1"])
	}
}

func TestShouldAutoSubmit(t *testing.T) {
	// Single question, non-multiSelect, predefined answer → should auto-submit
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A"}, {Label: "B"},
		}},
	})
	s.SetAnswer("Q1", "A", false)
	if !s.ShouldAutoSubmit() {
		t.Fatal("should auto-submit for single question with predefined answer")
	}

	// "Other" answer → should NOT auto-submit
	s2 := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A"}, {Label: "B"},
		}},
	})
	s2.SetAnswer("Q1", OtherOptionLabel, false)
	if s2.ShouldAutoSubmit() {
		t.Fatal("should not auto-submit for 'Other' answer")
	}

	// Multi-question → should NOT auto-submit
	s3 := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}}},
		{QuestionText: "Q2", Header: "H2", Options: []QuestionOption{{Label: "B"}}},
	})
	s3.SetAnswer("Q1", "A", false)
	if s3.ShouldAutoSubmit() {
		t.Fatal("should not auto-submit for multi-question")
	}

	// MultiSelect → should NOT auto-submit
	s4 := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", MultiSelect: true, Options: []QuestionOption{
			{Label: "A"}, {Label: "B"},
		}},
	})
	s4.SetAnswer("Q1", "A", false)
	if s4.ShouldAutoSubmit() {
		t.Fatal("should not auto-submit for multiSelect")
	}
}

func TestHideSubmitTab(t *testing.T) {
	// Single non-multiSelect → hide
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}}},
	})
	if !s.HideSubmitTab() {
		t.Fatal("should hide submit tab for single non-multiSelect question")
	}

	// Multi-question → show
	s2 := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1"},
		{QuestionText: "Q2", Header: "H2"},
	})
	if s2.HideSubmitTab() {
		t.Fatal("should not hide submit tab for multiple questions")
	}
}

func TestNavigation(t *testing.T) {
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1"}, {QuestionText: "Q2"}, {QuestionText: "Q3"},
	})

	s.PrevQuestion() // should stay at 0
	if s.CurrentQuestionIndex != 0 {
		t.Fatalf("should stay at 0, got %d", s.CurrentQuestionIndex)
	}

	s.NextQuestion()
	s.NextQuestion()
	if s.CurrentQuestionIndex != 2 {
		t.Fatalf("expected 2, got %d", s.CurrentQuestionIndex)
	}

	s.GoToQuestion(0)
	if s.CurrentQuestionIndex != 0 {
		t.Fatalf("expected 0 after GoToQuestion, got %d", s.CurrentQuestionIndex)
	}

	// Go to submit page
	s.GoToQuestion(3)
	if !s.IsOnSubmitPage() {
		t.Fatal("should be on submit page at index 3")
	}
}

func TestAnnotations(t *testing.T) {
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Header: "H1"},
	})

	s.SetAnnotation("Q1", Annotation{Preview: "preview text", Notes: "my notes"})
	ann := s.Annotations["Q1"]
	if ann.Preview != "preview text" || ann.Notes != "my notes" {
		t.Fatalf("unexpected annotation: %+v", ann)
	}
}

func TestHasPreview(t *testing.T) {
	// No preview
	s := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Options: []QuestionOption{{Label: "A"}}},
	})
	if s.HasPreview() {
		t.Fatal("should not have preview")
	}

	// With preview
	s2 := NewMultiChoiceState([]Question{
		{QuestionText: "Q1", Options: []QuestionOption{
			{Label: "A", Preview: "some code"},
			{Label: "B"},
		}},
	})
	if !s2.HasPreview() {
		t.Fatal("should have preview")
	}
}
