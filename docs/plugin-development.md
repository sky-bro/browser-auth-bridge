# Plugin development (contract v1)

[Plugin directory](../plugins/README.md) · [Home](../README.md)

BAB is one CLI and Native Host for independently installed, trusted site plugins.
Adding commands never requires rebuilding BAB. Ordinary page scripts need no
extension rebuild; declared browser components require assembly and Chrome reload.
Browser plugins are page code with account authority on explicitly granted
origins; CLI delegates execute trusted native applications. Neither is sandboxed. `access: read/write` documents intent; it is not an
execution restriction. Installing never runs plugin code or dependency hooks.

## Create your own command

```sh
bab plugin init my-site --origin https://example.com
bab plugin install ~/.bab/sources/my-site
```

The default source directory is private user configuration, outside the BAB Git
repository. The starter's `status.js` reads page metadata; it does not claim the
account is logged in. Edit that script and `bab-plugin.json`, then explicitly
update the installed snapshot. Use `--dir /path/to/plugin-repo` to keep source in
another repository or initialized submodule. No BAB core changes are needed.

For a CLI delegate, add `--executable my-cli` and optionally
`--native-host org.example.client`; the CLI owns its BAB SDK integration and site
policy. The scaffold does not turn an arbitrary executable into a browser client.

## Install and authorize

Set up BAB once using the [installation guide](../README.md#install).
For package-specific commands, use the [plugin guides](../plugins/README.md).
A runnable local example lives with the [echo fixture](../examples/plugins/echo/README.md).

`auth login` opens the extension's site authorization page. Click **授权**, then the site-opening arrow to sign in. Chrome requires an actual user
gesture for permission grants; the CLI cannot silently approve a new domain.
`auth status` reports browser permissions, with `login: unchecked`. A site's
`whoami` command verifies its login separately; permission alone never proves
login. `doctor` checks transport availability, not website authentication.
Site grants cover Chrome's host scope across ports, while routing pins the exact
manifest origin. Revoking a shared host can affect another provider on that host.

Installed packages are copied into `~/.bab/plugins/SITE`. Source edits take
effect only after `bab plugin update PATH`. New subcommands are discovered from
the installed manifest each invocation. Install/update attempts to sync the
site catalog immediately. If Chrome is disconnected, local installation still
succeeds and prints a reconnect instruction. Reconnect loads the catalog from
the current Native Host. `bab plugin sync` explicitly refreshes it.
`bab plugin remove SITE` removes the package and browser leases. Chrome's host
grant is not automatically removed; run `bab auth revoke SITE` before removing,
or remove that grant in Chrome extension settings.

## Source catalogs

See the [plugin directory](../plugins/README.md) for bundled plugins and examples.

Plugins may live in this repository, another checked-out repository, or an
initialized Git submodule. Install an explicit local path directly, or register
a source catalog that resolves names to those directories:

```json
{
  "version": 1,
  "plugins": [
    {"name":"my-site","path":"./modules/my-site","description":"My site"}
  ]
}
```

```sh
bab plugin catalog add /path/to/catalog.json
bab plugin available
bab plugin install my-site
bab plugin install ./plugins/one /path/to/another/plugin
bab plugin catalog list
bab plugin catalog remove /path/to/catalog.json
```

Catalog paths are relative to the catalog file. An external entry may set
`originRequired: true` to require `--origin` at first installation. Missing or
uninitialized submodule sources are reported as unavailable; BAB never clones
repositories, downloads executables or runs installation hooks. Multiple
selected sources are validated before installation starts. `bab plugin install`
resolves bundled packages or local directories, without remote plugin version resolution.

Configured catalog paths are stored in `~/.bab/catalogs.json`; installed
plugins, selected origins and generated native wrappers also remain in the
user's BAB directory. Put organization-specific catalogs in user/private
configuration, not the public catalog, examples, tests, screenshots or submodule
URLs. A private plugin directory can be referenced locally without recording it
in the public repository. Public submodules may be listed in a public catalog;
review their contents and `.gitmodules` before publishing a plugin repository.

## List installed plugins

`bab plugin list` (also `bab list`) defaults to a compact table of command,
runtime, enabled state, local executable availability and description. This reads local
metadata only; it does not check browser login or remote connectivity.
Use `bab plugin list --json` or `bab list --json` in scripts for the full
manifest array. JSON values and fields are preserved, and an empty registry
returns `[]`. Human-readable descriptions are normalized to a single line.

## Author a plugin

Keep each plugin's installation, configuration and command reference in
`README.md` beside its `bab-plugin.json`; use `README.zh-CN.md` for Chinese.
BAB-wide guides should link to these files rather than duplicate them.

A minimal package contains two files:

```json
{
  "version": 1,
  "site": "example",
  "origin": "https://example.com",
  "description": "Example site",
  "commands": {
    "search": {
      "description": "Search the site",
      "access": "read",
      "script": "search.js",
      "positional": [{"name": "query", "required": true}],
      "options": [{"name": "limit", "default": "10"}]
    }
  }
}
```

`search.js` is an async function body, not an ES module:

```js
const response = await ctx.fetch('/api/search?q=' + encodeURIComponent(args.query));
if (!response.ok) return {error: {code: 'http_error'}};
const data = await response.json();
return data.items.map(item => ({id: item.id, title: item.title}));
```

`args` contains strings from declared positional/options fields. Unknown options
and missing required arguments fail before browser execution. `--json` is
accepted; results always use JSON. `bab SITE --help` and
`bab SITE COMMAND --help` describe the installed schema. The site namespace
is exclusive: extend a plugin by updating its package; collisions are rejected.

`ctx.fetch(path)` uses the current origin and browser login, rejects cross-origin
URLs and redirects, and has a 40-second timeout. It is a convenience wrapper,
not a network sandbox: trusted scripts execute in the page main world. Return
only projected business fields. Never export cookies, storage, authorization
headers, tokens, private tracker URLs or raw HTML containing credentials.
Use `{error:{code:'login_required'}}` to request normal browser login; unknown
adapter errors are reported without raw exception text. No command is retried
automatically, including commands labeled read. Timeout may leave an unknown
outcome; inspect before retrying.

For binary artifacts, set `"artifact": true` and return
`ctx.artifact(new Uint8Array(...))`. Invoke with `--output PATH`. BAB transfers
192 KiB chunks within the existing result limit and writes a new mode-0600 file;
existing files are never overwritten. Artifacts are bounded to 16 MiB. Failure
removes partial output, and the browser buffer expires after three minutes.
A consumer must validate the file format before using it. This supports torrent
metadata; media payloads should be downloaded by the NAS.

A browser-script plugin may declare any HTTPS origin, or explicit HTTP loopback origins for
fixtures. Packaged browser components follow the assembly and reload rules below;
PTY adapters additionally require a declared delegate Native Host. CLI delegates
may register their existing Native Host as described below. Adding a new
transport/runtime remains a core change. The installer supports distributed examples and registered local sources;
remote distribution/version resolution is not included.

## Register an existing CLI

A plugin can forward a complete namespace to an existing trusted executable:

```json
{"version":1,"site":"terminal","origin":"https://terminal.example.com",
 "description":"My terminal", "delegate":{"executable":"my-terminal-cli",
 "nativeHost":"org.example.terminal"},"commands":{}}
```

`bab terminal ARGS...` preserves argv, stdin/stdout/stderr and the exit status;
no shell is used. The executable must already be installed. `~/` is supported
for an explicit local executable path. A missing executable remains listed as
unavailable. Installation never executes the CLI or plugin hooks. `bab setup`
and installing a delegate after setup register its declared Native Host against
the selected extension. Browser scripts still use the one BAB Native Host.
The delegated CLI retains its own transport and origin validation. Its browser
protocol adapter must be supplied by its plugin or a trusted custom assembly;
registering an executable alone does not implement an unknown protocol.

Optional `origins` lists additional exact sites belonging to that command.
Each scope is displayed and authorized separately. The popup and command list
come from the installed plugin registry, with no independent Integration or
Site URL form. Origin configuration belongs in the plugin/CLI configuration.

A CLI delegate may declare `originEnv` (an uppercase name ending in `_ORIGIN`).
BAB sets it to the installed plugin origin for CLI and Native Host invocations.
The generated host wrapper launches the same registered delegate through BAB,
so Chrome's environment cannot silently select a different server. Installation
configuration contains no browser credentials. Updating a configured origin
changes routing, not its Chrome host grant; authorize the new host explicitly.

## Shell completion

```sh
bab completion zsh                         # print the script
bab completion zsh --install               # install only the script
bab completion zsh --install --enable      # install and enable for this user
```

`make install-bab` installs the script automatically; enabling the shell is an
explicit first-time step. Installed plugins and source catalogs are queried
locally whenever completion runs. Private plugin names are never embedded into
the distributed `_bab` script. Browser-plugin subcommands, options and choices
come from their existing manifest declarations. No adapter is executed during
completion. Updating an installed manifest takes effect on the next Tab.

CLI delegates can declare optional completion metadata in their manifest:

```json
"completion": {
  "commands": {
    "inspect": {
      "description": "Inspect local metadata",
      "options": [{"name":"format","choices":["short","full"]}]
    }
  }
}
```

Nested `commands`, `positional` arguments and `options` describe the delegated
syntax without running the executable. The bundled ttyd example provides its
own defaults; compatible older installed snapshots can use those bundled
metadata defaults too. Other delegated commands without metadata offer their
namespace and help, rather than probing a CLI that could have side effects.

The enable command installs one idempotent block in `$ZDOTDIR/.zshrc`, falling
back to `~/.zshrc`. It preserves existing content and permissions, follows a
startup-file symlink, and only initializes `compinit` if `compdef` is not already
defined. It does not source the user's startup file or reconfigure the current
terminal. Open a new zsh session after enabling. The generated code follows the
[zsh completion system](https://zsh.sourceforge.io/Doc/Release/Completion-System.html).


## Browser code belongs to the plugin

`browser-script` and `external-cli` in `bab plugin list` describe where commands
run. Both can use browser authentication. A CLI owns its business commands and
consumes the public SDK; BAB dispatches its executable without importing it.
Browser plugins own their page scripts and parsing. BAB does not know their
endpoints, DOM selectors or response schemas.

A plugin that needs packaged browser components can declare them:

```json
"extension": {
  "files": ["browser/adapter.js"],
  "ptyAdapter": "console",
  "ptyScripts": ["browser/adapter.js"]
}
```

This PTY example also requires `delegate.nativeHost`. All file names are relative
to the plugin package; BAB installs only declared regular files and packages them
under `plugins/<site>/`. A plugin cannot replace core assets or another plugin's
files. The directory can live in this repository, another checkout, or an
initialized submodule. No core edits or install hooks are required.

For an existing consumer with its own browser router, `extension.background` is
an ordered subset of `files`. Trusted background scripts can register grant-aware
services using `BrowserAuthBridge.registerService({id, origin, start, stop,
contentScript?})`. `origin` is a Chrome match pattern derived from one of the
plugin's exact registered origins. `contentScript` may declare `matches`, `js`
and `runAt`; scripts must belong to that plugin (or the shared stream executor).
Services stay stopped until the native registry confirms the plugin is installed
and Chrome grants its origin. Removal stops services and unregisters content
scripts; reload also prunes persisted registrations from removed packages.
Background code is trusted extension code, not a sandboxed third-party runtime.
Only install components whose code you trust.

`bab setup` and `bab extension build` assemble only installed plugin snapshots.
Install/update/remove automatically rebuild the standard `~/.bab/extension` if
it exists. **Reload the same extension in `chrome://extensions` after changing
browser components.** Chrome does not permit BAB to download and execute new
extension background code at runtime. A custom assembly can be rebuilt with
`bab extension build --output PATH`; BAB refuses to replace an unowned directory.
Changing ordinary browser-plugin command scripts needs no extension rebuild.

The core's default provider list is empty. The optional ttyd example owns its
protocol code under its plugin package; its selected deployment origin reaches
both CLI and browser through registration. No ttyd endpoint is built into the
core transport.


## Enable and disable independently of site authorization

`bab plugin enable SITE` and `bab plugin disable SITE` operate on installed
packages. Install with `--disabled` to keep a selected package inactive from the
start. State lives in the user's `disabled.json`, separate from package manifests;
updates preserve it and removal clears it. An invalid state file fails closed.

Disabled commands cannot run through BAB, including registered native-consumer
wrappers. The extension rejects disabled page operations and stops managed
provider sessions. Browser component code stays packaged until uninstall and
reload, so installed plugins must still be trusted. Standalone consumers in a
custom extension build remain the operator's responsibility.

No Chrome host grant or website login is revoked by disabling a plugin. Enabling
again reuses existing authorization. A shared host grant can affect multiple
plugins if explicitly revoked; plugin-scoped stopping does not revoke that grant.
Offline state changes are saved locally and applied when the extension reconnects.
CLI tables expose state; `list --json` includes `disabled` when true and
`available --json` exposes installed/enabled flags. Completion never runs plugins.
