# Usage

English | [简体中文](usage.zh-CN.md) · [Home](../README.md)

## Installation and updates

The [quick start](../README.md#install) installs release binaries and generates
the extension. No source checkout is needed. Add `~/.local/bin` to `PATH` if the
installer asks you to. See [source installation](development.md#build-and-check)
if no binary release is available.

| Installer option | Purpose |
| --- | --- |
| `--version VERSION` | Pin a published release |
| `--prefix PATH` | Change the install prefix (default: `~/.local`) |

To pass options, download and inspect [install.sh](../install.sh), then run
`sh install.sh OPTIONS`. Rerun the installer to update BAB; reload the same Chrome
extension after browser component changes. It selects a published stable release,
falling back to a prerelease if no stable release exists.

## Manage plugins

Installation, plugin enablement, Chrome authorization and website login are
separate steps. Installation enables a plugin unless `--disabled` is supplied.

```sh
bab plugin available
bab plugin install SITE --disabled
bab plugin enable SITE
bab auth login SITE
bab plugin disable SITE
bab plugin update SITE
```

Replace `SITE` with the plugin name. Click the authorization button in BAB and sign in normally on the website.
Disabling blocks commands and stops managed sessions while keeping the package,
login and host grant. Updating preserves the enabled state. Bundled plugin updates
use the installed BAB version; update BAB first to obtain newer plugin code.

`bab plugin list` shows installed plugins, runtime, enabled state and executable
availability. Both `list` and `available` accept `--json` for automation.

To remove a plugin and its Chrome grant:

```sh
bab auth revoke SITE
bab plugin remove SITE
```

Host grants are shared across plugins on the same host, including its ports.
Revoking a grant can affect those plugins. Removing a package alone keeps the grant.

## Plugin guides

Choose a plugin from the [plugin directory](../plugins/README.md).
Its setup and commands live in the plugin's own documentation.

## Local and third-party plugins

```sh
bab plugin init my-site --origin https://example.com
bab plugin install ~/.bab/sources/my-site
# After editing the source:
bab plugin update ~/.bab/sources/my-site
```

Use `--dir /path/to/plugin-repo` with `init` to choose another source directory.
Existing plugins can be installed from local checkouts or initialized Git submodules.
Installation copies a snapshot; it does not clone repositories or run install hooks.

`browser-script` runs plugin-owned page code; `external-cli` invokes a registered
executable. Both can use browser login. Page scripts need no extension reload;
plugins with packaged browser components require reloading the same extension
after install, update or removal. See the [plugin contract](plugin-development.md).

## Zsh completion

```sh
bab completion zsh --install --enable
```

Open a new shell afterward. Completion reads local metadata without executing
plugins or browser requests. `bab completion zsh` prints the script for manual setup.

## Troubleshooting

| Check | Meaning / next step |
| --- | --- |
| `bab doctor` | Checks bridge connectivity. Reconnect from the popup if needed. |
| `bab auth status SITE` | Checks host authorization; `login: unchecked` does not verify website login. |
| `bab SITE whoami` | Checks login when the plugin provides this command. Sign in again if expired. |
| `bab plugin list` | Check that the plugin is enabled and its executable is available. |
| Browser component changes | Reload the same extension in `chrome://extensions`. |

If the BAB binary moves, rerun `bab setup --extension-id YOUR_ID`. Keep using
one shared extension. See [Security](../SECURITY.md) for trust and lifecycle limits.
