package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type LsOptions struct {
	Layer  int
	Filter string
	Path   string
	FetchFlags
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
	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunLs(ctx context.Context, cli *CLI, imageRef string, opts *LsOptions) error {
	logger := cli.Logger()
	logger.Debug("Listing files in image", "image", imageRef)

	img, _, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	logger.Debug("Found layers", "count", len(layers))

	files, err := listFiles(layers, opts.Layer)
	if err != nil {
		return err
	}

	if opts.Path != "" {
		files = filterByPath(files, opts.Path)
	}

	if opts.Filter != "" {
		files = filterFiles(files, opts.Filter)
	}

	return cli.Ls().Render(&view.LsData{
		Files:  files,
		Path:   opts.Path,
		Filter: opts.Filter,
	})
}

// filterFiles applies glob or substring matching to filter the file list.
// Patterns without wildcards match as substrings. Patterns without slashes
// implicitly match against basenames with **/ prefix.
func filterFiles(files []view.FileInfo, pattern string) []view.FileInfo {
	var filtered []view.FileInfo
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

func filterByPath(files []view.FileInfo, path string) []view.FileInfo {
	// Tar paths are always absolute. Normalize to "/foo" to handle both "foo" and "/foo/".
	normalizedPath := "/" + strings.Trim(path, "/")

	var filtered []view.FileInfo
	for _, file := range files {
		// Suffix "/" prevents "/bin" matching "/sbin".
		if file.Path == normalizedPath || strings.HasPrefix(file.Path, normalizedPath+"/") {
			filtered = append(filtered, file)
		}
	}
	return filtered
}
