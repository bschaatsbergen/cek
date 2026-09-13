package view

import (
	"encoding/json"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/oci"
)

// CpData summarizes a copy out of an image.
type CpData struct {
	ImageRef    string
	Source      string
	Destination string
	// Files counts regular files and symlinks written.
	Files int
	Bytes int64
	// Skipped counts entries that cannot be written to disk, such as device
	// nodes and fifos.
	Skipped int
}

type CpView interface {
	Render(data *CpData) error
}

// Human view implementation
type cpHumanView struct {
	*HumanView
}

func newCpHumanView(hv *HumanView) *cpHumanView {
	return &cpHumanView{HumanView: hv}
}

func (v *cpHumanView) Render(data *CpData) error {
	noun := "files"
	if data.Files == 1 {
		noun = "file"
	}
	v.Printf("Copied %d %s (%s) from %s:%s to %s", data.Files, noun, oci.FormatBytes(data.Bytes), data.ImageRef, data.Source, data.Destination)
	if data.Skipped > 0 {
		v.Printf(", skipped %d special files", data.Skipped)
	}
	v.Printf("\n")
	return nil
}

// JSON view implementation
type cpJSONView struct {
	*JSONView
}

func newCpJSONView(jv *JSONView) *cpJSONView {
	return &cpJSONView{JSONView: jv}
}

func (v *cpJSONView) Render(data *CpData) error {
	type jsonOutput struct {
		Image       string `json:"image"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
		Files       int    `json:"files"`
		Bytes       int64  `json:"bytes"`
		Skipped     int    `json:"skipped"`
	}

	output := jsonOutput{
		Image:       data.ImageRef,
		Source:      data.Source,
		Destination: data.Destination,
		Files:       data.Files,
		Bytes:       data.Bytes,
		Skipped:     data.Skipped,
	}

	encoder := json.NewEncoder(v.Writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
