# 开发指南

[English](development.md) | 简体中文 · [首页](../README.zh-CN.md)

## 构建与检查

源码构建需要 Go 1.24+ 和 `make`；运行检查还需要 Node.js 24+ 与 Python 3.9+。

```sh
git clone https://github.com/sky-bro/browser-auth-bridge.git
cd browser-auth-bridge
make install
export PATH="$HOME/.local/bin:$PATH"
bab setup
```

使用输出的扩展目录完成[首次 Chrome 注册](../README.zh-CN.md#安装)。

```sh
make check build smoke
```

`check` 运行 Go 竞态测试、浏览器模拟测试、Python 测试和源码审计。
`smoke` 使用临时安装目录和空插件目录，不打开 Chrome、不注册 Native Host，
也不访问真实账号。CI 覆盖 macOS 和 Linux；浏览器验收需要单独手动进行。

## 本地示例

[echo 示例](../examples/plugins/echo/README.zh-CN.md)在本机运行，无需真实账号。
启动与命令说明放在该插件目录中。

## SDK 与架构

Go 模块：`github.com/sky-bro/browser-auth-bridge`。
组件图、执行路径和包职责见[项目架构](architecture.zh-CN.md)，
接入契约见[插件开发规范（英文）](plugin-development.md)。

插件专属的安装、配置与命令说明放在其 `bab-plugin.json` 所在目录，
使用 `README.md` 和 `README.zh-CN.md`；通用指南只保留链接。

## 源码与发布

浏览器源码位于 `internal/extension`；`extension/`、`bin/`、`dist/`
是被 Git 忽略的生成目录。生成自定义开发扩展：

```sh
bab extension build --output ./extension
```

组件变化后重新加载同一个扩展。用户目录、私有插件、部署设置和凭据不应进入 Git。

`make release` 构建 macOS/Linux 的 arm64/amd64 程序及校验文件。
推送与版本一致的 `vVERSION` 标签会执行检查并发布 Release。
详见[发布流程（英文）](../PUBLICATION.md)与[安全说明（英文）](../SECURITY.md)。
