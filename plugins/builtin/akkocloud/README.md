# AkkoCloud traffic

[简体中文](README.zh-CN.md) · [Plugin directory](../../README.md)

Read AkkoCloud service traffic with your existing Chrome login. The adapter only
performs same-origin GETs to the service list and selected product detail pages.
It does not execute the VPS control scripts, submit forms, follow SSO links, or
renew, reset, restart, or otherwise change a service.

## Install and authorize

```sh
bab plugin install akkocloud
bab auth login akkocloud
```

Click **Authorize site** in the extension page, then **Open login page**, and
sign in normally. The scope is `https://www.akkocloud.com`. No password or Cookie
export is required. For a source checkout before a release containing this
plugin, use `bab plugin install ./plugins/builtin/akkocloud` instead.

```sh
bab akkocloud whoami --json
bab akkocloud traffic --json
bab akkocloud traffic --service-id 123 --json
```

The optional numeric service ID must appear in the logged-in service list. The
adapter never probes arbitrary service IDs. `whoami` returns only `logged_in`,
not an account name or address. BAB permissions alone do not establish login.

## JSON contract

`traffic` returns:

```json
{
  "schema_version": 1,
  "provider": "akkocloud",
  "fetched_at": "2026-11-07T10:00:00.000Z",
  "services": [{
    "id": "123",
    "name": "Example VPS",
    "status": "Active",
    "traffic": {
      "used_bytes": 125000000000,
      "limit_bytes": 500000000000,
      "remaining_bytes": 375000000000,
      "upload_bytes": null,
      "download_bytes": null,
      "reset_at": null,
      "period_start": null,
      "direction": "both",
      "raw_unit": "GB",
      "raw_used": "125 GB",
      "raw_limit": "500 GB",
      "raw_remaining": "375 GB",
      "raw_reset": null,
      "raw_period_start": null,
      "precision": "display-rounded",
      "unit_basis": "label-SI-or-IEC"
    },
    "warnings": []
  }]
}
```

This is a snapshot of the provider's **displayed combined traffic**, not an
exact byte-level API. Values retain the page's rounding and refresh delay.
Explicit `GB` is normalized using SI (1000³); `GiB` uses IEC (1024³). The adapter
preserves the original labels; it does not assume that an unlabeled `G` means
either. A provider may internally use a different convention despite its label,
so use the provider ratio for quota alarms and keep raw values for comparisons.

Upload/download breakdowns remain null unless the adapter has verified separate
fields. Reset dates and period starts remain null when absent. Billing due dates
are **never** used as traffic reset dates. Even an explicit traffic date remains
in `raw_reset`/`raw_period_start` without an invented timestamp if its timezone is
unknown. Service status is the provider's localized label, not a connectivity
probe.

`warnings: ["traffic_unavailable"]` means a service exists but its traffic fields
could not be read. Null must stay unknown; never coerce it to zero. Login expiry,
unexpected pages, invalid IDs, and HTTP errors produce bounded error codes without
returning HTML, hidden fields, credentials, or raw exception messages.

For monitoring, run this command on the machine with authorized, connected Chrome
and pass **only this projected JSON** to the monitoring server. Do not copy the
browser session to the NAS. Mark the snapshot stale when `fetched_at` stops
advancing. The command does not create a scheduler or configure notifications.

## Update

```sh
bab plugin update akkocloud
```

For an unreleased checkout, update its explicit plugin path instead. Ordinary
page-script updates need no Chrome extension rebuild or reload.
