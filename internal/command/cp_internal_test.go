package command

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"

	"github.com/bschaatsbergen/cek/internal/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func layersFrom(t *testing.T, members ...member) (*overlay.FS, []overlay.Layer) {
	t.Helper()
	l := layerFrom(t, members...)
	fs, err := overlay.Build([]overlay.Layer{l})
	require.NoError(t, err)
	return fs, []overlay.Layer{l}
}

func TestCopier_RefusesToWriteThroughSymlink(t *testing.T) {
	outside := t.TempDir()
	dest := filepath.Join(t.TempDir(), "out")

	// A layer that plants a symlink and then a file "inside" it. Written
	// naively, pwned would land in the outside directory.
	fs, layers := layersFrom(t,
		dirm("app/"),
		lnk("app/escape", outside),
		reg("app/escape/pwned", "owned", 0o644),
	)
	root, ok := fs.Lookup("/app")
	require.True(t, ok)

	c := &copier{fs: fs, layers: layers}
	err := c.copyTree(root, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to write through symlink")

	_, err = os.Stat(filepath.Join(outside, "pwned"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCopier_PreservesModesAndSymlinks(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "out")

	fs, layers := layersFrom(t,
		dirm("app/"),
		member{hdr: tar.Header{Name: "app/private", Typeflag: tar.TypeDir, Mode: 0o700}},
		reg("app/private/secret", "s3cret", 0o600),
		reg("app/run.sh", "#!/bin/sh", 0o755),
		lnk("app/current", "run.sh"),
		member{hdr: tar.Header{Name: "app/fifo", Typeflag: tar.TypeFifo, Mode: 0o600}},
	)
	root, ok := fs.Lookup("/app")
	require.True(t, ok)

	c := &copier{fs: fs, layers: layers}
	require.NoError(t, c.copyTree(root, dest))

	assert.Equal(t, 3, c.files, "two files and one symlink")
	assert.Equal(t, int64(len("s3cret")+len("#!/bin/sh")), c.bytes)
	assert.Equal(t, 1, c.skipped, "the fifo")

	info, err := os.Stat(filepath.Join(dest, "run.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())

	info, err = os.Stat(filepath.Join(dest, "private", "secret"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	info, err = os.Stat(filepath.Join(dest, "private"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	target, err := os.Readlink(filepath.Join(dest, "current"))
	require.NoError(t, err)
	assert.Equal(t, "run.sh", target)

	_, err = os.Lstat(filepath.Join(dest, "fifo"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
