package command

import (
	"context"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type ManifestOptions struct {
	FetchFlags
}

func NewManifestCommand(cli *CLI) *cobra.Command {
	opts := ManifestOptions{}

	cmd := &cobra.Command{
		Use:   "manifest <image>",
		Short: "Print the image manifest",
		Long: highlight("cek manifest nginx:latest | jq '.layers[-1]'") + "\n\n" +
			"Print the image manifest as JSON: the config descriptor, every layer\n" +
			"descriptor with its media type, size, digest and annotations, and the\n" +
			"manifest's own annotations.\n\n" +
			"The output is indented for reading. With --json the exact bytes are\n" +
			"written instead, so the output hashes to the manifest digest.\n\n" +
			"A manifest served by a local container daemon is what the daemon\n" +
			"exports, not the registry's copy. Use --pull always for the latter.\n\n" +
			"Examples:\n" +
			"  cek manifest nginx:latest\n" +
			"  cek manifest nginx:latest | jq '.layers[-1]'\n" +
			"  cek --json manifest --pull always nginx:latest | shasum -a 256\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			return RunManifest(cmd.Context(), cli, imageRef, &opts)
		},
	}

	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunManifest(ctx context.Context, cli *CLI, imageRef string, opts *ManifestOptions) error {
	logger := cli.Logger()
	logger.Debug("Printing manifest", "image", imageRef)

	img, _, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
	if err != nil {
		return err
	}

	raw, err := img.RawManifest()
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	return cli.Raw().Render(&view.RawData{
		Content: raw,
	})
}
