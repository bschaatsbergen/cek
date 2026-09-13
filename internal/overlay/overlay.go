// Package overlay assembles the filesystem of an OCI image from its layers,
// applying whiteouts the way a container runtime does when it mounts them.
package overlay

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
)

// Layer is the part of v1.Layer this package needs.
type Layer interface {
	Uncompressed() (io.ReadCloser, error)
}

const (
	whiteoutPrefix = ".wh."
	opaqueWhiteout = ".wh..wh..opq"
	maxSymlinkHops = 40
)

var (
	ErrNotExist = errors.New("no such file or directory")
	ErrNotDir   = errors.New("not a directory")
	ErrLoop     = errors.New("too many levels of symbolic links")
)

// Entry describes one path in the filesystem.
type Entry struct {
	// Path is absolute and clean, without a trailing slash.
	Path string
	// Typeflag is the tar type of the entry.
	Typeflag byte
	// Mode holds the permission and special bits from the tar header.
	Mode int64
	// Size is the content size in bytes. Hardlinks carry their target's size.
	Size int64
	// Linkname is the target of a symlink or hardlink.
	Linkname string
	// Layer is the index of the layer that provides this version of the entry.
	Layer int
	// Digest is the hex SHA-256 of the content, set for regular files and
	// hardlinks when the filesystem was built WithDigests.
	Digest string
}

func (e *Entry) IsDir() bool      { return e.Typeflag == tar.TypeDir }
func (e *Entry) IsSymlink() bool  { return e.Typeflag == tar.TypeSymlink }
func (e *Entry) IsHardlink() bool { return e.Typeflag == tar.TypeLink }

// IsRegular reports whether the entry has file content: a regular file or a
// hardlink to one.
func (e *Entry) IsRegular() bool {
	return e.Typeflag == tar.TypeReg || e.Typeflag == tar.TypeLink
}

// ModeString renders the type and permission bits the way ls -l does.
func (e *Entry) ModeString() string {
	var typeChar byte
	switch e.Typeflag {
	case tar.TypeDir:
		typeChar = 'd'
	case tar.TypeSymlink:
		typeChar = 'l'
	case tar.TypeBlock:
		typeChar = 'b'
	case tar.TypeChar:
		typeChar = 'c'
	case tar.TypeFifo:
		typeChar = 'p'
	default:
		typeChar = '-'
	}

	perm := []byte("rwxrwxrwx")
	for i := range 9 {
		if e.Mode&(1<<(8-i)) == 0 {
			perm[i] = '-'
		}
	}
	// setuid, setgid and sticky replace the execute bit, as in ls.
	if e.Mode&0o4000 != 0 {
		perm[2] = specialBit(perm[2], 's', 'S')
	}
	if e.Mode&0o2000 != 0 {
		perm[5] = specialBit(perm[5], 's', 'S')
	}
	if e.Mode&0o1000 != 0 {
		perm[8] = specialBit(perm[8], 't', 'T')
	}

	return string(typeChar) + string(perm)
}

func specialBit(current, withExec, withoutExec byte) byte {
	if current == 'x' {
		return withExec
	}
	return withoutExec
}

// contentPath is the tar name that holds the entry's bytes.
func (e *Entry) contentPath() string {
	if e.IsHardlink() {
		return clean(e.Linkname)
	}
	return e.Path
}

// FS is a filesystem assembled from layers.
type FS struct {
	entries map[string]*Entry
}

// Option configures Build.
type Option func(*builder)

// WithDigests hashes regular file content while scanning, so entries carry
// a Digest. It costs reading every file once.
func WithDigests() Option {
	return func(b *builder) { b.digests = true }
}

type builder struct {
	digests bool
}

// Build assembles the merged filesystem. Layers apply in order, later ones
// overriding earlier ones, and whiteouts remove what lower layers added.
// Entry.Layer indexes into layers, so pass the same slice to Open and
// Extract. A single-element slice yields the view of one layer.
func Build[L Layer](layers []L, opts ...Option) (*FS, error) {
	b := builder{}
	for _, opt := range opts {
		opt(&b)
	}

	fs := &FS{entries: make(map[string]*Entry)}
	for i, layer := range layers {
		if err := b.apply(fs, layer, i); err != nil {
			return nil, fmt.Errorf("layer %d: %w", i+1, err)
		}
	}
	return fs, nil
}

func (b *builder) apply(fs *FS, layer Layer, index int) error {
	rc, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	var (
		adds      []*Entry
		links     []*Entry
		whiteouts []string // paths removed from lower layers, subtree included
		opaques   []string // directories whose lower-layer children are removed
	)

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}

		p := clean(hdr.Name)
		if p == "/" {
			continue
		}

		base := path.Base(p)
		if base == opaqueWhiteout {
			opaques = append(opaques, path.Dir(p))
			continue
		}
		if strings.HasPrefix(base, whiteoutPrefix) {
			whiteouts = append(whiteouts, path.Join(path.Dir(p), strings.TrimPrefix(base, whiteoutPrefix)))
			continue
		}

		e := &Entry{
			Path:     p,
			Typeflag: hdr.Typeflag,
			Mode:     hdr.Mode,
			Linkname: hdr.Linkname,
			Layer:    index,
		}
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA: //nolint:staticcheck // old archives still use TypeRegA
			e.Typeflag = tar.TypeReg
			e.Size = hdr.Size
			if b.digests {
				h := sha256.New()
				if _, err := io.Copy(h, tr); err != nil {
					return fmt.Errorf("failed to read %s: %w", p, err)
				}
				e.Digest = hex.EncodeToString(h.Sum(nil))
			}
		case tar.TypeLink:
			links = append(links, e)
		}
		adds = append(adds, e)
	}

	// Whiteouts only affect lower layers, so they run before this layer's
	// own entries land, whatever order the archive put them in.
	for _, w := range whiteouts {
		fs.removeTree(w)
	}
	for _, d := range opaques {
		fs.removeChildren(d)
	}
	for _, e := range adds {
		fs.entries[e.Path] = e
	}

	// A hardlink's content lives at its target in the same archive.
	for _, l := range links {
		if target, ok := fs.entries[clean(l.Linkname)]; ok {
			l.Size = target.Size
			l.Digest = target.Digest
		}
	}

	return nil
}

func (fs *FS) removeTree(p string) {
	delete(fs.entries, p)
	fs.removeChildren(p)
}

func (fs *FS) removeChildren(dir string) {
	prefix := dir + "/"
	for p := range fs.entries {
		if strings.HasPrefix(p, prefix) {
			delete(fs.entries, p)
		}
	}
}

// Len returns the number of entries.
func (fs *FS) Len() int {
	return len(fs.entries)
}

// Lookup returns the entry at p without following symlinks.
func (fs *FS) Lookup(p string) (*Entry, bool) {
	e, ok := fs.entries[clean(p)]
	return e, ok
}

// Entries returns every entry sorted by path.
func (fs *FS) Entries() []*Entry {
	entries := make([]*Entry, 0, len(fs.entries))
	for _, e := range fs.entries {
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b *Entry) int {
		return strings.Compare(a.Path, b.Path)
	})
	return entries
}

// Under returns the entry at p, if any, followed by every entry below it,
// sorted by path. Symlinks are not followed.
func (fs *FS) Under(p string) []*Entry {
	p = clean(p)
	prefix := p + "/"
	if p == "/" {
		prefix = "/"
	}

	var entries []*Entry
	for _, e := range fs.entries {
		if e.Path == p || strings.HasPrefix(e.Path, prefix) {
			entries = append(entries, e)
		}
	}
	slices.SortFunc(entries, func(a, b *Entry) int {
		return strings.Compare(a.Path, b.Path)
	})
	return entries
}

// Resolve returns the entry at p, following symlinks in every component the
// way a kernel path walk does. Intermediate directories missing from the
// layers are treated as present, since archives often omit them. The root
// resolves to a synthetic directory.
func (fs *FS) Resolve(p string) (*Entry, error) {
	return fs.resolve(clean(p), 0)
}

func (fs *FS) resolve(p string, hops int) (*Entry, error) {
	if hops > maxSymlinkHops {
		return nil, fmt.Errorf("%s: %w", p, ErrLoop)
	}
	if p == "/" {
		return &Entry{Path: "/", Typeflag: tar.TypeDir, Mode: 0o755}, nil
	}

	comps := strings.Split(strings.TrimPrefix(p, "/"), "/")
	cur := "/"
	for i, c := range comps {
		next := path.Join(cur, c)
		last := i == len(comps)-1

		e, ok := fs.entries[next]
		if !ok {
			if last {
				return nil, fmt.Errorf("%s: %w", p, ErrNotExist)
			}
			cur = next
			continue
		}

		if e.IsSymlink() {
			target := e.Linkname
			if !path.IsAbs(target) {
				target = path.Join(cur, target)
			}
			rest := path.Join(comps[i+1:]...)
			return fs.resolve(clean(path.Join(target, rest)), hops+1)
		}

		if last {
			return e, nil
		}
		if !e.IsDir() {
			return nil, fmt.Errorf("%s: %w", next, ErrNotDir)
		}
		cur = next
	}

	return nil, fmt.Errorf("%s: %w", p, ErrNotExist)
}

// Open returns the content of a regular file, read from the layer that
// provides it. The caller must close the reader.
func Open(layer Layer, e *Entry) (io.ReadCloser, error) {
	if !e.IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", e.Path)
	}
	want := e.contentPath()

	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get uncompressed layer: %w", err)
	}

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			_ = rc.Close()
			return nil, fmt.Errorf("%s: content not found in layer", e.Path)
		}
		if err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}
		if isRegular(hdr) && clean(hdr.Name) == want {
			return &entryReader{Reader: tr, closer: rc}, nil
		}
	}
}

type entryReader struct {
	io.Reader
	closer io.Closer
}

func (r *entryReader) Close() error {
	return r.closer.Close()
}

// Extract streams the content of regular files from one layer in a single
// pass, calling fn for each entry as its content is reached. Every entry
// must belong to layer and be regular.
func Extract(layer Layer, entries []*Entry, fn func(e *Entry, r io.Reader) error) error {
	wanted := make(map[string][]*Entry, len(entries))
	for _, e := range entries {
		if !e.IsRegular() {
			return fmt.Errorf("%s: not a regular file", e.Path)
		}
		wanted[e.contentPath()] = append(wanted[e.contentPath()], e)
	}
	if len(wanted) == 0 {
		return nil
	}

	rc, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	tr := tar.NewReader(rc)
	for len(wanted) > 0 {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}
		if !isRegular(hdr) {
			continue
		}

		p := clean(hdr.Name)
		es, ok := wanted[p]
		if !ok {
			continue
		}
		delete(wanted, p)

		if len(es) == 1 {
			if err := fn(es[0], tr); err != nil {
				return err
			}
			continue
		}

		// Several hardlinks share this content; a tar entry reads once.
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", p, err)
		}
		for _, e := range es {
			if err := fn(e, bytes.NewReader(data)); err != nil {
				return err
			}
		}
	}

	for p := range wanted {
		return fmt.Errorf("%s: content not found in layer", p)
	}
	return nil
}

func isRegular(hdr *tar.Header) bool {
	return hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA //nolint:staticcheck // old archives still use TypeRegA
}

// clean turns any tar name into an absolute, clean path without a trailing
// slash: "./etc/", "etc/" and "/etc/" all become "/etc".
func clean(name string) string {
	return path.Clean("/" + strings.TrimPrefix(name, "/"))
}
