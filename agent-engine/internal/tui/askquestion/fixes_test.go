package askquestion

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── P1: ResponseJSON includes all response fields ───────────────────────────

func TestResponseJSON_IncludesPastedContents(t *testing.T) {
	questions := []Question{{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}}}}
	resp := AskQuestionResponse{
		Answers: map[string]string{"Q1": "A"},
		PastedContents: []PastedContent{
			{ID: "img_1", Type: "image", Content: "base64data", MediaType: "image/png"},
		},
	}
	result := ResponseJSON(questions, resp)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["pastedContents"] == nil {
		t.Error("ResponseJSON should include pastedContents when present")
	}
}

func TestResponseJSON_IncludesCancelledFlag(t *testing.T) {
	resp := AskQuestionResponse{Cancelled: true}
	result := ResponseJSON(nil, resp)
	var parsed map[string]interface{}
	json.Unmarshal([]byte(result), &parsed)
	if parsed["cancelled"] != true {
		t.Error("ResponseJSON should include cancelled=true")
	}
}

func TestResponseJSON_IncludesFeedback(t *testing.T) {
	resp := AskQuestionResponse{
		RespondToClaude: true,
		Feedback:        "User wants to discuss",
	}
	result := ResponseJSON(nil, resp)
	var parsed map[string]interface{}
	json.Unmarshal([]byte(result), &parsed)
	if parsed["respondToClaude"] != true {
		t.Error("ResponseJSON should include respondToClaude=true")
	}
	if parsed["feedback"] == nil {
		t.Error("ResponseJSON should include feedback string")
	}
}

func TestResponseJSON_IncludesFinishInterview(t *testing.T) {
	resp := AskQuestionResponse{FinishInterview: true}
	result := ResponseJSON(nil, resp)
	var parsed map[string]interface{}
	json.Unmarshal([]byte(result), &parsed)
	if parsed["finishInterview"] != true {
		t.Error("ResponseJSON should include finishInterview=true")
	}
}

func TestResponseJSON_OmitsEmptyFields(t *testing.T) {
	resp := AskQuestionResponse{
		Answers: map[string]string{"Q1": "A"},
	}
	result := ResponseJSON(nil, resp)
	var parsed map[string]interface{}
	json.Unmarshal([]byte(result), &parsed)
	// These should be absent when zero-valued
	for _, key := range []string{"pastedContents", "cancelled", "respondToClaude", "feedback", "finishInterview"} {
		if _, ok := parsed[key]; ok {
			t.Errorf("ResponseJSON should omit %q when zero-valued", key)
		}
	}
}

// ── P2: Multi-select empty validation ───────────────────────────────────────

func TestDialogMultiSelectEmptyBlocksSubmit(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", MultiSelect: true, Options: []QuestionOption{
			{Label: "A"}, {Label: "B"}, {Label: "C"},
		}},
	}, ch)

	// Navigate down to the "Next/Submit" button (options=3 + Other=1 + Submit=1 at index 4)
	for i := 0; i < 4; i++ {
		d.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	// Press enter with no selections — should be blocked
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("entering submit with no multi-select selections should be blocked (no cmd)")
	}

	// Verify dialog is still visible
	if !d.IsVisible() {
		t.Fatal("dialog should still be visible after blocked empty submit")
	}

	select {
	case <-ch:
		t.Fatal("no response should be sent when multi-select submit is blocked")
	default:
		// expected
	}
}

// ── P2: UTF-8 backspace ─────────────────────────────────────────────────────

func TestDialogUTF8Backspace(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
	}, ch)

	// Navigate to "Other" option (index = len(options) = 2)
	d.Update(tea.KeyMsg{Type: tea.KeyDown})
	d.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Enter "Other" text mode
	d.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type a multi-byte character (Chinese: 你)
	d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'你'}})
	d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'好'}})

	if d.textInput != "你好" {
		t.Fatalf("expected '你好', got %q", d.textInput)
	}

	// Backspace should remove the whole last rune, not just one byte
	d.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if d.textInput != "你" {
		t.Fatalf("backspace should remove last rune; expected '你', got %q", d.textInput)
	}
}

// ── P3: Dynamic divider width ───────────────────────────────────────────────

func TestRenderFooterDividerWidth(t *testing.T) {
	vm := &QuestionViewModel{
		Question: Question{QuestionText: "Q", Options: []QuestionOption{{Label: "A"}}},
		Width:    60,
	}
	styles := DefaultQuestionViewStyles()
	footer := renderFooter(vm, styles)

	// The divider should be width-4 = 56 dashes, not the old hardcoded 37
	if strings.Count(footer, "─") < 50 {
		t.Errorf("footer divider should be dynamic (expected ~56 dashes for width=60), got:\n%s", footer)
	}
}

func TestRenderFooterDividerDefault(t *testing.T) {
	vm := &QuestionViewModel{
		Question: Question{QuestionText: "Q", Options: []QuestionOption{{Label: "A"}}},
		Width:    0, // zero width → should fallback to 37
	}
	styles := DefaultQuestionViewStyles()
	footer := renderFooter(vm, styles)

	if strings.Count(footer, "─") != 37 {
		t.Errorf("footer divider should fallback to 37 dashes when width=0, got %d", strings.Count(footer, "─"))
	}
}

// ── P4: ImagePasteStore thread safety ───────────────────────────────────────

func TestImagePasteStoreConcurrent(t *testing.T) {
	store := NewImagePasteStore()
	done := make(chan struct{})

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			store.Add("Q1", PastedContent{ID: "a", Type: "image"})
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			store.Add("Q2", PastedContent{ID: "b", Type: "image"})
		}
		done <- struct{}{}
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = store.All()
			_ = store.HasImages()
			_ = store.Get("Q1")
		}
		done <- struct{}{}
	}()

	<-done
	<-done
	<-done
	// No race detector panic = pass
}

// ── P4: Base64 detection threshold ──────────────────────────────────────────

func TestDetectBase64Image_ShortStringNotDetected(t *testing.T) {
	// 200-char valid base64 should NOT be detected (threshold raised to 500)
	data := strings.Repeat("AAAA", 50) // 200 chars of valid base64
	result := DetectBase64Image(data)
	if result != nil {
		t.Error("short base64 string (<500 chars) should not be detected as image")
	}
}

func TestDetectBase64Image_DataURIStillWorks(t *testing.T) {
	// data: URIs should always be detected regardless of length
	uri := "data:image/png;base64,iVBORw0KGgo="
	result := DetectBase64Image(uri)
	if result == nil {
		t.Fatal("data:image/ URI should always be detected")
	}
	if result.MediaType != "image/png" {
		t.Errorf("expected image/png, got %s", result.MediaType)
	}
}

// ── P1: buildFeedbackSummary ────────────────────────────────────────────────

func TestBuildFeedbackSummary_NoAnswers(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}}},
	}, ch)

	feedback := d.buildFeedbackSummary("User chose to chat.")
	if !strings.HasPrefix(feedback, "User chose to chat.") {
		t.Errorf("feedback should start with action text, got: %s", feedback)
	}
	if strings.Contains(feedback, "Partial answers") {
		t.Error("feedback should not mention partial answers when none are provided")
	}
}

func TestBuildFeedbackSummary_WithPartialAnswers(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}}},
		{QuestionText: "Q2", Header: "H2", Options: []QuestionOption{{Label: "B"}}},
	}, ch)

	d.state.SetAnswer("Q1", "A", false)

	feedback := d.buildFeedbackSummary("User skipped.")
	if !strings.Contains(feedback, "Partial answers provided (1 of 2 questions)") {
		t.Errorf("feedback should mention partial answers, got: %s", feedback)
	}
	if !strings.Contains(feedback, "Q1: A") {
		t.Errorf("feedback should list answered questions, got: %s", feedback)
	}
}
