package notebookedit

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

type Input struct {
	NotebookPath string `json:"notebook_path"`
	CellNumber   int    `json:"cell_number"`
	NewSource    string `json:"new_source"`
	CellType     string `json:"cell_type,omitempty"` // "code" | "markdown"
	EditMode     string `json:"edit_mode,omitempty"` // "replace" | "insert"
}

type NotebookEditTool struct{ tool.BaseTool }

func New() *NotebookEditTool { return &NotebookEditTool{} }

func (t *NotebookEditTool) Name() string                      { return "NotebookEdit" }
func (t *NotebookEditTool) UserFacingName() string            { return "notebook_edit" }
func (t *NotebookEditTool) Description() string               { return "Edit a Jupyter notebook cell." }
func (t *NotebookEditTool) IsReadOnly(_ json.RawMessage) bool                  { return false }
func (t *NotebookEditTool) IsConcurrencySafe(_ json.RawMessage) bool           { return false }
func (t *NotebookEditTool) MaxResultSizeChars() int           { return 0 }
func (t *NotebookEditTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *NotebookEditTool) IsDestructive(_ json.RawMessage) bool               { return true }
func (t *NotebookEditTool) ShouldDefer() bool                 { return true }
func (t *NotebookEditTool) GetPath(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.NotebookPath
}

func (t *NotebookEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"notebook_path":{"type":"string","description":"Path to the .ipynb file."},
			"cell_number":{"type":"integer","description":"0-indexed cell number to edit or insert at."},
			"new_source":{"type":"string","description":"New source content for the cell."},
			"cell_type":{"type":"string","enum":["code","markdown"],"description":"Cell type (for insert mode)."},
			"edit_mode":{"type":"string","enum":["replace","insert"],"description":"Operation mode. Default: replace."}
		},
		"required":["notebook_path","cell_number","new_source"]
	}`)
}

func (t *NotebookEditTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *NotebookEditTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.NotebookPath == "" {
		return fmt.Errorf("notebook_path must not be empty")
	}
	return nil
}

func (t *NotebookEditTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		path := in.NotebookPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(uctx.WorkDir, path)
		}

		// Read notebook JSON
		raw, err := util.ReadTextFile(path)
		if err != nil {
			ch <- errBlock("read notebook: " + err.Error())
			return
		}

		var nb map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &nb); err != nil {
			ch <- errBlock("parse notebook: " + err.Error())
			return
		}

		cells, ok := nb["cells"].([]interface{})
		if !ok {
			ch <- errBlock("notebook has no cells")
			return
		}

		editMode := in.EditMode
		if editMode == "" {
			editMode = "replace"
		}
		cellType := in.CellType
		if cellType == "" {
			cellType = "code"
		}

		newCell := map[string]interface{}{
			"cell_type":       cellType,
			"source":          []string{in.NewSource},
			"metadata":        map[string]interface{}{},
			"outputs":         []interface{}{},
			"execution_count": nil,
		}

		switch editMode {
		case "insert":
			idx := in.CellNumber
			if idx < 0 {
				idx = 0
			}
			if idx > len(cells) {
				idx = len(cells)
			}
			cells = append(cells[:idx], append([]interface{}{newCell}, cells[idx:]...)...)
		default: // replace
			if in.CellNumber < 0 || in.CellNumber >= len(cells) {
				ch <- errBlock(fmt.Sprintf("cell_number %d out of range (notebook has %d cells)", in.CellNumber, len(cells)))
				return
			}
			cells[in.CellNumber] = newCell
		}

		nb["cells"] = cells

		b, err := json.MarshalIndent(nb, "", " ")
		if err != nil {
			ch <- errBlock("marshal notebook: " + err.Error())
			return
		}
		if err := util.WriteTextContent(path, string(b)); err != nil {
			ch <- errBlock("write notebook: " + err.Error())
			return
		}

		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Successfully edited cell %d in %s", in.CellNumber, path),
		}
	}()
	return ch, nil
}

func errBlock(msg string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
}
