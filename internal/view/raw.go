package view

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RawData carries a JSON document exactly as the registry or daemon served
// it, such as an image manifest or config blob.
type RawData struct {
	Content []byte
}

type RawView interface {
	Render(data *RawData) error
}

// Human view implementation: indented for reading.
type rawHumanView struct {
	*HumanView
}

func newRawHumanView(hv *HumanView) *rawHumanView {
	return &rawHumanView{HumanView: hv}
}

func (v *rawHumanView) Render(data *RawData) error {
	var out bytes.Buffer
	if err := json.Indent(&out, data.Content, "", "  "); err != nil {
		// Not valid JSON: show it as is rather than hide it.
		out.Reset()
		out.Write(data.Content)
	}
	out.WriteByte('\n')

	if _, err := v.Writer.Write(out.Bytes()); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// JSON view implementation: the exact bytes, so the output hashes to the
// document's digest.
type rawJSONView struct {
	*JSONView
}

func newRawJSONView(jv *JSONView) *rawJSONView {
	return &rawJSONView{JSONView: jv}
}

func (v *rawJSONView) Render(data *RawData) error {
	if _, err := v.Writer.Write(data.Content); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
