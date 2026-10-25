# Independent ttyd consumer

This example depends only on the core SDK. Its Go CLI and browser adapter own
all ttyd knowledge; the core owns origin grants, sessions, tab cleanup and routing.
Service deployment belongs in the operator's infrastructure repository.

Build the CLI with `make build`. Install `bin/bridge-ttyd` on your PATH, run
`make install-bab` and `bab setup`, then reload the same shared extension.
The local BAB installation uses one registered command card per plugin.

Select the example and configure your own deployment origin:

```sh
make install-ttyd
bab plugin available
bab plugin install ttyd --origin https://terminal.example.com
bab extension build  # reload this same extension in Chrome
bab auth login ttyd
```

The popup shows `bab ttyd`, the configured host and an authorization button.
Authorize that site, sign in normally, then use `bab ttyd shell` or
`bab ttyd exec -- printf 'hello\n'`. The origin is supplied to both CLI and
Native Host through the installed plugin's `BRIDGE_TTYD_ORIGIN` environment
setting; changing the example does not edit a distributed server address.
`bab plugin update ttyd` preserves the origin. To switch deployments, pass
`--origin https://another-terminal.example.com` and authorize that new host.

You can also copy `examples/plugins/ttyd` into another repository or initialized
submodule and register its local directory/catalog. Installation never builds
or downloads the executable automatically. Keep using your existing shared
extension installation if one is already loaded.

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

Registration stores only the origin in `~/.config/bridge-ttyd/config.json` and
writes a Chrome native host manifest pointing to the absolute executable path.
Restart/disable-enable the extension after registration. Re-register if moved.
Log in normally at that origin, leaving an authenticated page open, then:

```sh
./bin/bridge-ttyd --help
./bin/bridge-ttyd doctor
./bin/bridge-ttyd shell
./bin/bridge-ttyd exec -- printf 'hello\n'
./bin/bridge-ttyd exec -- bash -c 'exit 7'
```

`doctor` checks local configuration, stty, the Chrome registration and broker
socket presence. It does not open a terminal, check browser login or prove that
a socket is healthy. A moved binary must be re-registered. `--version` reports
`dev` unless built with `make build VERSION=...`.

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
