# Browser Auth Bridge

English | [简体中文](README.zh-CN.md)

Use your logged-in Chrome session from local tools without exporting cookies.
One CLI, one Chrome extension, and optional site plugins.

**Alpha · macOS / Linux · arm64 / amd64 · Chrome 105+**

## Install

Install a published GitHub release. No repository clone, Go, Node.js or Python is required:

```sh
curl -fsSL https://raw.githubusercontent.com/sky-bro/browser-auth-bridge/main/install.sh | sh
```

For source installation, or if no binary release is available, see
[Build and check](docs/development.md#build-and-check).

Register Chrome once:

1. Open `chrome://extensions`, enable **Developer mode**, and **Load unpacked**
   from the directory printed by the installer (normally `~/.bab/extension`).
2. Open the BAB popup, run its displayed `bab setup --extension-id YOUR_ID`
   command, then click **Reconnect**.
3. Run `bab doctor` to check connectivity.

## Use

Plugins are optional; none are installed by default.

```sh
bab plugin available
bab plugin list
```

Browse the [plugin directory](plugins/README.md) for setup and commands.
Add commands with `bab plugin init` without changing BAB itself.

Plugins can act as your logged-in account. Install only trusted code;
see [Security](SECURITY.md) for permissions and Alpha limitations.

## Documentation

| Guide | Contents |
| --- | --- |
| [Usage](docs/usage.md) | Plugin management, completion, updates and troubleshooting |
| [Plugin development](docs/plugin-development.md) | Scaffolding, manifests, external CLIs and catalogs |
| [Architecture](docs/architecture.md) | Components, plugin paths and trust boundaries |
| [Development](docs/development.md) | Source builds, SDK, architecture and local example |
| [Releases](PUBLICATION.md) | Binary assets and publication workflow |
| [Security](SECURITY.md) | Trust boundaries and known limitations |

[MIT License](LICENSE)
