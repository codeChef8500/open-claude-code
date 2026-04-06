package askquestion

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewAskQuestionDialog(t *testing.T) {
	questions := []Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A", Description: "desc A"},
			{Label: "B", Description: "desc B"},
		}},
	}
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog(questions, ch)

	if !d.IsVisible() {
		t.Fatal("dialog should be visible after creation")
	}
	if d.state == nil {
		t.Fatal("state should be initialized")
	}
	if d.imageStore == nil {
		t.Fatal("imageStore should be initialized")
	}
}

func TestDialogSetDimensions(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}}}, ch)
	d.SetDimensions(120, 40)

	if d.width != 120 || d.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", d.width, d.height)
	}
}

func TestDialogSetPlanMode(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}}}, ch)
	d.SetPlanMode("/tmp/plan.md")

	if !d.showPlanMode {
		t.Fatal("plan mode should be enabled")
	}
	if d.planFilePath != "/tmp/plan.md" {
		t.Fatalf("expected plan path, got %q", d.planFilePath)
	}
}

func TestDialogView_NotEmpty(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Which library?", Header: "Library", Options: []QuestionOption{
			{Label: "React", Description: "UI library"},
			{Label: "Vue", Description: "Progressive framework"},
		}},
	}, ch)
	d.SetDimensions(80, 24)

	view := d.View()
	if view == "" {
		t.Fatal("view should not be empty")
	}
	if len(view) < 20 {
		t.Fatalf("view seems too short: %q", view)
	}
}

func TestDialogView_Invisible(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}}}, ch)
	d.visible = false

	view := d.View()
	if view != "" {
		t.Fatalf("invisible dialog should render empty, got: %q", view)
	}
}

func TestDialogEscCancels(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A"}, {Label: "B"},
		}},
	}, ch)

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should produce a command")
	}

	// Execute the cmd to get the result
	msg := cmd()
	dr, ok := msg.(DialogResult)
	if !ok {
		t.Fatalf("expected DialogResult, got %T", msg)
	}
	if !dr.Response.Cancelled {
		t.Fatal("esc should cancel the dialog")
	}

	// Channel should have the response
	resp := <-ch
	if !resp.Cancelled {
		t.Fatal("channel should receive cancelled response")
	}
}

func TestDialogArrowNavigation(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A"}, {Label: "B"}, {Label: "C"},
		}},
	}, ch)

	// Initially focused on index 0
	if d.focusedIndex != 0 {
		t.Fatalf("expected focused 0, got %d", d.focusedIndex)
	}

	// Move down
	d.Update(tea.KeyMsg{Type: tea.KeyDown})
	if d.focusedIndex != 1 {
		t.Fatalf("expected focused 1, got %d", d.focusedIndex)
	}

	// Move up
	d.Update(tea.KeyMsg{Type: tea.KeyUp})
	if d.focusedIndex != 0 {
		t.Fatalf("expected focused 0, got %d", d.focusedIndex)
	}

	// Move up again - should stay at 0
	d.Update(tea.KeyMsg{Type: tea.KeyUp})
	if d.focusedIndex != 0 {
		t.Fatalf("expected focused 0, got %d", d.focusedIndex)
	}
}

func TestDialogSingleSelectAutoSubmit(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A", Description: "opt A"},
			{Label: "B", Description: "opt B"},
		}},
	}, ch)

	// Press enter on first option (A) - should auto-submit for single question
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on single question should produce submit command")
	}

	// Execute the tea.Cmd to trigger the channel send
	cmd()

	resp := <-ch
	if resp.Cancelled {
		t.Fatal("should not be cancelled")
	}
	if resp.Answers["Q1"] != "A" {
		t.Fatalf("expected answer 'A', got %q", resp.Answers["Q1"])
	}
}

func TestDialogTabNavigation(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
		{QuestionText: "Q2", Header: "H2", Options: []QuestionOption{{Label: "C"}, {Label: "D"}}},
	}, ch)

	if d.state.CurrentQuestionIndex != 0 {
		t.Fatal("should start at question 0")
	}

	// Tab → next question
	d.Update(tea.KeyMsg{Type: tea.KeyTab})
	if d.state.CurrentQuestionIndex != 1 {
		t.Fatalf("expected question 1 after tab, got %d", d.state.CurrentQuestionIndex)
	}

	// Shift+Tab → prev question
	d.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if d.state.CurrentQuestionIndex != 0 {
		t.Fatalf("expected question 0 after shift+tab, got %d", d.state.CurrentQuestionIndex)
	}
}

func TestDialogNumberShortcut(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
			{Label: "A"}, {Label: "B"}, {Label: "C"},
		}},
		{QuestionText: "Q2", Header: "H2", Options: []QuestionOption{
			{Label: "D"}, {Label: "E"},
		}},
	}, ch)

	// Press "2" to select option B and auto-advance (since multi-question, won't auto-submit)
	d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	if d.state.Answers["Q1"] != "B" {
		t.Fatalf("expected answer 'B', got %q", d.state.Answers["Q1"])
	}
}

func TestDialogWindowSizeMsg(t *testing.T) {
	ch := make(chan AskQuestionResponse, 1)
	d := NewAskQuestionDialog([]Question{{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}}}, ch)

	d.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if d.width != 100 || d.height != 50 {
		t.Fatalf("expected 100x50, got %dx%d", d.width, d.height)
	}
}

func TestResponseJSON(t *testing.T) {
	questions := []Question{
		{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}}},
	}
	resp := AskQuestionResponse{
		Answers:     map[string]string{"Q1": "A"},
		Annotations: map[string]Annotation{"Q1": {Notes: "test"}},
	}

	result := ResponseJSON(questions, resp)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["answers"] == nil {
		t.Error("missing answers in JSON")
	}
	if parsed["questions"] == nil {
		t.Error("missing questions in JSON")
	}
}
