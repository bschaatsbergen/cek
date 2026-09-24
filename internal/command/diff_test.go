package command_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const distroless = "gcr.io/distroless/static-debian12:latest"

func TestNewDiffCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewDiffCommand(cli)

	assert.Equal(t, "diff", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
	assert.NotNil(t, cmd.Flags().Lookup("platform"))
	assert.NotNil(t, cmd.Flags().Lookup("pull"))
}

func TestDiffCommand_RequiresTwoImages(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewDiffCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts between 2 and 3 arg(s)")
}

func TestRunDiff_SameImage(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewDiffCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "alpine:latest"})

	require.NoError(t, cmd.Execute())

	output := buf.String()
	assert.Contains(t, output, "Layers:\n  = sha256:")
	assert.NotContains(t, output, "\n  + ")
	assert.NotContains(t, output, "\n  - ")
	assert.Contains(t, output, "Files: no changes\n")
}

func TestRunDiff_DifferentImages(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewDiffCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", distroless})

	require.NoError(t, cmd.Execute())

	output := buf.String()
	assert.Regexp(t, `\n  - sha256:[0-9a-f]{64}\s+\S+ [KMG]?B\n`, output, "alpine's layer is not in distroless")
	assert.Regexp(t, `\n  \+ sha256:[0-9a-f]{64}\s+\S+ [KMG]?B\n`, output, "distroless layers are not in alpine")
	assert.Regexp(t, `\n  - /bin/sh\s+-> \S*busybox\n`, output, "distroless has no shell")
	assert.Regexp(t, `\n  \+ /etc/debian_version\s+\d+ B\n`, output)
	assert.Regexp(t, `\n  ~ /etc/passwd\s+\d+ B -> \d+ B\n`, output)
	assert.Regexp(t, `\n\d+ added, \d+ removed, \d+ modified\n$`, output)
	assert.NotContains(t, output, "\x1b[", "no escape codes when not writing to a terminal")
}

func TestRunDiff_ScopedToPath(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewDiffCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", distroless, "/etc"})

	require.NoError(t, cmd.Execute())

	inFiles := false
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "Files:" {
			inFiles = true
			continue
		}
		if !inFiles || !strings.HasPrefix(line, "  ") {
			continue
		}
		assert.Regexp(t, `^  [-+~] /etc/`, line)
	}
	assert.True(t, inFiles)
}

func TestRunDiff_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewJSON, buf, view.LogLevelSilent)
	cmd := command.NewDiffCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", distroless, "/etc"})

	require.NoError(t, cmd.Execute())

	var output struct {
		A     string `json:"a"`
		B     string `json:"b"`
		Files []struct {
			Status string `json:"status"`
			Path   string `json:"path"`
		} `json:"files"`
		Summary struct {
			Removed int `json:"removed"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))
	assert.Equal(t, "alpine:latest", output.A)
	assert.Equal(t, distroless, output.B)
	assert.NotEmpty(t, output.Files)
	assert.Positive(t, output.Summary.Removed)
	for _, f := range output.Files {
		assert.True(t, strings.HasPrefix(f.Path, "/etc/"), f.Path)
	}
}

func TestRunDiff_UnknownImage(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewDiffCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "this-image-does-not-exist-12345:nonexistent"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "this-image-does-not-exist-12345:nonexistent:")
	assert.Empty(t, buf.String())
}
