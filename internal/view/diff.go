package view

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/fatih/color"
)

// DiffStatus says how an item differs between two images.
type DiffStatus string

const (
	DiffAdded    DiffStatus = "added"
	DiffRemoved  DiffStatus = "removed"
	DiffModified DiffStatus = "modified"
	DiffShared   DiffStatus = "shared"
)

// DiffData is the comparison of two images.
type DiffData struct {
	A      string
	B      string
	Layers []LayerDiff
	Files  []FileDiff
}

// LayerDiff is one layer and whether both images have it.
type LayerDiff struct {
	Status DiffStatus
	Digest string
	Size   int64
}

// FileDiff is one path that differs between the images. A and B describe
// the file on each side and are nil where it is absent.
type FileDiff struct {
	Status DiffStatus
	Path   string
	A      *FileState
	B      *FileState
}

// FileState is what a diff compares: type and permissions, content, and
// where a symlink points.
type FileState struct {
	Mode   string
	Size   int64
	Link   string
	Digest string
}

// IsSymlink reports whether the state describes a symlink.
func (s *FileState) IsSymlink() bool {
	return strings.HasPrefix(s.Mode, "l")
}

// Summary counts the file changes by status.
func (d *DiffData) Summary() (added, removed, modified int) {
	for _, f := range d.Files {
		switch f.Status {
		case DiffAdded:
			added++
		case DiffRemoved:
			removed++
		case DiffModified:
			modified++
		}
	}
	return added, removed, modified
}

type DiffView interface {
	Render(data *DiffData) error
}

// marker follows terraform plan: + added, - removed, ~ changed, = same.
func marker(status DiffStatus) string {
	switch status {
	case DiffAdded:
		return color.GreenString("+")
	case DiffRemoved:
		return color.RedString("-")
	case DiffModified:
		return color.YellowString("~")
	default:
		return "="
	}
}

// Human view implementation
type diffHumanView struct {
	*HumanView
}

func newDiffHumanView(hv *HumanView) *diffHumanView {
	return &diffHumanView{HumanView: hv}
}

func (v *diffHumanView) Render(data *DiffData) error {
	v.Printf("Layers:\n")
	w := tabwriter.NewWriter(v.Writer, 0, 0, 2, ' ', 0)
	for _, l := range data.Layers {
		_, _ = fmt.Fprintf(w, "  %s %s\t%s\n", marker(l.Status), l.Digest, oci.FormatBytes(l.Size))
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	v.Printf("\n")
	if len(data.Files) == 0 {
		v.Printf("Files: no changes\n")
		return nil
	}

	v.Printf("Files:\n")
	w = tabwriter.NewWriter(v.Writer, 0, 0, 2, ' ', 0)
	for _, f := range data.Files {
		_, _ = fmt.Fprintf(w, "  %s %s\t%s\n", marker(f.Status), f.Path, fileDetail(&f))
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	added, removed, modified := data.Summary()
	v.Printf("\n%d added, %d removed, %d modified\n", added, removed, modified)
	return nil
}

// fileDetail says what a row is about: the size or link target of an added
// or removed file, and for a modified one, each aspect that changed.
func fileDetail(f *FileDiff) string {
	switch f.Status {
	case DiffAdded:
		return stateDetail(f.B)
	case DiffRemoved:
		return stateDetail(f.A)
	}

	var changes []string
	if f.A.Mode != f.B.Mode {
		changes = append(changes, fmt.Sprintf("mode %s -> %s", f.A.Mode, f.B.Mode))
	}
	if f.A.Link != f.B.Link {
		changes = append(changes, fmt.Sprintf("link %s -> %s", f.A.Link, f.B.Link))
	}
	if f.A.Digest != f.B.Digest {
		changes = append(changes, fmt.Sprintf("%s -> %s", oci.FormatBytes(f.A.Size), oci.FormatBytes(f.B.Size)))
	}
	return strings.Join(changes, ", ")
}

func stateDetail(s *FileState) string {
	if s.IsSymlink() {
		return "-> " + s.Link
	}
	return oci.FormatBytes(s.Size)
}

// JSON view implementation
type diffJSONView struct {
	*JSONView
}

func newDiffJSONView(jv *JSONView) *diffJSONView {
	return &diffJSONView{JSONView: jv}
}

func (v *diffJSONView) Render(data *DiffData) error {
	type jsonLayer struct {
		Status DiffStatus `json:"status"`
		Digest string     `json:"digest"`
		Size   int64      `json:"size"`
	}

	type jsonState struct {
		Mode   string `json:"mode"`
		Size   int64  `json:"size"`
		Link   string `json:"link,omitempty"`
		Digest string `json:"digest,omitempty"`
	}

	type jsonFile struct {
		Status DiffStatus `json:"status"`
		Path   string     `json:"path"`
		A      *jsonState `json:"a,omitempty"`
		B      *jsonState `json:"b,omitempty"`
	}

	type jsonSummary struct {
		Added    int `json:"added"`
		Removed  int `json:"removed"`
		Modified int `json:"modified"`
	}

	type jsonOutput struct {
		A       string      `json:"a"`
		B       string      `json:"b"`
		Layers  []jsonLayer `json:"layers"`
		Files   []jsonFile  `json:"files"`
		Summary jsonSummary `json:"summary"`
	}

	state := func(s *FileState) *jsonState {
		if s == nil {
			return nil
		}
		return &jsonState{Mode: s.Mode, Size: s.Size, Link: s.Link, Digest: s.Digest}
	}

	layers := make([]jsonLayer, len(data.Layers))
	for i, l := range data.Layers {
		layers[i] = jsonLayer(l)
	}

	files := make([]jsonFile, len(data.Files))
	for i, f := range data.Files {
		files[i] = jsonFile{Status: f.Status, Path: f.Path, A: state(f.A), B: state(f.B)}
	}

	added, removed, modified := data.Summary()
	output := jsonOutput{
		A:       data.A,
		B:       data.B,
		Layers:  layers,
		Files:   files,
		Summary: jsonSummary{Added: added, Removed: removed, Modified: modified},
	}

	encoder := json.NewEncoder(v.Writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
