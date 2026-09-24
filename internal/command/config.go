package command

import (
	"context"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type ConfigOptions struct {
	FetchFlags
}

func NewConfigCommand(cli *CLI) *cobra.Command {
	opts := ConfigOptions{}

	cmd := &cobra.Command{
		Use:   "config <image>",
		Short: "Print the image config",
		Long: highlight("cek config nginx:latest | jq '.config.Env'") + "\n\n" +
			"Print the image config blob as JSON: the runtime config (entrypoint,\n" +
			"cmd, env, user, ports, labels), the rootfs diff IDs and the build\n" +
			"history.\n\n" +
			"The output is indented for reading. With --json the exact bytes are\n" +
			"written instead, so the output hashes to the config digest.\n\n" +
			"Examples:\n" +
			"  cek config nginx:latest\n" +
			"  cek config nginx:latest | jq '.config.Env'\n" +
			"  cek config nginx:latest | jq -r '.history[].created_by'\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			return RunConfig(cmd.Context(), cli, imageRef, &opts)
		},
	}

	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunConfig(ctx context.Context, cli *CLI, imageRef string, opts *ConfigOptions) error {
	logger := cli.Logger()
	logger.Debug("Printing config", "image", imageRef)

	img, _, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
	if err != nil {
		return err
	}

	raw, err := img.RawConfigFile()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	return cli.Raw().Render(&view.RawData{
		Content: raw,
	})
}
