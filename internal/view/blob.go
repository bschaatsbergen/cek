package view

import (
	"errors"
	"fmt"
	"io"
)

// BlobData carries the raw blob bytes to be written.
type BlobData struct {
	Reader io.Reader
}

type BlobView interface {
	Render(data *BlobData) error
}

// Human view implementation
type blobHumanView struct {
	*HumanView
}

func newBlobHumanView(hv *HumanView) *blobHumanView {
	return &blobHumanView{HumanView: hv}
}

func (v *blobHumanView) Render(data *BlobData) error {
	if _, err := io.Copy(v.Writer, data.Reader); err != nil {
		return fmt.Errorf("failed to write blob: %w", err)
	}
	return nil
}

// JSON view implementation
type blobJSONView struct {
	*JSONView
}

func newBlobJSONView(jv *JSONView) *blobJSONView {
	return &blobJSONView{JSONView: jv}
}

func (v *blobJSONView) Render(data *BlobData) error {
	return errors.New("blob writes raw bytes and does not support JSON output")
}
