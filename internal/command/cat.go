package command

import (
	"context"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/overlay"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type CatOptions struct {
	Layer int
	FetchFlags
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
			"By default, shows the file as it appears in the merged filesystem,\n" +
			"which is what you'd see in a running container: files deleted by an\n" +
			"upper layer are gone, and symlinks are followed. Use --layer to read\n" +
			"from a specific layer.\n\n" +
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
	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunCat(ctx context.Context, cli *CLI, imageRef, filePath string, opts *CatOptions) error {
	logger := cli.Logger()
	logger.Debug("Reading file from image", "image", imageRef, "file", filePath)

	img, _, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	logger.Debug("Found layers", "count", len(layers))

	fs, layers, err := buildFS(layers, opts.Layer)
	if err != nil {
		return err
	}

	entry, err := fs.Resolve(filePath)
	if err != nil {
		return err
	}
	switch {
	case entry.IsDir():
		return fmt.Errorf("%s: is a directory", entry.Path)
	case !entry.IsRegular():
		return fmt.Errorf("%s: not a regular file", entry.Path)
	}

	logger.Debug("Resolved file", "path", entry.Path, "layer", entry.Layer+1)

	rc, err := overlay.Open(layers[entry.Layer], entry)
	if err != nil {
		return err
	}
	defer func() {
		_ = rc.Close()
	}()

	return cli.Cat().Render(&view.CatData{
		Reader: rc,
	})
}
