package oci

import (
	"context"
	"fmt"

	"github.com/bschaatsbergen/cek/internal/spinner"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type PullPolicy string

const (
	PullAlways       PullPolicy = "always"
	PullIfNotPresent PullPolicy = "if-not-present"
	PullNever        PullPolicy = "never"
)

type FetchOptions struct {
	Platform        string
	PullPolicy      PullPolicy
	DisableProgress bool
}

// FetchImage retrieves an OCI image from either the local daemon or remote registry
// based on the pull policy. Defaults to if-not-present to avoid registry rate limits.
func FetchImage(ctx context.Context, imageRef string, opts *FetchOptions) (v1.Image, name.Reference, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	pullPolicy := PullIfNotPresent
	if opts != nil && opts.PullPolicy != "" {
		pullPolicy = opts.PullPolicy
	}

	// Check daemon cache first to avoid registry rate limits.
	if pullPolicy != PullAlways {
		img, err := fetchFromDaemon(ref, opts)
		if err == nil {
			return img, ref, nil
		}
		if pullPolicy == PullNever {
			return nil, nil, fmt.Errorf("image not found locally and pull policy is 'never': %w", err)
		}
	}

	return fetchFromRemote(ctx, ref, opts)
}

func fetchFromDaemon(ref name.Reference, opts *FetchOptions) (v1.Image, error) {
	daemonOpts := []daemon.Option{}

	// Platform selection is not supported by the daemon package.
	// Returns whatever platform the daemon has cached locally.
	img, err := daemon.Image(ref, daemonOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from daemon: %w", err)
	}

	return img, nil
}

func fetchFromRemote(ctx context.Context, ref name.Reference, opts *FetchOptions) (v1.Image, name.Reference, error) {
	if opts != nil && !opts.DisableProgress {
		s := spinner.New(fmt.Sprintf("Pulling image: %s...", ref.String()))
		s.Start()
		defer s.Stop()
	}

	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
	}

	if opts != nil && opts.Platform != "" {
		platform, err := v1.ParsePlatform(opts.Platform)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse platform: %w", err)
		}
		remoteOpts = append(remoteOpts, remote.WithPlatform(*platform))
	}

	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch image: %w", err)
	}

	img, err := desc.Image()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get image: %w", err)
	}

	return img, ref, nil
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
