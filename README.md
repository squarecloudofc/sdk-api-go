<div align="center">
  <img alt="Square Cloud Banner" src="https://cdn.squarecloud.app/png/github-readme.png">
</div>

<h1 align="center">sdk-api-go</h1>

<p align="center">The official Go SDK for the <a href="https://squarecloud.app" target="_blank">Square Cloud</a> API.</p>

<div align="center">
  <a href="https://pkg.go.dev/github.com/squarecloudofc/sdk-api-go/v4"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/squarecloudofc/sdk-api-go/v4.svg"></a>
  <img alt="License" src="https://img.shields.io/github/license/squarecloudofc/sdk-api-go">
</div>

- **Zero runtime dependencies.**
- Runs on **Go 1.22** or newer. Every method takes a `context.Context`, and a `*Client` is safe for concurrent use.
- Covers all **69 operations** of the Square Cloud API, checked against the pinned OpenAPI spec on every CI run.
- Uploads and snapshot downloads **stream** without loading files into memory, and realtime logs and usage arrive through an **iterator** (`Next`).
- **One error type**, `*APIError`, for every API, network and local failure.

[Documentation](https://docs.squarecloud.app/en/sdks/go/client) · [Releases](https://github.com/squarecloudofc/sdk-api-go/releases) · [Migration guide](MIGRATION.md)

## Installation

```bash
go get github.com/squarecloudofc/sdk-api-go/v4
```

The SDK needs Go 1.22 or newer.

## API key

Create an API key at [squarecloud.app/account/security](https://squarecloud.app/account/security). You can limit a key to some scopes, such as `apps:read` or `apps:deploy`, and to some applications or databases. A call outside those limits returns an `*APIError` with 403 `MISSING_SCOPE` or `RESOURCE_NOT_ALLOWED`, and listings return only the resources that the key can see.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/squarecloudofc/sdk-api-go/v4"
)

func main() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	me, err := c.Account.Me(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Hi %s, you have %d apps\n", me.User.Name, len(me.Applications))
	if len(me.Applications) == 0 {
		return
	}

	appID := me.Applications[0].ID
	if err := c.Apps.Restart(ctx, appID); err != nil {
		log.Fatal(err)
	}
	logs, err := c.Apps.Logs(ctx, appID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(logs)
}
```

Every method takes `ctx` first, then the ids. Create one `*Client` and share it, because it is safe for concurrent use. [`example_test.go`](example_test.go) has more examples.

## Configuration

```go
c := squarecloud.New(key,
	squarecloud.WithBaseURL(url),            // default squarecloud.DefaultBaseURL
	squarecloud.WithTimeout(30*time.Second), // default 30 seconds
	squarecloud.WithMaxRetries(2),           // default 2
	squarecloud.WithHTTPClient(hc),          // default &http.Client{}
	squarecloud.WithUserAgent("my-tool/1"),  // default "squarecloud-sdk-go/<Version>"
)
```

| Option | Default | Notes |
|---|---|---|
| `apiKey`, the first argument of `New` | required | `New` cannot return an error. With an empty or whitespace-only key, every call except `Service.Status` fails with `INVALID_API_KEY` without sending anything. |
| `WithBaseURL(u)` | `DefaultBaseURL` | Sends the requests to a proxy or a test server instead of the Square Cloud API. A trailing `/` is removed. |
| `WithTimeout(d)` | 30 seconds | Applies only when `ctx` has no deadline. A value of 0 or less turns every time limit off, including the 2-minute minimum of long calls. See [Retries, timeouts and rate limits](#retries-timeouts-and-rate-limits). |
| `WithMaxRetries(n)` | 2 | A negative `n` counts as 0. |
| `WithHTTPClient(hc)` | `&http.Client{}` | `nil` keeps the default. Do not set `hc.Timeout`, because it would cut realtime streams and large downloads. |
| `WithUserAgent(ua)` | `squarecloud-sdk-go/<Version>` | Replaces the whole `User-Agent` header. |

The SDK never logs. To trace requests, wrap the `http.RoundTripper` of the client you pass to `WithHTTPClient`.

## API

Wherever a method takes an application id, you can pass the composite id `"<appId>-<workspaceId>"` instead. It acts on an application that another account shares with you through a workspace.

| Group | Methods |
|---|---|
| `c.Account` | `Me`, `Snapshots(scope)` |
| `c.Service` | `Status` (works without an API key) |
| `c.AI` | `Chat` (OpenAI format, without streaming) |
| `c.Apps` | `Create(zip)`, `Get`, `Delete`, `StatusAll(workspaceID)`, `Status`, `StatusRaw`, `Start`, `Stop`, `Restart`, `Logs`, `Metrics(id, rng)`, `Realtime`, `Domains`, `LoadBalancers`, `Commit(id, r, path, filename)` |
| `c.Apps.Deploys` | `SetWebhook(id, accessToken)`, `LinkGithubApp(id, repository, branch)`, `UnlinkGithubApp`, `List`, `Current` |
| `c.Apps.Envs` | `Get`, `Set(id, envs)` (merge), `Replace(id, envs)`, `Delete(id, keys...)` |
| `c.Apps.Files` | `List(id, path)`, `Read(id, path)` → `[]byte`, `Write(id, path, content)`, `Move(id, path, to)`, `Delete(id, path)` |
| `c.Apps.Snapshots` | `List`, `Create`, `Restore(id, name, versionID)`, `RestoreState` |
| `c.Apps.Network` | `Analytics(id, start, end, filters)`, `Errors(id, start, end, include4xx)`, `Logs(id, start, end)`, `Performance(id, start, end)`, `DNS`, `SetDomain(id, domain)`, `PurgeCache` |
| `c.Databases` | `Create(DatabaseCreate)`, `Get`, `Update(id, DatabaseUpdate)`, `Delete`, `Start`, `Stop`, `Status`, `StatusRaw`, `Metrics(id, rng)`, `StatusAll`, `Certificate`, `ResetCredentials(id, ResetPassword \| ResetCertificate)` → the new password, or `""` |
| `c.Databases.Snapshots` | `List`, `Create`, `Restore(id, name, versionID)`, `RestoreState` |
| `c.Workspaces` | `Create(name)`, `List`, `Get`, `Delete`, `Leave` |
| `c.Workspaces.Members` | `Add(workspaceID, code, group)`, `Update(workspaceID, memberID, group)`, `Remove(workspaceID, memberID)`, `InviteCode` |
| `c.Workspaces.Apps` | `Add(workspaceID, appID)`, `Remove(workspaceID, appID)` |
| `c` | `DownloadSnapshot(ctx, url, w)` |

- Types are structs whose fields follow the API's names, such as `CreatedAt` for `created_at`, so the [API reference](https://docs.squarecloud.app/api-reference) applies as it is. Fields that the API can send as `null` are pointers.
- `start` and `end` are `time.Time` values, and the SDK sends them in UTC.
- An id that is empty, `.` or `..` fails with `INVALID_ID` before anything is sent, because it would reach a different operation.
- `Network.Analytics`, `Errors` and `Performance` return `nil` when nothing was served in the period. An analytics provider is written as `"NAME (ASN)"`, such as `"GOOGLE (15169)"`, and the `Provider` filter takes that exact value. A filter that the API does not accept returns 400 `INVALID_FILTER`.
- `Metrics` returns the newest point first. `rng` is `"24h"`, the default that `""` also gets, or `"7d"`. With `"7d"`, the Pro and Enterprise plans get hourly points with the hour's average and peaks (`CPUMax`, `RAMMax`) for applications of 512 MB or more. Other plans and smaller applications get the 24-hour answer without an error, so check the points instead of assuming 7 days. Any other `rng` returns 400 `INVALID_PARAMETERS`. Applications below 512 MB get a point every 10 minutes, where they used to get 400 `METRICS_NOT_SUPPORTED`.
- `Service.Status` also lists the open `Incidents` and the `Components` that are not fully working. Its `Status` `"unknown"` means that the check could not run, not that the platform is down.

## Usage

### Uploads

`Apps.Create` and `Apps.Commit` take an `io.Reader` and stream it, so the file is never loaded into memory. A commit unpacks a `.zip` into `path`, where `""` is the application's root directory, and writes any other file to `path/<filename>`. The file name is the `filename` argument, or else the name of the `*os.File`, or else `app.zip` for a create and `commit.zip` for a commit. A zip over 100 MB fails with `FILE_TOO_LARGE`. When its size is known, as for an `*os.File`, nothing is sent. Otherwise the upload stops as soon as it passes 100 MB. Uploads have no time limit, so cancel `ctx` to stop one.

```go
f, err := os.Open("app.zip")
if err != nil {
	log.Fatal(err)
}
defer f.Close()
app, err := c.Apps.Create(ctx, f)
if err != nil {
	log.Fatal(err)
}

patch, err := os.Open("main.py")
if err != nil {
	log.Fatal(err)
}
defer patch.Close()
err = c.Apps.Commit(ctx, app.ID, patch, "src", "") // The file lands at src/main.py.
```

### Files

```go
data, err := c.Apps.Files.Read(ctx, appID, "/package.json")
err = c.Apps.Files.Write(ctx, appID, "/logo.png", pngBytes)
err = c.Apps.Files.Move(ctx, appID, "/logo.png", "/assets/logo.png")
```

`Read` and `Write` keep text and binary files intact, byte for byte, and empty content creates an empty file. `Write` fails with `FILE_TOO_LARGE` before sending content over 10 MB, so upload larger files with `Apps.Commit`. The API returns 400 `WRITE_FAILED` when it cannot write the file. Content over 1 MiB has no time limit, like an upload. `Read` returns 413 `FILE_TOO_LARGE` for a file over 10 MB, and 400 `READ_FAILED` for a file the server cannot read. `List` returns 404 `FILE_NOT_FOUND` for a missing directory and 403 `BLOCKED_PATH` for a protected one. Paths are at most 256 characters long.

### Snapshots

`Create` returns `Pending: true` while the snapshot is still being taken, which is not an error. The snapshot then appears in `List` on its own, usually within 2 minutes. Poll `List` and do not call `Create` again, because `Create` allows one call every 3 minutes and each call counts against your plan's daily snapshot quota.

```go
snap, err := c.Apps.Snapshots.Create(ctx, appID)
if err != nil {
	log.Fatal(err)
}
if !snap.Pending {
	out, err := os.Create("backup.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	err = c.DownloadSnapshot(ctx, snap.URL, out) // Streamed to the file, never held in memory.
	if err != nil {
		log.Fatal(err)
	}
}

list, err := c.Apps.Snapshots.List(ctx, appID)
res, err := c.Apps.Snapshots.Restore(ctx, appID, list[0].Name, list[0].VersionID)
st := squarecloud.SnapshotRestoreState{State: "success"}
for res.Pending { // Still running. Follow it, and never call Restore again.
	time.Sleep(10 * time.Second)
	if st, err = c.Apps.Snapshots.RestoreState(ctx, appID); err != nil {
		log.Fatal(err)
	}
	res.Pending = st.State == "queued" || st.State == "running"
}
switch st.State {
case "success":
	fmt.Println("restored")
case "error":
	log.Fatal("restore failed: ", st.Code)
default: // "none" means that the result is no longer available.
	fmt.Println("restore result unknown")
}
```

Every listed `Snapshot` has a `VersionID`, which `Restore` takes, and a `URL`, a download link for `DownloadSnapshot` that is valid for 1 hour. Only `Create` can return an empty `URL` and `Key`, when the API key lacks the `snapshots:read` scope. Listings return 503 `SNAPSHOTS_UNAVAILABLE` while snapshot storage cannot answer, where they used to return an empty list. `DownloadSnapshot` never sends your API key with the download and never puts the link in its errors. It has no time limit, so cancel `ctx` to stop it.

A restore that takes longer than about 90 seconds keeps running after the call returns. `Restore` then returns `Pending: true`, which is not an error. `RestoreState` reports the last restore. Its state is `queued` or `running` while the restore runs, with the current `Step`, and then `success` or `error`, with a `Code`, for 1 hour after it ends. It is `none` when there was no recent restore. `RestoreState` allows 30 calls a minute, so poll it every few seconds.

A restore can fail with these errors:

- 400 `MISSING_PARAMETERS`, `INVALID_SNAPSHOT_ID` or `INVALID_VERSION_ID`, and `SNAPSHOT_DATABASE_MISMATCH` for databases.
- 404 `APP_NOT_FOUND` or `DATABASE_NOT_FOUND`, `SNAPSHOT_NOT_FOUND`, `SNAPSHOT_RESTORE_FAILED`, `SNAPSHOT_CORRUPT`, `RESTORE_INTERRUPTED` or `CONTAINER_NOT_FOUND`.
- 409 `RESTORE_IN_PROGRESS` or `SNAPSHOT_IN_PROGRESS`, while another restore or a snapshot is running.
- 413 `PAYLOAD_TOO_LARGE`.
- 503 `CLUSTER_MAINTENANCE_TRY_LATER`, during maintenance.

Snapshot `Create` can also return 409 `RESTORE_IN_PROGRESS` and 503 `CLUSTER_MAINTENANCE_TRY_LATER`.

### Realtime

```go
s, err := c.Apps.Realtime(ctx, appID)
if err != nil {
	log.Fatal(err)
}
defer s.Close()
for {
	ev, err := s.Next()
	if err != nil {
		break // The error is io.EOF when the stream ends normally.
	}
	switch ev.Event {
	case "logs":
		fmt.Println(ev.Stream, ev.Line) // Stream is "stdout" or "stderr".
	case "status":
		fmt.Println(ev.Status.CPU) // Status is never nil and holds the full state.
	default: // A "system" or "error" event.
		fmt.Println(ev.Data) // A code such as REALTIME_DISCONNECTED.
	}
}
```

Every event has `Event`, `Data`, which is the text the server sent, and `ID`. When the stream cannot open, `Realtime` returns the error before any event, such as 429 `REALTIME_MAX_CONNECTIONS`. When the connection drops, or the server sends `REALTIME_RECONNECT`, the stream reopens by itself, up to 3 times in a row. A `logs` or `status` event resets that count. The API allows one open per application every 5 seconds, so each reopen waits until at least 5.5 seconds have passed since the previous one. When the reopens run out, `Next` returns `NETWORK_ERROR`. The client timeout applies only until the stream opens, and after that only `ctx` limits it. The loop ends when the server closes the stream after its 10-minute limit, after a `REALTIME_DISCONNECTED` event, or when you cancel `ctx`. You can have at most 5 streams open per account and 30 per application.

### GitHub deploys

```go
// Deploy from a GitHub webhook. SetWebhook returns the URL to add on GitHub,
// or "" after "@" removes the webhook.
url, err := c.Apps.Deploys.SetWebhook(ctx, appID, "ghp_xxx")

// Or deploy through the Square Cloud GitHub App.
repo, err := c.Apps.Deploys.LinkGithubApp(ctx, appID, "octocat/hello-world", "main")
fmt.Println(repo.ID, repo.FullName, repo.Branch)

cur, err := c.Apps.Deploys.Current(ctx, appID) // The zero value when nothing is set up.
err = c.Apps.Deploys.UnlinkGithubApp(ctx, appID)
```

Linking needs the `apps:deploy` scope and a GitHub account connected to Square Cloud, or it returns 403 `GITHUB_NOT_CONNECTED`. Your GitHub account must have the Square Cloud GitHub App installed on the repository, or the call returns 403 `REPOSITORY_NOT_AVAILABLE`. It must also be able to write to the repository, or the call returns 403 `REPOSITORY_PERMISSION_REQUIRED`. A 502 `FAILED_TO_FETCH` means that GitHub did not confirm the branch, and it is safe to try again. When another application, of any account, already deploys from the same repository and branch, the call returns 409 `REPOSITORY_BRANCH_ALREADY_CONFIGURED`. Its `Message` names that application only when it is yours. To link a different repository, unlink first, or the call returns 400 `GIT_ALREADY_CONFIGURED`. Unlinking without a link returns 400 `GIT_NOT_CONFIGURED`. Linking and unlinking share a limit of 3 calls a minute.

## Errors

Every API, network and local failure is an `*APIError` with these fields:

- `Status`, the HTTP status, or `0` when no response arrived.
- `Code`, such as `APP_NOT_FOUND`.
- `Message`, the server's explanation, or `""` when it sent only a code.
- `Method` and `Path`. `Path` never includes the query string.
- The cause, which `Unwrap()` returns.

When you cancel `ctx`, `errors.Is(err, context.Canceled)` reports true. A few mistakes in the arguments return a plain error instead. They are a nil upload reader, a snapshot link or base URL that cannot be parsed, a value that `encoding/json` cannot encode, and an error from the `io.Writer` given to `DownloadSnapshot`.

```go
_, err := c.Apps.Get(ctx, appID)
var apiErr *squarecloud.APIError
if errors.As(err, &apiErr) && apiErr.Code == squarecloud.CodeMissingScope {
	fmt.Println(apiErr.Message) // Names the scope that the key lacks.
}
```

- When no response arrives, `Status` is `0` and `Code` is `NETWORK_ERROR` or `TIMEOUT`. `Unwrap` returns the original error.
- Local checks send nothing and leave `Status` at `0`. They return `FILE_TOO_LARGE`, `INVALID_ID` for an id that is empty, `.` or `..`, and `INVALID_API_KEY` for an empty key. Their `Message` says what to fix.
- A response without a code, such as a proxy page or a failed snapshot download, gets `UNKNOWN_ERROR` with the message `HTTP <status>`. A successful response whose body is not JSON gets `UNKNOWN_ERROR` with the message `Invalid JSON in HTTP <status> response`.
- An API key that is missing, unknown, revoked or expired gets 401 `ACCESS_DENIED`.
- When `Start`, `Stop` or `Restart` cannot act, it returns 409 with a code and no message. The codes are `CONTAINER_ALREADY_STARTED`, `CONTAINER_ALREADY_STOPPED`, `CONTAINER_TEMPORARILY_SUSPENDED` for applications, `CONTAINER_NOT_FOUND`, `CONTAINER_INSUFFICIENT_DISK_SPACE`, `CONTAINER_NETWORK_CONFLICT`, `ACTION_FAILED` and `RESTORE_IN_PROGRESS`. The SDK returns an "already started" or "already stopped" answer as an error like any other, so treat it as success in your code when that is what you need.
- `RESTORE_IN_PROGRESS` is 409 while a snapshot restore holds the application or database. Delete, start and stop of applications and databases, application restart, and snapshot `Create` and `Restore` can return it. On delete and on database start and stop it used to be 403. A concurrent write to environment variables returns 409 `ENV_BUSY`. The SDK retries none of them.
- A successful response whose body reports an error fails too. A 202 is never an error. `SNAPSHOT_PROCESSING` becomes `SnapshotCreated{Pending: true}`, and `SNAPSHOT_RESTORE_PROCESSING` becomes `SnapshotRestored{Pending: true}`.
- Every `AI.Chat` error follows the OpenAI format, including authentication errors, 429 and 503. `Code` is its lowercase code, such as `access_denied`, `upgrade_required`, `rate_limit_exceeded`, `weekly_limit_reached`, `database_unavailable` or `server_overloaded`, or its type when there is no code.
- `Error()` returns `squarecloud: <METHOD> <path>: HTTP <status> <CODE>: <message>`. It leaves out `HTTP <status>` when the status is `0`, and `: <message>` when the message is empty.
- There is a `Code*` constant for every code that the API documents, checked against the spec in CI, and for the SDK's own codes. The API can add codes, so handle a code you do not know by its HTTP status.

## Retries, timeouts and rate limits

- **Timeouts.** `WithTimeout`, 30 seconds by default, applies only when `ctx` has no deadline, and one limit covers every attempt of a call. Calls that the API can hold open get at least 2 minutes. They are `Start`, `Stop` and `Restart` of applications and databases, `Databases.Create`, and snapshot `Create` and `Restore`. `AI.Chat` also gets at least 2 minutes. The AI answers within 90 seconds or returns 503 `server_overloaded`, which is safe to try again, but the SDK does not retry it. A longer timeout wins over the 2-minute minimum, and `WithTimeout(0)` turns every time limit off. Uploads, `Files.Write` over 1 MiB and `DownloadSnapshot` have no time limit, so cancel `ctx` to stop them. `Realtime` applies the timeout only until the stream opens.
- **Retries.** The SDK repeats a call only when that is safe, up to `WithMaxRetries` times, 2 by default. It waits longer before each new attempt, starting below half a second and doubling up to 8 seconds, with some randomness. It repeats network errors on GET requests, including a response cut off midway, a realtime open, and a snapshot download that has not started. It repeats 503 `UPLOAD_BUSY` and `ANALYTICS_BUSY` on any request, because the API refused those before doing any work. It repeats 503 `DATABASE_UNAVAILABLE` only on GET requests, because on other methods the change may already have started. You can repeat an idempotent change yourself. An upload is repeated only when its reader can be read again, such as an `*os.File`. The SDK never repeats a timeout or a 429. It does not repeat 503 `SNAPSHOTS_UNAVAILABLE`, for which the API asks you to wait about 30 seconds, or 503 `CLUSTER_MAINTENANCE_TRY_LATER`, which lasts minutes. It does not repeat a 409 from a busy resource either, such as `RESTORE_IN_PROGRESS`, `SNAPSHOT_IN_PROGRESS` or `ENV_BUSY`. You decide when to try those again.
- **Rate limits.** Every account has a limit of requests per minute, set by its plan ([values](https://docs.squarecloud.app/en/api-reference/limitations-and-restrictions)). Going over it, or over the limit of a single operation, returns 429 `RATE_LIMITED` or `KEEP_CALM`. `RATE_LIMITED` can block the account, API key or IP address for 30 minutes. It also replaces `RATE_LIMIT_EXCEEDED` on the network calls and on `Account.Snapshots`. `CodeRateLimit` and `CodeRateLimitExceeded` remain, marked deprecated. The API does not say when to try again, which is why the SDK never retries a 429.

## Development

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...
go test -race -count=1 ./...   # offline, against a local test server only
```

The offline suites (`transport_test.go`, `streams_test.go`, `conformance_test.go`) never reach the API. The live suite runs all 69 operations against the real API:

```bash
SQUARECLOUD_API_KEY=... go test -tags live -run TestLive -count=1 -timeout 30m .
```

> **Warning:** the live suite creates and deletes real resources on the key's account. They are an application, a database and a workspace named `sdk-live-go-<timestamp>`. The suite is skipped without `SQUARECLOUD_API_KEY` and never runs in CI. It spaces its requests 2.1 seconds apart, so run the JS, Python and Go live suites one after another, never at the same time.

## Contributing

Issues and pull requests are welcome at [squarecloudofc/sdk-api-go](https://github.com/squarecloudofc/sdk-api-go).

## License

MIT, see [LICENSE](LICENSE).

## Authors

Maintained by [Square Cloud](https://squarecloud.app).

Contributors:

- João Otávio Stivi ([@JoaoOtavioS](https://github.com/JoaoOtavioS))
- Richard Martins ([@richaardev](https://github.com/richaardev))
