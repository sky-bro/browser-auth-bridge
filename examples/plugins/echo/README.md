# Echo fixture

English | [简体中文](README.zh-CN.md) · [BAB development](../../../docs/development.md)

Local page metadata and a synthetic identity for testing BAB without a real
account. It does not execute a shell or verify website login.

From the BAB repository root, after [installing BAB](../../../docs/development.md#build-and-check):

```sh
make build
./bin/bridge-demo serve
```

Keep the server running. In another terminal:

```sh
bab plugin install ./examples/plugins/echo
bab auth login echo
# Authorize the loopback host in BAB and open the fixture page.
bab echo title
bab echo whoami
```

When done, stop the server with Ctrl-C and remove the plugin:

```sh
bab auth revoke echo
bab plugin remove echo
```
