package command_test

import (
	"bytes"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInspectCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)

	assert.Equal(t, "inspect", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
}

func TestInspectCommand_Flags(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)

	platformFlag := cmd.Flags().Lookup("platform")
	assert.NotNil(t, platformFlag)
	assert.Equal(t, "", platformFlag.DefValue)

	pullFlag := cmd.Flags().Lookup("pull")
	assert.NotNil(t, pullFlag)
	assert.Equal(t, "if-not-present", pullFlag.DefValue)
}

func TestInspectCommand_RequiresImageArg(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestInspectCommand_TooManyArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "nginx:latest"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestInspectCommand_BasicExecution(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Image:")
	assert.Contains(t, output, "Registry:")
	assert.Contains(t, output, "Digest:")
	assert.Contains(t, output, "Created:")
	assert.Contains(t, output, "OS/Arch:")
	assert.Contains(t, output, "Size:")
	assert.Contains(t, output, "Layers:")
}

func TestInspectCommand_OutputContainsExpectedFields(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "alpine:latest")
	assert.Contains(t, output, "sha256:")
	assert.Contains(t, output, "linux/")
}

func TestInspectCommand_WithPullAlways(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--pull", "always"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestInspectCommand_WithPlatform(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--platform", "linux/amd64", "--pull", "always"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "linux/amd64")
}

func TestInspectCommand_InvalidImageReference(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"this-image-does-not-exist-12345:nonexistent"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestInspectCommand_FullyQualifiedImage(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"gcr.io/distroless/static-debian12:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "gcr.io")
	assert.Contains(t, output, "distroless/static-debian12")
}

func TestRunInspect_ValidImage(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Image: alpine:latest")
	assert.Contains(t, output, "Registry:")
	assert.Contains(t, output, "Digest: sha256:")
	assert.Contains(t, output, "Layers:")
}

func TestRunInspect_LayerInformation(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should have layer header
	assert.Contains(t, output, "Layers:")
	assert.Contains(t, output, "Digest")
	assert.Contains(t, output, "Size")
	// Should have at least one layer digest
	assert.Contains(t, output, "sha256:")
}

func TestRunInspect_WithPlatformSpecified(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--platform", "linux/amd64", "--pull", "always"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "OS/Arch: linux/amd64")
}

func TestRunInspect_SizeFormatting(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Size should be formatted (MB, KB, etc)
	assert.Regexp(t, `Size: \d+(\.\d+)? (B|KB|MB|GB)`, output)
}

func TestRunInspect_CreatedTimestamp(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewInspectCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should have RFC3339 formatted timestamp
	assert.Regexp(t, `Created: \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, output)
}
