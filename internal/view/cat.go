package view

import (
	"encoding/json"
	"fmt"
	"io"
)

// CatData carries the file content to be rendered.
type CatData struct {
	Reader io.Reader
}

type CatView interface {
	Render(data *CatData) error
}

// Human view implementation: the bytes, streamed.
type catHumanView struct {
	*HumanView
}

func newCatHumanView(hv *HumanView) *catHumanView {
	return &catHumanView{HumanView: hv}
}

func (v *catHumanView) Render(data *CatData) error {
	if _, err := io.Copy(v.Writer, data.Reader); err != nil {
		return fmt.Errorf("failed to write file contents: %w", err)
	}
	return nil
}

// JSON view implementation
type catJSONView struct {
	*JSONView
}

func newCatJSONView(jv *JSONView) *catJSONView {
	return &catJSONView{JSONView: jv}
}

func (v *catJSONView) Render(data *CatData) error {
	type jsonOutput struct {
		Content string `json:"content"`
	}

	content, err := io.ReadAll(data.Reader)
	if err != nil {
		return fmt.Errorf("failed to read file contents: %w", err)
	}

	output := jsonOutput{
		Content: string(content),
	}

	encoder := json.NewEncoder(v.Writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
