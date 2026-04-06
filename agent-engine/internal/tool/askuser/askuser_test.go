package askuser

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAskUserQuestionTool_Name(t *testing.T) {
	tool := New()
	if tool.Name() != "AskUserQuestion" {
		t.Fatalf("expected name 'AskUserQuestion', got %q", tool.Name())
	}
}

func TestAskUserQuestionTool_Description(t *testing.T) {
	tool := New()
	desc := tool.Description()
	if desc == "" {
		t.Fatal("description should not be empty")
	}
}

func TestAskUserQuestionTool_Flags(t *testing.T) {
	tool := New()

	if !tool.IsConcurrencySafe(nil) {
		t.Error("should be concurrency safe")
	}
	if !tool.IsReadOnly(nil) {
		t.Error("should be read only")
	}
	if !tool.ShouldDefer() {
		t.Error("should defer execution")
	}
	if !tool.RequiresUserInteraction() {
		t.Error("should require user interaction")
	}
}

func TestAskUserQuestionTool_InputSchema(t *testing.T) {
	tool := New()
	schema := tool.InputSchema()
	if schema == nil {
		t.Fatal("schema should not be nil")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}

	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties object")
	}

	if _, ok := props["questions"]; !ok {
		t.Error("missing 'questions' property")
	}
}

func TestAskUserQuestionTool_ValidateInput_Valid(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{
				QuestionText: "Which framework?",
				Header:       "Framework",
				Options: []QuestionOption{
					{Label: "React", Description: "UI library"},
					{Label: "Vue", Description: "Progressive"},
				},
			},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err != nil {
		t.Fatalf("valid input should not error: %v", err)
	}
}

func TestAskUserQuestionTool_ValidateInput_NoQuestions(t *testing.T) {
	tool := New()

	input := Input{Questions: []Question{}}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("empty questions should error")
	}
}

func TestAskUserQuestionTool_ValidateInput_TooManyQuestions(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
			{QuestionText: "Q2", Header: "H2", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
			{QuestionText: "Q3", Header: "H3", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
			{QuestionText: "Q4", Header: "H4", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
			{QuestionText: "Q5", Header: "H5", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("5 questions should error (max 4)")
	}
}

func TestAskUserQuestionTool_ValidateInput_TooFewOptions(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
				{Label: "A"},
			}},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("1 option should error (min 2)")
	}
}

func TestAskUserQuestionTool_ValidateInput_TooManyOptions(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
				{Label: "A"}, {Label: "B"}, {Label: "C"}, {Label: "D"}, {Label: "E"},
			}},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("5 options should error (max 4)")
	}
}

func TestAskUserQuestionTool_ValidateInput_DuplicateQuestions(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{QuestionText: "Same question?", Header: "H1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
			{QuestionText: "Same question?", Header: "H2", Options: []QuestionOption{{Label: "C"}, {Label: "D"}}},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("duplicate question texts should error")
	}
}

func TestAskUserQuestionTool_ValidateInput_DuplicateHeaders(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{QuestionText: "Q1", Header: "Same", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
			{QuestionText: "Q2", Header: "Same", Options: []QuestionOption{{Label: "C"}, {Label: "D"}}},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("duplicate headers should error")
	}
}

func TestAskUserQuestionTool_ValidateInput_DuplicateOptionLabels(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{QuestionText: "Q1", Header: "H1", Options: []QuestionOption{
				{Label: "A"}, {Label: "A"},
			}},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("duplicate option labels should error")
	}
}

func TestAskUserQuestionTool_ValidateInput_HeaderTooLong(t *testing.T) {
	tool := New()

	input := Input{
		Questions: []Question{
			{QuestionText: "Q1", Header: "This header is way too long for a chip label", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
		},
	}
	data, _ := json.Marshal(input)

	err := tool.ValidateInput(context.Background(), data)
	if err == nil {
		t.Fatal("long header should error")
	}
}

func TestQuestionTypes(t *testing.T) {
	// Ensure the exported types are usable
	q := Question{
		QuestionText: "Test?",
		Header:       "Test",
		Options: []QuestionOption{
			{Label: "A", Description: "Desc A", Preview: "preview"},
		},
		MultiSelect: true,
	}

	if q.QuestionText != "Test?" {
		t.Error("unexpected question text")
	}
	if !q.MultiSelect {
		t.Error("expected multi-select")
	}
	if q.Options[0].Preview != "preview" {
		t.Error("expected preview content")
	}
}
