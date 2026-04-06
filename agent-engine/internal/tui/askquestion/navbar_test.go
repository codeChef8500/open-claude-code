package askquestion

import (
	"strings"
	"testing"
)

func TestRenderNavBar_SingleQuestionHidden(t *testing.T) {
	questions := []Question{
		{QuestionText: "Q1", Header: "Auth"},
	}
	result := RenderNavBar(questions, 0, map[string]string{}, true, 80, DefaultNavBarStyles())
	if result != "" {
		t.Fatalf("expected empty navbar for single question with hideSubmitTab, got: %q", result)
	}
}

func TestRenderNavBar_MultipleQuestions(t *testing.T) {
	questions := []Question{
		{QuestionText: "Q1", Header: "Auth"},
		{QuestionText: "Q2", Header: "Library"},
	}
	answers := map[string]string{"Q1": "A"}

	result := RenderNavBar(questions, 0, answers, false, 80, DefaultNavBarStyles())
	if result == "" {
		t.Fatal("expected non-empty navbar")
	}
	// Should contain both headers
	if !strings.Contains(result, "Auth") {
		t.Error("navbar should contain 'Auth'")
	}
	if !strings.Contains(result, "Library") {
		t.Error("navbar should contain 'Library'")
	}
	// Should contain submit
	if !strings.Contains(result, "Submit") {
		t.Error("navbar should contain 'Submit'")
	}
}

func TestRenderNavBar_AnsweredIndicator(t *testing.T) {
	questions := []Question{
		{QuestionText: "Q1", Header: "Auth"},
		{QuestionText: "Q2", Header: "Library"},
	}
	answers := map[string]string{"Q1": "A"}

	result := RenderNavBar(questions, 0, answers, false, 80, DefaultNavBarStyles())
	// Q1 answered → ☑, Q2 unanswered → ☐
	if !strings.Contains(result, "☑") {
		t.Error("answered question should have ☑")
	}
	if !strings.Contains(result, "☐") {
		t.Error("unanswered question should have ☐")
	}
}

func TestRenderNavBarCompact(t *testing.T) {
	styles := DefaultNavBarStyles()
	result := RenderNavBarCompact(0, 3, false, styles)
	if result != "Question 1 of 3" {
		t.Fatalf("expected 'Question 1 of 3', got %q", result)
	}

	result = RenderNavBarCompact(0, 3, true, styles)
	if !strings.Contains(result, "Submit") {
		t.Fatalf("expected submit page text, got %q", result)
	}
}

func TestTruncateHeader(t *testing.T) {
	tests := []struct {
		header   string
		width    int
		nQ       int
		expected string
	}{
		{"Auth", 80, 2, "Auth"},
		{"Very long header text that should be truncated", 40, 4, "Very…"},
	}

	for _, tt := range tests {
		result := truncateHeader(tt.header, tt.width, tt.nQ)
		if len(result) > 20 && tt.width < 50 {
			t.Errorf("truncateHeader(%q, %d, %d) = %q, seems too long", tt.header, tt.width, tt.nQ, result)
		}
	}
}
