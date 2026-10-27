# Browser Auth Bridge

[English](README.md) | 简体中文

让本地工具复用 Chrome 中已登录的会话，无需导出 Cookie。
一个 CLI、一个浏览器扩展，站点功能通过插件按需安装。

**Alpha · macOS / Linux · arm64 / amd64 · Chrome 105+**

## 安装

从已发布的 GitHub Release 安装，无需克隆仓库，也无需 Go、Node.js 或 Python：

```sh
curl -fsSL https://raw.githubusercontent.com/sky-bro/browser-auth-bridge/main/install.sh | sh
```

源码安装或暂无二进制 Release 时，见[构建与检查](docs/development.zh-CN.md#构建与检查)。

首次使用需要注册 Chrome 扩展：

1. 打开 `chrome://extensions`，开启**开发者模式**，点击**加载已解压的扩展程序**，
   选择安装器输出的目录（通常为 `~/.bab/extension`）。
2. 打开 BAB 弹窗，执行其中显示的 `bab setup --extension-id YOUR_ID`，
   然后点击 **Reconnect**。
3. 运行 `bab doctor` 检查连接。

## 使用

默认不安装任何插件：

```sh
bab plugin available
bab plugin list
```

在[插件目录](plugins/README.zh-CN.md)选择插件，查看安装配置与命令。
使用 `bab plugin init` 扩展新命令，无需修改 BAB 本身。

插件可以使用你的登录身份操作站点，请只安装可信代码。
权限和 Alpha 阶段的限制见[安全说明（英文）](SECURITY.md)。

## 文档

| 文档 | 内容 |
| --- | --- |
| [使用指南](docs/usage.zh-CN.md) | 插件管理、补全、更新与排查 |
| [插件规范（英文）](docs/plugin-development.md) | 创建插件、manifest、外部 CLI 与目录注册 |
| [项目架构](docs/architecture.zh-CN.md) | 组件职责、插件执行路径与信任边界 |
| [开发指南](docs/development.zh-CN.md) | 源码构建、SDK、架构与本地示例 |
| [发布流程（英文）](PUBLICATION.md) | 二进制产物与发布流程 |
| [安全说明（英文）](SECURITY.md) | 信任边界与已知限制 |

[MIT 许可证](LICENSE)
