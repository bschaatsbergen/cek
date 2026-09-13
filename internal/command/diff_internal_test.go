package command

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/bschaatsbergen/cek/internal/overlay"
	"github.com/bschaatsbergen/cek/internal/view"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memLayer struct{ data []byte }

func (l *memLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.data)), nil
}

type member struct {
	hdr     tar.Header
	content string
}

func reg(name, content string, mode int64) member {
	return member{hdr: tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: mode, Size: int64(len(content))}, content: content}
}

func lnk(name, target string) member {
	return member{hdr: tar.Header{Name: name, Typeflag: tar.TypeSymlink, Mode: 0o777, Linkname: target}}
}

func dirm(name string) member {
	return member{hdr: tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}}
}

func fsFrom(t *testing.T, members ...member) *overlay.FS {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, m := range members {
		require.NoError(t, tw.WriteHeader(&m.hdr))
		_, err := tw.Write([]byte(m.content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	fs, err := overlay.Build([]overlay.Layer{&memLayer{data: buf.Bytes()}}, overlay.WithDigests())
	require.NoError(t, err)
	return fs
}

func TestDiffFiles(t *testing.T) {
	a := fsFrom(t,
		dirm("app/"), dirm("only-a/"),
		reg("app/same", "same", 0o644),
		reg("app/content", "v1", 0o644),
		reg("app/chmod", "x", 0o644),
		reg("app/removed", "gone", 0o644),
		lnk("app/link", "v1"),
		lnk("app/samelink", "target"),
		reg("app/type", "file", 0o644),
	)
	b := fsFrom(t,
		dirm("app/"), dirm("only-b/"),
		reg("app/same", "same", 0o644),
		reg("app/content", "v2", 0o644),
		reg("app/chmod", "x", 0o755),
		reg("app/added", "new", 0o644),
		lnk("app/link", "v2"),
		lnk("app/samelink", "target"),
		lnk("app/type", "elsewhere"),
	)

	got := diffFiles(a, b, "")

	type row struct {
		status view.DiffStatus
		path   string
	}
	var rows []row
	for _, d := range got {
		rows = append(rows, row{d.Status, d.Path})
	}
	assert.Equal(t, []row{
		{view.DiffAdded, "/app/added"},
		{view.DiffModified, "/app/chmod"},
		{view.DiffModified, "/app/content"},
		{view.DiffModified, "/app/link"},
		{view.DiffRemoved, "/app/removed"},
		{view.DiffModified, "/app/type"},
	}, rows, "directories and unchanged files are not listed; output is sorted")

	byPath := map[string]view.FileDiff{}
	for _, d := range got {
		byPath[d.Path] = d
	}
	assert.Equal(t, "-rw-r--r--", byPath["/app/chmod"].A.Mode)
	assert.Equal(t, "-rwxr-xr-x", byPath["/app/chmod"].B.Mode)
	assert.NotEqual(t, byPath["/app/content"].A.Digest, byPath["/app/content"].B.Digest)
	assert.Contains(t, byPath["/app/content"].A.Digest, "sha256:")
	assert.Equal(t, "v1", byPath["/app/link"].A.Link)
	assert.Equal(t, "v2", byPath["/app/link"].B.Link)
	assert.Nil(t, byPath["/app/added"].A)
	assert.Nil(t, byPath["/app/removed"].B)
}

func TestDiffFiles_Scoped(t *testing.T) {
	a := fsFrom(t, reg("etc/a", "1", 0o644), reg("usr/a", "1", 0o644))
	b := fsFrom(t, reg("etc/a", "2", 0o644), reg("usr/a", "2", 0o644))

	got := diffFiles(a, b, "/etc")
	require.Len(t, got, 1)
	assert.Equal(t, "/etc/a", got[0].Path)

	got = diffFiles(a, b, "etc/")
	require.Len(t, got, 1)
}

func TestDiffFiles_Identical(t *testing.T) {
	a := fsFrom(t, reg("a", "1", 0o644))
	b := fsFrom(t, reg("a", "1", 0o644))

	assert.Empty(t, diffFiles(a, b, ""))
}

func TestDiffLayers(t *testing.T) {
	desc := func(hex string, size int64) v1.Descriptor {
		return v1.Descriptor{Digest: v1.Hash{Algorithm: "sha256", Hex: hex}, Size: size}
	}
	a := []v1.Descriptor{desc("aaaa", 1), desc("bbbb", 2)}
	b := []v1.Descriptor{desc("aaaa", 1), desc("cccc", 3)}

	got := diffLayers(a, b)
	assert.Equal(t, []view.LayerDiff{
		{Status: view.DiffShared, Digest: "sha256:aaaa", Size: 1},
		{Status: view.DiffRemoved, Digest: "sha256:bbbb", Size: 2},
		{Status: view.DiffAdded, Digest: "sha256:cccc", Size: 3},
	}, got)
}
