package command

import (
	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/spf13/cobra"
)

// FetchFlags holds the flags shared by every command that fetches an image.
type FetchFlags struct {
	Platform string
	Pull     string
}

// commonPlatforms are offered by shell completion for --platform. Any
// os/arch[/variant] string is still accepted.
var commonPlatforms = []string{
	"linux/amd64",
	"linux/arm64",
	"linux/arm/v7",
	"linux/386",
	"linux/ppc64le",
	"linux/s390x",
	"windows/amd64",
}

// AddFetchFlags registers --platform and --pull on cmd, with shell
// completion for their values.
func AddFetchFlags(cmd *cobra.Command, f *FetchFlags) {
	cmd.Flags().StringVar(&f.Platform, "platform", "", "Specify platform (e.g., linux/amd64, linux/arm64)")
	cmd.Flags().StringVar(&f.Pull, "pull", string(oci.PullIfNotPresent), "Image pull policy (always, if-not-present, never)")

	_ = cmd.RegisterFlagCompletionFunc("platform", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return commonPlatforms, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("pull", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return oci.PullPolicies(), cobra.ShellCompDirectiveNoFileComp
	})
}

// FetchOptions converts the flags into options for oci.FetchImage.
func (f *FetchFlags) FetchOptions() *oci.FetchOptions {
	return &oci.FetchOptions{
		Platform:   f.Platform,
		PullPolicy: oci.PullPolicy(f.Pull),
	}
}
