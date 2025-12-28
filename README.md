# cek

List, inspect and explore OCI container images, their layers and contents.

cek is a command-line utility for exploring OCI container images without running
them. It can read images directly from local container daemons (Docker, Podman,
containerd, etc.) or pull them from remote registries, allowing you to inspect
metadata, browse files and directories, read file contents, and compare image
versions or layers.

## Installation

```bash
go install github.com/bschaatsbergen/cek@latest
```

Or build from source:

```bash
git clone https://github.com/bschaatsbergen/cek.git
cd cek
go build -o cek .
```

## Usage

### Inspect image metadata

View image details including digest, creation time, architecture, total size,
and individual layer information.

```bash
cek inspect nginx:latest
Image: nginx:latest
Registry: index.docker.io
Digest: sha256:ec0ee8695f2f71addca9b40f27df0fdfbde460485a2b68b834e18ea856542f1e
Created: 2025-12-09T22:50:18Z
OS/Arch: linux/arm64
Size: 55.6 MB

Layers:
#   Digest                                                             Size
1   sha256:f626fba1463b32b20f78d29b52dcf15be927dbb5372a9ba6a5f97aad47ae220b 28.7 MB
2   sha256:89d0a1112522e6e01ed53f0b339cb1a121ea7e19cfebdb325763bf5045ba7a47 26.8 MB
3   sha256:1b7c70849006971147c73371c868b789998c7220ba42e777d2d7e5894ac26e54 627 B
4   sha256:b8b0307e95c93307d99d02d3bdc61c3ed0b8d26685bb9bafc6c62d4170a2363e 954 B
5   sha256:fe1d23b41cb3b150a19a697809a56f455f1dac2bf8b60c8a1d0427965126aaf9 403 B
6   sha256:fda1d961e2b70f435ee701baaa260a569d7ea2eacd9f6dba8ac0320dc9b7d9fe 1.2 KB
7   sha256:10dbff0ec650f05c6cdcb80c2e7cc93db11c265b775a7a54e1dd48e4cbcebbbc 1.4 KB
```

### List files in an image

By default, `cek ls` shows the merged overlay filesystem, which is what you see
inside a running container. All layers are combined, with upper layers
overriding lower ones.

You can optionally specify a path to list only files under a specific directory.

```bash
# Show all files (merged overlay view)
cek ls nginx:latest

# List files in a specific directory
cek ls nginx:latest /etc
cek ls nginx:latest /etc/nginx

# Combine path with pattern filter
cek ls nginx:latest /etc/nginx --filter '*.conf'

# Filter by pattern (supports doublestar glob matching)
cek ls --filter '**/nginx/*.conf' nginx:latest

# Show files from a specific layer only
cek ls --layer 1 nginx:latest
```

Patterns without slashes match against basenames. Patterns with slashes match
against full paths. Use `**` for recursive directory matching.

### Read file contents

Write file contents to standard output from any image without creating a
container. Output can be piped to other commands or redirected to files for
inspection, diffing, or processing.

```bash
cek cat nginx:latest /etc/nginx/nginx.conf

# Read from a specific layer
cek cat --layer 2 nginx:latest /etc/os-release

# Pipe to other tools
cek cat alpine:latest /etc/os-release | grep VERSION_ID

# Compare configuration between image versions
diff <(cek cat nginx:1.25 /etc/nginx/nginx.conf) \
     <(cek cat nginx:1.24 /etc/nginx/nginx.conf)
```

The `cat` command searches layers top-down to find the final file state after
all overlays, just like in a running container.

### Compare two image tags

Compare images from the same repository to see what changed between versions.
Only files from unique layers are analyzed, making comparisons fast even for
large images with shared base layers.

```bash
cek compare alpine:3.19 alpine:3.18
```

The comparison skips shared base layers automatically, reducing I/O for images
with common ancestry.

### List available tags

List all tags in a repository from the remote registry, allowing you to find
available tags or a specific tag.

```bash
cek tags nginx

# Limit output to first N tags
cek tags alpine --limit 20

# Pipe to less for pagination
cek tags nginx | less

# Filter tags with grep
cek tags nginx | grep '^1\.2'
cek tags python | grep -E '^3\.(11|12)'
```

Note: This queries the remote registry directly, not the local daemon cache.

## Container Daemon Support

cek works with all popular container daemons by connecting to the container
daemon socket. The daemon provides access to locally cached images, avoiding
rate limits when exploring images you've already pulled.

Set `DOCKER_HOST` to point to your runtime's socket:

```bash
# Docker (standard Linux)
export DOCKER_HOST=unix:///var/run/docker.sock

# Docker Desktop (macOS)
export DOCKER_HOST=unix://$HOME/.docker/run/docker.sock

# Colima (macOS)
export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock

# Podman (Linux with XDG_RUNTIME_DIR)
export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock

# Podman Machine (macOS)
export DOCKER_HOST=unix://$HOME/.local/share/containers/podman/machine/podman.sock
```

If `DOCKER_HOST` is not set, cek will attempt to use the default Docker socket
location.

## Pull Policies

cek defaults to `if-not-present` to avoid registry rate limits. Images are
fetched from your local container daemon cache when available, falling back to
the remote registry only if needed.

```bash
# Use local cache if available, pull if missing (default)
cek inspect --pull if-not-present nginx:latest

# Always pull from registry, even if cached locally
# Useful for checking if :latest tag has been updated
cek inspect --pull always nginx:latest

# Only use local cache, never pull from registry
# Useful for offline work or avoiding network calls
cek inspect --pull never nginx:latest
```

Images pulled by `docker pull`, `nerdctl pull` or `podman pull` are immediately
available to cek without additional downloads.

When using `if-not-present`, cek checks the local container daemon first. If the
image exists locally, it's used immediately without any network calls. If not
found locally, cek pulls from the remote registry.
