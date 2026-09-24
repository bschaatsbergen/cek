package command_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManifestCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewManifestCommand(cli)

	assert.Equal(t, "manifest", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
	assert.NotNil(t, cmd.Flags().Lookup("platform"))
	assert.NotNil(t, cmd.Flags().Lookup("pull"))
}

func TestManifestCommand_RequiresImageArg(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewManifestCommand(cli)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestRunManifest_HumanIsIndentedJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewManifestCommand(cli)
	cmd.SetArgs([]string{"--pull", "always", "alpine:latest"})

	require.NoError(t, cmd.Execute())

	var manifest struct {
		SchemaVersion int `json:"schemaVersion"`
		Layers        []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &manifest))
	assert.Equal(t, 2, manifest.SchemaVersion)
	require.NotEmpty(t, manifest.Layers)
	assert.Contains(t, manifest.Layers[0].Digest, "sha256:")
	assert.Contains(t, buf.String(), "\n  \"schemaVersion\"")
	assert.True(t, bytes.HasSuffix(buf.Bytes(), []byte("\n")))
}

func TestRunManifest_JSONIsExactBytes(t *testing.T) {
	img, _, err := oci.FetchImage(t.Context(), "alpine:latest", &oci.FetchOptions{PullPolicy: oci.PullAlways})
	require.NoError(t, err)
	want, err := img.Digest()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewJSON, buf, view.LogLevelSilent)
	cmd := command.NewManifestCommand(cli)
	cmd.SetArgs([]string{"--pull", "always", "alpine:latest"})

	require.NoError(t, cmd.Execute())

	sum := sha256.Sum256(buf.Bytes())
	assert.Equal(t, want.Hex, hex.EncodeToString(sum[:]))
}
