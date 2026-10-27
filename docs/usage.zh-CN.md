# 使用指南

[English](usage.md) | 简体中文 · [首页](../README.zh-CN.md)

## 安装与更新

[快速开始](../README.zh-CN.md#安装)会安装二进制程序并生成扩展，无需克隆源码。
若安装器提示 PATH 未配置，将 `~/.local/bin` 加入 PATH。
没有可用的二进制 Release 时，使用[源码安装](development.zh-CN.md#构建与检查)。

| 安装参数 | 用途 |
| --- | --- |
| `--version VERSION` | 指定已发布的版本 |
| `--prefix PATH` | 修改安装位置，默认为 `~/.local` |

如需传参，可先下载并检查 [install.sh](../install.sh)，再运行 `sh install.sh 参数`。
重新运行安装器即可更新 BAB；浏览器组件变化后，重新加载原来的 Chrome 扩展。
安装器优先选择已发布的稳定版，没有稳定版时选择预发布版。

## 管理插件

安装、启用、Chrome 授权和站点登录是独立步骤。
安装后默认启用，也可以用 `--disabled` 保持停用：

```sh
bab plugin available
bab plugin install SITE --disabled
bab plugin enable SITE
bab auth login SITE
bab plugin disable SITE
bab plugin update SITE
```

将 `SITE` 替换为插件名称。在 BAB 中点击授权，再到站点正常登录。停用会阻止命令执行并关闭托管会话，
保留插件、登录和站点授权；更新会保留启停状态。
自带插件从当前安装的 BAB 版本读取更新，需要新插件代码时先更新 BAB。

`bab plugin list` 显示已安装插件、执行方式、启停状态和程序可用性。
`list`、`available` 均支持 `--json`，供脚本使用。

同时移除插件和 Chrome 授权：

```sh
bab auth revoke SITE
bab plugin remove SITE
```

同一主机上的插件共享 Chrome 授权，包括不同端口；撤销授权可能影响其他插件。
只删除插件会保留授权。

## 插件文档

在[插件目录](../plugins/README.zh-CN.md)选择插件。
各插件的安装、配置和命令说明在其目录内维护。

## 本地与第三方插件

```sh
bab plugin init my-site --origin https://example.com
bab plugin install ~/.bab/sources/my-site
# 修改源码后：
bab plugin update ~/.bab/sources/my-site
```

为 `init` 添加 `--dir /path/to/plugin-repo` 可以指定其他源码目录。
已有插件可以从本地仓库或已初始化的 Git 子模块安装。
安装会复制快照，不会自动克隆仓库，也不会执行安装钩子。

`browser-script` 执行插件的页面脚本，`external-cli` 调用已注册的外部程序，
两者均可复用浏览器登录。普通页面脚本无需重载扩展；包含浏览器组件的插件，
安装、更新或移除后需重新加载原扩展。完整契约见[插件规范（英文）](plugin-development.md)。

## Zsh 补全

```sh
bab completion zsh --install --enable
```

之后打开新终端。补全只读取本地元数据，不会执行插件或发起浏览器请求。
需要手动配置时，`bab completion zsh` 可以输出补全脚本。

## 排查问题

| 检查 | 含义与处理 |
| --- | --- |
| `bab doctor` | 检查桥接连接；有问题时在弹窗中点击 Reconnect。 |
| `bab auth status SITE` | 检查站点授权；`login: unchecked` 不代表已验证登录。 |
| `bab SITE whoami` | 插件提供该命令时，可检查登录；过期后重新登录站点。 |
| `bab plugin list` | 检查插件是否启用、依赖程序是否可用。 |
| 浏览器组件有变化 | 在 `chrome://extensions` 重新加载原扩展。 |

BAB 程序移动位置后，重新执行 `bab setup --extension-id YOUR_ID`。
保持使用同一个共享扩展。信任与会话限制见[安全说明（英文）](../SECURITY.md)。
