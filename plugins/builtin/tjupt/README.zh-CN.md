# TJUPT

[English](README.md) | 简体中文 · [BAB](../../../README.zh-CN.md)

使用已有 TJUPT 账号搜索资源、查询详情并获取种子元数据。
使用前先完成 [BAB 扩展注册](../../../README.zh-CN.md#安装)。

## 安装

```sh
bab plugin install tjupt
bab auth login tjupt
```

在 BAB 中授权 `tjupt.org`，再到 Chrome 中正常登录。
无需额外 CLI，也无需克隆插件源码。

## 命令

```sh
bab tjupt whoami
bab tjupt search '示例片名' --sort seeders --limit 10
bab tjupt details RELEASE_ID
bab tjupt torrent RELEASE_ID --output ./release.torrent
```

将 `RELEASE_ID` 替换为搜索结果中的 ID。`whoami` 检查站点登录状态；
Chrome 授权成功不代表已经登录。

种子命令以 `0600` 权限保存元数据，不覆盖已有文件，也不启动资源下载。
元数据可能包含账号的私有 tracker 凭据，请勿分享、记录到日志或提交到 Git。

## 更新

```sh
bab plugin update tjupt
```

先更新 BAB，才能获取更新的自带插件代码。页面脚本无需重建或重载扩展。
启停和移除见[插件管理](../../../docs/usage.zh-CN.md#管理插件)。
