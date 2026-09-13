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
cek cat --layer 2 nginx:latest /etc/nginx/nginx.conf

# Pipe to other tools
cek cat alpine:latest /usr/lib/os-release | grep VERSION_ID

# Compare configuration between image versions
diff <(cek cat nginx:1.28 /etc/nginx/nginx.conf) \
     <(cek cat nginx:1.26 /etc/nginx/nginx.conf)
```

The `cat` command searches layers top-down to find the final file state after
all overlays, just like in a running container.

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

View image details including digest, creation time, architecture, total size,
and individual layer information. Each layer is listed with its digest, size
and media type. Layers that carry annotations in the manifest, such as
encrypted layers, get a separate table with the annotation keys and values.

```bash
cek inspect nginx
Image: nginx
Registry: index.docker.io
Digest: sha256:988dc6ba913b85fe049a5d06452fe8766c4abda44a06614f47458ff4579330fd
Created: 2026-09-02T21:04:32Z
OS/Arch: linux/amd64
Size: 63.2 MB

Layers:
#  Digest                                                                   Size     Media Type
1  sha256:6310eb16bf4251731feab01e8f633bf5e2d75a657ccad97f420b1f83cce457be  28.4 MB  application/vnd.oci.image.layer.v1.tar+gzip
2  sha256:956faab5efb34579d85a5c0b79f1b44c111197a0c1fea9c18b2e836821d68480  34.8 MB  application/vnd.oci.image.layer.v1.tar+gzip
3  sha256:a44b5c8be61615ee48a9525b9aa459639de34e8e004ad3b3a6f52b793051da3c  629 B    application/vnd.oci.image.layer.v1.tar+gzip
4  sha256:02fc02c4ab8d7c507d6a06a08833d6b669278849c9533729535984c86ff199cb  956 B    application/vnd.oci.image.layer.v1.tar+gzip
5  sha256:c12f394dea35bb47b5511557233c5abec5360e40f9fade8f3ff1de488ecdc696  404 B    application/vnd.oci.image.layer.v1.tar+gzip
6  sha256:07db7bf2649b9fe0ccc9c86378fc5ed36e90bdfe6acf529e567c45ee96a2b9c3  1.2 KB   application/vnd.oci.image.layer.v1.tar+gzip
7  sha256:f340c1b7c1d6861ed76e1adc2630b145227b071b54259bc97e318297cb4b8156  1.4 KB   application/vnd.oci.image.layer.v1.tar+gzip
```

Long annotation values are cut short in the table. Use `--json` to get the
same data, including per-layer `mediaType` and full `annotations`, as
structured output.

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
