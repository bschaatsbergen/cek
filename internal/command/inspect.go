package command

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
)

type InspectOptions struct {
	FetchFlags
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
			"  - Runtime config (entrypoint, cmd, user, ports, env, labels)\n" +
			"  - Layer information (digest, size, media type and annotations)\n\n" +
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

	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunInspect(ctx context.Context, cli *CLI, imageRef string, opts *InspectOptions) error {
	logger := cli.Logger()
	logger.Debug("Inspecting image", "image", imageRef)

	img, ref, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
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

	// The manifest descriptors carry everything the registry knows about a
	// layer: digest, size, media type and annotations.
	manifest, err := img.Manifest()
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	var totalSize int64
	layerDataList := make([]view.LayerData, 0, len(manifest.Layers))
	for i, desc := range manifest.Layers {
		totalSize += desc.Size

		layerDataList = append(layerDataList, view.LayerData{
			Index:       i + 1,
			Digest:      desc.Digest,
			Size:        desc.Size,
			MediaType:   string(desc.MediaType),
			Annotations: desc.Annotations,
		})
	}

	return cli.Inspect().Render(&view.InspectData{
		ImageRef:     imageRef,
		Registry:     ref.Context().RegistryStr(),
		Digest:       digest,
		Created:      configFile.Created.Time,
		OS:           configFile.OS,
		Architecture: configFile.Architecture,
		TotalSize:    totalSize,
		Config:       configData(&configFile.Config),
		Layers:       layerDataList,
	})
}

// configData copies the runtime config into the view's shape. Set-valued
// fields come out sorted so the output is stable.
func configData(c *v1.Config) view.ConfigData {
	return view.ConfigData{
		Entrypoint:   c.Entrypoint,
		Cmd:          c.Cmd,
		User:         c.User,
		WorkingDir:   c.WorkingDir,
		ExposedPorts: slices.Sorted(maps.Keys(c.ExposedPorts)),
		Volumes:      slices.Sorted(maps.Keys(c.Volumes)),
		Env:          c.Env,
		Labels:       c.Labels,
	}
}
