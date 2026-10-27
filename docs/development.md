# Development

English | [简体中文](development.zh-CN.md) · [Home](../README.md)

## Build and check

Source builds require Go 1.24+ and `make`. Checks also require Node.js 24+
and Python 3.9+.

```sh
git clone https://github.com/sky-bro/browser-auth-bridge.git
cd browser-auth-bridge
make install
export PATH="$HOME/.local/bin:$PATH"
bab setup
```

Complete the [Chrome registration](../README.md#install) using the printed extension directory.

```sh
make check build smoke
```

`check` runs Go race tests, browser mocks, Python tests and the source audit.
`smoke` uses a temporary install prefix and empty registry without opening Chrome,
registering a Native Host or accessing live accounts. CI runs on macOS and Linux;
browser acceptance is a separate manual check.

## Local example

The [echo fixture](../examples/plugins/echo/README.md) runs locally without a
real account. Its plugin directory contains the setup and commands.

## SDK and architecture

Go module: `github.com/sky-bro/browser-auth-bridge`.
See [Architecture](architecture.md) for the component diagram, execution paths
and package responsibilities, and the [plugin contract](plugin-development.md)
for integration details.

Keep plugin-specific setup and command documentation beside its `bab-plugin.json`,
using `README.md` and `README.zh-CN.md`. Shared guides link to those files.

## Source layout and releases

Browser source lives in `internal/extension`. `extension/`, `bin/` and `dist/`
are ignored generated directories. To build a custom development installation:

```sh
bab extension build --output ./extension
```

Reload that same extension after component changes. Keep user catalogs,
private plugins, deployment settings and credentials outside tracked source.

`make release` builds macOS/Linux binaries for arm64/amd64 and their checksums.
Pushing a matching `vVERSION` tag runs checks and publishes the release.
See [Release procedure](../PUBLICATION.md) and [Security](../SECURITY.md).
