package view

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/bschaatsbergen/cek/internal/oci"
)

// FileInfo represents information about a file in an image.
type FileInfo struct {
	Mode string
	Size int64
	// Path is absolute and clean, without a trailing slash.
	Path string
	// Link is the target of a symlink, empty for anything else.
	Link string
}

// IsDir reports whether the entry is a directory, from its mode string.
func (f FileInfo) IsDir() bool {
	return strings.HasPrefix(f.Mode, "d")
}

// displayPath is the path as ls shows it: directories end in a slash and
// symlinks point at their target.
func (f FileInfo) displayPath() string {
	switch {
	case f.IsDir():
		return f.Path + "/"
	case f.Link != "":
		return f.Path + " -> " + f.Link
	default:
		return f.Path
	}
}

// LsData contains the file listing information to be rendered.
type LsData struct {
	Files  []FileInfo
	Path   string
	Filter string
}

type LsView interface {
	Render(data *LsData) error
}

// Human view implementation
type lsHumanView struct {
	*HumanView
}

func newLsHumanView(hv *HumanView) *lsHumanView {
	return &lsHumanView{HumanView: hv}
}

func (v *lsHumanView) Render(data *LsData) error {
	if len(data.Files) == 0 {
		switch {
		case data.Path != "" && data.Filter != "":
			v.Printf("No files matching pattern '%s' in path '%s'\n", data.Filter, data.Path)
		case data.Path != "":
			v.Printf("No files found in path '%s'\n", data.Path)
		case data.Filter != "":
			v.Printf("No files matching pattern '%s'\n", data.Filter)
		default:
			v.Printf("No files found\n")
		}
		return nil
	}

	w := tabwriter.NewWriter(v.Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Mode\tSize\tPath\n")

	for _, file := range data.Files {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", file.Mode, oci.FormatBytes(file.Size), file.displayPath())
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}

// JSON view implementation
type lsJSONView struct {
	*JSONView
}

func newLsJSONView(jv *JSONView) *lsJSONView {
	return &lsJSONView{JSONView: jv}
}

func (v *lsJSONView) Render(data *LsData) error {
	type jsonFile struct {
		Mode string `json:"mode"`
		Size int64  `json:"size"`
		Path string `json:"path"`
		Link string `json:"link,omitempty"`
	}

	type jsonOutput struct {
		Files   []jsonFile `json:"files,omitempty"`
		Message string     `json:"message,omitempty"`
	}

	output := jsonOutput{}

	if len(data.Files) == 0 {
		// Construct appropriate message based on filters
		switch {
		case data.Path != "" && data.Filter != "":
			output.Message = fmt.Sprintf("No files matching pattern '%s' in path '%s'", data.Filter, data.Path)
		case data.Path != "":
			output.Message = fmt.Sprintf("No files found in path '%s'", data.Path)
		case data.Filter != "":
			output.Message = fmt.Sprintf("No files matching pattern '%s'", data.Filter)
		default:
			output.Message = "No files found"
		}
	} else {
		files := make([]jsonFile, len(data.Files))
		for i, file := range data.Files {
			files[i] = jsonFile(file)
		}
		output.Files = files
	}

	encoder := json.NewEncoder(v.Writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
