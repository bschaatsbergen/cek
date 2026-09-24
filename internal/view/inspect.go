package view

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bschaatsbergen/cek/internal/oci"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// InspectData contains all the information needed to rendered.
type InspectData struct {
	ImageRef     string
	Registry     string
	Digest       v1.Hash
	Created      time.Time
	OS           string
	Architecture string
	TotalSize    int64
	Config       ConfigData
	Layers       []LayerData
}

// ConfigData is the runtime configuration from the image config.
type ConfigData struct {
	Entrypoint   []string
	Cmd          []string
	User         string
	WorkingDir   string
	ExposedPorts []string
	Volumes      []string
	Env          []string
	Labels       map[string]string
}

// LayerData contains information about a single layer.
type LayerData struct {
	Index       int
	Digest      v1.Hash
	Size        int64
	MediaType   string
	Annotations map[string]string
}

type InspectView interface {
	Render(data *InspectData) error
}

// maxAnnotationValueWidth caps annotation values in the human view. Values
// such as wrapped keys run to over a thousand characters and would drown the
// rest of the output; --json carries them in full.
const maxAnnotationValueWidth = 60

// Human view implementation
type inspectHumanView struct {
	*HumanView
}

func newInspectHumanView(hv *HumanView) *inspectHumanView {
	return &inspectHumanView{HumanView: hv}
}

func (v *inspectHumanView) Render(data *InspectData) error {
	v.Printf("Image: %s\n", data.ImageRef)
	v.Printf("Registry: %s\n", data.Registry)
	v.Printf("Digest: %s\n", data.Digest)
	v.Printf("Created: %s\n", data.Created.Format(time.RFC3339))
	v.Printf("OS/Arch: %s/%s\n", data.OS, data.Architecture)
	v.Printf("Size: %s\n", oci.FormatBytes(data.TotalSize))
	v.renderConfig(&data.Config)
	v.Printf("\n")
	v.Printf("Layers:\n")

	w := tabwriter.NewWriter(v.Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "#\tDigest\tSize\tMedia Type\n")

	for _, layer := range data.Layers {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", layer.Index, layer.Digest.String(), oci.FormatBytes(layer.Size), layer.MediaType)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	if !hasLayerAnnotations(data.Layers) {
		return nil
	}

	v.Printf("\n")
	v.Printf("Layer annotations:\n")

	w = tabwriter.NewWriter(v.Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "#\tKey\tValue\n")

	for _, layer := range data.Layers {
		for _, key := range slices.Sorted(maps.Keys(layer.Annotations)) {
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", layer.Index, key, truncate(layer.Annotations[key], maxAnnotationValueWidth))
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}

// renderConfig prints the runtime config, skipping anything unset so a
// minimal image stays a few lines. Entrypoint and Cmd read as a shell line.
func (v *inspectHumanView) renderConfig(c *ConfigData) {
	if len(c.Entrypoint) > 0 {
		v.Printf("Entrypoint: %s\n", shellJoin(c.Entrypoint))
	}
	if len(c.Cmd) > 0 {
		v.Printf("Cmd: %s\n", shellJoin(c.Cmd))
	}
	if c.User != "" {
		v.Printf("User: %s\n", c.User)
	}
	if c.WorkingDir != "" {
		v.Printf("WorkingDir: %s\n", c.WorkingDir)
	}
	if len(c.ExposedPorts) > 0 {
		v.Printf("Ports: %s\n", strings.Join(c.ExposedPorts, " "))
	}
	if len(c.Volumes) > 0 {
		v.Printf("Volumes: %s\n", strings.Join(c.Volumes, " "))
	}
	if len(c.Env) > 0 {
		v.Printf("Env:\n")
		for _, e := range c.Env {
			v.Printf("  %s\n", e)
		}
	}
	if len(c.Labels) > 0 {
		v.Printf("Labels:\n")
		for _, key := range slices.Sorted(maps.Keys(c.Labels)) {
			v.Printf("  %s=%s\n", key, truncate(c.Labels[key], maxAnnotationValueWidth))
		}
	}
}

// shellJoin renders an argv the way it would be typed in a shell: words
// separated by spaces, quoted only when they need it.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n\"'\\$`#;&|<>()") {
			quoted[i] = strconv.Quote(arg)
		} else {
			quoted[i] = arg
		}
	}
	return strings.Join(quoted, " ")
}

// truncate cuts s to width runes and marks the cut with an ellipsis.
func truncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width]) + "..."
}

func hasLayerAnnotations(layers []LayerData) bool {
	for _, layer := range layers {
		if len(layer.Annotations) > 0 {
			return true
		}
	}
	return false
}

// JSON view implementation
type inspectJSONView struct {
	*JSONView
}

func newInspectJSONView(jv *JSONView) *inspectJSONView {
	return &inspectJSONView{JSONView: jv}
}

func (v *inspectJSONView) Render(data *InspectData) error {
	type jsonLayer struct {
		Index       int               `json:"index"`
		Digest      string            `json:"digest"`
		Size        int64             `json:"size"`
		MediaType   string            `json:"mediaType"`
		Annotations map[string]string `json:"annotations,omitempty"`
	}

	type jsonConfig struct {
		Entrypoint   []string          `json:"entrypoint,omitempty"`
		Cmd          []string          `json:"cmd,omitempty"`
		User         string            `json:"user,omitempty"`
		WorkingDir   string            `json:"workingDir,omitempty"`
		ExposedPorts []string          `json:"exposedPorts,omitempty"`
		Volumes      []string          `json:"volumes,omitempty"`
		Env          []string          `json:"env,omitempty"`
		Labels       map[string]string `json:"labels,omitempty"`
	}

	type jsonOutput struct {
		Image    string      `json:"image"`
		Registry string      `json:"registry"`
		Digest   string      `json:"digest"`
		Created  string      `json:"created"`
		OS       string      `json:"os"`
		Arch     string      `json:"arch"`
		Size     int64       `json:"size"`
		Config   jsonConfig  `json:"config"`
		Layers   []jsonLayer `json:"layers"`
	}

	layers := make([]jsonLayer, len(data.Layers))
	for i, layer := range data.Layers {
		layers[i] = jsonLayer{
			Index:       layer.Index,
			Digest:      layer.Digest.String(),
			Size:        layer.Size,
			MediaType:   layer.MediaType,
			Annotations: layer.Annotations,
		}
	}

	output := jsonOutput{
		Image:    data.ImageRef,
		Registry: data.Registry,
		Digest:   data.Digest.String(),
		Created:  data.Created.Format(time.RFC3339),
		OS:       data.OS,
		Arch:     data.Architecture,
		Size:     data.TotalSize,
		Config:   jsonConfig(data.Config),
		Layers:   layers,
	}

	encoder := json.NewEncoder(v.Writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
