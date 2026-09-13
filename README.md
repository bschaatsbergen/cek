# cek (container exploration kit)

[![Go Report Card](https://goreportcard.com/badge/github.com/bschaatsbergen/cek)](https://goreportcard.com/report/github.com/bschaatsbergen/cek)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Explore OCI container images without running them.

cek is a command-line utility for filesystem exploration inside OCI container
images. It focuses on browsing files, reading contents, and inspecting layer
mechanics—without running containers. cek reads images directly from local
container daemons (Docker, Podman, containerd, etc.) or pulls them from remote
registries.

cek runs without root privileges and works with any OCI-compliant image
registry. While it does not require a container daemon, it can leverage one when
available to access locally cached images and avoid registry rate limits. Most
importantly, cek never runs containers.

## Installation

```bash
brew install cek
```

Or with Go:

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

### List files in an image

By default, `cek ls` shows the merged overlay filesystem, which is what you see
inside a running container. All layers are combined, with upper layers
overriding lower ones and whiteouts removing what a `RUN rm` deleted. Output
is sorted by path, directories end in a slash, and symlinks show their
target.

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
cek cat --layer 2 nginx:latest /etc/nginx/nginx.conf

# Pipe to other tools
cek cat alpine:latest /etc/os-release | grep VERSION_ID

# Compare configuration between image versions
diff <(cek cat nginx:1.28 /etc/nginx/nginx.conf) \
     <(cek cat nginx:1.26 /etc/nginx/nginx.conf)
```

The `cat` command reads the file as it exists in the merged filesystem, just
like in a running container: a file deleted by an upper layer is gone, and
symlinks are followed, so `/etc/os-release` on Alpine resolves through its
symlink to `/usr/lib/os-release`.

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

### Export images to tar files

Export OCI images to tar files, including manifest, config, and all layers.
These tarballs make it easy to move images between environments, share images
without a registry, or back them up for disaster recovery.

```bash
# Export an image to a tar file
cek export alpine:latest -o alpine.tar

# Export a specific platform
cek export --platform linux/amd64 ubuntu:22.04 -o ubuntu-amd64.tar

# Load the exported tar into Docker or Podman
docker load -i alpine.tar
podman load -i alpine.tar
```

Use cases include air-gapped deployments, image backups, sharing images without
pushing to a registry, and transferring images between different container
runtimes.

### Compare two images

`cek diff` shows which layers two images share and which files were added,
removed or modified between their merged filesystems. Files are compared by
content, so a rebuilt file with the same size still shows up. Permission
changes and symlink retargets count as modifications too. Directories are
not listed; their files are.

Markers follow `terraform plan`: `+` added, `-` removed, `~` modified,
`=` shared.

```bash
cek diff alpine:3.21 alpine:3.22 /etc
Layers:
  - sha256:897d797d2723cf0e318402f4d6f37d51b011517e5cf09246b22155f0fa90dc81  3.5 MB
  + sha256:f7ee36c9aa34bbb665f975c76e5c0d1607f0674b94c84cfb0061f87006ea5d10  3.6 MB

Files:
  ~ /etc/alpine-release                 7 B -> 7 B
  ~ /etc/apk/repositories               103 B -> 103 B
  ~ /etc/issue                          54 B -> 51 B
  - /etc/modprobe.d/kms.conf            91 B
  ~ /etc/secfixes.d/alpine              97 B -> 97 B
  ~ /etc/ssl/certs/ca-certificates.crt  212.7 KB -> 175.2 KB
  ~ /etc/ssl/openssl.cnf                12.0 KB -> 12.1 KB
  ~ /etc/ssl/openssl.cnf.dist           12.0 KB -> 12.1 KB

0 added, 1 removed, 7 modified
```

Three of those files changed without changing size, which a size or
timestamp comparison would miss.

Scope the comparison to a directory by passing a path. With `--json`, each
file carries the mode, size, symlink target and content digest on both
sides, which makes the output easy to filter:

```bash
cek --json diff myapp:v1 myapp:v2 | jq '.files[] | select(.status == "modified") | .path'
```

### Display directory tree

Show the directory tree structure of an OCI image, making it easy to visualize
the filesystem layout.

```bash
# Show the top-level directories in an image
cek tree nginx:latest -L 1

# Inspect the /usr/local/bin folder of a specific layer
cek tree --layer 4 python:3.12-slim /usr/local/bin
```

### Inspect image metadata

View image details: digest, creation time, architecture, total size, the
runtime config and every layer with its digest, size and media type. Only
config fields that are set are printed. Layers that carry annotations in
the manifest, such as encrypted layers, get a separate table with the
annotation keys and values.

```bash
cek inspect nginx
Image: nginx
Registry: index.docker.io
Digest: sha256:8b76a8da0aa5533dda5053935f320f3ebe07beec9e12ea1e8e3b9e1ae1a6bf5a
Created: 2026-09-02T21:05:47Z
OS/Arch: linux/arm64
Size: 62.0 MB
Entrypoint: /docker-entrypoint.sh
Cmd: nginx -g "daemon off;"
Ports: 80/tcp
Env:
  PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
  NGINX_VERSION=1.31.5
  NJS_VERSION=1.0.1
  NJS_RELEASE=1~trixie
  ACME_VERSION=0.4.1
  PKG_RELEASE=1~trixie
  DYNPKG_RELEASE=1~trixie
Labels:
  maintainer=NGINX Docker Maintainers <docker-maint@nginx.com>

Layers:
#  Digest                                                                   Size     Media Type
1  sha256:bf7af0229701decd1b9f42143504fc8f69e5664c37e57001d198e731e4f86c2e  28.8 MB  application/vnd.docker.image.rootfs.diff.tar.gzip
2  sha256:ac3ce1865cbf71ffb4cb1e8a38faa751b340b68868cd6f3904f80ae326353261  33.2 MB  application/vnd.docker.image.rootfs.diff.tar.gzip
3  sha256:50355e0ebcbc3c053fbd5e75aa03114809f9f4f1177379c71790757c33b04f5c  629 B    application/vnd.docker.image.rootfs.diff.tar.gzip
4  sha256:dd00b62474a6e527cc71136b97d2c2d0a68aa282b9685568f40d5e399bdb990a  957 B    application/vnd.docker.image.rootfs.diff.tar.gzip
5  sha256:fef72342d9bd299793d88c4a3baeb75aac54c538c85bafd45776883f4b8bf78d  405 B    application/vnd.docker.image.rootfs.diff.tar.gzip
6  sha256:4225c79b86e9402dd4857cf5d32e29a13acecf02d57bba22bf46f0926f05a177  1.2 KB   application/vnd.docker.image.rootfs.diff.tar.gzip
7  sha256:a3d95972273c02fbedb65572c40fb5501ce9f8c1514ba62c8d51ff3a9ddaed0e  1.4 KB   application/vnd.docker.image.rootfs.diff.tar.gzip
```

Long label and annotation values are cut short in the table. Use `--json`
to get the same data, including the full `config` and per-layer
`mediaType` and `annotations`, as structured output.

### Print the manifest and config

`cek manifest` prints the image manifest: the config descriptor and every
layer descriptor with its media type, size, digest and annotations.
`cek config` prints the config blob: the runtime config (entrypoint, cmd,
env, user, ports, labels), the rootfs diff IDs and the build history. Both
are the documents the registry serves, not a reinterpretation.

```bash
cek manifest nginx:latest | jq '.layers[-1]'
cek config nginx:latest | jq '.config.Env'
cek config nginx:latest | jq -r '.history[].created_by'
```

The output is indented for reading. With `--json` the exact bytes are
written instead, so the output hashes to the document's digest:

```bash
cek --json manifest --pull always nginx:latest | shasum -a 256
cek inspect --pull always nginx:latest | grep Digest
```

### Write a raw layer blob

Write the bytes of a layer blob to standard output exactly as the registry
stores them: no decompression, no tar parsing. Use it to see what a registry
actually holds, to hash a layer, or to save a layer for offline inspection.
Layers are 1-indexed, matching the `#` column of `cek inspect`.

```bash
# A gzip layer starts with the gzip magic bytes 1f 8b
cek blob --layer 1 alpine:latest | head -c 32 | xxd

# Hash the blob; the digest matches the layer digest shown by cek inspect
cek blob --layer 2 --pull always nginx:latest | shasum -a 256

# Save a layer for offline inspection
cek blob --layer 2 --pull always nginx:latest > layer.tar.gz
```

Use `--pull always` to read the blob from the registry. A blob served by a
local container daemon is re-exported by the daemon and may not be
byte-identical to the registry copy, so its hash can differ from the
registry's layer digest.

## Shell Completion

cek generates completion scripts for bash, zsh, fish and PowerShell.
Completion covers subcommands, flags, and the values of `--pull` and
`--platform`.

```bash
# bash
cek completion bash > /etc/bash_completion.d/cek

# zsh
cek completion zsh > "${fpath[1]}/_cek"

# fish
cek completion fish > ~/.config/fish/completions/cek.fish

# PowerShell
cek completion powershell | Out-String | Invoke-Expression
```

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

## Registry Authentication

cek sends the credentials that `docker login` stores, and it honors the
credential helpers configured in `~/.docker/config.json`. Private
repositories work the same way they do with `docker pull` and `crane`.

```bash
docker login ghcr.io
cek ls ghcr.io/org/private-image:latest
```

Logging in to Docker Hub also lifts the anonymous pull rate limit, which
the `--pull always` policy runs into quickly.

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
