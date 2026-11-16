# Migrating from v3 to v4

v4 is a breaking release because two signatures change. Snapshot `Restore` now tells you whether the restore is still running, and `Metrics` takes a range. Everything else is added on top: two new operations, for 69 in all, new fields and new error codes. The guide from v2 to v3 is in the [v3.0.0 tag](https://github.com/squarecloudofc/sdk-api-go/blob/v3.0.0/MIGRATION.md).

## At a glance

| | v3 | v4 |
|---|---|---|
| Module | `github.com/squarecloudofc/sdk-api-go/v3` | `github.com/squarecloudofc/sdk-api-go/v4` |
| Operations | 67 | 69, adding `Apps.Snapshots.RestoreState` and `Databases.Snapshots.RestoreState` |
| Snapshot restore | `Restore(ctx, id, name, versionID) error`. A restore that was still running after about 90 seconds returned `nil`, as if it had finished. | `Restore(ctx, id, name, versionID) (SnapshotRestored, error)`. `Pending` is `true` while the restore keeps running, and `RestoreState` follows it. |
| Metrics | `Metrics(ctx, id)`, for the last 24 hours only | `Metrics(ctx, id, rng)`, where `rng` is `""` or `"24h"`, or `"7d"` for hourly points with peaks |
| `RESTORE_IN_PROGRESS` | HTTP 403 | HTTP 409 |
| `CodeMetricsNotSupported` | sent by the API | deprecated, because the API no longer sends it |
| Go version and dependencies | Go 1.22, no dependencies | unchanged |

## Construction and options

Only the import path changes. `New` and every `Option` keep their signatures and defaults, and the default `User-Agent` becomes `squarecloud-sdk-go/4.0.0`.

```diff
-import "github.com/squarecloudofc/sdk-api-go/v3"
+import "github.com/squarecloudofc/sdk-api-go/v4"
```

```bash
go get github.com/squarecloudofc/sdk-api-go/v4@v4.0.0
```

## Method by method

| old | new | notes |
|---|---|---|
| `c.Account.Me(ctx)` | unchanged | |
| `c.Account.Snapshots(ctx, scope)` | unchanged | Returns 503 `SNAPSHOTS_UNAVAILABLE` while snapshot storage cannot answer, where it used to return an empty list. `URL` is valid for 1 hour. |
| `c.Service.Status(ctx)` | unchanged | `ServiceStatus` gains `Incidents` and `Components`. `Status` can be `"maintenance"`, and `"unknown"` means that the check could not run. |
| `c.AI.Chat(ctx, req)` | unchanged | `ChatRequest.MaxCompletionTokens` wins over `MaxTokens`. A 429 from the AI can be `weekly_limit_reached`. |
| `c.Apps.Create(ctx, zip)` | unchanged | |
| `c.Apps.Commit(ctx, id, r, path, filename)` | unchanged | Returns 403 `BLOCKED_PATH` for a protected destination. |
| `c.Apps.Get(ctx, id)` | unchanged | |
| `c.Apps.Delete(ctx, id)` | unchanged | `RESTORE_IN_PROGRESS` is 409. It was 403. |
| `c.Apps.Start(ctx, id)` | unchanged | Can return 409 `RESTORE_IN_PROGRESS`. |
| `c.Apps.Stop(ctx, id)` | unchanged | Can return 409 `RESTORE_IN_PROGRESS`. |
| `c.Apps.Restart(ctx, id)` | unchanged | Can return 409 `RESTORE_IN_PROGRESS`. |
| `c.Apps.Status(ctx, id)` | unchanged | Adds `RuntimeStats.Restoring`. |
| `c.Apps.StatusRaw(ctx, id)` | unchanged | Adds `RuntimeStatsRaw.Restoring`. |
| `c.Apps.StatusAll(ctx, workspaceID)` | unchanged | |
| `c.Apps.Logs(ctx, id)` | unchanged | |
| `c.Apps.Metrics(ctx, id)` | `c.Apps.Metrics(ctx, id, rng)` | `""` keeps the v3 behavior of 24 hours. `"7d"` gives hourly points with `CPUMax` and `RAMMax` on the Pro and Enterprise plans for applications of 512 MB or more. Others get the 24-hour answer without an error. Applications below 512 MB now get a point every 10 minutes instead of 400 `METRICS_NOT_SUPPORTED`. Any other `rng` returns 400 `INVALID_PARAMETERS`. |
| `c.Apps.Realtime(ctx, id)` | unchanged | |
| `c.Apps.Domains(ctx)` | unchanged | |
| `c.Apps.LoadBalancers(ctx)` | unchanged | |
| `c.Apps.Deploys.SetWebhook(ctx, id, accessToken)` | unchanged | |
| `c.Apps.Deploys.LinkGithubApp(ctx, id, repository, branch)` | unchanged | |
| `c.Apps.Deploys.UnlinkGithubApp(ctx, id)` | unchanged | |
| `c.Apps.Deploys.List(ctx, id)` | unchanged | |
| `c.Apps.Deploys.Current(ctx, id)` | unchanged | |
| `c.Apps.Envs.Get(ctx, id)` | unchanged | |
| `c.Apps.Envs.Set(ctx, id, envs)` | unchanged | Returns 409 `ENV_BUSY` when another write is in progress. With an API key that lacks `envs:read`, it returns only the variables you sent. |
| `c.Apps.Envs.Replace(ctx, id, envs)` | unchanged | Returns 409 `ENV_BUSY` when another write is in progress. |
| `c.Apps.Envs.Delete(ctx, id, keys...)` | unchanged | Returns 409 `ENV_BUSY` when another write is in progress. With an API key that lacks `envs:read`, it returns an empty set. |
| `c.Apps.Files.List(ctx, id, path)` | unchanged | |
| `c.Apps.Files.Read(ctx, id, path)` | unchanged | Returns 400 `READ_FAILED` when the server cannot read the file. |
| `c.Apps.Files.Write(ctx, id, path, content)` | unchanged | Returns 400 `WRITE_FAILED` when the server cannot write the file. |
| `c.Apps.Files.Move(ctx, id, path, to)` | unchanged | |
| `c.Apps.Files.Delete(ctx, id, path)` | unchanged | |
| `c.Apps.Snapshots.List(ctx, id)` | unchanged | Returns 503 `SNAPSHOTS_UNAVAILABLE` where it used to return an empty list. `URL` is valid for 1 hour. |
| `c.Apps.Snapshots.Create(ctx, id)` | unchanged | `URL` is valid for 1 hour instead of 30 days. Can return 409 `RESTORE_IN_PROGRESS` and 503 `CLUSTER_MAINTENANCE_TRY_LATER`. |
| `err := c.Apps.Snapshots.Restore(ctx, id, name, versionID)` | `res, err := c.Apps.Snapshots.Restore(ctx, id, name, versionID)` | When `res.Pending` is true, the restore keeps running. Poll `RestoreState`, and never send the restore again. See [Behavior changes](#behavior-changes). |
| none | `c.Apps.Snapshots.RestoreState(ctx, id)` | New. Returns the `SnapshotRestoreState` of the last restore. |
| `c.Apps.Network.Analytics(ctx, id, start, end, f)` | unchanged | The visits buckets gain `BytesIn`. |
| `c.Apps.Network.Errors(ctx, id, start, end, include4xx)` | unchanged | |
| `c.Apps.Network.Logs(ctx, id, start, end)` | unchanged | |
| `c.Apps.Network.Performance(ctx, id, start, end)` | unchanged | |
| `c.Apps.Network.DNS(ctx, id)` | unchanged | `DNSRecord` gains `Errors` and `Fallback`. |
| `c.Apps.Network.SetDomain(ctx, id, domain)` | unchanged | |
| `c.Apps.Network.PurgeCache(ctx, id)` | unchanged | |
| `c.Databases.Create(ctx, in)` | unchanged | |
| `c.Databases.Get(ctx, id)` | unchanged | |
| `c.Databases.Update(ctx, id, in)` | unchanged | |
| `c.Databases.Delete(ctx, id)` | unchanged | `RESTORE_IN_PROGRESS` is 409. It was 403. |
| `c.Databases.Start(ctx, id)` | unchanged | `RESTORE_IN_PROGRESS` is 409. It was 403. |
| `c.Databases.Stop(ctx, id)` | unchanged | `RESTORE_IN_PROGRESS` is 409. It was 403. |
| `c.Databases.Status(ctx, id)` | unchanged | Adds `RuntimeStats.Restoring`. |
| `c.Databases.StatusRaw(ctx, id)` | unchanged | Adds `RuntimeStatsRaw.Restoring`. |
| `c.Databases.StatusAll(ctx)` | unchanged | |
| `c.Databases.Metrics(ctx, id)` | `c.Databases.Metrics(ctx, id, rng)` | Works as `Apps.Metrics`. `"7d"` needs a Pro or Enterprise plan. |
| `c.Databases.Certificate(ctx, id)` | unchanged | |
| `c.Databases.ResetCredentials(ctx, id, reset)` | unchanged | |
| `c.Databases.Snapshots.List(ctx, id)` | unchanged | Same changes as `Apps.Snapshots.List`. |
| `c.Databases.Snapshots.Create(ctx, id)` | unchanged | Same changes as `Apps.Snapshots.Create`. |
| `err := c.Databases.Snapshots.Restore(ctx, id, name, versionID)` | `res, err := c.Databases.Snapshots.Restore(ctx, id, name, versionID)` | Same change as `Apps.Snapshots.Restore`. |
| none | `c.Databases.Snapshots.RestoreState(ctx, id)` | New, like `Apps.Snapshots.RestoreState`. |
| `c.Workspaces.Create(ctx, name)` | unchanged | |
| `c.Workspaces.List(ctx)` | unchanged | |
| `c.Workspaces.Get(ctx, workspaceID)` | unchanged | |
| `c.Workspaces.Delete(ctx, workspaceID)` | unchanged | |
| `c.Workspaces.Leave(ctx, workspaceID)` | unchanged | |
| `c.Workspaces.Members.Add(ctx, workspaceID, code, group)` | unchanged | |
| `c.Workspaces.Members.Update(ctx, workspaceID, memberID, group)` | unchanged | |
| `c.Workspaces.Members.Remove(ctx, workspaceID, memberID)` | unchanged | |
| `c.Workspaces.Members.InviteCode(ctx)` | unchanged | |
| `c.Workspaces.Apps.Add(ctx, workspaceID, appID)` | unchanged | |
| `c.Workspaces.Apps.Remove(ctx, workspaceID, appID)` | unchanged | |
| `c.DownloadSnapshot(ctx, url, w)` | unchanged | Download soon, because a snapshot `URL` now expires after 1 hour. |

## Types

| v3 | v4 |
|---|---|
| none | `SnapshotRestored{Pending bool}`, which `Restore` returns |
| none | `SnapshotRestoreState{State, Step, Code string; StartedAt, FinishedAt *int64}`. `State` is `none`, `queued`, `running`, `success` or `error`, and the times are in Unix milliseconds. |
| none | `ServiceIncident{Type, Title, Status string; StartedAt time.Time; EndsAt *time.Time; Affected []string; Update string; UpdatedAt *time.Time}` |
| none | `ServiceComponent{Name, Status string}` |
| `ServiceStatus{Status, Message, CheckedAt, Stale, Services, Dependencies}` | Adds `Incidents []ServiceIncident` and `Components []ServiceComponent`. |
| `MetricPoint{Date, CPU, RAM, Net}` | Adds `Disk []int64`, the bytes read and written since the last start, and `Storage *int64`, the bytes on disk. Both are nil on a point without a reading. Also adds `CPUMax` and `RAMMax *float64`, the hourly peaks, set only with `"7d"`. |
| `RuntimeStats`, `RuntimeStatsRaw` | Add `Restoring bool`. |
| `DNSRecord{Type, Name, Value, Status}` | Adds `Errors []string`, up to 3 messages that say why a record is not active yet, and `Fallback bool`, which marks the one `_acme-challenge` CNAME that can replace every SSL TXT record. |
| `AnalyticsTimeBucket{Date, Visits, Requests, Bytes}` | Adds `BytesIn int64`. |
| `ChatRequest` | Adds `MaxCompletionTokens int`. |

The new fields break only struct literals of these types that leave out the field names. Use field names.

## Errors

- **`RESTORE_IN_PROGRESS` moved from 403 to 409** on application and database delete and on database start and stop. It is also new, as 409, on application start, stop and restart and on snapshot create and restore. Code that matched `e.Status == 403 && e.Code == squarecloud.CodeRestoreInProgress` must check for 409, or match on `Code` alone.
- New constants: `CodeAIWeeklyLimitReached`, `CodeEnvBusy` (409), `CodeGitBranchDeleted` and `CodeGitRefNotSupported` (sent only to GitHub push webhooks), `CodeRestoreInterrupted`, `CodeSnapshotsUnavailable` (503), `CodeSnapshotCorrupt`, `CodeSnapshotInProgress` (409), `CodeSnapshotRestoreProcessing` (reported as `SnapshotRestored.Pending`, never as an error), `CodeWriteFailed`, and `CodeWrongHost` (421, never returned to the calls of this SDK).
- `CodeMetricsNotSupported` is deprecated but kept. The API no longer sends it.
- A restore can fail with these errors. `SNAPSHOT_RESTORE_FAILED` is no longer the only one.
  - 400 `MISSING_PARAMETERS`, `INVALID_SNAPSHOT_ID` or `INVALID_VERSION_ID`, and `SNAPSHOT_DATABASE_MISMATCH` for databases.
  - 404 `APP_NOT_FOUND` or `DATABASE_NOT_FOUND`, `SNAPSHOT_NOT_FOUND`, `SNAPSHOT_RESTORE_FAILED`, `SNAPSHOT_CORRUPT`, `RESTORE_INTERRUPTED` or `CONTAINER_NOT_FOUND`.
  - 409 `RESTORE_IN_PROGRESS` or `SNAPSHOT_IN_PROGRESS`.
  - 413 `PAYLOAD_TOO_LARGE`.
  - 503 `CLUSTER_MAINTENANCE_TRY_LATER`.
- Snapshot listings can return 503 `SNAPSHOTS_UNAVAILABLE`, and snapshot create and restore can return 503 `CLUSTER_MAINTENANCE_TRY_LATER`. Metrics return 400 `INVALID_PARAMETERS` for an unknown range. `Files.Write` and `Files.Read` can return 400 `WRITE_FAILED` and `READ_FAILED`, and `Commit` can return 403 `BLOCKED_PATH`.
- The SDK retries none of the new codes. `SNAPSHOTS_UNAVAILABLE` asks you to wait about 30 seconds and `CLUSTER_MAINTENANCE_TRY_LATER` lasts minutes, both longer than the retry waits. A 409 from a busy resource is yours to decide.
- The SDK's own errors, `INVALID_API_KEY`, `INVALID_ID` and `FILE_TOO_LARGE`, have new messages that say what went wrong and how to fix it. Compare `Code`, not `Message`.

## Behavior changes

- **Restore.** A restore that takes longer than about 90 seconds keeps running after the call returns. v3 returned `nil` for it, so callers reported "restored" while it was still running. v4 returns `SnapshotRestored{Pending: true}`, and a finished restore returns `Pending: false`. Follow a pending restore with `RestoreState` every few seconds, which allows 30 calls a minute, until `State` is `success` or `error`. Never send the restore again:

  ```go
  res, err := c.Apps.Snapshots.Restore(ctx, appID, snap.Name, snap.VersionID)
  if err != nil {
  	return err
  }
  for res.Pending {
  	time.Sleep(10 * time.Second)
  	st, err := c.Apps.Snapshots.RestoreState(ctx, appID)
  	if err != nil {
  		return err
  	}
  	if st.State == "error" {
  		return fmt.Errorf("restore failed: %s", st.Code)
  	}
  	res.Pending = st.State != "success"
  }
  ```

  `RestoreState` keeps the result for 1 hour after the restore ends, and then reports `none`.
- **Snapshot links expire after 1 hour.** They used to last 30 days, so download right after listing or creating a snapshot. Only `Create` can return an empty `URL` and `Key`, with `Pending: false`, when the API key lacks `snapshots:read`.
- **Metrics.** Applications below 512 MB now have metrics, with one point every 10 minutes. `"7d"` on a plan or application that does not qualify returns the 24-hour answer without an error, so check the points instead of assuming 7 days.
- **Snapshot listings** return 503 `SNAPSHOTS_UNAVAILABLE` instead of an empty list when snapshot storage cannot answer.
- **Keys that cannot read variables.** With an API key that lacks `envs:read`, `Envs.Set` returns only the variables you sent, not the full set, and `Envs.Delete` returns an empty set.
- **Parallel uploads.** The API limits how many large uploads an account can send at the same time. Going over the limit returns 429 `RATE_LIMITED`, which the SDK never retries.
- **User-Agent** is `squarecloud-sdk-go/4.0.0`.
