## 2026-06-20 - Task: Fix notification timezone and stale rate groups
### What was done
- Notification message timestamps now render in mainland China time.
- Successful rate refreshes now remove current group snapshots that no longer exist upstream, so deleted upstream groups stop appearing in the panel.
- Added focused tests for China-time formatting and stale group detection.

### Testing
- `git diff --check` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.
- Backend Go tests and `gofmt` could not be run locally because `go`, `gofmt`, `docker`, and WSL are not installed or not available on this machine.

### Notes
- `backend/internal/notify/templates.go`: formats notification timestamps with `Asia/Shanghai` and a UTC+8 fallback.
- `backend/internal/notify/templates_test.go`: verifies UTC input renders as China mainland time.
- `backend/internal/storage/rates.go`: adds current rate snapshot cleanup for groups missing from the latest upstream result.
- `backend/internal/storage/rates_test.go`: verifies stale group names are detected without deleting active groups.
- `backend/internal/monitor/service.go`: runs stale snapshot cleanup after a successful rate refresh.
- `docs/monitoring.md`: documents notification timezone and current rate snapshot cleanup behavior.
- Rollback: revert this change set or reset to the commit before this task.
