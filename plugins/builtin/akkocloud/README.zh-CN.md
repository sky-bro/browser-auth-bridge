# AkkoCloud 流量查询

[English](README.md) · [插件目录](../../README.zh-CN.md)

通过 Chrome 已登录会话读取 AkkoCloud 的服务及流量统计。插件只 GET 服务列表和
所选产品详情页，不执行 VPS 管理脚本，不提交表单，不访问 SSO 链接，也不执行
续费、开关机、重启、重置等操作。

## 安装与登录

```sh
bab plugin install akkocloud
bab auth login akkocloud
```

在扩展页点击 **授权**，再点击 **打开登录页**，正常登录。授权域名是
`https://www.akkocloud.com`，无需导出密码或 Cookie。尚未发布的源码版本可使用
`bab plugin install ./plugins/builtin/akkocloud` 安装。

```sh
bab akkocloud whoami --json
bab akkocloud traffic --json
bab akkocloud traffic --service-id 123 --json
```

`whoami` 只返回是否登录，不返回账号或邮箱。站点已授权不等于已经登录。
指定服务 ID 时，该 ID 必须先出现在当前用户的服务列表中，不会探测任意 ID。

## 结果与统计口径

完整 JSON 示例见 [English 文档](README.md#json-contract)。稳定的顶层字段为
`schema_version: 1`、`provider: "akkocloud"`、ISO8601 `fetched_at` 和 `services`。
每个服务有 `id`、`name`、本地化 `status` 和 `traffic`：

- `used_bytes`、`limit_bytes`、`remaining_bytes`：根据页面明确单位归一化的合计流量。
- `upload_bytes`、`download_bytes`：未验证的上下行拆分保持 `null`。
- `reset_at`、`period_start`：只有页面明确给出流量日期及其时区才转为 ISO8601；
  付款到期日绝不充当流量重置日。缺失字段保持 `null`。
- `raw_used`、`raw_limit`、`raw_remaining`、`raw_unit`：保留页面显示值和单位。
- `precision: "display-rounded"`：页面有舍入和更新延迟，不能冒充精确字节 API。
- `raw_reset`、`raw_period_start`：明确的流量日期没有时区时，原值保留，时间戳留空。

明确的 `GB` 按 SI（1000³）、`GiB` 按 IEC（1024³）换算；裸 `G` 不推断。服务商
内部可能使用不同于显示标签的计数方式，因此额度告警优先使用服务商显示的
已用/总量比例，对账时保留原始单位。

服务存在但流量无法读取时，`warnings` 为 `["traffic_unavailable"]`。任何 `null`
都必须显示为未知，不能当成零流量。登录失效、页面变化、无效 ID 或 HTTP 失败
返回受限错误码，不返回页面 HTML、隐藏字段、凭据或原始异常。

监控应在拥有已授权 Chrome 的机器上运行命令，只把上述业务 JSON 传给 NAS，
不要复制浏览器会话。`fetched_at` 不再更新时标记数据过期。插件本身不安装定时
任务，也不发送通知。

## 更新

```sh
bab plugin update akkocloud
```

未发布源码版本使用明确的插件路径更新。普通页面脚本更新不需要重建或重载扩展。
