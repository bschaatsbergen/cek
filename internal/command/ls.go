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
	"github.com/bschaatsbergen/lix/internal/oci"
	"github.com/spf13/cobra"
)

type LsOptions struct {
	Layer    int
	Filter   string
	Platform string
	Pull     string
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
		Use:   "ls <image>",
		Short: "List files in an OCI image or specific layer",
		Long: highlight("lix ls alpine:latest") + "\n\n" +
			"List files in an OCI image or specific layer.\n\n" +
			"By default, shows files from the top layer. Use --layer\n" +
			"to show files from a specific layer.\n\n" +
			"Filter patterns support doublestar matching:\n" +
			"  fontconfig              Substring match anywhere in path\n" +
			"  *.conf                  Files ending with .conf (basename only)\n" +
			"  **/fontconfig/*.conf    .conf files in any fontconfig directory\n" +
			"  /etc/**/*.conf          .conf files under /etc\n\n" +
			"Examples:\n" +
			"  lix ls alpine:latest\n" +
			"  lix ls --layer 1 alpine:latest\n" +
			"  lix ls --filter '*.conf' nginx:alpine\n" +
			"  lix ls --filter '**/nginx/*.conf' nginx:alpine\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
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

	// Fetch the image
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

	// If a specific layer is requested, validate and select it
	// Otherwise, default to the top layer
	var layerIdx int
	if opts.Layer > 0 {
		if opts.Layer > len(layers) {
			return fmt.Errorf("layer %d does not exist (image has %d layers)", opts.Layer, len(layers))
		}
		layerIdx = opts.Layer - 1 // Convert to 0-indexed
	} else {
		// Default to the top layer
		layerIdx = len(layers) - 1
	}

	layer := layers[layerIdx]

	// Extract and list files from the layer
	files, err := extractFilesFromLayer(layer)
	if err != nil {
		return fmt.Errorf("failed to extract files from layer %d: %w", layerIdx+1, err)
	}

	// Filter files if pattern is specified
	if opts.Filter != "" {
		files = filterFiles(files, opts.Filter)
	}

	if len(files) == 0 {
		if opts.Filter != "" {
			cli.Printf("No files matching pattern '%s'\n", opts.Filter)
		} else {
			cli.Printf("No files found\n")
		}
		return nil
	}

	// Print files in tabular format
	//TODO: move this to the view package
	w := tabwriter.NewWriter(cli.Stream.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Mode\tSize\tPath\n")

	for _, file := range files {
		fmt.Fprintf(w, "%s\t%s\t%s\n", file.Mode, oci.FormatBytes(file.Size), file.Path)
	}

	w.Flush()

	return nil
}

// extractFilesFromLayer extracts file information from a layer's tar archive
func extractFilesFromLayer(layer interface {
	Uncompressed() (io.ReadCloser, error)
}) ([]FileInfo, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer rc.Close()

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

		// Skip if it's a whiteout file (used for deletions in layers)
		if strings.HasPrefix(filepath.Base(header.Name), ".wh.") {
			continue
		}

		// Determine file mode string (similar to ls -l)
		modeStr := formatFileMode(header.Typeflag, header.Mode)

		files = append(files, FileInfo{
			Mode: modeStr,
			Size: header.Size,
			Path: "/" + strings.TrimPrefix(header.Name, "/"),
		})
	}

	return files, nil
}

// formatFileMode converts tar type and mode to ls-style string
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

	// Convert mode to rwxrwxrwx format
	modeStr := fmt.Sprintf("%c%s%s%s",
		typeChar,
		formatPermission(mode>>6&7),
		formatPermission(mode>>3&7),
		formatPermission(mode&7),
	)

	return modeStr
}

// formatPermission converts a 3-bit permission to rwx format
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

// filterFiles filters files using doublestar pattern matching
func filterFiles(files []FileInfo, pattern string) []FileInfo {
	var filtered []FileInfo

	// If pattern has no wildcards or special chars, use substring matching
	hasWildcard := strings.ContainsAny(pattern, "*?[")

	for _, file := range files {
		matched := false

		if hasWildcard {
			// Use doublestar for glob patterns (supports ** for directory matching)
			pathForMatch := strings.TrimPrefix(file.Path, "/")

			// If pattern doesn't contain /, treat it as basename-only pattern
			if !strings.Contains(pattern, "/") {
				// Match against basename with implicit **/ prefix
				expandedPattern := "**/" + pattern
				if m, _ := doublestar.Match(expandedPattern, pathForMatch); m {
					matched = true
				}
			} else {
				// Pattern contains /, match against full path
				if m, _ := doublestar.Match(pattern, pathForMatch); m {
					matched = true
				}

				// Also try with leading slash if pattern starts with /
				if !matched && strings.HasPrefix(pattern, "/") {
					if m, _ := doublestar.Match(pattern, file.Path); m {
						matched = true
					}
				}
			}
		} else {
			// No wildcards: simple substring match
			matched = strings.Contains(file.Path, pattern)
		}

		if matched {
			filtered = append(filtered, file)
		}
	}
	return filtered
}
