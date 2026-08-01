package command

import (
	"context"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type InspectOptions struct {
	Platform string
	Pull     string
}

func NewInspectCommand(cli *CLI) *cobra.Command {
	opts := InspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect <image>",
		Short: "Inspect an OCI image and display information",
		Long: highlight("cek inspect alpine:latest") + "\n\n" +
			"Inspect an OCI image and display information including:\n" +
			"  - Registry location\n" +
			"  - Image digest and metadata\n" +
			"  - Creation timestamp\n" +
			"  - OS/Architecture\n" +
			"  - Total size\n" +
			"  - Layer information (digest and size)\n\n" +
			"The image reference can be:\n" +
			"  - A tagged image: alpine:latest\n" +
			"  - A specific digest: alpine@sha256:...\n" +
			"  - A full registry path: gcr.io/project/image:tag\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			if err := RunInspect(cmd.Context(), cli, imageRef, &opts); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.Platform, "platform", "", "Specify platform (e.g., linux/amd64, linux/arm64)")
	cmd.Flags().StringVar(&opts.Pull, "pull", "if-not-present", "Image pull policy (always, if-not-present, never)")

	return cmd
}

func RunInspect(ctx context.Context, cli *CLI, imageRef string, opts *InspectOptions) error {
	logger := cli.Logger()
	logger.Debug("Inspecting image", "image", imageRef)

	fetchOpts := &oci.FetchOptions{
		Platform:        opts.Platform,
		PullPolicy:      oci.PullPolicy(opts.Pull),
		DisableProgress: cli.DisableProgress,
	}
	img, ref, err := oci.FetchImage(ctx, imageRef, fetchOpts)
	if err != nil {
		return err
	}

	logger.Debug("Parsed reference", "ref", ref.String())
	logger.Debug("Registry", "registry", ref.Context().RegistryStr())
	logger.Debug("Repository", "repo", ref.Context().RepositoryStr())
	logger.Debug("Fetched image descriptor")

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("failed to get image digest: %w", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return fmt.Errorf("failed to get config file: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	data, err := RunWithSpinner(cli, "Inspecting...", func() (*view.InspectData, error) {
		var totalSize int64
		layerDataList := make([]view.LayerData, 0, len(layers))
		for i, layer := range layers {
			layerDigest, err := layer.Digest()
			if err != nil {
				return nil, fmt.Errorf("failed to get layer digest: %w", err)
			}

			size, err := layer.Size()
			if err != nil {
				return nil, fmt.Errorf("failed to get layer size: %w", err)
			}
			totalSize += size

			layerDataList = append(layerDataList, view.LayerData{
				Index:  i + 1,
				Digest: layerDigest,
				Size:   size,
			})
		}

		return &view.InspectData{
			ImageRef:     imageRef,
			Registry:     ref.Context().RegistryStr(),
			Digest:       digest,
			Created:      configFile.Created.Time,
			OS:           configFile.OS,
			Architecture: configFile.Architecture,
			TotalSize:    totalSize,
			Layers:       layerDataList,
		}, nil
	})
	if err != nil {
		return err
	}

	return cli.Inspect().Render(data)
}
