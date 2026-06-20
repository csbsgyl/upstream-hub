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

## 2026-06-20 - Task: Add live web update progress
### What was done
- Web one-click updates now expose a status endpoint that reads the update log and reports current phase, progress percentage, terminal state, and recent log lines.
- The deployment script emits stable update stage markers, and the settings page now polls them every 2 seconds while an update is active.
- The update panel now shows phase steps, animated progress, the current log tail, and unlocks retry after failed or unknown states.

### Testing
- `git diff --check` passed.
- `D:\rj-gj\Git\bin\bash.exe -n scripts/deploy.sh` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.
- Backend Go tests and `gofmt` could not be run locally because `go` and `gofmt` are not installed or not available on this machine.

### Notes
- backend/internal/api/ops.go: registers the update status endpoint.
- backend/internal/api/ops_update.go: adds update log parsing, status response generation, and updater stage markers.
- backend/internal/api/validation_test.go: verifies update status parsing and docker updater marker injection.
- frontend/lib/api-types.ts: adds the update status response type.
- frontend/app/settings-page.tsx: replaces the background-only update panel with live progress polling, stage UI, and log tail display.
- scripts/deploy.sh: emits machine-readable stage markers during deployment.
- docs/monitoring.md: documents live web update progress behavior.
- progress.md: records this task and verification status.
- Rollback: revert this change set or reset to the commit before this task.

## 2026-06-20 - Task: Add settings return navigation
### What was done
- The top-left logo/title now links back to the dashboard homepage.
- The settings page header now includes a visible return-home button.

### Testing
- `git diff --check` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.

### Notes
- frontend/components/monitor/monitor-header.tsx: makes the logo/title area a dashboard link.
- frontend/app/settings-page.tsx: adds a return-home button beside the operations title.
- progress.md: records this navigation fix.
- Rollback: revert this change set or reset to the commit before this task.

## 2026-06-20 - Task: Optimize alert notification copy
### What was done
- Balance, failure, and rate-change alerts now use clear icons and concise business-focused text.
- Alert bodies now keep only the affected upstream, key values, and China-time timestamp.
- Removed generic advice blocks from real alert notifications to reduce noisy pushes.

### Testing
- `git diff --check` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.
- Backend Go tests and `gofmt` could not be run locally because `go` and `gofmt` are not installed or not available on this machine.

### Notes
- backend/internal/notify/templates.go: simplifies balance and failure alert subjects/bodies with icons and key fields only.
- backend/internal/notify/policy.go: simplifies rate-change alert subjects/bodies, adds direction icons, and removes generic rate-change advice text.
- backend/internal/notify/templates_test.go: updates balance/failure alert expectations and verifies noisy advice text is absent.
- backend/internal/notify/policy_test.go: updates rate-change alert expectations and verifies noisy advice text is absent.
- docs/monitoring.md: documents the concise alert notification format.
- progress.md: records this alert-copy optimization and verification status.
- Rollback: revert this change set or reset to the commit before this task.

## 2026-06-20 - Task: Show channel origin site links
### What was done
- Channel cards now show the configured origin site beside the channel name and type.
- The display hides the http/https protocol while keeping the domain, port, and meaningful path.
- The site label opens the original upstream site in a new tab.

### Testing
- `git diff --check` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.

### Notes
- frontend/components/monitor/channel-cards.tsx: adds a compact external site link to each channel card header.
- frontend/lib/format.ts: adds a site URL display formatter that strips the protocol from labels.
- progress.md: records this UI improvement and verification status.
- Rollback: revert this change set or reset to the commit before this task.

## 2026-06-20 - Task: Improve channel site link color
### What was done
- The channel origin site link now uses a brighter cyan badge style so it reads as a clickable site entry.
- Added hover color and subtle focus-like shadow feedback without changing the channel card layout.

### Testing
- `git diff --check` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.

### Notes
- frontend/components/monitor/channel-cards.tsx: updates the origin site link badge color and hover treatment.
- progress.md: records this visual polish and verification status.
- Rollback: revert this change set or reset to the commit before this task.

## 2026-06-20 - Task: Restore dashboard update reminders
### What was done
- The dashboard header now checks the full version check result and shows a persistent update entry when a newer GitHub commit exists.
- The update toast now suppresses duplicate reminders only for the current browser session instead of permanently.
- Documented the dashboard update reminder behavior.

### Testing
- `git diff --check` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.

### Notes
- frontend/components/monitor/monitor-header.tsx: shows a persistent update shortcut when `has_update` is true.
- frontend/components/update/version-toast.tsx: changes update reminder suppression from local storage to session storage.
- docs/monitoring.md: documents dashboard update reminder behavior.
- progress.md: records this update reminder fix and verification status.
- Rollback: revert this change set or reset to the commit before this task.

## 2026-06-20 - Task: Hide channel origin labels until hover
### What was done
- Channel cards now keep origin site text hidden by default to avoid exposing upstream addresses in screenshots.
- The origin site entry remains clickable and expands when the card is hovered or focused.
- Documented the privacy-oriented channel site label behavior.

### Testing
- `git diff --check` passed.
- `npm.cmd run lint` passed in `frontend`.
- `npm.cmd run build` passed in `frontend`.

### Notes
- frontend/components/monitor/channel-cards.tsx: changes origin site labels to reveal only on hover or focus while keeping the external link available.
- docs/monitoring.md: documents the screenshot-safe default behavior for channel origin labels.
- progress.md: records this privacy-oriented UI adjustment.
- Rollback: revert this change set or reset to the commit before this task.
