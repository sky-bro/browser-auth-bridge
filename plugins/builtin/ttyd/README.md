# ttyd

English | [简体中文](README.zh-CN.md) · [BAB](../../../README.md)

Run commands or open a shell through your own ttyd deployment using Chrome login.
BAB does not deploy the server. Requires macOS/Linux, `stty`, and a compatible
remote shell. The adapter supports ttyd 1.7.x.

## Install

Select the client and configure your deployment URL:

```sh
curl -fsSL https://raw.githubusercontent.com/sky-bro/browser-auth-bridge/main/install.sh | sh -s -- --with ttyd --origin https://terminal.example.com
```

Replace the URL with your own origin. This installs `bridge-ttyd` and its BAB
plugin. Complete the [Chrome registration](../../../README.md#install).
For an existing installation, reload the same extension to load the adapter.

If building from source, run these from the BAB repository root instead:

```sh
make install-ttyd
bab plugin install ttyd --origin https://terminal.example.com
```

BAB must already be installed and configured. Reload the same extension afterward.

## Authorize and use

```sh
bab auth login ttyd
# Authorize your host in BAB and sign in normally on the website.
bab ttyd exec -- printf 'hello\n'
bab ttyd exec --timeout 30m -- tail -f /path/to/log
bab ttyd shell
```

`exec` has no overall timeout unless specified; the connection handshake has a
20-second timeout. Ctrl-C cancels exec and closes its PTY. In `shell`, Ctrl-C
passes through to the remote terminal.

Output is UTF-8 terminal text, not a binary-safe pipe. Commands are not replayed
automatically. Closing a session does not guarantee detached remote processes
stop. See [terminal behavior and limits](DEVELOPMENT.md#terminal-behavior-and-limits).

## Update or change origin

```sh
bab plugin update ttyd
# To change deployments:
bab plugin update ttyd --origin https://another-terminal.example.com
```

Update BAB and rerun the installer with `--with ttyd` to update the client binary.
Plugin updates preserve the origin unless explicitly changed. After adapter
changes, reload the same extension; authorize a new host before using it.

For enable/disable and removal, see [plugin management](../../../docs/usage.md#manage-plugins).
For SDK integration, adapter assembly and live checks, see [Development](DEVELOPMENT.md).
