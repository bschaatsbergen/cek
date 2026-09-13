package command

import (
	"github.com/spf13/cobra"
)

func NewVersionCommand(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long: highlight("cek version") + "\n\n" +
			"Display the current version of the cek CLI.\n\n" +
			"This information is useful for bug reports, ensuring team\n" +
			"consistency, and verifying compatibility with documentation\n" +
			"and automation scripts.\n",
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli.PrintVersion()
		},
	}
	return cmd
}
