# Browser Auth Bridge

Use your logged-in Chrome session from local tools. BAB provides a Go SDK,
a shared browser extension, and one CLI for independently installed plugins.
Browser scripts and existing CLIs can use the same browser-owned login without
a manual cookie export workflow.

BAB focuses on browser-authenticated requests and long-lived terminal streams.
Site endpoints, page parsing, terminal protocols and business workflows belong
to plugins. The core handles local transport, origin checks and session cleanup.

**Developer preview (Alpha).** Requires Chrome 105+ on macOS/Linux. macOS browser workflows have been exercised.
Linux support is implemented and included in CI, but a fresh Linux Chrome
installation has not been manually verified. Windows is unsupported.
See [validation and release status](PUBLICATION.md).

## Install

After the first GitHub binary release is published:

```sh
curl -fsSL https://raw.githubusercontent.com/sky-bro/browser-auth-bridge/main/install.sh | sh
```

The installer selects macOS/Linux and arm64/amd64, verifies the release checksum,
installs `bab` plus its completion file, and runs `bab setup`. No Go, Node.js or
Python is needed. It prints the extension directory and a PATH hint if needed.
You can download and inspect `install.sh` before running it.

1. Open `chrome://extensions`, enable Developer mode, and load the printed
   directory (normally `~/.bab/extension`).
2. Open its BAB popup, run the displayed `bab setup --extension-id YOUR_ID`
   command, then click **Reconnect**.
3. Run `bab doctor`. Bridge connectivity and website login are separate checks.

This one-time Chrome registration is shared by every plugin. New hosts still
need a real authorization gesture. Optional shell integration is enabled with
`bab completion zsh --install --enable`.

For a local source checkout before a binary release exists:

```sh
make install
export PATH="$HOME/.local/bin:$PATH"
bab setup
```

Source builds need Go 1.24+ and `make`. Checks also need Node.js 24+ and Python
3.9+. Update an installed CLI by rerunning the installer; reload the same Chrome
extension after browser component changes. `install.sh --version VERSION` pins
a release; `--prefix PATH` selects another installation prefix.

## Try a local example

This fixture returns local page metadata; it does not execute a shell. Its
`whoami` command returns a synthetic fixture identity, not an account login check.

```sh
make build
./bin/bridge-demo serve
```

Leave that server running. In another terminal, from this source directory:

```sh
bab plugin install ./examples/plugins/echo
bab auth login echo
# In the BAB page, authorize the loopback host, then open its page.
bab echo title
bab echo whoami
```

Stop the fixture with Ctrl-C. To remove it, revoke permission before removing
its registry entry:

```sh
bab auth revoke echo
bab plugin remove echo
```

## Use TJUPT

The public catalog also includes a TJUPT browser-script plugin for an existing
account. Install it by name; no CLI adapter, NAS or private source checkout is
required:

```sh
bab plugin install tjupt
bab auth login tjupt
# Authorize the host and sign in normally in Chrome.
bab tjupt whoami
bab tjupt search 'example title' --limit 10 --sort seeders
```

No extension reload is needed for these page scripts. The plugin also provides
release details and protected local torrent metadata output; see
[the TJUPT commands](plugins/builtin/tjupt/README.md). The package contains no
account credentials and does not start payload downloads.

## Use your own ttyd deployment

The optional ttyd reference client owns its browser adapter. The core has no
default providers.

```sh
curl -fsSL https://raw.githubusercontent.com/sky-bro/browser-auth-bridge/main/install.sh | sh -s -- --with ttyd --origin https://terminal.example.com
```

This selects the optional CLI and plugin before the first extension load. Finish
Chrome registration, authorize the host in BAB and sign in normally, then:

```sh
bab ttyd exec -- printf 'hello\n'
bab ttyd shell
```

On an existing installation, reload the same extension for the new adapter.
From source, use `make install-ttyd` and
`bab plugin install ttyd --origin https://terminal.example.com`.

BAB does not deploy a ttyd server. Its origin is chosen at installation and
passed to both CLI and Native Host. `bab plugin update ttyd` preserves that
choice. See [ttyd setup and limitations](examples/ttyd/README.md).

## Choose installed and enabled plugins

No plugin is installed automatically. Installation enables the selected plugin;
Chrome authorization and website login remain separate steps.

```sh
bab plugin available                   # optional plugin catalog
bab plugin install tjupt --disabled     # install without enabling it
bab plugin enable tjupt
bab auth login tjupt                    # authorize and log in once
bab plugin disable tjupt                # keep package, config and site grants
bab plugin enable tjupt
bab plugin update tjupt                 # preserves enabled/disabled state
bab plugin remove tjupt
```

`bab plugin list` shows execution placement, enabled state and executable
availability separately. Use `--json` with `list` or `available` for automation.
Disabling blocks the plugin's commands and stops its managed bridge sessions;
it does not revoke another plugin's shared host grant or log out of the website.
The popup shows disabled cards and can copy the CLI command to change state.
To revoke Chrome access as well, run `bab auth revoke SITE` before removing it.

## Add commands without changing BAB

Create a starter in your private user directory:

```sh
bab plugin init my-site --origin https://example.com
bab plugin install ~/.bab/sources/my-site
```

Edit `status.js` and the manifest, then run `bab plugin update` with that source
path. Use `--dir /path/to/your/plugin-repo` to keep it in a separate repository.
For an existing CLI, add `--executable my-cli` and its optional `--native-host`
to the scaffold command. The CLI must implement BAB SDK integration itself.

Install trusted local plugin directories or register catalogs from another
checkout or initialized Git submodule. Installation copies a snapshot; update
it explicitly after editing the source. No remote download, dependency install
or plugin hook is run.

```sh
bab plugin install /path/to/my-plugin
bab plugin update /path/to/my-plugin
bab plugin list           # human-readable table
bab plugin list --json    # full manifests for automation
```

`RUNTIME` describes execution placement: `browser-script` runs plugin-owned page
code, while `external-cli` invokes a registered executable. Both can use browser
login. The extension lists only registered plugins; each new host needs an
explicit authorization gesture.

Ordinary page-command scripts are CLI-owned and need no extension rebuild.
Plugins with an `extension` declaration package trusted browser components under
`plugins/<site>/`. Install/update/remove rebuilds the standard extension when it
exists; **reload Chrome's same extension after component changes**. Plugins do
not need to modify or rebuild BAB itself.

See the [plugin contract](plugins/README.md) for command schemas, CLI delegation,
completion metadata, source catalogs and browser component declarations.

## Zsh completion

`make install-bab` installs `_bab` in `PREFIX/share/zsh/site-functions`. Enable it
once and open a new shell:

```sh
bab completion zsh --install --enable
```

This preserves other startup configuration and reuses existing completion setup.
Completion reads local metadata; it never executes a plugin or browser request.
For manual packaging, `bab completion zsh` prints the script.

## SDK and architecture

| Layer | Responsibility |
| --- | --- |
| `bridge`, `nativehost` | Scoped contracts, Native Messaging, private Unix sockets |
| `internal/extension` | Origin checks, page leases, browser execution, stream routing |
| Plugin / consumer CLI | Site protocol, target policy, parsing and business actions |
| `commandstream`, `pool` | Optional consumer-side framing and worker reuse |
| Operator | Service deployment, credentials and chosen site origins |

`browser-v1` supports bounded `open` / `eval` / `close` workflows with a shared
page lease and JSON results. `pty-v1` carries full-duplex terminal sessions via
CLI-owned JSON recipes or consumer-packaged protocol adapters. Native Hosts must
validate their consumer's complete endpoint or recipe before browser use.

The Go module is `github.com/sky-bro/browser-auth-bridge`. The
[demo host](cmd/bridge-demo) and [ttyd consumer](cmd/bridge-ttyd) show SDK use.
The SDK and extension are independently buildable without sibling repositories.

## Trust and limits

Plugins and same-user local callers are trusted. Browser scripts can act as the
logged-in account; `access: read/write` is documentation, not enforced isolation.
The extension requires Chrome's broad `debugger` permission, with additional
BAB origin checks and optional host grants. These checks do not sandbox hostile
plugins. Results or downloaded artifacts can contain secrets even though BAB
does not require cookie export. Read [SECURITY.md](SECURITY.md) before installing.

There is no command replay after an ambiguous outcome or durable session
recovery. Terminal output is UTF-8 text, not a binary-safe pipe; browser receive
backpressure is not guaranteed. Account login can expire independently of a
healthy bridge. Use only providers, scripts and binaries you trust.

## Develop and release

```sh
make check build smoke
make release
```

The repository root is the public Go module. Browser source lives in
`internal/extension`. The root
`extension/` directory is an ignored generated development installation and can
be rebuilt from your installed registry with `bab extension build --output ./extension`.
Keep user catalogs, private plugins and deployment settings outside Git.

`make release` builds four installable archives (macOS/Linux, arm64/amd64), each
containing the two independent programs and license notices, plus `SHA256SUMS`. Only `bab` is
installed by default; `--with ttyd` selects the reference client. These are binary
installers, not custom source exports. GitHub provides ordinary source downloads.

Pushing a `v*` tag invokes the release workflow: it first runs checks, then builds
and publishes binaries. `VERSION` and the tag must match. The default installer
selects a published release, so an unreleased VERSION change on main does not
break installation.
No tag or release has been published by local builds. See
[release status and procedure](PUBLICATION.md).

`make smoke` uses a temporary prefix and registry; it never registers a Native
Host, opens Chrome or reads live accounts. Browser acceptance is a separate
manual check. CI covers macOS/Linux checks and this smoke flow.

Licensed under [MIT](LICENSE). Read [SECURITY.md](SECURITY.md) before installing
plugins; source scanning does not establish ownership or a full security audit.
