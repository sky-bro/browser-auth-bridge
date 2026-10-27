# Release status and procedure

This repository root is the public Go module. It has a fresh history and no
private wrapper project. Runtime configuration, installed plugins, private
sources and generated extensions belong outside tracked source.

## Review fixes

Plugins can be installed selectively, installed disabled, enabled or disabled
without removing their package or changing Chrome authorization. Native attach
validation and browser routing enforce that state. Managed providers wait for a
confirmed native registry and stop on removal, disable or bridge disconnect.
Permission cleanup handles hosts with ports and preserves user-owned pages;
plugin-scoped cleanup leaves other plugins on a shared host running.

Installer tests cover stable/prerelease discovery, repeated selected-client
installation, checksum failures and rejected archive entries. Public catalog and
installed-plugin views are tables by default with explicit JSON output options.

## Public plugin catalog

The optional catalog contains `tjupt` and `ttyd`; neither is installed by default.
TJUPT ships only its manifest and page scripts, with no account data, NAS setup or
private consumer dependency. A user installs it with `bab plugin install tjupt`,
authorizes its site and logs in normally. ttyd keeps its separate optional client
and per-user deployment origin. Page-only plugin installs do not rebuild the
extension; browser component changes still require a Chrome reload.

## What users install

The release assets are executable programs, not a custom source export:

- `bab_VERSION_darwin_arm64.tar.gz`
- `bab_VERSION_darwin_amd64.tar.gz`
- `bab_VERSION_linux_arm64.tar.gz`
- `bab_VERSION_linux_amd64.tar.gz`
- `SHA256SUMS`

Each archive contains the independent `bab` and `bridge-ttyd` executables,
project license/notice files and the build toolchain's `GO_LICENSE`.
`install.sh` selects the platform, checks the checksum and installs only BAB by
default. `--with ttyd` selects the reference client; `--origin` can select its
plugin and deployment before the first Chrome load. Go, Node.js and Python are
not runtime prerequisites. The installer copies programs atomically and invokes
BAB's completion installer and setup guidance. It never grants a Chrome host
permission or logs a user into a website.

The installer selects a published stable release, falling back to a public
prerelease when no stable release exists. An unreleased VERSION change on main
does not affect installation. `--version` can pin an older published release. Until the first release
exists, build from source or use `--from-dir dist --version VERSION` after a local
release build. Installing local release assets does not upload anything.

GitHub already provides source downloads. Release automation builds installable
programs; GitHub provides the normal source downloads.

## Local checks

```sh
make check build smoke
make release
```

`make check` covers Go race tests, browser mocks, plugin lifecycle and scaffold
validation, source boundaries, and installer checksum/archive checks. `make smoke`
uses a temporary prefix and empty user registry without Chrome, host registration
or live accounts. `make release` cross-builds the supported platform assets.

An optional external operator policy may be used for local audits:

```sh
make check DENY_PATTERNS=/path/to/private-policy.txt
```

Never commit that private policy, real site catalogs, credentials or generated
installation directories. `internal/extension` is the generic source;
`extension/`, `bin/`, `dist/` and Python caches are ignored generated content.
Private plugins can live in `~/.bab/sources/` or another private checkout.

## Publishing a release

1. Review changes, ownership/contribution rights and MIT attribution.
2. Update `VERSION` and the release code together. Push the reviewed commit.
3. Push the matching `vVERSION` tag. This is the explicit publication trigger.
4. The release workflow reuses the macOS/Linux checks, builds the four binary
   assets with the current stable Go toolchain and creates the GitHub release only after checks pass. A suffix such
   as `-alpha.1` marks it as a prerelease. The installer continues to use a published version during development.
5. Verify the published installer, checksum and first-run Chrome flow. Configure
   private vulnerability reporting before advertising it as enabled.

Workflow Actions are pinned to official release commits, checkout credentials
are not persisted, and only the release job has `contents: write`. Ordinary
pushes run checks and do not create a release.

## Alpha validation limits

macOS page-script and ttyd browser paths were manually exercised on an existing
installation. Fresh isolated CLI installation, Go 1.24/1.26 tests, race checks and
Linux cross-compilation pass. GitHub CI runs these checks on Linux and macOS.
Binary publication and the hosted installer must also be verified for each release.
A wholly fresh Chrome profile and Linux Chrome/Native Messaging installation
have not been manually verified. Windows, binary-safe terminal output, durable
session recovery and automatic replay are not supported. No independent security
audit has been performed. See [SECURITY.md](SECURITY.md).
