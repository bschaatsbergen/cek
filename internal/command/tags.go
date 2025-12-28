package command

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

type TagsOptions struct {
	Limit int
}

func NewTagsCommand(cli *CLI) *cobra.Command {
	opts := TagsOptions{}

	cmd := &cobra.Command{
		Use:   "tags <image>",
		Short: "List all tags for an image repository",
		Long: highlight("cek tags nginx") + "\n\n" +
			"List all tags for an image repository from the registry.\n\n" +
			"This queries the remote registry, not the local daemon.\n" +
			"For large repositories with many tags, pipe to less for pagination:\n" +
			"  cek tags nginx | less\n\n" +
			"Examples:\n" +
			"  cek tags nginx\n" +
			"  cek tags gcr.io/distroless/static-debian12\n" +
			"  cek tags nginx | less\n" +
			"  cek tags nginx | grep '^1\\.2'",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageRef := args[0]
			return RunTags(cmd.Context(), cli, imageRef, &opts)
		},
	}

	cmd.Flags().IntVar(&opts.Limit, "limit", 0, "Limit the number of tags returned (0 = unlimited)")

	return cmd
}

func RunTags(ctx context.Context, cli *CLI, imageRef string, opts *TagsOptions) error {
	logger := cli.Logger()
	logger.Debug("Listing tags", "image", imageRef)

	// Parse image reference either by tag or digest.
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %w", err)
	}

	repo := ref.Context()

	logger.Debug("Fetching tags from registry", "repository", repo.String())

	// List tags from remote registry
	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
	}

	tags, err := remote.List(repo, remoteOpts...)
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tags) == 0 {
		cli.Printf("No tags found for %s\n", repo.String())
		return nil
	}

	// Reverse to display newest tags first.
	// Users typically care most about the latest versions.
	for i, j := 0, len(tags)-1; i < j; i, j = i+1, j-1 {
		tags[i], tags[j] = tags[j], tags[i]
	}

	if opts.Limit > 0 && opts.Limit < len(tags) {
		tags = tags[:opts.Limit]
	}

	for _, tag := range tags {
		cli.Printf("%s\n", tag)
	}

	logger.Debug("Listed tags", "count", len(tags))

	return nil
}
