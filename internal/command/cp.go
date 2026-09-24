package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bschaatsbergen/cek/internal/oci"
	"github.com/bschaatsbergen/cek/internal/overlay"
	"github.com/bschaatsbergen/cek/internal/view"
	"github.com/spf13/cobra"
)

type CpOptions struct {
	Layer int
	FetchFlags
}

func NewCpCommand(cli *CLI) *cobra.Command {
	opts := CpOptions{
		Layer: -1, // -1 means the merged filesystem
	}

	cmd := &cobra.Command{
		Use:   "cp <image> <src> <dest>",
		Short: "Copy a file or directory out of an OCI image",
		Long: highlight("cek cp nginx:latest /etc/nginx ./nginx-conf") + "\n\n" +
			"Copy a file or directory from an image to the local filesystem, without\n" +
			"creating a container. The source is read from the merged filesystem,\n" +
			"or from one layer with --layer.\n\n" +
			"Destination rules follow docker cp:\n" +
			"  - A file is written to <dest>, or into <dest> if that is a directory.\n" +
			"  - A directory is copied to <dest>, or into <dest>/<name> if <dest>\n" +
			"    already exists. End the source with /. to copy only its contents.\n\n" +
			"The source path itself is resolved through symlinks. Inside a copied\n" +
			"directory, symlinks stay symlinks and permissions are preserved.\n" +
			"Device nodes and fifos are skipped.\n\n" +
			"Examples:\n" +
			"  cek cp alpine:latest /etc/os-release .\n" +
			"  cek cp nginx:latest /etc/nginx ./nginx-conf\n" +
			"  cek cp nginx:latest /etc/nginx/. ./nginx-conf\n" +
			"  cek cp --layer 2 myapp:latest /app ./app-layer-2\n",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunCp(cmd.Context(), cli, args[0], args[1], args[2], &opts)
		},
	}

	cmd.Flags().IntVar(&opts.Layer, "layer", -1, "Copy from a specific layer (1-indexed)")
	AddFetchFlags(cmd, &opts.FetchFlags)

	return cmd
}

func RunCp(ctx context.Context, cli *CLI, imageRef, src, dest string, opts *CpOptions) error {
	logger := cli.Logger()
	logger.Debug("Copying from image", "image", imageRef, "src", src, "dest", dest)

	img, _, err := oci.FetchImage(ctx, imageRef, opts.FetchOptions())
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	imageFS, layers, err := buildFS(layers, opts.Layer)
	if err != nil {
		return err
	}

	contentsOnly := strings.HasSuffix(src, "/.")
	root, err := imageFS.Resolve(src)
	if err != nil {
		return err
	}

	c := &copier{fs: imageFS, layers: make([]overlay.Layer, len(layers))}
	for i, l := range layers {
		c.layers[i] = l
	}

	target := dest
	if root.IsDir() {
		if !contentsOnly && isDir(dest) {
			target = filepath.Join(dest, path.Base(root.Path))
		}
		err = c.copyTree(root, target)
	} else {
		if isDir(dest) {
			target = filepath.Join(dest, path.Base(root.Path))
		}
		err = c.copyFile(root, target)
	}
	if err != nil {
		return err
	}

	logger.Debug("Copy complete", "files", c.files, "bytes", c.bytes, "skipped", c.skipped)

	return cli.Cp().Render(&view.CpData{
		ImageRef:    imageRef,
		Source:      src,
		Destination: target,
		Files:       c.files,
		Bytes:       c.bytes,
		Skipped:     c.skipped,
	})
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// copier writes entries of an image filesystem to disk.
type copier struct {
	fs     *overlay.FS
	layers []overlay.Layer

	files   int
	bytes   int64
	skipped int
}

// copyFile writes one regular file to dst. The entry must already be
// resolved through symlinks.
func (c *copier) copyFile(e *overlay.Entry, dst string) error {
	switch {
	case e.IsDir():
		return fmt.Errorf("%s: is a directory", e.Path)
	case !e.IsRegular():
		return fmt.Errorf("%s: not a regular file", e.Path)
	}

	rc, err := overlay.Open(c.layers[e.Layer], e)
	if err != nil {
		return err
	}
	defer func() {
		_ = rc.Close()
	}()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(dst), err)
	}
	return c.writeFile(dst, e, rc)
}

// copyTree writes the directory at root and everything under it to dst.
// Entries are grouped by layer so each layer is read once.
func (c *copier) copyTree(root *overlay.Entry, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dst, err)
	}

	byLayer := make(map[int][]*overlay.Entry)
	dests := make(map[*overlay.Entry]string)
	var dirs []*overlay.Entry

	for _, e := range c.fs.Under(root.Path) {
		rel := strings.TrimPrefix(e.Path, root.Path)
		target := filepath.Join(dst, filepath.FromSlash(rel))
		dests[e] = target

		if err := refuseSymlinkParents(dst, target); err != nil {
			return err
		}

		switch {
		case e.IsDir():
			// Directories get their final mode last, so a read-only
			// directory does not block the files written into it.
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", target, err)
			}
			dirs = append(dirs, e)
		case e.IsSymlink():
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", filepath.Dir(target), err)
			}
			if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("failed to replace %s: %w", target, err)
			}
			if err := os.Symlink(e.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", target, err)
			}
			c.files++
		case e.IsRegular():
			byLayer[e.Layer] = append(byLayer[e.Layer], e)
		default:
			c.skipped++
		}
	}

	for _, layer := range slices.Sorted(maps.Keys(byLayer)) {
		err := overlay.Extract(c.layers[layer], byLayer[layer], func(e *overlay.Entry, r io.Reader) error {
			target := dests[e]
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", filepath.Dir(target), err)
			}
			return c.writeFile(target, e, r)
		})
		if err != nil {
			return err
		}
	}

	for _, e := range dirs {
		if err := os.Chmod(dests[e], perm(e)); err != nil {
			return fmt.Errorf("failed to set mode on %s: %w", dests[e], err)
		}
	}

	return nil
}

func (c *copier) writeFile(dst string, e *overlay.Entry, r io.Reader) error {
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm(e))
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dst, err)
	}

	n, err := io.Copy(f, r)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}

	// OpenFile applies the umask; set the image's bits exactly.
	if err := os.Chmod(dst, perm(e)); err != nil {
		return fmt.Errorf("failed to set mode on %s: %w", dst, err)
	}

	c.files++
	c.bytes += n
	return nil
}

func perm(e *overlay.Entry) os.FileMode {
	return os.FileMode(e.Mode) & os.ModePerm
}

// refuseSymlinkParents errors when any directory between root and p is a
// symlink on disk. A layer can ship a symlink and then files "inside" it;
// writing those would land wherever the link points, possibly outside root.
func refuseSymlinkParents(root, p string) error {
	rel, err := filepath.Rel(root, filepath.Dir(p))
	if err != nil || rel == "." {
		return nil
	}

	cur := root
	for _, comp := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, comp)
		info, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: refusing to write through symlink %s", p, cur)
		}
	}
	return nil
}
