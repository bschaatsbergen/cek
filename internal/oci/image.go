package oci

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// PullPolicy defines when to pull images
type PullPolicy string

const (
	PullAlways       PullPolicy = "always"         // Always pull from registry
	PullIfNotPresent PullPolicy = "if-not-present" // Use local if available, otherwise pull
	PullNever        PullPolicy = "never"          // Only use local images
)

// FetchOptions holds options for fetching an OCI image
type FetchOptions struct {
	Platform   string
	PullPolicy PullPolicy
}

// FetchImage fetches an OCI image from local daemon or remote registry
func FetchImage(ctx context.Context, imageRef string, opts *FetchOptions) (v1.Image, name.Reference, error) {
	// Parse the image as a reference, does s either by tag or digest.
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Determine pull policy
	pullPolicy := PullIfNotPresent
	if opts != nil && opts.PullPolicy != "" {
		pullPolicy = opts.PullPolicy
	}

	// Try local daemon first since it handles image caching, avoiding registry
	// rate limits (unless pull policy is "always")
	if pullPolicy != PullAlways {
		img, err := fetchFromDaemon(ref, opts)
		if err == nil {
			return img, ref, nil
		}
		// Pull policy "never" means fail if not local, we must NOT fall back to
		//  the remote
		if pullPolicy == PullNever {
			return nil, nil, fmt.Errorf("image not found locally and pull policy is 'never': %w", err)
		}
		// Otherwise, fall through to remote fetch below
	}

	// Fetch from remote registry
	return fetchFromRemote(ctx, ref, opts)
}

// fetchFromDaemon attempts to fetch an image from the local Docker daemon
func fetchFromDaemon(ref name.Reference, opts *FetchOptions) (v1.Image, error) {
	daemonOpts := []daemon.Option{}

	// Note: The daemon package doesn't support platform selection directly It
	// will use the image available in the local daemon
	img, err := daemon.Image(ref, daemonOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from daemon: %w", err)
	}

	return img, nil
}

// fetchFromRemote fetches an image from a remote registry
func fetchFromRemote(ctx context.Context, ref name.Reference, opts *FetchOptions) (v1.Image, name.Reference, error) {
	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
	}

	// If platform is specified, add it to the options
	if opts != nil && opts.Platform != "" {
		platform, err := v1.ParsePlatform(opts.Platform)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse platform: %w", err)
		}
		remoteOpts = append(remoteOpts, remote.WithPlatform(*platform))
	}

	// Fetch the image descriptor
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

// FormatBytes converts bytes to a human-readable format
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
