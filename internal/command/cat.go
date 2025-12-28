package command

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/bschaatsbergen/cek/internal/oci"
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
		Long: highlight("cek cat alpine:latest /etc/alpine-release") + "\n\n" +
			"Show file contents from an OCI image.\n\n" +
			"By default, shows the file as it appears in the final overlay\n" +
			"(top layer), which is what you'd see in a running container.\n" +
			"Use --layer to read from a specific layer.\n\n" +
			"Examples:\n" +
			"  cek cat alpine:latest /etc/alpine-release\n" +
			"  cek cat --layer 2 nginx:alpine /etc/nginx/nginx.conf\n" +
			"  cek cat ubuntu:latest /etc/os-release\n",
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

	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}

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

	var layersToSearch []int
	if opts.Layer > 0 {
		if opts.Layer > len(layers) {
			return fmt.Errorf("layer %d does not exist (image has %d layers)", opts.Layer, len(layers))
		}
		layersToSearch = []int{opts.Layer - 1}
	} else {
		// Search top-down to find the final file state after all overlays.
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

	return fmt.Errorf("file not found: %s", filePath)
}

// extractFileFromLayer returns file contents if found in the layer's tar archive.
// Returns (content, found, error) where found indicates whether the file exists.
func extractFileFromLayer(layer interface {
	Uncompressed() (io.ReadCloser, error)
}, targetPath string) (content string, found bool, err error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return "", false, fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	tr := tar.NewReader(rc)
	normalizedTarget := "/" + strings.TrimPrefix(targetPath, "/")

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false, fmt.Errorf("failed to read tar header: %w", err)
		}

		tarPath := "/" + strings.TrimPrefix(header.Name, "/")

		if tarPath == normalizedTarget {
			// Whiteout files indicate the file was deleted in this layer.
			if strings.HasPrefix(filepath.Base(header.Name), ".wh.") {
				return "", false, nil
			}

			if header.Typeflag != tar.TypeReg {
				return "", false, fmt.Errorf("%s is not a regular file (type: %c)", normalizedTarget, header.Typeflag)
			}

			content, err := io.ReadAll(tr)
			if err != nil {
				return "", false, fmt.Errorf("failed to read file contents: %w", err)
			}

			return string(content), true, nil
		}
	}

	return "", false, nil
}
