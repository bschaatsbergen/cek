package command_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBlobCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)

	assert.Equal(t, "blob", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
}

func TestBlobCommand_Flags(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)

	layerFlag := cmd.Flags().Lookup("layer")
	require.NotNil(t, layerFlag)
	assert.Equal(t, "0", layerFlag.DefValue)

	platformFlag := cmd.Flags().Lookup("platform")
	require.NotNil(t, platformFlag)
	assert.Equal(t, "", platformFlag.DefValue)

	pullFlag := cmd.Flags().Lookup("pull")
	require.NotNil(t, pullFlag)
	assert.Equal(t, "if-not-present", pullFlag.DefValue)
}

func TestBlobCommand_RequiresImageArg(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "1"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestBlobCommand_TooManyArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "1", "alpine:latest", "nginx:latest"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestBlobCommand_RequiresLayerFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `required flag(s) "layer" not set`)
	assert.Empty(t, buf.String())
}

func TestBlobCommand_LayerBelowOne(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "0", "alpine:latest"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "layer must be 1 or greater")
	assert.Empty(t, buf.String())
}

func TestBlobCommand_LayerOutOfRange(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "99", "alpine:latest"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "layer 99 does not exist")
	assert.Empty(t, buf.String())
}

func TestBlobCommand_InvalidImageReference(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "1", "this-image-does-not-exist-12345:nonexistent"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Empty(t, buf.String())
}

func TestRunBlob_WritesRawLayerBytes(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "1", "--pull", "always", "alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	blob := buf.Bytes()
	require.Greater(t, len(blob), 2)
	// A gzip layer blob starts with the gzip magic bytes.
	assert.Equal(t, []byte{0x1f, 0x8b}, blob[:2])
}

func TestRunBlob_BytesHashToLayerDigest(t *testing.T) {
	img, _, err := oci.FetchImage(t.Context(), "alpine:latest", &oci.FetchOptions{
		PullPolicy: oci.PullAlways,
	})
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	require.NotEmpty(t, layers)

	want, err := layers[0].Digest()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "1", "--pull", "always", "alpine:latest"})

	require.NoError(t, cmd.Execute())

	sum := sha256.Sum256(buf.Bytes())
	assert.Equal(t, want.Hex, hex.EncodeToString(sum[:]))
}

func TestBlobCommand_JSONViewUnsupported(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewJSON, buf, view.LogLevelSilent)
	cmd := command.NewBlobCommand(cli)
	cmd.SetArgs([]string{"--layer", "1", "alpine:latest"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON")
	assert.Empty(t, buf.String())
}
