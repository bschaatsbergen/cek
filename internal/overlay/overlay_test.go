package overlay_test

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/bschaatsbergen/cek/internal/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tarEntry is a compact description of one archive member.
type tarEntry struct {
	name     string
	typeflag byte
	mode     int64
	content  string
	linkname string
}

func file(name, content string) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeReg, mode: 0o644, content: content}
}

func dir(name string) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeDir, mode: 0o755}
}

func symlink(name, target string) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeSymlink, mode: 0o777, linkname: target}
}

func hardlink(name, target string) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeLink, mode: 0o644, linkname: target}
}

func whiteout(name string) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeReg, mode: 0o644}
}

// memLayer is an uncompressed tar held in memory.
type memLayer struct {
	data []byte
}

func (l *memLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.data)), nil
}

func layer(t *testing.T, entries ...tarEntry) overlay.Layer {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Mode:     e.mode,
			Linkname: e.linkname,
			Size:     int64(len(e.content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(e.content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	return &memLayer{data: buf.Bytes()}
}

func paths(entries []*overlay.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Path
	}
	return out
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestBuild_LaterLayersOverride(t *testing.T) {
	layers := []overlay.Layer{
		layer(t, dir("etc/"), file("etc/a", "one"), file("etc/b", "two")),
		layer(t, file("etc/a", "three")),
	}

	fs, err := overlay.Build(layers)
	require.NoError(t, err)

	a, ok := fs.Lookup("/etc/a")
	require.True(t, ok)
	assert.Equal(t, 1, a.Layer)
	assert.Equal(t, int64(5), a.Size)

	b, ok := fs.Lookup("/etc/b")
	require.True(t, ok)
	assert.Equal(t, 0, b.Layer)

	assert.Equal(t, []string{"/etc", "/etc/a", "/etc/b"}, paths(fs.Entries()))
}

func TestBuild_NormalizesNames(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, dir("./"), dir("./etc/"), file("etc/motd", "hi"), file("/usr/bin/x", "")),
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"/etc", "/etc/motd", "/usr/bin/x"}, paths(fs.Entries()))
}

func TestBuild_WhiteoutRemovesLowerFile(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, file("etc/motd", "hi"), file("etc/keep", "")),
		layer(t, whiteout("etc/.wh.motd")),
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"/etc/keep"}, paths(fs.Entries()))
}

func TestBuild_WhiteoutRemovesSubtree(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, dir("var/lib/apt/"), file("var/lib/apt/lists", ""), file("var/lib/aptitude", "")),
		layer(t, whiteout("var/lib/.wh.apt")),
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"/var/lib/aptitude"}, paths(fs.Entries()))
}

func TestBuild_WhiteoutDoesNotAffectSameLayer(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, file("etc/motd", "old")),
		layer(t, file("etc/motd", "new"), whiteout("etc/.wh.motd")),
	})
	require.NoError(t, err)

	e, ok := fs.Lookup("/etc/motd")
	require.True(t, ok)
	assert.Equal(t, 1, e.Layer)
	assert.Equal(t, int64(3), e.Size)
}

func TestBuild_OpaqueWhiteoutClearsLowerChildren(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, dir("opt/"), file("opt/old", ""), dir("opt/sub/"), file("opt/sub/x", "")),
		layer(t, whiteout("opt/.wh..wh..opq"), file("opt/new", "")),
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"/opt", "/opt/new"}, paths(fs.Entries()))
	e, _ := fs.Lookup("/opt")
	assert.Equal(t, 0, e.Layer, "the directory itself survives an opaque whiteout")
}

func TestBuild_WhiteoutInLaterLayerRemovesEarlierVersionOnly(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, file("a", "1")),
		layer(t, whiteout(".wh.a")),
		layer(t, file("a", "3")),
	})
	require.NoError(t, err)

	e, ok := fs.Lookup("/a")
	require.True(t, ok)
	assert.Equal(t, 2, e.Layer)
}

func TestBuild_SingleLayerDropsWhiteoutEntries(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, whiteout("etc/.wh.motd"), whiteout("opt/.wh..wh..opq"), file("etc/new", "")),
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"/etc/new"}, paths(fs.Entries()))
}

func TestUnder(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, dir("bin/"), file("bin/sh", ""), dir("sbin/"), file("sbin/init", ""), file("binary", "")),
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"/bin", "/bin/sh"}, paths(fs.Under("/bin")))
	assert.Equal(t, []string{"/bin", "/bin/sh"}, paths(fs.Under("bin/")))
	assert.Equal(t, []string{"/bin", "/bin/sh", "/binary", "/sbin", "/sbin/init"}, paths(fs.Under("/")))
	assert.Empty(t, fs.Under("/nope"))
}

func TestResolve_FollowsSymlinks(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t,
			dir("etc/"), symlink("etc/os-release", "../usr/lib/os-release"),
			dir("usr/lib/"), file("usr/lib/os-release", "NAME=Alpine"),
			symlink("bin", "usr/bin"), dir("usr/bin/"), file("usr/bin/sh", "#!"),
			symlink("abs", "/usr/lib/os-release"),
			symlink("chain", "abs"),
		),
	})
	require.NoError(t, err)

	tests := []struct {
		path string
		want string
	}{
		{"/etc/os-release", "/usr/lib/os-release"},
		{"etc/os-release", "/usr/lib/os-release"},
		{"/bin/sh", "/usr/bin/sh"},
		{"/abs", "/usr/lib/os-release"},
		{"/chain", "/usr/lib/os-release"},
		{"/usr/lib/os-release", "/usr/lib/os-release"},
		{"/etc", "/etc"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			e, err := fs.Resolve(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.want, e.Path)
		})
	}
}

func TestResolve_ImplicitDirectories(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, file("usr/lib/os-release", "x")),
	})
	require.NoError(t, err)

	e, err := fs.Resolve("/usr/lib/os-release")
	require.NoError(t, err)
	assert.Equal(t, "/usr/lib/os-release", e.Path)
}

func TestResolve_Root(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{layer(t, file("a", ""))})
	require.NoError(t, err)

	e, err := fs.Resolve("/")
	require.NoError(t, err)
	assert.True(t, e.IsDir())
	assert.Equal(t, "/", e.Path)
}

func TestResolve_Errors(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, file("file", ""), symlink("a", "b"), symlink("b", "a"), symlink("dangling", "nowhere")),
	})
	require.NoError(t, err)

	_, err = fs.Resolve("/missing")
	assert.ErrorIs(t, err, overlay.ErrNotExist)

	_, err = fs.Resolve("/dangling")
	assert.ErrorIs(t, err, overlay.ErrNotExist)

	_, err = fs.Resolve("/file/child")
	assert.ErrorIs(t, err, overlay.ErrNotDir)

	_, err = fs.Resolve("/a")
	assert.ErrorIs(t, err, overlay.ErrLoop)
}

func TestBuild_WithDigests(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{
		layer(t, file("a", "hello"), hardlink("b", "a"), dir("d/")),
	}, overlay.WithDigests())
	require.NoError(t, err)

	a, _ := fs.Lookup("/a")
	assert.Equal(t, sha("hello"), a.Digest)

	b, _ := fs.Lookup("/b")
	assert.True(t, b.IsHardlink())
	assert.True(t, b.IsRegular())
	assert.Equal(t, sha("hello"), b.Digest, "a hardlink carries its target's digest")
	assert.Equal(t, int64(5), b.Size, "a hardlink carries its target's size")

	d, _ := fs.Lookup("/d")
	assert.Empty(t, d.Digest)
}

func TestBuild_WithoutDigests(t *testing.T) {
	fs, err := overlay.Build([]overlay.Layer{layer(t, file("a", "hello"))})
	require.NoError(t, err)

	a, _ := fs.Lookup("/a")
	assert.Empty(t, a.Digest)
}

func TestOpen(t *testing.T) {
	layers := []overlay.Layer{
		layer(t, file("a", "first"), dir("d/")),
		layer(t, file("a", "second"), hardlink("link", "a")),
	}
	fs, err := overlay.Build(layers)
	require.NoError(t, err)

	read := func(p string) string {
		t.Helper()
		e, err := fs.Resolve(p)
		require.NoError(t, err)
		rc, err := overlay.Open(layers[e.Layer], e)
		require.NoError(t, err)
		defer rc.Close()
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		return string(data)
	}

	assert.Equal(t, "second", read("/a"))
	assert.Equal(t, "second", read("/link"))

	d, _ := fs.Lookup("/d")
	_, err = overlay.Open(layers[d.Layer], d)
	assert.ErrorContains(t, err, "not a regular file")
}

func TestExtract(t *testing.T) {
	l := layer(t, file("a", "aaa"), file("b", "bb"), hardlink("c", "a"), hardlink("d", "a"), file("skip", "no"))
	fs, err := overlay.Build([]overlay.Layer{l})
	require.NoError(t, err)

	var want []*overlay.Entry
	for _, p := range []string{"/a", "/b", "/c", "/d"} {
		e, ok := fs.Lookup(p)
		require.True(t, ok)
		want = append(want, e)
	}

	got := map[string]string{}
	err = overlay.Extract(l, want, func(e *overlay.Entry, r io.Reader) error {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		got[e.Path] = string(data)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"/a": "aaa", "/b": "bb", "/c": "aaa", "/d": "aaa"}, got)
}

func TestExtract_MissingContent(t *testing.T) {
	l := layer(t, file("a", "aaa"))
	other := layer(t, file("b", "bbb"))
	fs, err := overlay.Build([]overlay.Layer{l, other})
	require.NoError(t, err)

	b, _ := fs.Lookup("/b")
	err = overlay.Extract(l, []*overlay.Entry{b}, func(*overlay.Entry, io.Reader) error { return nil })
	assert.ErrorContains(t, err, "content not found")
}

func TestExtract_PropagatesCallbackError(t *testing.T) {
	l := layer(t, file("a", "aaa"))
	fs, err := overlay.Build([]overlay.Layer{l})
	require.NoError(t, err)

	a, _ := fs.Lookup("/a")
	boom := errors.New("boom")
	err = overlay.Extract(l, []*overlay.Entry{a}, func(*overlay.Entry, io.Reader) error { return boom })
	assert.ErrorIs(t, err, boom)
}

func TestEntry_ModeString(t *testing.T) {
	tests := []struct {
		name     string
		typeflag byte
		mode     int64
		want     string
	}{
		{"file", tar.TypeReg, 0o644, "-rw-r--r--"},
		{"dir", tar.TypeDir, 0o755, "drwxr-xr-x"},
		{"symlink", tar.TypeSymlink, 0o777, "lrwxrwxrwx"},
		{"hardlink", tar.TypeLink, 0o600, "-rw-------"},
		{"setuid", tar.TypeReg, 0o4755, "-rwsr-xr-x"},
		{"setuid no exec", tar.TypeReg, 0o4644, "-rwSr--r--"},
		{"setgid", tar.TypeReg, 0o2755, "-rwxr-sr-x"},
		{"sticky dir", tar.TypeDir, 0o1777, "drwxrwxrwt"},
		{"fifo", tar.TypeFifo, 0o600, "prw-------"},
		{"char", tar.TypeChar, 0o666, "crw-rw-rw-"},
		{"block", tar.TypeBlock, 0o660, "brw-rw----"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &overlay.Entry{Typeflag: tt.typeflag, Mode: tt.mode}
			assert.Equal(t, tt.want, e.ModeString())
		})
	}
}
