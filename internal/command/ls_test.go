package command_test

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLsCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)

	assert.Equal(t, "ls", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)
}

func TestLsCommand_Flags(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)

	layerFlag := cmd.Flags().Lookup("layer")
	assert.NotNil(t, layerFlag)
	assert.Equal(t, "-1", layerFlag.DefValue)

	filterFlag := cmd.Flags().Lookup("filter")
	assert.NotNil(t, filterFlag)
	assert.Equal(t, "", filterFlag.DefValue)

	platformFlag := cmd.Flags().Lookup("platform")
	assert.NotNil(t, platformFlag)
	assert.Equal(t, "", platformFlag.DefValue)

	pullFlag := cmd.Flags().Lookup("pull")
	assert.NotNil(t, pullFlag)
	assert.Equal(t, "if-not-present", pullFlag.DefValue)
}

func TestLsCommand_RequiresImageArg(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts between 1 and 2 arg(s)")
}

func TestLsCommand_TooManyArgs(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc", "extra"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts between 1 and 2 arg(s)")
}

func TestLsCommand_BasicExecution(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Mode")
	assert.Contains(t, output, "Size")
	assert.Contains(t, output, "Path")
}

func TestLsCommand_WithPath(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output)
	// All paths should be under /etc
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		assert.Contains(t, line, "/etc")
	}
}

func TestLsCommand_WithFilter(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"nginx:latest", "--filter", "*.conf"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output)
	// All results should end with .conf
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		assert.Contains(t, line, ".conf")
	}
}

func TestLsCommand_WithPathAndFilter(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"nginx:latest", "/etc/nginx", "--filter", "*.conf"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		assert.Contains(t, line, "/etc/nginx")
		assert.Contains(t, line, ".conf")
	}
}

func TestLsCommand_WithLayer(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--layer", "1"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Mode")
}

func TestLsCommand_InvalidLayer(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--layer", "999"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestLsCommand_WithPlatform(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--platform", "linux/amd64"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestLsCommand_WithPullAlways(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--pull", "always"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestLsCommand_NoFilesFound(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--filter", "this-file-definitely-does-not-exist.xyz"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No files matching pattern")
}

func TestLsCommand_NoFilesInPath(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/this-path-does-not-exist"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No files found in path")
}

func TestRunLs_BasicExecution(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Mode")
	assert.Contains(t, output, "Size")
	assert.Contains(t, output, "Path")
	assert.Contains(t, output, "/bin")
	assert.Contains(t, output, "/etc")
}

func TestRunLs_WithSpecificPath(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		assert.Contains(t, line, "/etc")
	}
}

func TestRunLs_WithFilter(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"nginx:latest", "--filter", "*.conf"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		assert.Contains(t, line, ".conf")
	}
}

func TestRunLs_WithSpecificLayer(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--layer", "1"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Mode")
	assert.NotEmpty(t, output)
}

func TestRunLs_InvalidLayer(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--layer", "999"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestRunLs_NoMatchingFiles(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--filter", "this-does-not-exist-*.xyz"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No files matching pattern")
}

func TestRunLs_OutputFormatting(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Check for proper tabular format
	assert.Contains(t, output, "Mode")
	assert.Contains(t, output, "Size")
	assert.Contains(t, output, "Path")

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.GreaterOrEqual(t, len(lines), 2) // At least header + 1 file
}

func TestRunLs_FileModeFormatting(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/bin"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should have file modes like -rwxr-xr-x or drwxr-xr-x
	assert.Regexp(t, `[dl-]r[w-]x`, output)
}

func TestRunLs_WithDoublestarFilter(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"nginx:latest", "--filter", "**/nginx/*.conf"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	if !strings.Contains(output, "No files matching pattern") {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			assert.Contains(t, line, "nginx")
			assert.Contains(t, line, ".conf")
		}
	}
}

func TestRunLs_SubstringFilter(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "--filter", "bin"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		assert.Contains(t, line, "bin")
	}
}

func TestRunLs_PathNormalization(t *testing.T) {
	// Test with trailing slash
	buf1 := new(bytes.Buffer)
	cli1 := command.NewCLI(view.ViewHuman, buf1, view.LogLevelSilent)
	cmd1 := command.NewLsCommand(cli1)
	cmd1.SetArgs([]string{"alpine:latest", "/etc/"})

	err := cmd1.Execute()
	require.NoError(t, err)

	// Test without leading slash
	buf2 := new(bytes.Buffer)
	cli2 := command.NewCLI(view.ViewHuman, buf2, view.LogLevelSilent)
	cmd2 := command.NewLsCommand(cli2)
	cmd2.SetArgs([]string{"alpine:latest", "etc"})

	err = cmd2.Execute()
	require.NoError(t, err)

	// Both should produce similar results
	output1 := buf1.String()
	output2 := buf2.String()
	assert.NotEmpty(t, output1)
	assert.NotEmpty(t, output2)
}

func TestRunLs_OutputIsSortedByPath(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewHuman, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest"})

	require.NoError(t, cmd.Execute())

	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n")[1:] {
		fields := strings.Fields(line)
		require.NotEmpty(t, fields)
		paths = append(paths, fields[len(fields)-1])
	}
	require.NotEmpty(t, paths)
	assert.True(t, sort.StringsAreSorted(paths), "ls output must be sorted by path")
	assert.Contains(t, paths, "/etc/")
}

func TestRunLs_JSONPathsAreClean(t *testing.T) {
	buf := new(bytes.Buffer)
	cli := command.NewCLI(view.ViewJSON, buf, view.LogLevelSilent)
	cmd := command.NewLsCommand(cli)
	cmd.SetArgs([]string{"alpine:latest", "/etc"})

	require.NoError(t, cmd.Execute())

	var output struct {
		Files []struct {
			Mode string `json:"mode"`
			Path string `json:"path"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &output))
	require.NotEmpty(t, output.Files)
	assert.Equal(t, "/etc", output.Files[0].Path)
	assert.True(t, strings.HasPrefix(output.Files[0].Mode, "d"))
	for _, f := range output.Files {
		assert.False(t, strings.HasSuffix(f.Path, "/"), "path %q must not end in a slash", f.Path)
	}
}
