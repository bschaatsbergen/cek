package command

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type TreeOptions struct {
	Layer     int
	Platform  string
	Pull      string
	Path      string
	Level     int
	All       bool
	DirsOnly  bool
	Exclude   string
	Human     bool
	DirsFirst bool
}

func NewTreeCommand(cli *CLI) *cobra.Command {
	opts := TreeOptions{
		Layer:     -1, // -1 means overlay (merged) filesystem
		Level:     -1, // -1 means unlimited depth
		DirsFirst: true,
	}

	cmd := &cobra.Command{
		Use:   "tree <image> [path]",
		Short: "Display directory tree structure of an OCI image",
		Long: highlight("cek tree nginx:latest") + "\n\n" +
			"Display the directory tree structure of an OCI image.\n\n" +
			"By default, shows the merged overlay filesystem (all layers combined).\n" +
			"Use --layer to show files from a specific layer only.\n\n" +
			"Optionally specify a path to show tree starting from that directory.\n\n" +
			"Examples:\n" +
			"  cek tree alpine:latest\n" +
			"  cek tree nginx:latest /etc\n" +
			"  cek tree --layer 1 alpine:latest\n" +
			"  cek tree -L 2 nginx:latest\n" +
			"  cek tree -d nginx:latest\n" +
			"  cek tree -a alpine:latest /root\n" +
			"  cek tree --human nginx:latest /etc/nginx\n" +
			"  cek tree -I '*.conf' nginx:latest /etc\n",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			if len(args) > 1 {
				opts.Path = args[1]
			}
			return RunTree(cmd.Context(), cli, imageRef, &opts)
		},
	}

	cmd.Flags().IntVar(&opts.Layer, "layer", -1, "Show files from a specific layer (1-indexed)")
	cmd.Flags().IntVarP(&opts.Level, "L", "L", -1, "Descend only level directories deep")
	cmd.Flags().BoolVarP(&opts.All, "all", "a", false, "Show all files including hidden files")
	cmd.Flags().BoolVarP(&opts.DirsOnly, "d", "d", false, "List directories only")
	cmd.Flags().StringVarP(&opts.Exclude, "I", "I", "", "Exclude files matching pattern (glob)")
	cmd.Flags().BoolVar(&opts.Human, "human", false, "Show file sizes in human-readable format")
	cmd.Flags().BoolVar(&opts.DirsFirst, "dirsfirst", false, "List directories before files")
	cmd.Flags().StringVar(&opts.Platform, "platform", "", "Specify platform (e.g., linux/amd64, linux/arm64)")
	cmd.Flags().StringVar(&opts.Pull, "pull", "if-not-present", "Image pull policy (always, if-not-present, never)")

	return cmd
}

func RunTree(ctx context.Context, cli *CLI, imageRef string, opts *TreeOptions) error {
	logger := cli.Logger()
	logger.Debug("Building tree for image", "image", imageRef)

	fetchOpts := &oci.FetchOptions{
		Platform:   opts.Platform,
		PullPolicy: oci.PullPolicy(opts.Pull),
	}
	img, _, err := oci.FetchImage(ctx, imageRef, fetchOpts)
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	logger.Debug("Found layers", "count", len(layers))

	var files []view.FileInfo

	if opts.Layer > 0 {
		if opts.Layer > len(layers) {
			return fmt.Errorf("layer %d does not exist (image has %d layers)", opts.Layer, len(layers))
		}
		layerIdx := opts.Layer - 1
		layer := layers[layerIdx]

		var err error
		files, err = extractFilesFromLayer(layer)
		if err != nil {
			return fmt.Errorf("failed to extract files from layer %d: %w", layerIdx+1, err)
		}
	} else {
		var err error
		files, err = extractMergedFilesystem(layers)
		if err != nil {
			return fmt.Errorf("failed to extract merged filesystem: %w", err)
		}
	}

	rootPath := "/"
	if opts.Path != "" {
		rootPath = "/" + strings.Trim(opts.Path, "/")
	}

	printTreeSimple(cli, files, rootPath, opts)

	return nil
}

type dirEntry struct {
	name  string
	isDir bool
	path  string
	size  int64
}

func printTreeSimple(cli *CLI, files []view.FileInfo, rootPath string, opts *TreeOptions) {
	dirMap := make(map[string][]dirEntry)

	rootPath = strings.TrimSuffix(rootPath, "/")
	if rootPath == "" {
		rootPath = "/"
	}

	// Apply filters (path prefix, hidden files, dirs-only, exclude patterns,
	// depth).
	for _, file := range files {
		if !strings.HasPrefix(file.Path, rootPath) && file.Path != rootPath {
			continue
		}

		relPath := strings.TrimPrefix(file.Path, rootPath)
		relPath = strings.TrimPrefix(relPath, "/")
		relPath = strings.TrimSuffix(relPath, "/")
		if relPath == "" {
			continue
		}

		cleanPath := strings.TrimSuffix(file.Path, "/")
		baseName := filepath.Base(cleanPath)
		isDir := strings.HasPrefix(file.Mode, "d")

		if !opts.All && strings.HasPrefix(baseName, ".") && baseName != "." && baseName != ".." {
			continue
		}

		if opts.DirsOnly && !isDir {
			continue
		}

		// Exclude pattern matching tries both basename (e.g., "*.conf") and
		// full path (e.g., "/etc/**/*.conf") to match tree(1) behavior.
		if opts.Exclude != "" {
			matched, err := doublestar.Match(opts.Exclude, baseName)
			if err == nil && matched {
				continue
			}
			matched, err = doublestar.Match(opts.Exclude, cleanPath)
			if err == nil && matched {
				continue
			}
		}

		// Depth is calculated from the root path. A file directly under root
		// has depth 1. We trim trailing slashes to avoid counting empty
		// segments.
		depth := strings.Count(relPath, "/") + 1
		if opts.Level > 0 && depth > opts.Level {
			continue
		}

		dir := filepath.Dir(cleanPath)
		if dir == "." {
			dir = "/"
		}

		entry := dirEntry{
			name:  baseName,
			isDir: isDir,
			path:  cleanPath,
			size:  file.Size,
		}

		dirMap[dir] = append(dirMap[dir], entry)
	}

	// Sort entries within each directory. When --dirsfirst is enabled,
	// directories appear before files. Within each group, entries are
	// alphabetical.
	for dir := range dirMap {
		entries := dirMap[dir]
		sort.Slice(entries, func(i, j int) bool {
			if opts.DirsFirst && entries[i].isDir != entries[j].isDir {
				return entries[i].isDir
			}
			return entries[i].name < entries[j].name
		})
		dirMap[dir] = entries
	}

	rootName := filepath.Base(rootPath)
	if rootName == "/" || rootName == "" {
		rootName = "."
	}
	_, _ = fmt.Fprintf(cli.Writer, "%s\n", rootName)

	printTreeDir(cli, dirMap, rootPath, "", opts, 0)
}

func printTreeDir(cli *CLI, dirMap map[string][]dirEntry, dirPath, prefix string, opts *TreeOptions, currentDepth int) {
	if opts.Level > 0 && currentDepth > opts.Level {
		return
	}

	entries, exists := dirMap[dirPath]
	if !exists {
		return
	}

	for i, entry := range entries {
		isLast := i == len(entries)-1

		connector := "├── "
		if isLast {
			connector = "└── "
		}

		name := entry.name
		if entry.isDir {
			name += "/"
		}

		if opts.Human {
			sizeStr := oci.FormatBytes(entry.size)
			_, _ = fmt.Fprintf(cli.Writer, "%s%s[%5s]  %s\n", prefix, connector, sizeStr, name)
		} else {
			_, _ = fmt.Fprintf(cli.Writer, "%s%s%s\n", prefix, connector, name)
		}

		if entry.isDir {
			childPrefix := prefix
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
			printTreeDir(cli, dirMap, entry.path, childPrefix, opts, currentDepth+1)
		}
	}
}
