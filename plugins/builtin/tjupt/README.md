# TJUPT

Optional browser-script plugin for an existing TJUPT account. No account,
password, cookie or tracker credential is included. BAB does not create an
account or bypass website login.

```sh
bab plugin install tjupt
bab auth login tjupt
# Authorize tjupt.org in BAB and sign in normally in Chrome.
bab tjupt whoami
bab tjupt search 'example title' --sort seeders --limit 10
bab tjupt details RELEASE_ID
bab tjupt torrent RELEASE_ID --output ./release.torrent
```

The plugin owns search parsing, release details, identity checks and torrent
metadata fetching. It has no NAS, download-client, private CLI or deployment
dependency. The torrent command writes a local mode-0600 artifact; it does not
start a payload download. Torrent metadata may contain account-specific tracker
credentials: do not log it, share it or commit it. `--output` refuses an existing
file. Use your account's normal site access and permissions.

Installation and updates of these page scripts need no extension rebuild or
reload. A new host still requires Chrome's explicit authorization gesture.
`bab plugin update tjupt` refreshes the bundled source from the installed BAB
version. Update BAB first to obtain newer bundled plugin code.
