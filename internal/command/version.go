package command

import (
	"github.com/spf13/cobra"
)

// VersionOptions holds the options for the version command.
type VersionOptions struct {
	Path string
}

func newVersionCommand(cli *CLI) *cobra.Command {
	opts := VersionOptions{
		Path: ".", // Default to current directory
	}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long: highlight("cek version") + "\n\n" +
			"Display the current version of the cek CLI.\n\n" +
			"This information is useful for bug reports, ensuring team\n" +
			"consistency, and verifying compatibility with documentation\n" +
			"and automation scripts.\n",
		Args: MaxArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				opts.Path = args[0]
			}

			cli.PrintVersion()
		},
	}
	return cmd
}
