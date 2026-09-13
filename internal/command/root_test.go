package command_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCommand(t *testing.T) {
	cmd := command.NewRootCommand()

	assert.Equal(t, "cek", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Version)
	assert.True(t, cmd.SilenceUsage)
	assert.True(t, cmd.SilenceErrors)
	assert.False(t, cmd.CompletionOptions.DisableDefaultCmd)
}

func TestNewRootCommand_HasJSONFlag(t *testing.T) {
	cmd := command.NewRootCommand()

	flag := cmd.PersistentFlags().Lookup("json")
	assert.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "Output in JSON format", flag.Usage)
}

func TestNewRootCommand_VersionFlag(t *testing.T) {
	cmd := command.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), cmd.Version)
}

func TestNewRootCommand_NoArgs_ShowsHelp(t *testing.T) {
	cmd := command.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "cek")
}

func TestAddCommands(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	root := command.NewRootCommand()
	command.AddCommands(root, cli)

	expectedCommands := []string{"version", "inspect", "ls", "cat", "tree", "tags", "export", "blob"}
	for _, name := range expectedCommands {
		cmd, _, err := root.Find([]string{name})
		assert.NoError(t, err, "command %s should exist", name)
		assert.Equal(t, name, cmd.Name())
	}
}

func TestAddCommands_Count(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	root := command.NewRootCommand()
	command.AddCommands(root, cli)

	assert.True(t, root.HasSubCommands())
	assert.Len(t, root.Commands(), 8)
}

func TestConfigureView_JSONFlagAfterSubcommandFlags(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	root := command.NewRootCommand()
	command.ConfigureView(root, cli)
	command.AddCommands(root, cli)
	root.SetArgs([]string{"inspect", "--pull", "always", "alpine:latest", "--json"})

	require.NoError(t, root.Execute())

	var output struct {
		Image string `json:"image"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output), "expected JSON output, got: %s", buf.String())
	assert.Equal(t, "alpine:latest", output.Image)
}

func TestConfigureView_JSONFlagBeforeSubcommand(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	root := command.NewRootCommand()
	command.ConfigureView(root, cli)
	command.AddCommands(root, cli)
	root.SetArgs([]string{"--json", "inspect", "--pull", "always", "alpine:latest"})

	require.NoError(t, root.Execute())

	var output struct {
		Image string `json:"image"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output), "expected JSON output, got: %s", buf.String())
	assert.Equal(t, "alpine:latest", output.Image)
}

func TestConfigureView_DefaultsToHumanView(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	root := command.NewRootCommand()
	command.ConfigureView(root, cli)
	command.AddCommands(root, cli)
	root.SetArgs([]string{"inspect", "--pull", "always", "alpine:latest"})

	require.NoError(t, root.Execute())
	assert.Contains(t, buf.String(), "Image: alpine:latest")
}

func TestRootCommand_ShellCompletion(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
			root := command.NewRootCommand()
			command.AddCommands(root, cli)
			root.SetOut(buf)
			root.SetArgs([]string{"completion", shell})

			require.NoError(t, root.Execute())
			assert.Contains(t, buf.String(), "cek")
		})
	}
}

func TestFetchFlags_CompletePullPolicies(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	root := command.NewRootCommand()
	command.AddCommands(root, cli)
	root.SetOut(buf)
	root.SetArgs([]string{cobra.ShellCompRequestCmd, "inspect", "--pull", ""})

	require.NoError(t, root.Execute())
	assert.Contains(t, buf.String(), "always\n")
	assert.Contains(t, buf.String(), "if-not-present\n")
	assert.Contains(t, buf.String(), "never\n")
}
