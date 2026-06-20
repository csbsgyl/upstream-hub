# Monitoring Behavior

- Notification timestamps are formatted with the `Asia/Shanghai` timezone. If the timezone database is unavailable, the service falls back to a fixed UTC+8 location.
- Rate snapshots represent the current upstream group list for each channel. After a successful rate refresh, groups that are no longer returned by the upstream are removed from the current snapshot table. Historical rate-change logs are preserved.
- Web updates expose live progress through `/api/ops/update/status`. The endpoint reads the current update log, maps deploy markers to phases, and returns the latest log tail for the settings page.
- Alert notifications keep the payload concise: icon/title, affected upstream, key values, and China-time timestamp. Generic advice text is omitted to reduce notification noise.
- When a newer GitHub commit is available, the dashboard header shows a persistent update entry and the browser session shows one bottom-right reminder.
- Channel origin site labels are hidden by default on dashboard cards and expand only on card hover or keyboard focus to reduce accidental screenshot exposure.
- Version checks treat unknown local build commits as updateable when GitHub returns a known latest commit, so older deployments can still discover the first update.
- Rate-change notifications show only changed rate fields; unchanged completion ratios are omitted to avoid confusing 0 -> 0 entries, and zero-baseline changes are labeled as "原值为 0".
