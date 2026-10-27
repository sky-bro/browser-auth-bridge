# Trust boundaries

BAB is an Alpha for trusted local tools and trusted plugins. It is not a sandbox
for hostile code and has not had an independent security audit.

## Who is trusted

A same-user local process is a trusted caller, not an authenticated executable.
Unix sockets use mode 0600. Random session IDs route messages; they do not prove
which binary is calling. Native Messaging registration limits Chrome callers to
one explicitly selected extension ID. Each consumer Native Host must validate
its attach payload and pin its intended site recipe or endpoint.

Browser page scripts can use the logged-in account and may have side effects.
`access: read/write` describes intent; neither that metadata nor `ctx.fetch`
enforces read-only access or confines arbitrary JavaScript. CLI delegates run
trusted local executables with the user's OS privileges. Review all plugin code,
including scripts and binary provenance, before installing it.

## Browser permissions and credentials

The extension requires Chrome's `debugger` permission for browser-v1 evaluation.
Chrome warns that it can read and change data on websites. This is broader than
an individual optional host grant. BAB separately checks granted hosts and exact
origins before routing requests, and rechecks the page origin before evaluation;
these application checks do not restrict malicious extension code.
See [Chrome's permission list](https://developer.chrome.com/docs/extensions/reference/permissions-list#debugger).

The manifest declares HTTPS hosts as optionally requestable; this does not grant
access to every HTTPS site. Each new host requires Chrome's user permission
gesture. Chrome host grants cover ports on that host; consumers must independently
pin their exact origin and endpoint. Consumers on the same host share the Chrome
grant, so revoking it can affect another consumer.

BAB does not request the `cookies` permission or require exporting browser
cookies. Requests and protocol handshakes can use browser-owned login. This does
not make arbitrary plugin results secret-free: page scripts, returned JSON,
terminal output and downloaded artifacts may contain tokens or other credentials.
Do not log or publish them. The ttyd reference adapter keeps its handshake token
inside the browser. Login expiry is separate from BAB transport availability.

HTTPS is required for provider origins except explicit loopback fixtures. The
echo fixture listens on 127.0.0.1 and does not expose shell execution.

## Plugin installation and packaged browser code

Installed manifests, configured origins, local catalogs and native wrappers are
trusted user configuration. The registry is checked by the BAB Native Host on
every attach. Only registered site metadata travels over its catalog messages;
remote pages cannot use those messages to install code or select native hosts.
The popup has no independent integration or origin input.

Ordinary browser-script plugins keep command scripts in the local CLI registry.
A plugin may also declare an `extension` component containing local background,
content or PTY adapter scripts. Installation copies only declared files and
assembles them into `plugins/<site>/` in the generated extension. That process
can install extension code from a trusted package; it does not execute it during
installation. Chrome loads it after the user reloads the same extension.
Packaged extension code is never supplied over Native Messaging. Ordinary
browser-v1 commands do carry trusted CLI-owned page script text for execution;
those commands are separate from installing extension components.

Packaged background components have extension privileges. File namespaces prevent
accidental asset overwrites, not malicious access to another plugin's data or
browser authority. Registered compatibility services require the installed
catalog and a host grant to activate. Removal stops those services, unregisters
their content scripts and rebuilds the standard extension. Reload after component
changes so removed background code is no longer loaded. Persisted managed
content registrations are pruned on reload.

A CLI delegate can declare its Native Host and an origin environment variable.
Setup registers available declared hosts without running the executable. Missing
executables are not downloaded. Installation grants no new Chrome host access and
runs no plugin hooks. Generated extension configuration contains native-host
identities and local deployment choices; keep it outside published source.

## Lifecycle and output limits

Ambiguous script or command outcomes are not automatically replayed. Close page
leases explicitly. BAB closes only its owned tabs. There is no durable session
recovery across browser/service-worker restarts and no automatic reconnect loop
for a missing or broken Native Host; the user can reconnect from the popup.

The terminal contract carries UTF-8 terminal text, not arbitrary binary stdout.
Consumer-side writers bound their own buffering, but this does not establish
browser WebSocket/Native Messaging receive-credit flow control. Review the
[ttyd reference limitations](plugins/builtin/ttyd/DEVELOPMENT.md#terminal-behavior-and-limits) for command cancellation,
timeouts and backpressure.

## Reporting a vulnerability

Do not post cookies, credentials, private site URLs or sensitive reproduction
artifacts in a public issue. Once a public repository is available, use GitHub's
private vulnerability reporting if it is enabled. If no private reporting channel
is configured, use a public issue only to request one, without exploit details
or sensitive data. Repository settings must enable private reporting before a
release advertises that channel as available.

## Binary installation

The official installer downloads release programs over HTTPS and verifies the
selected archive against the release's `SHA256SUMS` before replacing programs.
It rejects unexpected archive paths and nonregular entries. Checksums detect
corrupt or mismatched downloads; they are not an independent signature or an
audit of the publisher. Inspect the installer and trust the release source.
The installer can generate extension files and selected plugin configuration,
but Chrome registration and new host authorization still require user action.


## Plugin state

Enable/disable is local runtime policy, separate from Chrome authorization.
Disabled plugins retain their package and host grants, but BAB rejects their
operations and stops registered managed providers. State is updated atomically
under a same-user file lock. Enabling is not a new host permission grant.
Installed background code remains trusted and can remain loaded until extension
reload; disabling is not a security sandbox for hostile plugins. A local caller
with the same OS identity can modify its own registry and state by design.
