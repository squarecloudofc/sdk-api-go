# Migrating from v2 to v3

v3 is a breaking release. It uses one package, a concrete `*Client`, `ctx` as the first argument everywhere, resource groups, and fixes for every known v2 bug. It covers all 67 operations of the current API. The only known consumer is the Square Cloud CLI (`cli/go.mod` pins `v2.0.0`). This guide lists every v2 symbol the CLI uses.

## At a glance

| | v2 | v3 |
|---|---|---|
| Module | `.../v2` (`rest` + `squarecloud` packages) | `github.com/squarecloudofc/sdk-api-go/v3` (one package) |
| Client | `rest.New(rest.NewClient(key, ...))` (interface) | `squarecloud.New(key, ...Option)` (`*Client`) |
| Calls | `api.GetApplicationStatus(id, rest.WithContext(ctx))` | `c.Apps.Status(ctx, id)` |
| Errors | `*rest.APIError{StatusCode}`, network errors untyped | `*squarecloud.APIError{Status, Code, Message, Method, Path}` for everything |
| Timeouts | `http.Client{Timeout: 30s}` on everything | per call through `ctx`; `WithTimeout`; streams unbounded |
| Retries | none | GET network errors and 503 `*_BUSY`/`DATABASE_UNAVAILABLE` (`WithMaxRetries`) |
| Logging | `WithLogger` (leaked secrets) | none |
| License | AGPL-3.0 | MIT |

## Construction and options

```diff
-import (
-	"github.com/squarecloudofc/sdk-api-go/v2/rest"
-	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
-)
-client := rest.NewClient(token, rest.WithUserAgent(ua), rest.WithLogger(logger))
-api := rest.New(client)
-defer client.Close()
+import "github.com/squarecloudofc/sdk-api-go/v3"
+
+c := squarecloud.New(token, squarecloud.WithUserAgent(ua))
```

```bash
go get github.com/squarecloudofc/sdk-api-go/v3@v3.0.0
```

| v2 | v3 |
|---|---|
| packages `rest` + `squarecloud` | one package `squarecloud` (module `.../v3`) |
| `rest.NewClient(token, opts...)` + `rest.New(client)` | `squarecloud.New(token, opts...)` |
| `rest.Rest` (interface) | `*squarecloud.Client` (concrete struct). To mock it, declare your own small interface or use `httptest` |
| `rest.ConfigOpt` | `squarecloud.Option` |
| `rest.WithHTTPClient(hc)` | `squarecloud.WithHTTPClient(hc)`. Do not set `hc.Timeout` (it would cut realtime streams) |
| `rest.WithURL(u)` | `squarecloud.WithBaseURL(u)` (still includes `/v2`) |
| `rest.WithUserAgent(ua)` | `squarecloud.WithUserAgent(ua)` |
| `rest.WithLogger(l)` | removed: the SDK never logs (v2 leaked secrets in debug logs). Wrap `hc.Transport` to trace |
| `client.Close()`, `client.HTTPClient()` | removed: keep your own `*http.Client` and call `CloseIdleConnections` on it |
| `rest.APIURL`, `rest.APIVersion`, `rest.Endpoint*` | removed; `squarecloud.DefaultBaseURL` is a const |
| — | `squarecloud.WithMaxRetries(n)` (new; default 2) |
| `http.Client{Timeout: 30s}` (fixed) | `squarecloud.WithTimeout(d)` (new; default 30s, `<= 0` disables every default deadline) |

Per-request options:

| v2 | v3 |
|---|---|
| `rest.WithContext(ctx)` | `ctx` is the first argument of every method |
| `rest.WithToken(token)` (e.g. validating a key at login) | build a throwaway client: `squarecloud.New(token).Account.Me(ctx)` (cheap, no connections are held) |
| `rest.WithQueryParam("path", dir)` on commit | `c.Apps.Commit(ctx, id, r, dir, "")` |
| `rest.WithQueryParam(filter, v)` on analytics | `c.Apps.Network.Analytics(ctx, id, start, end, squarecloud.AnalyticsFilters{Country, IP, Path, Status, OS, Browser, Protocol, Referer, Provider, ContentType, Bot})` |
| `rest.WithQueryParam("include_4xx", "true")` | `c.Apps.Network.Errors(ctx, id, start, end, true)` |
| `rest.WithQueryParam("workspaceId", ws)` on list status | `c.Apps.StatusAll(ctx, ws)` |
| `rest.WithHeader`, `rest.RequestOpt`, `rest.RequestConfig`, `rest.DefaultRequestConfig` | removed |

## Method by method

`api` is the v2 `rest.Rest`, `c` the v3 `*squarecloud.Client`.

| old | new | notes |
|---|---|---|
| `api.SelfUser()` | `me, err := c.Account.Me(ctx)`, then `me.User` | returns `Account` |
| `api.GetApplications()` | `c.Account.Me(ctx)`, then `me.Applications` (`[]AppSummary`) | |
| `api.GetDatabases()` | `c.Account.Me(ctx)`, then `me.Databases` (`[]DatabaseSummary`) | |
| `api.UserSnapshots(scope)` | `c.Account.Snapshots(ctx, scope)` | |
| `api.ServiceStatus()` | `c.Service.Status(ctx)` | new model, see [Types](#types) |
| `api.PostApplications(r)` → `*ApplicationUploaded` | `c.Apps.Create(ctx, r)` → `AppCreated` (a value) | v2 buffered the zip in memory and named the part `upload.zip`; v3 streams it and names the part after an `*os.File` (its base name), else `app.zip`. `Subdomain` is gone: read `Domain`, the full host (`my-app.squareweb.app`), `""` for non-web apps |
| `api.GetApplication(id)` | `c.Apps.Get(ctx, id)` | |
| `api.DeleteApplication(id)` | `c.Apps.Delete(ctx, id)` | |
| `api.PostApplicationSignal(id, squarecloud.ApplicationSignalStart/Stop/Restart)` | `c.Apps.Start(ctx, id)` / `c.Apps.Stop(ctx, id)` / `c.Apps.Restart(ctx, id)` | |
| `api.PostApplicationCommit(id, r, rest.WithQueryParam("path", p))` | `c.Apps.Commit(ctx, id, r, p, filename)` | v2 always named the part `commit.zip`; v3 uses `filename`, else an `*os.File`'s own name, so `commit --file main.py` now lands as `main.py` instead of failing as a zip |
| `api.GetApplicationStatus(id)` | `c.Apps.Status(ctx, id)` | |
| `api.GetApplicationStatusRaw(id)` | `c.Apps.StatusRaw(ctx, id)` | |
| `api.GetApplicationListStatus()` | `c.Apps.StatusAll(ctx, "")` | returns `[]StatusListItem` |
| `api.GetApplicationLogs(id)` → `ApplicationLogs{Logs}` | `c.Apps.Logs(ctx, id)` → `string` | |
| `api.GetApplicationMetrics(id)` | `c.Apps.Metrics(ctx, id)` | points come newest first, as the API sends them |
| `api.ApplicationRealtime(id, rest.WithContext(ctx))` | `c.Apps.Realtime(ctx, id)` | returns `*Realtime`; see Behavior changes |
| `api.GetApplicationDomains()` | `c.Apps.Domains(ctx)` | |
| `api.GetLoadBalancers()` | `c.Apps.LoadBalancers(ctx)` | |
| `api.GetApplicationEnvs(id)` | `c.Apps.Envs.Get(ctx, id)` | |
| `api.SetApplicationEnvs(id, envs)` | `c.Apps.Envs.Set(ctx, id, envs)` | |
| `api.ReplaceApplicationEnvs(id, envs)` | `c.Apps.Envs.Replace(ctx, id, envs)` | |
| `api.DeleteApplicationEnvs(id, keys)` → `error` | `c.Apps.Envs.Delete(ctx, id, keys...)` → `(EnvVars, error)` (the remaining vars) | |
| `api.GetApplicationFiles(id, path)` → `[]FileInfo` | `c.Apps.Files.List(ctx, id, path)` → `[]FileEntry` | a missing directory is now 404 `FILE_NOT_FOUND` (it was an empty list); a protected path is 403 `BLOCKED_PATH` |
| `api.ReadApplicationFile(id, path)` → `FileContent{Type, Data}` | `c.Apps.Files.Read(ctx, id, path)` → `[]byte` | sends `?encoding=base64` and returns the decoded bytes; a file over 10 MB is 413 `FILE_TOO_LARGE` |
| `api.PutApplicationFile(id, path, b)` → `(FileWritten, error)` | `c.Apps.Files.Write(ctx, id, path, b)` → `error`. Drop any `written` check: success is a nil error | the content is always sent base64-encoded, so binary files are safe; empty content writes an empty file |
| `api.MoveApplicationFile(id, from, to)` | `c.Apps.Files.Move(ctx, id, path, to)` | |
| `api.DeleteApplicationFile(id, path)` | `c.Apps.Files.Delete(ctx, id, path)` (works now; always 400 in v2) | |
| `api.GetApplicationSnapshots(id)` | `c.Apps.Snapshots.List(ctx, id)` | each `Snapshot` now carries `VersionID` and `URL` (signed download link) from the API |
| `api.CreateApplicationSnapshot(id)` | `c.Apps.Snapshots.Create(ctx, id)`. Check `.Pending` (202) before using `.URL` | |
| `api.RestoreApplicationSnapshot(id, snapID, verID)` | `c.Apps.Snapshots.Restore(ctx, id, snap.Name, snap.VersionID)` | `VersionID` is a field the API sends. Drop the CLI's `url.ParseQuery(snap.Key)` lookup (`cmdutil/snapshot_list.go`): nothing needs to be parsed out of `Key` any more. A failed restore is 404 `SNAPSHOT_RESTORE_FAILED` |
| `api.GetApplicationDeployments(id)` | `c.Apps.Deploys.List(ctx, id)` | |
| `api.GetApplicationCurrentDeployment(id)` | `c.Apps.Deploys.Current(ctx, id)` | |
| `api.PostApplicationDeployWebhook(id, token)` → `GithubWebhook{Webhook}` | `c.Apps.Deploys.SetWebhook(ctx, id, token)` → `string` | |
| `api.LinkApplicationGithubApp(id, repo, branch)` → `GithubAppLink` | `c.Apps.Deploys.LinkGithubApp(ctx, id, repo, branch)` → `LinkedRepository` | API keys work now (scope `apps:deploy`); the `repository` wrapper is gone; an account without a connected GitHub is 403 `GITHUB_NOT_CONNECTED` |
| `api.UnlinkApplicationGithubApp(id)` | `c.Apps.Deploys.UnlinkGithubApp(ctx, id)` | 400 `GIT_NOT_CONFIGURED` without a link |
| `api.GetApplicationDNS(id)` | `c.Apps.Network.DNS(ctx, id)` | |
| `api.SetApplicationCustomDomain(id, d)` | `c.Apps.Network.SetDomain(ctx, id, d)` | |
| `api.GetApplicationAnalytics(id, s, e, opts...)` | `c.Apps.Network.Analytics(ctx, id, s, e, squarecloud.AnalyticsFilters{...})` | returns `*NetworkAnalytics`, `nil` for a window with no traffic |
| `api.GetApplicationNetworkErrors(id, s, e, opts...)` | `c.Apps.Network.Errors(ctx, id, s, e, include4xx)` | returns `*NetworkErrors`, `nil` when empty |
| `api.GetApplicationNetworkLogs(id, s, e)` | `c.Apps.Network.Logs(ctx, id, s, e)` | |
| `api.GetApplicationNetworkPerformance(id, s, e)` | `c.Apps.Network.Performance(ctx, id, s, e)` | returns `*NetworkPerformance`, `nil` when empty |
| `api.PurgeApplicationCache(id)` | `c.Apps.Network.PurgeCache(ctx, id)` | |
| `api.CreateDatabase(opts)` | `c.Databases.Create(ctx, squarecloud.DatabaseCreate{...})` | |
| `api.GetDatabase(id)` | `c.Databases.Get(ctx, id)` | |
| `api.UpdateDatabase(id, opts)` | `c.Databases.Update(ctx, id, squarecloud.DatabaseUpdate{...})` | |
| `api.DeleteDatabase(id)` | `c.Databases.Delete(ctx, id)` | |
| `api.StartDatabase(id)` / `api.StopDatabase(id)` | `c.Databases.Start(ctx, id)` / `c.Databases.Stop(ctx, id)` | |
| `api.GetDatabaseStatus(id)` / `api.GetDatabaseStatusRaw(id)` | `c.Databases.Status(ctx, id)` / `c.Databases.StatusRaw(ctx, id)` | |
| `api.GetDatabaseListStatus()` | `c.Databases.StatusAll(ctx)` | returns `[]StatusListItem` |
| `api.GetDatabaseMetrics(id)` | `c.Databases.Metrics(ctx, id)` | |
| `api.GetDatabaseCertificate(id)` → `DatabaseCertificate{Certificate}` | `c.Databases.Certificate(ctx, id)` → `string` (base64 PEM) | |
| `api.ResetDatabaseCredentials(id, t)` → `DatabasePasswordReset{Password}` | `c.Databases.ResetCredentials(ctx, id, t)` → `string` (password, empty for certificate resets) | |
| `api.GetDatabaseSnapshots` / `CreateDatabaseSnapshot` / `RestoreDatabaseSnapshot` | `c.Databases.Snapshots.List` / `Create` / `Restore` | as for apps: restore with `snap.Name` and `snap.VersionID` |
| `api.GetWorkspaces()` | `c.Workspaces.List(ctx)` | |
| `api.GetWorkspace(id)` | `c.Workspaces.Get(ctx, id)` | |
| `api.CreateWorkspace(name)` | `c.Workspaces.Create(ctx, name)` | |
| `api.DeleteWorkspace(id)` | `c.Workspaces.Delete(ctx, id)` | |
| `api.LeaveWorkspace(id)` | `c.Workspaces.Leave(ctx, id)` | |
| `api.AddWorkspaceMember(ws, code, group)` | `c.Workspaces.Members.Add(ctx, ws, code, group)` | |
| `api.UpdateWorkspaceMember(ws, member, group)` | `c.Workspaces.Members.Update(ctx, ws, member, group)` | |
| `api.RemoveWorkspaceMember(ws, member)` | `c.Workspaces.Members.Remove(ctx, ws, member)` | |
| `api.GetWorkspaceInviteCode()` → `WorkspaceInviteCode{Code}` | `c.Workspaces.Members.InviteCode(ctx)` → `string` | |
| `api.AddWorkspaceApplication(ws, app)` / `RemoveWorkspaceApplication` | `c.Workspaces.Apps.Add(ctx, ws, app)` / `Remove` | |
| CLI `cmdutil.DownloadFile` (`http.Get`) | `c.DownloadSnapshot(ctx, url, w)` | never sends the key |
| `rest.IsRateLimit(err)` | `errors.As(err, &e) && e.Status == 429` | removed: not part of the standard surface |
| `rest.ErrorCode(err)` | `errors.As(err, &e)`, then `e.Code` | removed |
| `client.Request(...)`, `client.Stream(...)` (`rest.Client`) | the typed methods above | removed: every operation has its own method |
| `rest.NewApplications(client)`, `rest.NewDatabases(client)`, `rest.NewWorkspaces(client)` | `squarecloud.New(key)`, then the `c.Apps`, `c.Databases`, `c.Workspaces` fields | removed |
| `rest.Applications`, `rest.Databases`, `rest.Workspaces` (interfaces) | `squarecloud.AppsAPI`, `DatabasesAPI`, `WorkspacesAPI` (the types of those fields) | removed; declare your own interface to mock |
| `rest.Config{Logger, HTTPClient, URL, UserAgent}`, `rest.DefaultConfig()`, `(*rest.Config).Apply(opts)` | `squarecloud.New(key, opts...)` with `WithHTTPClient`, `WithBaseURL`, `WithUserAgent` | removed (use `Option`; there is no logger) |
| `(*rest.RequestConfig).Apply(opts)` | pass `ctx` and typed arguments | removed |
| `(*rest.RealtimeStream).Next()` / `Close()` | `(*squarecloud.Realtime).Next()` / `Close()` | `Next` returns `io.EOF` after `REALTIME_DISCONNECTED` |
| — | `c.AI.Chat(ctx, squarecloud.ChatRequest{...})` | new |

## Types

| v2 (`squarecloud.`) | v3 (`squarecloud.`) |
|---|---|
| `User` (from `SelfUser`) | `User` (inside `Account`, the result of `Account.Me`) |
| `UserPlan`, `UserPlanMemory` | `Plan`, `PlanMemory`. `Plan.Duration` is the expiry in Unix **milliseconds**: the CLI's `whoami` passes it to `time.Unix(d, 0)`; use `time.UnixMilli(d)` |
| `UserApplication`, `UserDatabase{Type string}` | `AppSummary`, `DatabaseSummary{Type DatabaseType}` |
| `Application` | `App` |
| `ApplicationUploaded{CPU int}`, `ApplicationLanguage` | `AppCreated{CPU float64}`, `AppLanguage` |
| `ApplicationStatus`, `DatabaseStatus` | `RuntimeStats` (shared) |
| `ApplicationStatusRaw`, `DatabaseStatusRaw` | `RuntimeStatsRaw` |
| `ApplicationStatusNetwork[Raw]` | `StatsNetwork[Raw]` |
| `ApplicationStatusListItem`, `DatabaseStatusListItem` | `StatusListItem` |
| `ApplicationLogs` | `string` |
| `ApplicationSignal*` | removed (use `Start`/`Stop`/`Restart`) |
| `FileInfo{Type FileType, LastModified int64}`, `FileType*` consts | `FileEntry{Type string, LastModified *float64}` (`"file"`/`"directory"`; the API sends fractional Unix ms, or `null`) |
| `FileContent`, `ByteArray` | `[]byte` |
| `FileWritten` | removed (undocumented field) |
| `Deployment{State DeploymentState}` | `DeployEvent{State, Source, Code, Message string}` (new `Source`, always `"git"`; an `"error"` event carries `Code`, e.g. `CLONE_FAILED`, and sometimes `Message`) |
| `DeploymentState*` (`DeploymentStateError` = `"error"`) | `Deploy*` string consts (`DeployPending`, `DeployClone`, `DeployCommit`, `DeployRestarting`, `DeploySuccess`, `DeployError` = `"error"`) |
| `DeploymentFiles`, `DeploymentCurrent`, `DeploymentGithubApp` | `DeployFiles`, `DeployCurrent`, `DeployRepository` |
| `GithubWebhook` | `string` |
| `GithubAppLink`, `GithubAppRepository` | `LinkedRepository{ID, FullName, Branch}` (returned directly, no `Repository` wrapper) |
| `Snapshot` | `Snapshot` + `VersionID`, `URL`, `Runtime`, `Origin` (all fields the API sends) |
| `SnapshotCreated` | `SnapshotCreated` + `Pending` |
| `DatabaseType` with `DatabaseTypeMongo`, `DatabaseTypeMySQL`, `DatabaseTypeRedis`, `DatabaseTypePostgres` | `DatabaseType` (unchanged) with `DatabaseMongo`, `DatabaseMySQL`, `DatabaseRedis`, `DatabasePostgres` (constants renamed) |
| `DatabaseCreateOptions`, `DatabaseUpdateOptions` | `DatabaseCreate`, `DatabaseUpdate` |
| `DatabaseCreated{CPU int, Certificate string}` | `DatabaseCreated{CPU float64, Certificate *string}` (`nil` when the API sends none) |
| `DatabaseResetType`, `DatabaseResetPassword/Certificate` | `DatabaseReset`, `ResetPassword` / `ResetCertificate` |
| `DatabaseCertificate`, `DatabasePasswordReset` | `string` |
| `WorkspaceMemberGroup`, `WorkspaceGroup*` | input: `WorkspaceGroup` (`GroupAdmin`, `GroupMaintain`, `GroupManager`, `GroupView`); `WorkspaceMember.Group` is a `string` (may be `"owner"`) |
| `WorkspaceMember{Name string}` | `WorkspaceMember{Name *string}` (`nil` when the API sends `null`). Workspace ids are 32 or 40 hex characters |
| `WorkspaceInviteCode` | `string` |
| `ServiceStatus{Status, Message}` | `ServiceStatus{Status, Message, CheckedAt, Stale, Services, Dependencies}` (`Status` is now e.g. `"online"`; entries are `ServiceEntry`) |
| `rest.RealtimeStream` | `*squarecloud.Realtime` (`Next`, `Close`) |
| `RealtimeEvent{Event, Data}` | `RealtimeEvent{Event, Data, ID, Stream, Line, Status}` |
| query options on analytics | `AnalyticsFilters` (no `Start`/`End`: they are arguments) |
| `NetworkErrorsSummaryClass{Class4xx, Class5xx}` | `NetworkErrorsSummary.ByClass map[string]int64` (`["4xx"]`, `["5xx"]`) |
| `NetworkErrorsByStatus`, `NetworkErrorsTimeseries`, `NetworkErrorsTopPath`, `NetworkErrorsByMethod` | `NetworkErrorsStatus`, `NetworkErrorsBucket`, `NetworkErrorsPath`, `NetworkErrorsMethod` |
| `NetworkLatency`, `NetworkPerformance*` | `Percentiles` (`P50`/`P95`/`P99` are `*float64`, nil for a window with no requests), `PerformanceSummary`, `PerformanceBucket`, `PerformanceRegion` (countries and colos; `P50`/`P95` `*float64`, `City`/`Country` `*string`), `PerformancePath` (`P95`/`P99` `*float64`) |
| `RealtimeStatus` | `RealtimeStatus.CPULimit` is a number of cores (e.g. `1`, `0.5`) |
| analytics provider values | `"NAME (ASN)"` (e.g. `"GOOGLE (15169)"`); `AnalyticsFilters.Provider` takes that exact value |
| `AppDomainType*` | `AppDomain.Type` is a `string` |
| `APIResponse[T]` | removed (internal) |
| `int` sizes and counters (`FileInfo.Size`, `Snapshot.Size`, analytics `Visits`/`Requests`, network-error totals and maps, performance `Requests`) | `int64` |

## Errors

`rest.APIError{StatusCode, Code, Message}` → `squarecloud.APIError{Status, Code, Message, Method, Path}`. Rename `StatusCode` → `Status`. `rest.ErrorCode(err)` and `rest.IsRateLimit(err)` were removed: use `errors.As` and check `Code` or `Status == 429`. Network failures are now `*APIError` with `Status 0`, `Code` `NETWORK_ERROR`/`TIMEOUT` and the cause's text as `Message`, and they unwrap to the cause. So are local checks (`Status 0`, `INVALID_ID`, `FILE_TOO_LARGE`, `INVALID_API_KEY`), a 2xx body that says `"status":"error"` (v2 reported it as success; the cluster refusals of app and database start/stop now arrive as 409 `CONTAINER_ALREADY_STARTED`, `CONTAINER_ALREADY_STOPPED`, `CONTAINER_TEMPORARILY_SUSPENDED`, `CONTAINER_NOT_FOUND`, `CONTAINER_INSUFFICIENT_DISK_SPACE`, `CONTAINER_NETWORK_CONFLICT` or `ACTION_FAILED`, with no message; the SDK returns an "already" answer as an error, so treat it as success yourself if you need to), and a 2xx that is not JSON (`UNKNOWN_ERROR`, `Invalid JSON in HTTP <status> response`). A response without a code has `Code` `UNKNOWN_ERROR`. An expired API key is 401 `ACCESS_DENIED`, like an unknown one (there is no `APIKEY_EXPIRED`). v3 has a `Code*` constant for every code the API documents. Every `AI.Chat` error is OpenAI-shaped with a lowercase code (`access_denied`, `rate_limit_exceeded`, `server_overloaded`, ...), which `Code` carries verbatim. `Error()` renders `squarecloud: <METHOD> <path>: HTTP <status> <CODE>: <message>` (v2: `squarecloud: <message> (<CODE>, HTTP <status>)`); match on fields, not on the text. The API now sends 429 `RATE_LIMITED` (`CodeRateLimited`) where it sent `RATE_LIMIT` and `RATE_LIMIT_EXCEEDED`; `CodeRateLimit` and `CodeRateLimitExceeded` remain, deprecated.

## Behavior changes

- **Empty API key:** `New("")` (or a whitespace-only key) still returns a client (it cannot return an error), but every call except `Service.Status` fails locally with `INVALID_API_KEY`. The JS and Python SDKs reject such a key in their constructors.
- **Snapshot 202:** v2 returned `*APIError{StatusCode: 202}`. v3 returns `SnapshotCreated{Pending: true}` with a nil error.
- **Realtime:** `stream.Next()` now returns `RealtimeEvent`. The CLI's `message` branch never matched in v2 and can go. Switch on `ev.Event` (`system`, `status`, `logs`, `error`). For logs, print `ev.Line` (the `\u0001`/`\u0002` byte is stripped; `ev.Data` stays the raw frame) and use `ev.Stream` for stdout/stderr. For status, use `ev.Status`: never nil, shallow-merged across frames and reconnects. After `REALTIME_DISCONNECTED`, `Next` returns `io.EOF`. The stream no longer dies after 30s, reconnects wait at least 5.5s after the previous open, and the open is bounded by the client timeout until the headers arrive.
- **Timeouts:** v2 used `http.Client{Timeout: 30s}` for everything. v3 applies a default deadline only when `ctx` has none: the client timeout (`WithTimeout`, 30s) for JSON calls, at least 2 minutes for start/stop/restart, database create and snapshot create/restore (the API holds them up to 95s), at least 2 minutes for `AI.Chat` (the AI gateway answers 503 `server_overloaded` after 90s), and none for uploads, file writes over 1 MiB of content and downloads. `WithTimeout(0)` disables all of them.
- **Empty network windows:** `Analytics`, `Errors` and `Performance` return `nil` pointers when the API answers `{}`.
- **Headers:** every API request sends `Accept: application/json` (`text/event-stream` for realtime). The default `User-Agent` changed from `Square GO` to `squarecloud-sdk-go/3.0.0` (`WithUserAgent` still overrides it).
- **Ids:** every id is now percent-encoded as one path segment (v2 pasted it into the path as is), and an empty, `.` or `..` id fails locally with `INVALID_ID`.
- **File writes:** v2 always sent the content as a string, which corrupted binary files, and could not write an empty file. v3 always sends the content base64-encoded (`{path, content, encoding: "base64"}`), so every byte round-trips, empty content writes an empty file, and content over 10 MB fails locally with `FILE_TOO_LARGE`. The API answers 400 `INVALID_CONTENT` for content it cannot decode.
- **File reads:** v3 always asks for `?encoding=base64` and decodes it, instead of the JSON byte array v2 read (which the API has deprecated). A file over 10 MB is 413 `FILE_TOO_LARGE`.
- **File listing:** listing a directory that does not exist is 404 `FILE_NOT_FOUND`; it used to be an empty list.
- **Snapshots:** list entries carry `VersionID` and `URL` from the API; nothing is parsed out of `Key`.
- **Retries:** new. GET network errors, 503 `UPLOAD_BUSY`/`ANALYTICS_BUSY` and 503 `DATABASE_UNAVAILABLE` on GET are retried twice by default; `WithMaxRetries(0)` restores the v2 behavior. `DATABASE_UNAVAILABLE` can fire after a mutation has started, so the SDK never retries it on other methods; retry an idempotent mutation yourself if you want to.
- **Go version:** the `go` directive dropped from 1.24.2 to 1.22.
