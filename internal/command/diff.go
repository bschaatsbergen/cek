package command

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/overlay"
	"github.com/bschaatsbergen/cek/internal/view"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
)

type DiffOptions struct {
	Path string
	FetchFlags
}

func NewDiffCommand(cli *CLI) *cobra.Command {
	opts := DiffOptions{}

	cmd := &cobra.Command{
		Use:   "diff <image-a> <image-b> [path]",
		Short: "Compare two images",
		Long: highlight("cek diff nginx:1.26 nginx:1.28") + "\n\n" +
			"Compare two images: which layers they share, and which files were\n" +
			"added, removed or modified between their merged filesystems.\n\n" +
			"Files are compared by content, not by size or timestamp. A file counts\n" +
			"as modified when its content, its permissions or its symlink target\n" +
			"changed. Directories are not listed; their files are.\n\n" +
			"Markers follow terraform plan: + added, - removed, ~ modified, = shared.\n\n" +
			"Optionally specify a path to compare only files under that directory.\n\n" +
			"Examples:\n" +
			"  cek diff nginx:1.26 nginx:1.28\n" +
			"  cek diff nginx:1.26 nginx:1.28 /etc/nginx\n" +
			"  cek diff --platform linux/amd64 myapp:v1 myapp:v2\n" +
			"  cek --json diff myapp:v1 myapp:v2 | jq '.files[] | select(.status == \"modified\")'\n",
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				opts.Path = args[2]
			}
			return RunDiff(cmd.Context(), cli, args[0], args[1], &opts)
		},
	}

	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunDiff(ctx context.Context, cli *CLI, refA, refB string, opts *DiffOptions) error {
	logger := cli.Logger()
	logger.Debug("Comparing images", "a", refA, "b", refB)

	a, err := loadForDiff(ctx, refA, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", refA, err)
	}
	b, err := loadForDiff(ctx, refB, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", refB, err)
	}

	files := diffFiles(a.fs, b.fs, opts.Path)
	logger.Debug("Compared files", "changed", len(files))

	return cli.Diff().Render(&view.DiffData{
		A:      refA,
		B:      refB,
		Layers: diffLayers(a.layers, b.layers),
		Files:  files,
	})
}

// diffSide is one image loaded for comparison.
type diffSide struct {
	layers []v1.Descriptor
	fs     *overlay.FS
}

func loadForDiff(ctx context.Context, imageRef string, opts *DiffOptions) (*diffSide, error) {
	img, _, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
	if err != nil {
		return nil, err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	fs, _, err := buildFS(layers, -1, overlay.WithDigests())
	if err != nil {
		return nil, err
	}

	return &diffSide{layers: manifest.Layers, fs: fs}, nil
}

// diffLayers walks a's layers in order, marking each shared or removed,
// then lists the layers only b has.
func diffLayers(a, b []v1.Descriptor) []view.LayerDiff {
	inA := make(map[string]bool, len(a))
	for _, d := range a {
		inA[d.Digest.String()] = true
	}
	inB := make(map[string]bool, len(b))
	for _, d := range b {
		inB[d.Digest.String()] = true
	}

	var diffs []view.LayerDiff
	for _, d := range a {
		status := view.DiffRemoved
		if inB[d.Digest.String()] {
			status = view.DiffShared
		}
		diffs = append(diffs, view.LayerDiff{Status: status, Digest: d.Digest.String(), Size: d.Size})
	}
	for _, d := range b {
		if !inA[d.Digest.String()] {
			diffs = append(diffs, view.LayerDiff{Status: view.DiffAdded, Digest: d.Digest.String(), Size: d.Size})
		}
	}
	return diffs
}

// diffFiles compares the two filesystems under scope, or everywhere when
// scope is empty. Directories only appear when one side has something else
// at that path.
func diffFiles(a, b *overlay.FS, scope string) []view.FileDiff {
	entriesA := scopedEntries(a, scope)
	entriesB := scopedEntries(b, scope)

	paths := make(map[string]struct{}, len(entriesA)+len(entriesB))
	for p := range entriesA {
		paths[p] = struct{}{}
	}
	for p := range entriesB {
		paths[p] = struct{}{}
	}

	var diffs []view.FileDiff
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		ea, inA := entriesA[p]
		eb, inB := entriesB[p]

		switch {
		case inA && inB:
			if ea.IsDir() && eb.IsDir() {
				continue
			}
			if !fileChanged(ea, eb) {
				continue
			}
			diffs = append(diffs, view.FileDiff{Status: view.DiffModified, Path: p, A: fileState(ea), B: fileState(eb)})
		case inA:
			if ea.IsDir() {
				continue
			}
			diffs = append(diffs, view.FileDiff{Status: view.DiffRemoved, Path: p, A: fileState(ea)})
		default:
			if eb.IsDir() {
				continue
			}
			diffs = append(diffs, view.FileDiff{Status: view.DiffAdded, Path: p, B: fileState(eb)})
		}
	}
	return diffs
}

func scopedEntries(fs *overlay.FS, scope string) map[string]*overlay.Entry {
	var entries []*overlay.Entry
	if scope == "" {
		entries = fs.Entries()
	} else {
		entries = fs.Under(scope)
	}

	byPath := make(map[string]*overlay.Entry, len(entries))
	for _, e := range entries {
		byPath[e.Path] = e
	}
	return byPath
}

func fileChanged(a, b *overlay.Entry) bool {
	return a.Typeflag != b.Typeflag ||
		a.Mode != b.Mode ||
		a.Linkname != b.Linkname ||
		a.Digest != b.Digest
}

func fileState(e *overlay.Entry) *view.FileState {
	s := &view.FileState{
		Mode: e.ModeString(),
		Size: e.Size,
	}
	if e.IsSymlink() {
		s.Link = e.Linkname
	}
	if e.Digest != "" {
		s.Digest = "sha256:" + e.Digest
	}
	return s
}
