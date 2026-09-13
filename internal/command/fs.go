package command

import (
	"fmt"

	"github.com/bschaatsbergen/cek/internal/overlay"
	"github.com/bschaatsbergen/cek/internal/view"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// buildFS assembles the merged filesystem of an image, or the view of a
// single layer when layer is 1 or more. The returned slice holds the layers
// the filesystem was built from, which is what Entry.Layer indexes into.
func buildFS(layers []v1.Layer, layer int, opts ...overlay.Option) (*overlay.FS, []v1.Layer, error) {
	if layer > 0 {
		if layer > len(layers) {
			return nil, nil, fmt.Errorf("layer %d does not exist (image has %d layers)", layer, len(layers))
		}
		layers = layers[layer-1 : layer]
	}

	fs, err := overlay.Build(layers, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read layers: %w", err)
	}
	return fs, layers, nil
}

// listFiles returns every file of the merged filesystem, or of one layer
// when layer is 1 or more, sorted by path.
func listFiles(layers []v1.Layer, layer int) ([]view.FileInfo, error) {
	fs, _, err := buildFS(layers, layer)
	if err != nil {
		return nil, err
	}

	entries := fs.Entries()
	files := make([]view.FileInfo, 0, len(entries))
	for _, e := range entries {
		files = append(files, fileInfo(e))
	}
	return files, nil
}

func fileInfo(e *overlay.Entry) view.FileInfo {
	return view.FileInfo{
		Mode: e.ModeString(),
		Size: e.Size,
		Path: e.Path,
	}
}
