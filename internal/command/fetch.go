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

// AddFetchFlags registers --platform and --pull on cmd.
func AddFetchFlags(cmd *cobra.Command, f *FetchFlags) {
	cmd.Flags().StringVar(&f.Platform, "platform", "", "Specify platform (e.g., linux/amd64, linux/arm64)")
	cmd.Flags().StringVar(&f.Pull, "pull", string(oci.PullIfNotPresent), "Image pull policy (always, if-not-present, never)")
}

// FetchOptions converts the flags into options for oci.FetchImage.
func (f *FetchFlags) FetchOptions() *oci.FetchOptions {
	return &oci.FetchOptions{
		Platform:   f.Platform,
		PullPolicy: oci.PullPolicy(f.Pull),
	}
}
