package command_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCatCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)

	assert.Equal(t, "cat", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)

	layerFlag := cmd.Flags().Lookup("layer")
	require.NotNil(t, layerFlag)
	assert.Equal(t, "-1", layerFlag.DefValue)
	assert.NotNil(t, cmd.Flags().Lookup("platform"))
	assert.NotNil(t, cmd.Flags().Lookup("pull"))
}

func TestCatCommand_RequiresTwoArgs(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg(s)")
}

func TestRunCat_ReadsFile(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc/alpine-release"})

	require.NoError(t, cmd.Execute())
	assert.Regexp(t, `^\d+\.\d+`, buf.String())
}

func TestRunCat_PathWithoutLeadingSlash(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "etc/alpine-release"})

	require.NoError(t, cmd.Execute())
	assert.Regexp(t, `^\d+\.\d+`, buf.String())
}

func TestRunCat_FollowsSymlinks(t *testing.T) {
	// On alpine /etc/os-release is a symlink to ../usr/lib/os-release.
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc/os-release"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Alpine Linux")
}

func TestRunCat_FollowsSymlinksWithinLayer(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--layer", "1", "/etc/os-release"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Alpine Linux")
}

func TestRunCat_JSONOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewJSON, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc/os-release"})

	require.NoError(t, cmd.Execute())

	var output struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))
	assert.Contains(t, output.Content, "Alpine Linux")
}

func TestRunCat_MissingFile(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc/does-not-exist"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/etc/does-not-exist: no such file or directory")
	assert.Empty(t, buf.String())
}

func TestRunCat_Directory(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/etc: is a directory")
	assert.Empty(t, buf.String())
}

func TestRunCat_InvalidLayer(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewCatCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--layer", "999", "/etc/alpine-release"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "layer 999 does not exist")
}
