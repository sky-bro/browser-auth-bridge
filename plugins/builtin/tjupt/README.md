# TJUPT

English | [简体中文](README.zh-CN.md) · [BAB](../../../README.md)

Search releases and fetch torrent metadata using an existing TJUPT account.
Requires a [configured BAB extension](../../../README.md#install).

## Install

```sh
bab plugin install tjupt
bab auth login tjupt
```

Authorize `tjupt.org` in BAB and sign in normally in Chrome.
No separate CLI or source checkout is required.

## Commands

```sh
bab tjupt whoami
bab tjupt search 'example title' --sort seeders --limit 10
bab tjupt details RELEASE_ID
bab tjupt torrent RELEASE_ID --output ./release.torrent
```

Replace `RELEASE_ID` with an ID from search results. `whoami` checks website login;
Chrome authorization alone does not prove login.

The torrent command saves metadata with mode `0600` and refuses an existing
output file. It does not start a payload download. Metadata can contain private
tracker credentials; do not share it, log it or commit it.

## Update

```sh
bab plugin update tjupt
```

Update BAB first to obtain newer bundled plugin code. These page scripts need
no extension rebuild or reload. For enable/disable and removal, see
[plugin management](../../../docs/usage.md#manage-plugins).
