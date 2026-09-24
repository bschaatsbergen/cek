package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type PullPolicy string

const (
	PullAlways       PullPolicy = "always"
	PullIfNotPresent PullPolicy = "if-not-present"
	PullNever        PullPolicy = "never"
)

// PullPolicies lists the accepted pull policy values.
func PullPolicies() []string {
	return []string{string(PullAlways), string(PullIfNotPresent), string(PullNever)}
}

// ParsePullPolicy validates a pull policy given on the command line.
func ParsePullPolicy(s string) (PullPolicy, error) {
	switch p := PullPolicy(s); p {
	case PullAlways, PullIfNotPresent, PullNever:
		return p, nil
	default:
		return "", fmt.Errorf("invalid pull policy %q: must be one of %s", s, strings.Join(PullPolicies(), ", "))
	}
}

type FetchOptions struct {
	Platform   string
	PullPolicy PullPolicy
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
		pullPolicy, err = ParsePullPolicy(string(opts.PullPolicy))
		if err != nil {
			return nil, nil, err
		}
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

// remoteOptions returns the options shared by every registry request: the
// context and the credentials from docker login and credential helpers, the
// same ones docker and crane use.
func remoteOptions(ctx context.Context) []remote.Option {
	return []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}
}

// ListTags lists the tags of a repository from the registry.
func ListTags(ctx context.Context, repo name.Repository) ([]string, error) {
	tags, err := remote.List(repo, remoteOptions(ctx)...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tags, nil
}

func fetchFromRemote(ctx context.Context, ref name.Reference, opts *FetchOptions) (v1.Image, name.Reference, error) {
	remoteOpts := remoteOptions(ctx)

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
