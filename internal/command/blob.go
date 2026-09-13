package command

import (
	"context"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type BlobOptions struct {
	Layer int
	FetchFlags
}

func NewBlobCommand(cli *CLI) *cobra.Command {
	opts := BlobOptions{}

	cmd := &cobra.Command{
		Use:   "blob <image>",
		Short: "Write a raw layer blob to standard output",
		Long: highlight("cek blob --layer 1 alpine:latest | head -c 32 | xxd") + "\n\n" +
			"Write the raw bytes of a layer blob to standard output, exactly as the\n" +
			"registry stores them. The blob is neither decompressed nor unpacked, so\n" +
			"a gzip layer starts with the gzip magic bytes 1f 8b.\n\n" +
			"Use --pull always to read the blob from the registry. A blob served by\n" +
			"a local container daemon is re-exported by the daemon and may not be\n" +
			"byte-identical to the registry copy.\n\n" +
			"Examples:\n" +
			"  cek blob --layer 1 alpine:latest | head -c 32 | xxd\n" +
			"  cek blob --layer 2 --pull always nginx:latest > layer.tar.gz\n" +
			"  cek blob --layer 2 --pull always nginx:latest | shasum -a 256\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			return RunBlob(cmd.Context(), cli, imageRef, &opts)
		},
	}

	cmd.Flags().IntVar(&opts.Layer, "layer", 0, "Layer to write (1-indexed, required)")
	_ = cmd.MarkFlagRequired("layer")
	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunBlob(ctx context.Context, cli *CLI, imageRef string, opts *BlobOptions) error {
	logger := cli.Logger()
	logger.Debug("Writing layer blob", "image", imageRef, "layer", opts.Layer)

	if opts.Layer < 1 {
		return fmt.Errorf("layer must be 1 or greater, got %d", opts.Layer)
	}

	img, _, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	logger.Debug("Found layers", "count", len(layers))

	if opts.Layer > len(layers) {
		return fmt.Errorf("layer %d does not exist (image has %d layers)", opts.Layer, len(layers))
	}
	layer := layers[opts.Layer-1]

	// Compressed returns the blob as stored: no gzip handling, no tar parsing.
	rc, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("failed to open layer blob: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	return cli.Blob().Render(&view.BlobData{
		Reader: rc,
	})
}
