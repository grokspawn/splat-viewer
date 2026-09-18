# splat-viewer

`splat-viewer` turns **File-Based Catalog (FBC)** data into an interactive
three-dimensional graph in your browser. Explore bundle upgrade paths,
package dependencies, and catalog evolution across catalog revisions without
modifying the catalog. It is particularly useful for OpenShift operator
catalogs, but revisions supplied through `--refs` are not restricted to
OpenShift version strings.

The viewer is a Go CLI with an embedded HTML/JavaScript frontend. It can read
local catalog files, local catalog directories, or OCI catalog pullspecs.

## What it shows

- **Z axis:** catalog revisions, one depth plane per revision.
- **X axis:** bundles within a package and release, ordered by version.
- **Y axis:** packages, arranged alphabetically.
- **Nodes:** bundles, channel heads, deprecated bundles, skipped bundles,
  bundles with optional `maxOpenShiftVersion` metadata, and inferred (phantom)
  upgrade endpoints.
- **Upgrade edges:** `replaces`, `skips`, `skipRange`, and cross-release links.
- **Dependency edges:** package and GVK dependencies.

The browser viewer provides package/bundle search, deprecated-node visibility,
dependency-edge visibility, reset/zoom controls, node details, and a legend.
It also honors `prefers-reduced-motion` for users who request reduced motion.

## Requirements

- Go **1.26.3** (the version declared by `go.mod`)
- A browser with WebGL support
- Network access to `unpkg.com` when loading the viewer libraries
- Access to the catalog YAML files or OCI registries being inspected

## Build and test

```sh
git clone <your-repository-url>
cd splat-viewer

make build
make test
```

The commands build `./splat-viewer` and run `go test ./... -count=1`,
respectively. You can also build or test directly:

```sh
go build -o splat-viewer .
go test ./...
```

## Quick start

### Read local catalog files

Use `--refs` for one or more local YAML files or directories:

```sh
./splat-viewer serve \
  --refs /path/to/catalog-4.17.yaml,/path/to/catalog-4.18.yaml
```

The command starts a local HTTP server on port `8080` and opens the viewer in
your default browser. Prevent automatic browser launching with `--no-browser`.

### Read catalogs by revision directory

For local catalog caches organized by revision, use `--catalog-dir`,
`--catalog`, and optionally `--releases`:

```sh
./splat-viewer serve \
  --catalog-dir /path/to/catalog-data \
  --catalog redhat-operator-index \
  --releases 4.17,4.18,4.19
```

The expected convention is:

```text
/path/to/catalog-data/
└── redhat-operator-index/
    ├── 4.17/redhat-operator-index-v4.17.yaml
    ├── 4.18/redhat-operator-index-v4.18.yaml
    └── 4.19/redhat-operator-index-v4.19.yaml
```

If `--releases` is omitted, this directory-layout shortcut discovers only
child directories that contain the expected catalog file. The child directory
name becomes the revision label; it may be an OpenShift version such as `4.17`
or `5.0`, or another label such as `stable`.

This convention is intended for local catalog caches because FBC/OCI catalog
versioning does not provide one universal filesystem naming standard. Each
revision directory must contain:

```text
<catalog-name>-v<revision>.yaml
```

For example, `stable` must contain
`redhat-operator-index-vstable.yaml`. Unrelated directories are ignored. Use
explicit `--releases` values or `--refs` when your local layout differs.

### Read OCI catalog pullspecs

Pass OCI pullspecs through `--refs`:

```sh
./splat-viewer serve \
  --refs registry.redhat.io/redhat/redhat-operator-index:v4.17,registry.redhat.io/redhat/redhat-operator-index:v4.18
```

Local files, local directories, and OCI pullspecs may be combined in one
`--refs` value.

## Commands

### `serve`

Builds the graph and serves the interactive viewer over HTTP.

```text
./splat-viewer serve [flags]
```

Serve-specific flags:

| Flag | Default | Description |
| --- | --- | --- |
| `--port` | `8080` | Local HTTP port |
| `--no-browser` | `false` | Do not open a browser automatically |

### `export`

Writes an HTML viewer and raw graph data to an output directory:

```sh
./splat-viewer export \
  --refs /path/to/catalog.yaml \
  --output-dir ./splat-output
```

The output contains:

```text
./splat-output/
├── graph.json    # Graph data for programmatic use
└── viewer.html   # HTML viewer with graph data embedded
```

`viewer.html` embeds the graph data, so it does not need a running
`splat-viewer` server. It still loads the 3D graph libraries from the unpkg
CDN, so internet access is required unless those dependencies are made
available through another mechanism.

## Common flags

These flags apply to both `serve` and `export`:

| Flag | Description |
| --- | --- |
| `--refs` | Comma-separated local files, directories, or OCI pullspecs |
| `--catalog-dir` | Base directory containing catalog release data |
| `--catalog` | Catalog name used with `--catalog-dir` |
| `--releases` | Comma-separated catalog revision labels to load |
| `--package` | Restrict the graph to one package across releases |
| `--skip-tls-verify` | Skip TLS verification for OCI registries |
| `--skip-range-edges` | Materialize `skipRange` edges; may greatly increase graph size |

For example, focus on one package while avoiding automatic browser launch:

```sh
./splat-viewer serve \
  --refs registry.redhat.io/redhat/redhat-operator-index:v4.17 \
  --package 3scale-operator \
  --no-browser
```

Use `--skip-tls-verify` only when required by a trusted registry environment.

## Working with large catalogs

Catalogs are loaded into memory before the graph is built, and the browser
renders the resulting graph client-side. Full catalogs can therefore require
substantial memory and may become slow to render or interact with. For better
performance:

1. Start with `--package` to inspect a focused slice.
2. Leave `--skip-range-edges` disabled unless those concrete edges are needed.
3. Use the viewer search field to reduce the visible graph.
4. Use the dependency toggle to hide dependency edges while exploring upgrades.

The viewer displays a large-graph notice when more than 2,000 nodes are
present. The application does not upload catalog data or persist a backend
database; `serve` keeps the graph in the running process.

## Project layout

```text
cmd/                 Cobra commands and shared input flags
pkg/catalog/         FBC loading and OCI rendering
pkg/graph/            Graph model, catalog transformation, and layout
pkg/server/           Local HTTP server and browser launching
web/                 Embedded viewer HTML
main.go              CLI entrypoint
Makefile             Build, test, lint, and clean targets
```

## Development

Run the standard checks before submitting changes:

```sh
make test
go vet ./...
make build
```

The `make lint` target runs `go vet` and `golangci-lint`; if the linter is not
installed, the target installs the configured version first.

## Limitations and security notes

- Catalog YAML is fully loaded in memory; size your environment accordingly.
- The browser depends on WebGL and CDN-hosted `3d-force-graph` and Three.js.
- Exported HTML contains the graph data in the file; treat exports as
  potentially sensitive catalog artifacts.
- `--skip-tls-verify` weakens registry transport verification and should not
  be used routinely.
- The tool visualizes catalog data; it does not edit catalogs, perform
  upgrades, or provide authentication and multi-user collaboration.
