<div align="center">
  <img alt="Square Cloud Banner" src="https://cdn.squarecloud.app/png/github-readme.png">
</div>

<h1 align="center">sdk-api-go</h1>

<p align="center">The official Go SDK for the <a href="https://squarecloud.app" target="_blank">Square Cloud</a> API.</p>

<div align="center">
  <a href="https://pkg.go.dev/github.com/squarecloudofc/sdk-api-go/v3"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/squarecloudofc/sdk-api-go/v3.svg"></a>
  <img alt="License" src="https://img.shields.io/github/license/squarecloudofc/sdk-api-go">
</div>

- **Zero runtime dependencies**: only the Go standard library.
- Runs on **Go 1.22+**; every method takes a `context.Context`, and a `*Client` is safe for concurrent use.
- Covers all **67 operations** of the Square Cloud API, checked against the [pinned spec](spec/openapi.json) on every CI run.
- Uploads and snapshot downloads **stream**; realtime logs and status come as an **iterator** (`Next`).
- **One error type**, `*APIError`, for every API, network and local failure.

[Documentation](https://docs.squarecloud.app/en/sdks/go/client) · [Releases](https://github.com/squarecloudofc/sdk-api-go/releases) · [Migration guide](MIGRATION.md)

## Installation

```bash
go get github.com/squarecloudofc/sdk-api-go/v3
```

Requires Go 1.22 or newer.

## API key

Create one at [squarecloud.app/account/security](https://squarecloud.app/account/security). A key can be limited to scopes (`apps:read`, `apps:deploy`, ...) and to specific apps or databases. A call outside those limits returns an `*APIError` with 403 `MISSING_SCOPE` or `RESOURCE_NOT_ALLOWED`, and list endpoints return only the resources the key can see.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/squarecloudofc/sdk-api-go/v3"
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

`ctx` is the first argument of every method and ids come next. A `*Client` is safe for concurrent use; build it once. More in [`example_test.go`](example_test.go).

## Configuration

```go
c := squarecloud.New(key,
	squarecloud.WithBaseURL(url),            // default squarecloud.DefaultBaseURL
	squarecloud.WithTimeout(30*time.Second), // default 30s
	squarecloud.WithMaxRetries(2),           // default 2
	squarecloud.WithHTTPClient(hc),          // default &http.Client{}; don't set hc.Timeout, it cuts streams
	squarecloud.WithUserAgent("my-tool/1"),  // default "squarecloud-sdk-go/<Version>"
)
```

| Option | Default | Notes |
|---|---|---|
| `apiKey` (1st argument of `New`) | required | Sent raw in `Authorization`. `New` cannot return an error, so an empty or whitespace-only key fails every call except `Service.Status` locally with `INVALID_API_KEY`. |
| `WithBaseURL(u)` | `DefaultBaseURL` = `https://api.squarecloud.app/v2` | A trailing `/` is stripped. |
| `WithTimeout(d)` | 30s | Only when `ctx` has no deadline. `<= 0` disables every default deadline, floors included. See [Retries, timeouts and rate limits](#retries-timeouts-and-rate-limits). |
| `WithMaxRetries(n)` | 2 | Negative counts as 0. |
| `WithHTTPClient(hc)` | `&http.Client{}` | `nil` keeps the default. |
| `WithUserAgent(ua)` | `squarecloud-sdk-go/<Version>` | Replaces the whole `User-Agent` header. |

The SDK never logs. To trace requests, wrap the `http.RoundTripper` of the client you pass to `WithHTTPClient`.

## API

Every app id may also be the composite `"<appId>-<workspaceId>"` to act on an app shared with you through a workspace.

| Group | Methods |
|---|---|
| `c.Account` | `Me`, `Snapshots(scope)` |
| `c.Service` | `Status` (public, no key needed) |
| `c.AI` | `Chat` (OpenAI-compatible, non-streaming) |
| `c.Apps` | `Create(zip)`, `Get`, `Delete`, `StatusAll(workspaceID)`, `Status`, `StatusRaw`, `Start`, `Stop`, `Restart`, `Logs`, `Metrics`, `Realtime`, `Domains`, `LoadBalancers`, `Commit(id, r, path, filename)` |
| `c.Apps.Deploys` | `SetWebhook(id, accessToken)`, `LinkGithubApp(id, repository, branch)`, `UnlinkGithubApp`, `List`, `Current` |
| `c.Apps.Envs` | `Get`, `Set(id, envs)` (merge), `Replace(id, envs)`, `Delete(id, keys...)` |
| `c.Apps.Files` | `List(id, path)`, `Read(id, path)` → `[]byte`, `Write(id, path, content)`, `Move(id, path, to)`, `Delete(id, path)` |
| `c.Apps.Snapshots` | `List`, `Create`, `Restore(id, name, versionID)` |
| `c.Apps.Network` | `Analytics(id, start, end, filters)`, `Errors(id, start, end, include4xx)`, `Logs(id, start, end)`, `Performance(id, start, end)`, `DNS`, `SetDomain(id, domain)`, `PurgeCache` |
| `c.Databases` | `Create(DatabaseCreate)`, `Get`, `Update(id, DatabaseUpdate)`, `Delete`, `Start`, `Stop`, `Status`, `StatusRaw`, `Metrics`, `StatusAll`, `Certificate`, `ResetCredentials(id, ResetPassword \| ResetCertificate)` → the new password, or `""` |
| `c.Databases.Snapshots` | `List`, `Create`, `Restore(id, name, versionID)` |
| `c.Workspaces` | `Create(name)`, `List`, `Get`, `Delete`, `Leave` |
| `c.Workspaces.Members` | `Add(workspaceID, code, group)`, `Update(workspaceID, memberID, group)`, `Remove(workspaceID, memberID)`, `InviteCode` |
| `c.Workspaces.Apps` | `Add(workspaceID, appID)`, `Remove(workspaceID, appID)` |
| `c` | `DownloadSnapshot(ctx, url, w)` |

Types are structs with `json` tags named after the API's fields (`CreatedAt` for `created_at`), so the [API reference](https://docs.squarecloud.app/api-reference) applies as-is. `start`/`end` are `time.Time`, sent as RFC 3339 in UTC. Empty, `.` and `..` ids are rejected with `INVALID_ID` before sending, because they would reach another route. `Network.Analytics`, `Errors` and `Performance` return `nil` for a window with no traffic. Fields the API can send as `null` are pointers. `Metrics` come newest first, as the API sends them. An analytics provider reads `"NAME (ASN)"` (e.g. `"GOOGLE (15169)"`), and the `Provider` filter takes that exact value; a filter the API rejects is 400 `INVALID_FILTER`.

## Usage

### Uploads

`Apps.Create` and `Apps.Commit` take an `io.Reader` and stream it as multipart (head + file + tail), never buffered; any `io.Seeker` gets a `Content-Length`. A commit unpacks a `.zip` at `path` (`""` is the app root); any other file lands at `path/<filename>`. The filename is the `filename` argument, else the `*os.File`'s own name, else `app.zip` on create and `commit.zip` on commit. A zip over 100 MB fails locally with `FILE_TOO_LARGE`: before sending when the size is known, otherwise as soon as the stream passes 100 MB. Uploads have no default deadline: cancel them with `ctx`.

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
err = c.Apps.Commit(ctx, app.ID, patch, "src", "") // lands at src/main.py
```

### Files

```go
data, err := c.Apps.Files.Read(ctx, appID, "/package.json")
err = c.Apps.Files.Write(ctx, appID, "/logo.png", pngBytes)
err = c.Apps.Files.Move(ctx, appID, "/logo.png", "/assets/logo.png")
```

File content travels base64-encoded both ways, so text and binary files round-trip byte for byte: `Read` asks for `?encoding=base64` and returns the decoded bytes, and `Write` always sends `{path, content: <base64>, encoding: "base64"}` (about 1.33x the content on the wire). Empty content creates an empty file. Content over 10 MB fails locally with `FILE_TOO_LARGE` (the API answers 413 too, and 400 `INVALID_CONTENT` for content it cannot decode), and content over 1 MiB gets no default deadline, like an upload. `Read` of a file over 10 MB is 413 `FILE_TOO_LARGE`. `List` of a missing directory is 404 `FILE_NOT_FOUND`, a protected path is 403 `BLOCKED_PATH`, and paths are at most 256 characters.

### Snapshots

`Create` returns `Pending: true` while the API is still generating the snapshot (HTTP 202 `SNAPSHOT_PROCESSING`). It then appears in `List` on its own, usually within 2 minutes: poll `List`, and never call `Create` again, which is limited to one per 180 seconds and counts against the plan's daily snapshot quota.

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
	err = c.DownloadSnapshot(ctx, snap.URL, out) // streamed, nothing buffered
	if err != nil {
		log.Fatal(err)
	}
}

list, err := c.Apps.Snapshots.List(ctx, appID)
err = c.Apps.Snapshots.Restore(ctx, appID, list[0].Name, list[0].VersionID)
```

Every listed `Snapshot` carries the API's `VersionID` (what `Restore` takes) and `URL` (a signed download link, valid for 30 days, for `DownloadSnapshot`). A failed restore is 404 `SNAPSHOT_RESTORE_FAILED`. `DownloadSnapshot` never sends the API key to the snapshot host, never puts the URL in its errors, and has no default deadline (cancel `ctx`).

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
		break // io.EOF on a normal end
	}
	switch ev.Event {
	case "logs":
		fmt.Println(ev.Stream, ev.Line) // stdout | stderr
	case "status":
		fmt.Println(ev.Status.CPU) // never nil: the full, merged state
	default: // "system" or "error"
		fmt.Println(ev.Data) // a code such as REALTIME_DISCONNECTED
	}
}
```

Every event has `Event`, `Data` (the raw frame text) and `ID`. The HTTP status is checked before streaming, so 429 `REALTIME_MAX_CONNECTIONS` is returned by `Realtime`. A dropped connection, or the API's `REALTIME_RECONNECT` hand-off, reopens up to 3 times in a row (a `logs` or `status` event resets the count), each reopen at least 5.5 s after the previous open to stay under the API's pace of one per 5 s; past that `Next` returns `NETWORK_ERROR`. The open is timed until the response headers arrive; the stream itself is bounded only by `ctx`. The loop ends on a clean close (10-minute server limit), on `REALTIME_DISCONNECTED`, or when `ctx` is canceled. Max 5 concurrent streams per account and 30 per app.

### GitHub deploys

```go
// A GitHub webhook: returns its URL ("" when removed with "@")
url, err := c.Apps.Deploys.SetWebhook(ctx, appID, "ghp_xxx")

// Or the Square Cloud GitHub App
repo, err := c.Apps.Deploys.LinkGithubApp(ctx, appID, "octocat/hello-world", "main")
fmt.Println(repo.ID, repo.FullName, repo.Branch)

cur, err := c.Apps.Deploys.Current(ctx, appID) // zero value when nothing is set
err = c.Apps.Deploys.UnlinkGithubApp(ctx, appID)
```

Linking needs scope `apps:deploy` and a GitHub account connected to Square Cloud (403 `GITHUB_NOT_CONNECTED` otherwise), and the repository must belong to a GitHub App installation your connected GitHub account holds (403 `REPOSITORY_NOT_AVAILABLE` otherwise) and be writable by it (403 `REPOSITORY_PERMISSION_REQUIRED`). 502 `FAILED_TO_FETCH` means GitHub did not confirm the branch; it is safe to retry. A repository and branch already linked to another app, of any account, is 409 `REPOSITORY_BRANCH_ALREADY_CONFIGURED`; its `Message` names that app only when it is yours. Re-linking needs an unlink first (400 `GIT_ALREADY_CONFIGURED`), and unlinking without a link is 400 `GIT_NOT_CONFIGURED`. Link and unlink share a limit of 3 calls per 60 s.

## Errors

Every API, network and local failure is an `*APIError` with `Status`, `Code`, `Message`, `Method`, `Path` (`/v2/apps/...`, never the query string) and the cause in `Unwrap()`. Caller cancellation is detectable with `errors.Is(err, context.Canceled)`. Only caller-side problems are plain errors: a nil upload reader, an unparsable snapshot or base URL, an input `encoding/json` rejects, and a failing `io.Writer` in `DownloadSnapshot`.

```go
_, err := c.Apps.Get(ctx, appID)
var apiErr *squarecloud.APIError
if errors.As(err, &apiErr) && apiErr.Code == squarecloud.CodeMissingScope {
	fmt.Println(apiErr.Message) // which scope the key lacks
}
```

- No response: `Status` `0` with `Code` `NETWORK_ERROR` (original error in `Unwrap`) or `TIMEOUT`.
- Local checks, nothing sent: `Status` `0` with `FILE_TOO_LARGE`, `INVALID_ID` for an id that is empty, `.` or `..`, or `INVALID_API_KEY` for an empty key.
- `Message` is the server's explanation, or `""` when it sent only a code. A body without a code (a proxy page, a failed snapshot download) is `UNKNOWN_ERROR` with the message `HTTP <status>`; a 2xx body that is not JSON is `UNKNOWN_ERROR` with `Invalid JSON in HTTP <status> response`.
- An API key that is missing, unknown, revoked or expired is 401 `ACCESS_DENIED`.
- Start/stop of apps and databases (and app restart) refused by the cluster is 409 with a code and no message: `CONTAINER_ALREADY_STARTED`, `CONTAINER_ALREADY_STOPPED`, `CONTAINER_TEMPORARILY_SUSPENDED` (apps), `CONTAINER_NOT_FOUND`, `CONTAINER_INSUFFICIENT_DISK_SPACE`, `CONTAINER_NETWORK_CONFLICT` or `ACTION_FAILED`. The SDK returns an "already started/stopped" answer as an error like any other; treat it as success in your code if that is what you need.
- A 2xx reply whose body is `{"status": "error"}` also fails; a 202 `SNAPSHOT_PROCESSING` stays `SnapshotCreated{Pending: true}`.
- Every `AI.Chat` error, auth, 429 and 503 included, is OpenAI-shaped, and `Code` is its lowercase `code` (`access_denied`, `upgrade_required`, `rate_limit_exceeded`, `database_unavailable`, `server_overloaded`, ...), or its `type` when there is no code.
- `Error()` is `squarecloud: <METHOD> <path>: HTTP <status> <CODE>: <message>`, without `HTTP <status>` when the status is `0` and without `: <message>` when it is empty. There is a `Code*` constant for every code the API documents (checked against the spec in CI) and for the SDK's own; the API may add codes, so handle an unknown one by its HTTP status.

## Retries, timeouts and rate limits

- **Timeouts:** `WithTimeout` (30 s) applies only when `ctx` has no deadline, and one deadline covers every attempt. Calls the server holds open wait at least 120 s (`Start`/`Stop`/`Restart` of apps and databases, `Databases.Create`, snapshot `Create`/`Restore`), and `AI.Chat` too (the AI gateway gives the whole request one 90 s deadline, then answers 503 `server_overloaded`, which is safe to retry but not retried by the SDK); a larger timeout wins. `WithTimeout(0)` disables every default deadline, floors included. Uploads, `Files.Write` above 1 MiB and `DownloadSnapshot` have none (cancel `ctx`); `Realtime` is timed only until it opens.
- **Retries:** the SDK retries only what is safe: network errors on `GET` (including a body cut off mid-read, realtime opens and `DownloadSnapshot` before the response), and 503 `UPLOAD_BUSY`/`ANALYTICS_BUSY` (plus `DATABASE_UNAVAILABLE` on `GET` only: it can fire after a mutation has started, so the SDK never repeats one; you may retry an idempotent mutation yourself), up to `WithMaxRetries` (2) times with exponential backoff (500 ms·2ⁿ, jitter, max 8 s). An upload is retried only when its body can be replayed (an `io.ReaderAt` with a known size, such as `*os.File`). Timeouts and 429 are never retried.
- **Rate limits:** every account has a global limit of requests per 60 s, set by its plan ([values](https://docs.squarecloud.app/en/api-reference/limitations-and-restrictions)). Going over it, or over a route's own limit, returns 429 `RATE_LIMITED` or `KEEP_CALM`. `RATE_LIMITED` can block the account, API key or IP for 30 minutes; it also replaces `RATE_LIMIT_EXCEEDED` on the app network endpoints and `GET /v2/users/snapshots`. `CodeRateLimit` and `CodeRateLimitExceeded` stay, deprecated. The API sends no `Retry-After`, which is why the SDK never retries a 429.

## Development

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...
go test -race -count=1 ./...   # offline: httptest on loopback only
```

The offline suites (`transport_test.go`, `streams_test.go`, `conformance_test.go`) never reach the API. The live suite runs all 67 operations against the real API:

```bash
SQUARECLOUD_API_KEY=... go test -tags live -run TestLive -count=1 -timeout 30m .
```

> **Warning:** the live suite creates and deletes real resources (an app, a database and a workspace named `sdk-live-go-<timestamp>`) on the key's account. It is skipped without `SQUARECLOUD_API_KEY` and never runs in CI. Requests are spaced 2.1 s apart; run the JS, Python and Go live suites one after another, never at the same time.

## Contributing

Issues and pull requests are welcome at [squarecloudofc/sdk-api-go](https://github.com/squarecloudofc/sdk-api-go).

## License

MIT, see [LICENSE](LICENSE).

## Authors

Maintained by [Square Cloud](https://squarecloud.app).

Contributors:

- João Otávio Stivi ([@JoaoOtavioS](https://github.com/JoaoOtavioS))
- Richard Martins ([@richaardev](https://github.com/richaardev))
