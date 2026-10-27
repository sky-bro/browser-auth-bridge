# Echo 示例

[English](README.md) | 简体中文 · [BAB 开发指南](../../../docs/development.zh-CN.md)

返回本地页面信息和模拟身份，用于无需真实账号的 BAB 验证。
不会执行 Shell，也不检查站点登录。

先[安装 BAB](../../../docs/development.zh-CN.md#构建与检查)，再从 BAB 仓库根目录运行：

```sh
make build
./bin/bridge-demo serve
```

保持服务运行，在另一个终端执行：

```sh
bab plugin install ./examples/plugins/echo
bab auth login echo
# 在 BAB 中授权回环地址，再打开示例页面。
bab echo title
bab echo whoami
```

使用完毕后，按 Ctrl-C 停止服务并移除插件：

```sh
bab auth revoke echo
bab plugin remove echo
```
