package command

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/bschaatsbergen/cek/internal/oci"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
)

type LsOptions struct {
	Layer    int
	Filter   string
	Platform string
	Pull     string
	Path     string
}

type FileInfo struct {
	Mode string
	Size int64
	Path string
}

func NewLsCommand(cli *CLI) *cobra.Command {
	opts := LsOptions{
		Layer: -1, // -1 means default to top layer
	}

	cmd := &cobra.Command{
		Use:   "ls <image> [path]",
		Short: "List files in an OCI image or specific layer",
		Long: highlight("cek ls alpine:latest /etc") + "\n\n" +
			"List files in an OCI image or specific layer.\n\n" +
			"By default, shows the merged overlay filesystem (all layers combined).\n" +
			"Use --layer to show files from a specific layer only.\n\n" +
			"Optionally specify a path to list only files under that directory.\n\n" +
			"Filter patterns support doublestar matching:\n" +
			"  fontconfig              Substring match anywhere in path\n" +
			"  *.conf                  Files ending with .conf (basename only)\n" +
			"  **/fontconfig/*.conf    .conf files in any fontconfig directory\n" +
			"  /etc/**/*.conf          .conf files under /etc\n\n" +
			"Examples:\n" +
			"  cek ls alpine:latest\n" +
			"  cek ls alpine:latest /etc\n" +
			"  cek ls nginx:latest /etc/nginx\n" +
			"  cek ls --layer 1 alpine:latest\n" +
			"  cek ls --filter '*.conf' nginx:alpine\n" +
			"  cek ls --filter '**/nginx/*.conf' nginx:alpine\n",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			if len(args) > 1 {
				opts.Path = args[1]
			}
			if err := RunLs(cmd.Context(), cli, imageRef, &opts); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&opts.Layer, "layer", -1, "Show files from a specific layer (1-indexed)")
	cmd.Flags().StringVar(&opts.Filter, "filter", "", "Filter file paths by pattern")
	cmd.Flags().StringVar(&opts.Platform, "platform", "", "Specify platform (e.g., linux/amd64, linux/arm64)")
	cmd.Flags().StringVar(&opts.Pull, "pull", "if-not-present", "Image pull policy (always, if-not-present, never)")

	return cmd
}

func RunLs(ctx context.Context, cli *CLI, imageRef string, opts *LsOptions) error {
	logger := cli.Logger()
	logger.Debug("Listing files in image", "image", imageRef)

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

	var files []FileInfo

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

	if opts.Path != "" {
		files = filterByPath(files, opts.Path)
	}

	if opts.Filter != "" {
		files = filterFiles(files, opts.Filter)
	}

	if len(files) == 0 {
		switch {
		case opts.Path != "" && opts.Filter != "":
			cli.Printf("No files matching pattern '%s' in path '%s'\n", opts.Filter, opts.Path)
		case opts.Path != "":
			cli.Printf("No files found in path '%s'\n", opts.Path)
		case opts.Filter != "":
			cli.Printf("No files matching pattern '%s'\n", opts.Filter)
		default:
			cli.Printf("No files found\n")
		}
		return nil
	}

	//TODO: move this to the view package
	w := tabwriter.NewWriter(cli.Writer, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Mode\tSize\tPath\n")

	for _, file := range files {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", file.Mode, oci.FormatBytes(file.Size), file.Path)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}

func extractFilesFromLayer(layer interface {
	Uncompressed() (io.ReadCloser, error)
}) ([]FileInfo, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	var files []FileInfo
	tr := tar.NewReader(rc)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if strings.HasPrefix(filepath.Base(header.Name), ".wh.") {
			continue
		}

		modeStr := formatFileMode(header.Typeflag, header.Mode)

		files = append(files, FileInfo{
			Mode: modeStr,
			Size: header.Size,
			Path: "/" + strings.TrimPrefix(header.Name, "/"),
		})
	}

	return files, nil
}

// extractMergedFilesystem builds the final overlay filesystem state by processing
// all layers bottom-up. Later layers override files from earlier layers.
func extractMergedFilesystem(layers []v1.Layer) ([]FileInfo, error) {
	fileMap := make(map[string]FileInfo)

	for _, layer := range layers {
		rc, err := layer.Uncompressed()
		if err != nil {
			return nil, fmt.Errorf("failed to get uncompressed layer: %w", err)
		}

		tr := tar.NewReader(rc)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				_ = rc.Close()
				return nil, fmt.Errorf("failed to read tar header: %w", err)
			}

			path := "/" + strings.TrimPrefix(header.Name, "/")

			// Whiteout files remove entries from the overlay.
			if strings.HasPrefix(filepath.Base(header.Name), ".wh.") {
				delete(fileMap, path)
				continue
			}

			modeStr := formatFileMode(header.Typeflag, header.Mode)
			fileMap[path] = FileInfo{
				Mode: modeStr,
				Size: header.Size,
				Path: path,
			}
		}
		_ = rc.Close()
	}

	files := make([]FileInfo, 0, len(fileMap))
	for _, file := range fileMap {
		files = append(files, file)
	}

	return files, nil
}

func formatFileMode(typeflag byte, mode int64) string {
	var typeChar byte
	switch typeflag {
	case tar.TypeDir:
		typeChar = 'd'
	case tar.TypeSymlink:
		typeChar = 'l'
	case tar.TypeBlock:
		typeChar = 'b'
	case tar.TypeChar:
		typeChar = 'c'
	case tar.TypeFifo:
		typeChar = 'p'
	default:
		typeChar = '-'
	}

	modeStr := fmt.Sprintf("%c%s%s%s",
		typeChar,
		formatPermission(mode>>6&7),
		formatPermission(mode>>3&7),
		formatPermission(mode&7),
	)

	return modeStr
}

func formatPermission(perm int64) string {
	r := "-"
	w := "-"
	x := "-"
	if perm&4 != 0 {
		r = "r"
	}
	if perm&2 != 0 {
		w = "w"
	}
	if perm&1 != 0 {
		x = "x"
	}
	return r + w + x
}

// filterFiles applies glob or substring matching to filter the file list.
// Patterns without wildcards match as substrings. Patterns without slashes
// implicitly match against basenames with **/ prefix.
func filterFiles(files []FileInfo, pattern string) []FileInfo {
	var filtered []FileInfo
	hasWildcard := strings.ContainsAny(pattern, "*?[")

	for _, file := range files {
		matched := false

		if hasWildcard {
			pathForMatch := strings.TrimPrefix(file.Path, "/")

			if !strings.Contains(pattern, "/") {
				expandedPattern := "**/" + pattern
				if m, _ := doublestar.Match(expandedPattern, pathForMatch); m {
					matched = true
				}
			} else {
				if m, _ := doublestar.Match(pattern, pathForMatch); m {
					matched = true
				}

				if !matched && strings.HasPrefix(pattern, "/") {
					if m, _ := doublestar.Match(pattern, file.Path); m {
						matched = true
					}
				}
			}
		} else {
			matched = strings.Contains(file.Path, pattern)
		}

		if matched {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func filterByPath(files []FileInfo, path string) []FileInfo {
	// Tar paths are always absolute. Normalize to "/foo" to handle both "foo" and "/foo/".
	normalizedPath := "/" + strings.Trim(path, "/")

	var filtered []FileInfo
	for _, file := range files {
		// Suffix "/" prevents "/bin" matching "/sbin".
		if file.Path == normalizedPath || strings.HasPrefix(file.Path, normalizedPath+"/") {
			filtered = append(filtered, file)
		}
	}
	return filtered
}
