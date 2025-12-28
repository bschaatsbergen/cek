package command

import (
	"context"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
)

type ExportOptions struct {
	Output   string
	Pull     string
	Platform string
}

func NewExportCommand(cli *CLI) *cobra.Command {
	opts := ExportOptions{}

	cmd := &cobra.Command{
		Use:   "export <image>",
		Short: "Export an OCI image to a tar file",
		Long: highlight("cek export alpine:latest -o alpine.tar") + "\n\n" +
			"Export saves an image to a tarball that can be loaded through a container daemon.\n\n" +
			"The exported tar contains the full image (manifest, config, and all layers)\n" +
			"in OCI format. Use this to transfer images between systems, create backups,\n" +
			"or share images without a registry.\n\n" +
			"Examples:\n" +
			"  cek export alpine:latest -o alpine.tar\n" +
			"  cek export nginx:latest --output nginx.tar --pull always\n" +
			"  cek export --platform linux/amd64 ubuntu:22.04 -o ubuntu-amd64.tar\n\n" +
			"Load the exported tar with:\n" +
			"  docker load -i alpine.tar\n" +
			"  podman load -i alpine.tar",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			return RunExport(cmd.Context(), cli, imageRef, &opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "Output file path (required)")
	_ = cmd.MarkFlagRequired("output")
	cmd.Flags().StringVar(&opts.Pull, "pull", "if-not-present", "Pull policy (always, if-not-present, never)")
	cmd.Flags().StringVar(&opts.Platform, "platform", "", "Target platform (e.g., linux/amd64, linux/arm64)")

	return cmd
}

func RunExport(ctx context.Context, cli *CLI, imageRef string, opts *ExportOptions) error {
	logger := cli.Logger()
	logger.Debug("Exporting image", "image", imageRef, "output", opts.Output)

	fetchOpts := &oci.FetchOptions{
		Platform:   opts.Platform,
		PullPolicy: oci.PullPolicy(opts.Pull),
	}
	img, ref, err := oci.FetchImage(ctx, imageRef, fetchOpts)
	if err != nil {
		return err
	}

	logger.Debug("Writing tarball", "path", opts.Output)

	if err := tarball.WriteToFile(opts.Output, ref, img); err != nil {
		return fmt.Errorf("failed to write tarball: %w", err)
	}

	cli.Printf("Exported %s to %s\n", imageRef, opts.Output)
	logger.Debug("Export complete")

	return nil
}
