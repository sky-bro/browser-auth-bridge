# ttyd

[English](README.md) | 简体中文 · [BAB](../../../README.zh-CN.md)

通过自己的 ttyd 服务执行命令或打开 Shell，复用 Chrome 登录。
BAB 不负责部署服务。需要 macOS/Linux、`stty` 和兼容的远端 Shell；
适配器支持 ttyd 1.7.x。

## 安装

选择客户端并配置自己的部署地址：

```sh
curl -fsSL https://raw.githubusercontent.com/sky-bro/browser-auth-bridge/main/install.sh | sh -s -- --with ttyd --origin https://terminal.example.com
```

将 URL 替换为自己的服务来源地址。这会安装 `bridge-ttyd` 及 BAB 插件。
完成 [Chrome 扩展注册](../../../README.zh-CN.md#安装)；已有安装需重新加载原扩展，
以加载适配器。

如果从源码安装，改为在 BAB 仓库根目录运行：

```sh
make install-ttyd
bab plugin install ttyd --origin https://terminal.example.com
```

需要事先安装并配置 BAB，完成后重新加载原扩展。

## 授权与使用

```sh
bab auth login ttyd
# 在 BAB 中授权自己的主机，再到网页中正常登录。
bab ttyd exec -- printf 'hello\n'
bab ttyd exec --timeout 30m -- tail -f /path/to/log
bab ttyd shell
```

`exec` 默认不限制总运行时间，连接握手超时为 20 秒。
Ctrl-C 会取消 exec 并关闭 PTY；在 `shell` 中，Ctrl-C 传给远端终端。

输出为 UTF-8 终端文本，不适合传输任意二进制数据；命令不会自动重放。
关闭会话不保证已脱离终端的远端进程停止。
详细限制见[终端行为（英文）](DEVELOPMENT.md#terminal-behavior-and-limits)。

## 更新或修改地址

```sh
bab plugin update ttyd
# 切换部署地址：
bab plugin update ttyd --origin https://another-terminal.example.com
```

更新 BAB 并重新运行带 `--with ttyd` 的安装器，可以更新客户端程序。
插件更新默认保留地址；适配器变化后重新加载原扩展，新主机需要重新授权。

启停和移除见[插件管理](../../../docs/usage.zh-CN.md#管理插件)。
SDK 集成、适配器构建与实测见[开发说明（英文）](DEVELOPMENT.md)。
