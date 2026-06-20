# Monitoring Behavior

- Notification timestamps are formatted with the `Asia/Shanghai` timezone. If the timezone database is unavailable, the service falls back to a fixed UTC+8 location.
- Rate snapshots represent the current upstream group list for each channel. After a successful rate refresh, groups that are no longer returned by the upstream are removed from the current snapshot table. Historical rate-change logs are preserved.
