package command

import (
	"context"
	"testing"

	"github.com/bschaatsbergen/lix/internal/oci"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFileFromLayer_FileFound(t *testing.T) {
	ctx := context.Background()

	img, _, err := oci.FetchImage(ctx, "nginx:latest", &oci.FetchOptions{
		PullPolicy: oci.PullIfNotPresent,
	})
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	require.NotEmpty(t, layers)

	// Test reading from first layer
	content, found, err := extractFileFromLayer(layers[0], "/etc/debian_version")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.NotEmpty(t, content)
}

func TestExtractFileFromLayer_FileNotFound(t *testing.T) {
	ctx := context.Background()

	img, _, err := oci.FetchImage(ctx, "nginx:latest", &oci.FetchOptions{
		PullPolicy: oci.PullIfNotPresent,
	})
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	require.NotEmpty(t, layers)

	content, found, err := extractFileFromLayer(layers[0], "/nonexistent/file.txt")
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, content)
}

func TestExtractFileFromLayer_PathWithSlash(t *testing.T) {
	ctx := context.Background()

	img, _, err := oci.FetchImage(ctx, "nginx:latest", &oci.FetchOptions{
		PullPolicy: oci.PullIfNotPresent,
	})
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	require.NotEmpty(t, layers)

	content, found, err := extractFileFromLayer(layers[0], "/etc/debian_version")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.NotEmpty(t, content)
}

func TestExtractFileFromLayer_PathWithoutSlash(t *testing.T) {
	ctx := context.Background()

	img, _, err := oci.FetchImage(ctx, "nginx:latest", &oci.FetchOptions{
		PullPolicy: oci.PullIfNotPresent,
	})
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	require.NotEmpty(t, layers)

	content, found, err := extractFileFromLayer(layers[0], "etc/debian_version")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.NotEmpty(t, content)
}
