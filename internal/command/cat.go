package command

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/bschaatsbergen/lix/internal/oci"
	"github.com/spf13/cobra"
)

type CatOptions struct {
	Layer    int
	Platform string
	Pull     string
}

func NewCatCommand(cli *CLI) *cobra.Command {
	opts := CatOptions{
		Layer: -1, // -1 means use overlay (top layer view)
	}

	cmd := &cobra.Command{
		Use:   "cat <image> <filepath>",
		Short: "Show file contents from an OCI image",
		Long: highlight("lix cat alpine:latest /etc/alpine-release") + "\n\n" +
			"Show file contents from an OCI image.\n\n" +
			"By default, shows the file as it appears in the final overlay\n" +
			"(top layer), which is what you'd see in a running container.\n" +
			"Use --layer to read from a specific layer.\n\n" +
			"Examples:\n" +
			"  lix cat alpine:latest /etc/alpine-release\n" +
			"  lix cat --layer 2 nginx:alpine /etc/nginx/nginx.conf\n" +
			"  lix cat ubuntu:latest /etc/os-release\n",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			filePath := args[1]
			if err := RunCat(cmd.Context(), cli, imageRef, filePath, &opts); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&opts.Layer, "layer", -1, "Read file from a specific layer (1-indexed)")
	cmd.Flags().StringVar(&opts.Platform, "platform", "", "Specify platform (e.g., linux/amd64, linux/arm64)")
	cmd.Flags().StringVar(&opts.Pull, "pull", "if-not-present", "Image pull policy (always, if-not-present, never)")

	return cmd
}

func RunCat(ctx context.Context, cli *CLI, imageRef, filePath string, opts *CatOptions) error {
	logger := cli.Logger()
	logger.Debug("Reading file from image", "image", imageRef, "file", filePath)

	// Normalize file path to have leading slash
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}

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
	var layersToSearch []int
	if opts.Layer > 0 {
		// Specific layer requested
		if opts.Layer > len(layers) {
			return fmt.Errorf("layer %d does not exist (image has %d layers)", opts.Layer, len(layers))
		}
		layersToSearch = []int{opts.Layer - 1} // Convert to 0-indexed
	} else {
		// Search from top layer down (overlay view)
		// Later layers override earlier ones
		for i := len(layers) - 1; i >= 0; i-- {
			layersToSearch = append(layersToSearch, i)
		}
	}

	for _, layerIdx := range layersToSearch {
		layer := layers[layerIdx]
		content, found, err := extractFileFromLayer(layer, filePath)
		if err != nil {
			return fmt.Errorf("failed to read layer %d: %w", layerIdx+1, err)
		}

		if found {
			cli.Printf("%s", content)
			return nil
		}
	}

	// File not found in any layer
	return fmt.Errorf("file not found: %s", filePath)
}

// extractFileFromLayer searches for a file in a layer and returns its contents
func extractFileFromLayer(layer interface {
	Uncompressed() (io.ReadCloser, error)
}, targetPath string) (string, bool, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return "", false, fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)

	// Normalize the target path to have a leading slash
	normalizedTarget := "/" + strings.TrimPrefix(targetPath, "/")

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false, fmt.Errorf("failed to read tar header: %w", err)
		}

		// Normalize the tar path
		tarPath := "/" + strings.TrimPrefix(header.Name, "/")

		// Check if this is the file we're looking for
		if tarPath == normalizedTarget {
			// Check if it's a whiteout file (deletion marker)
			if strings.HasPrefix(filepath.Base(header.Name), ".wh.") {
				return "", false, nil // File was deleted in this layer
			}

			// Only read regular files
			if header.Typeflag != tar.TypeReg {
				return "", false, fmt.Errorf("%s is not a regular file (type: %c)", normalizedTarget, header.Typeflag)
			}

			// Read the file contents
			content, err := io.ReadAll(tr)
			if err != nil {
				return "", false, fmt.Errorf("failed to read file contents: %w", err)
			}

			return string(content), true, nil
		}
	}

	return "", false, nil
}
