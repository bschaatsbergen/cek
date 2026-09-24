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

func TestNewConfigCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewConfigCommand(cli)

	assert.Equal(t, "config", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
	assert.NotNil(t, cmd.Flags().Lookup("platform"))
	assert.NotNil(t, cmd.Flags().Lookup("pull"))
}

func TestConfigCommand_RequiresImageArg(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewConfigCommand(cli)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestRunConfig_HumanIsIndentedJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewConfigCommand(cli)
	cmd.SetArgs([]string{"--pull", "always", "alpine:latest"})

	require.NoError(t, cmd.Execute())

	var config struct {
		Architecture string `json:"architecture"`
		Config       struct {
			Cmd []string `json:"Cmd"`
		} `json:"config"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &config))
	assert.NotEmpty(t, config.Architecture)
	assert.Equal(t, []string{"/bin/sh"}, config.Config.Cmd)
	assert.Contains(t, buf.String(), "\n  \"architecture\"")
}

func TestRunConfig_JSONIsExactBytes(t *testing.T) {
	img, _, err := oci.FetchImage(t.Context(), "alpine:latest", &oci.FetchOptions{PullPolicy: oci.PullAlways})
	require.NoError(t, err)
	want, err := img.ConfigName()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewJSON, buf, view.LogLevelSilent)
	cmd := command.NewConfigCommand(cli)
	cmd.SetArgs([]string{"--pull", "always", "alpine:latest"})

	require.NoError(t, cmd.Execute())

	sum := sha256.Sum256(buf.Bytes())
	assert.Equal(t, want.Hex, hex.EncodeToString(sum[:]))
}
