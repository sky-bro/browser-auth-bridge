# ttyd development

[Usage](README.md) · [中文使用说明](README.zh-CN.md) · [BAB development](../../../docs/development.md)

The client in `cmd/bridge-ttyd` and its browser adapter own the ttyd protocol;
the BAB core handles grants, sessions and routing. Run commands below from the
BAB repository root. Build with `make build`; the executable is `bin/bridge-ttyd`.

For normal installation and per-user origin setup, use the [plugin guide](README.md).
The source example package is `examples/plugins/ttyd`; it may also live in an
independent checkout or initialized submodule.

## Alternative adapter assembly

For additional packaged adapters, use the trusted build-time configuration:

```sh
cp examples/ttyd/providers.example.json bin/providers.local.json
python3 scripts/build_extension.py --config bin/providers.local.json
```

This produces a shared extension at `bin/extension/`. It is an alternative build
for an installation, not a companion extension to install alongside the default.
Add all required consumers to the same config and reload that installation at the
same path. JSON declares provider identities, default origins, editable-origin
policy and explicit adapter asset mappings. Private JSON belongs outside version
control. Native hosts and adapter script paths cannot be entered in the popup.
Providers sharing a host share Chrome's grant across ports; routing and each
native host still enforce the exact configured origin and endpoint.

## Standalone CLI

After normal BAB plugin setup, the consumer can also run directly. Set the
same origin used by the installed plugin; BAB's registered Native Host launcher
supplies that origin to the browser-side process:

```sh
export BRIDGE_TTYD_ORIGIN=https://terminal.example.com
./bin/bridge-ttyd doctor
./bin/bridge-ttyd shell
./bin/bridge-ttyd exec -- printf 'hello\n'
./bin/bridge-ttyd exec -- bash -c 'exit 7'
```

For a standalone custom extension assembly without BAB host registration:

```sh
./bin/bridge-ttyd install-host YOUR_EXTENSION_ID https://terminal.example.com
```

Use that assembly's extension ID and configured origin. This stores the origin
in `~/.config/bridge-ttyd/config.json` and registers the absolute executable path.
Reload the custom extension afterward, log in normally, and re-register if the
binary moves. This direct registration replaces the ttyd Native Host manifest;
use BAB's generated launcher for a normal BAB-managed installation.

`doctor` checks local configuration, stty, the Chrome registration and broker
socket presence. It does not open a terminal, check browser login or prove that
a socket is healthy. A moved binary must be re-registered. `--version` reports
`dev` unless built with `make build VERSION=...`.

## Terminal behavior and limits

The CLI uses raw terminal mode and SIGWINCH resize on macOS/Linux (`stty` must
be available). `exec` requires a compatible shell and stty on the remote (tested
with Bash and zsh). It streams output immediately, retaining only a possible
completion-marker suffix; there is no cumulative 16 MiB output limit. Exit status
still arrives separately when the random completion record is received. Separate
commands open independent PTYs. No pooling, file copying, reconnect or command
replay is implemented.

Use `exec --timeout 30m -- COMMAND ...` to bound a run; `--timeout 0` (the default)
allows continuous output such as `tail -f`. The connection handshake retains its
20-second timeout. Ctrl-C cancels the whole exec and closes its PTY rather than
injecting an interrupt character into the remote shell. Cancellation exits 130,
timeout 124; neither guarantees termination of detached remote processes. Partial
output already delivered remains visible when a command fails or disconnects.

Sink writes are bounded to 32 KiB with at most one in flight and no output queue;
blocking sinks apply local backpressure and cancellation stops waiting on them.
An uninterruptible underlying write can persist until the sink recovers or the
CLI exits. Browser WebSocket/Native Messaging receive-credit flow control is not
implemented, so this does not establish bounded browser buffering for indefinitely
stalled consumers. Remote PTYs can apply CRLF conversion. Ctrl-C in shell
passes through to the remote terminal; interrupted exec closes its session.

The adapter supports ttyd 1.7.x: same-origin `/token`, `/ws`, `tty` subprotocol,
binary-prefixed terminal frames and UTF-8 streaming decoding. AuthToken stays
inside the browser adapter. This is not a claim that arbitrary binary stdout
is preserved: the current bridge stream is UTF-8 terminal text.

Consumer adapters are packaged scripts selected by trusted extension config,
never script text sent through a PTY payload. `PTYEndpoint` pins origin, page,
socket and adapter identity. The native host independently checks the complete
endpoint against its own configuration. The older JSON recipe remains supported.

## Live acceptance checks

After browser login and native host registration, run the opt-in live check:

```sh
python3 examples/ttyd/cli-acceptance.py
python3 examples/ttyd/stream-acceptance.py
```

It verifies the actual CLI/native-host/shared-extension/ttyd chain: exit status,
literal argument quoting, two independent concurrent sessions, interactive
shell startup, terminal resize, Ctrl-C and clean exit. It does not read any
credentials or browser storage. It is intentionally separate from `make check`.

The streaming check verifies output before command completion, 20 MiB of output,
remote exit status, timeout, and cancellation with a stalled stdout consumer.
