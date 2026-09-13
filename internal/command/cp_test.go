package command_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bschaatsbergen/cek/internal/command"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCp(t *testing.T, vt view.ViewType, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	cli := command.NewCLI(vt, buf, view.LogLevelSilent)
	cmd := command.NewCpCommand(cli)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestNewCpCommand(t *testing.T) {
	cli := command.NewCLI(view.ViewHuman, &bytes.Buffer{}, view.LogLevelSilent)
	cmd := command.NewCpCommand(cli)

	assert.Equal(t, "cp", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.RunE)

	layerFlag := cmd.Flags().Lookup("layer")
	require.NotNil(t, layerFlag)
	assert.Equal(t, "-1", layerFlag.DefValue)
	assert.NotNil(t, cmd.Flags().Lookup("platform"))
	assert.NotNil(t, cmd.Flags().Lookup("pull"))
}

func TestCpCommand_RequiresThreeArgs(t *testing.T) {
	_, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/os-release")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 3 arg(s)")
}

func TestRunCp_FileToPath(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "release")

	output, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/alpine-release", dest)
	require.NoError(t, err)
	assert.Contains(t, output, "Copied 1 file (")
	assert.Contains(t, output, "from alpine:latest:/etc/alpine-release to "+dest)

	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Regexp(t, `^\d+\.\d+`, string(data))

	info, err := os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestRunCp_FileIntoDirectory(t *testing.T) {
	dir := t.TempDir()

	_, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/alpine-release", dir)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "alpine-release"))
	assert.NoError(t, err)
}

func TestRunCp_SourceSymlinkIsFollowed(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "os-release")

	_, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/os-release", dest)
	require.NoError(t, err)

	info, err := os.Lstat(dest)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())

	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Alpine Linux")
}

func TestRunCp_DirectoryToNewPath(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "apk")

	output, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/apk", dest)
	require.NoError(t, err)
	assert.Regexp(t, `Copied \d+ files \(`, output)

	_, err = os.Stat(filepath.Join(dest, "repositories"))
	assert.NoError(t, err)
	info, err := os.Stat(filepath.Join(dest, "keys"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestRunCp_DirectoryIntoExistingDirectory(t *testing.T) {
	dir := t.TempDir()

	output, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/apk", dir)
	require.NoError(t, err)
	assert.Contains(t, output, "to "+filepath.Join(dir, "apk"))

	_, err = os.Stat(filepath.Join(dir, "apk", "repositories"))
	assert.NoError(t, err)
}

func TestRunCp_DirectoryContentsOnly(t *testing.T) {
	dir := t.TempDir()

	_, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/apk/.", dir)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "repositories"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "apk"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunCp_SymlinksInsideTreeStaySymlinks(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "bin")

	_, err := runCp(t, view.ViewHuman, "alpine:latest", "/bin", dest)
	require.NoError(t, err)

	info, err := os.Lstat(filepath.Join(dest, "sh"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)

	target, err := os.Readlink(filepath.Join(dest, "sh"))
	require.NoError(t, err)
	assert.Contains(t, target, "busybox")

	info, err = os.Stat(filepath.Join(dest, "busybox"))
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
	assert.NotZero(t, info.Mode().Perm()&0o111, "busybox must stay executable")
}

func TestRunCp_FromLayer(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "release")

	_, err := runCp(t, view.ViewHuman, "alpine:latest", "--layer", "1", "/etc/alpine-release", dest)
	require.NoError(t, err)
	_, err = os.Stat(dest)
	assert.NoError(t, err)

	_, err = runCp(t, view.ViewHuman, "alpine:latest", "--layer", "999", "/etc/alpine-release", dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "layer 999 does not exist")
}

func TestRunCp_MissingSource(t *testing.T) {
	output, err := runCp(t, view.ViewHuman, "alpine:latest", "/etc/does-not-exist", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/etc/does-not-exist: no such file or directory")
	assert.Empty(t, output)
}

func TestRunCp_JSON(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "apk")

	output, err := runCp(t, view.ViewJSON, "alpine:latest", "/etc/apk", dest)
	require.NoError(t, err)

	var result struct {
		Image       string `json:"image"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
		Files       int    `json:"files"`
		Bytes       int64  `json:"bytes"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	assert.Equal(t, "alpine:latest", result.Image)
	assert.Equal(t, "/etc/apk", result.Source)
	assert.Equal(t, dest, result.Destination)
	assert.Positive(t, result.Files)
	assert.Positive(t, result.Bytes)
}
