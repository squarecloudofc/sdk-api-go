<div align="center">
  <img alt="Square Cloud Banner" src="https://cdn.squarecloud.app/png/github-readme.png">
</div>

<h1 align="center">sdk-api-go</h1>

<p align="center">A Go SDK for consuming the <a href="https://squarecloud.app" target="_blank">Square Cloud</a> API.</p>

<div align="center">
  <a href="https://pkg.go.dev/github.com/squarecloudofc/sdk-api-go/v2">
    <img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/squarecloudofc/sdk-api-go/v2.svg">
  </a>
  <img alt="License" src="https://img.shields.io/github/license/squarecloudofc/sdk-api-go">
</div>

## Installation

```bash
go get github.com/squarecloudofc/sdk-api-go/v2
```

Requires Go 1.24+. The SDK has **zero external dependencies**.

## Getting your API key

Go to the [Square Cloud](https://squarecloud.app) dashboard and log in, then
open `Dashboard` → `My Account` → `Regenerate API/CLI KEY` and copy the key.

## Documentation

Visit the [official documentation](https://docs.squarecloud.app/en/api-reference) for the full API reference.

## Quick start

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/squarecloudofc/sdk-api-go/v2/rest"
	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

func main() {
	client := rest.NewClient(os.Getenv("SQUARECLOUD_API_KEY"))
	defer client.Close()

	api := rest.New(client)

	// Fetch your account (user + applications + databases)
	user, err := api.SelfUser()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Hi %s! Plan: %s\n", user.Name, user.Plan.Name)

	// Control an application by ID
	appID := "abc123def456abc123def456"

	if err := api.PostApplicationSignal(appID, squarecloud.ApplicationSignalRestart); err != nil {
		log.Fatal(err)
	}

	logs, err := api.GetApplicationLogs(appID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(logs.Logs)
}
```

## Features

The client exposes the full v2 platform surface:

- **User & service** — `SelfUser`, `UserSnapshots`, `ServiceStatus`
- **Applications** — `GetApplications`, `PostApplications` (upload), `GetApplication`, `GetApplicationStatus[Raw]`, `GetApplicationListStatus`, `GetApplicationLogs`, `GetApplicationMetrics`, `PostApplicationSignal` (start/stop/restart), `PostApplicationCommit`, `DeleteApplication`, `ApplicationRealtime` (SSE)
  - **Files** — `GetApplicationFiles`, `ReadApplicationFile`, `PutApplicationFile`, `MoveApplicationFile`, `DeleteApplicationFile`
  - **Envs** — `GetApplicationEnvs`, `SetApplicationEnvs`, `ReplaceApplicationEnvs`, `DeleteApplicationEnvs`
  - **Snapshots** — `GetApplicationSnapshots`, `CreateApplicationSnapshot`, `RestoreApplicationSnapshot`
  - **Deploys** — `GetApplicationDeployments`, `GetApplicationCurrentDeployment`, `PostApplicationDeployWebhook`, `LinkApplicationGithubApp`, `UnlinkApplicationGithubApp`
  - **Network** *(websites only)* — `GetApplicationDNS`, `SetApplicationCustomDomain`, `GetApplicationAnalytics`, `GetApplicationNetworkErrors`, `GetApplicationNetworkLogs`, `GetApplicationNetworkPerformance`, `PurgeApplicationCache`
  - **Domains** — `GetApplicationDomains`, `GetLoadBalancers`
- **Databases** — `GetDatabases`, `CreateDatabase`, `GetDatabase`, `UpdateDatabase`, `DeleteDatabase`, `GetDatabaseStatus[Raw]`, `GetDatabaseListStatus`, `GetDatabaseMetrics`, `StartDatabase`, `StopDatabase`
  - **Credentials** — `GetDatabaseCertificate`, `ResetDatabaseCredentials`
  - **Snapshots** — `GetDatabaseSnapshots`, `CreateDatabaseSnapshot`, `RestoreDatabaseSnapshot`
- **Workspaces** — `GetWorkspaces`, `CreateWorkspace`, `GetWorkspace`, `DeleteWorkspace`, `LeaveWorkspace`
  - **Members** — `AddWorkspaceMember`, `UpdateWorkspaceMember`, `RemoveWorkspaceMember`, `GetWorkspaceInviteCode`
  - **Applications** — `AddWorkspaceApplication`, `RemoveWorkspaceApplication`

## Usage

### Uploading and committing

```go
file, _ := os.Open("app.zip")
defer file.Close()

uploaded, err := api.PostApplications(file)
fmt.Println(uploaded.ID, uploaded.Language.Name)

// Commit into a subdirectory with ?path=
commit, _ := os.Open("patch.zip")
defer commit.Close()

err = api.PostApplicationCommit(appID, commit, rest.WithQueryParam("path", "src"))
```

### Environment variables

```go
envs, err := api.SetApplicationEnvs(appID, squarecloud.EnvVars{
	"DATABASE_URL": "postgres://...",
})

err = api.DeleteApplicationEnvs(appID, []string{"OLD_KEY"})
```

### Edge analytics

```go
end := time.Now()
start := end.Add(-24 * time.Hour)

analytics, err := api.GetApplicationAnalytics(appID, start, end,
	rest.WithQueryParam("country", "BR"), // optional drill-down filter
)
```

### Realtime (SSE)

```go
stream, err := api.ApplicationRealtime(appID)
if err != nil {
	log.Fatal(err)
}
defer stream.Close()

for {
	event, err := stream.Next()
	if err != nil {
		break // io.EOF after the 10-minute TTL — reconnect if needed
	}
	fmt.Println(event.Event, event.Data)
}
```

### Contexts, cancellation and errors

Every method accepts request options:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

status, err := api.GetApplicationStatus(appID, rest.WithContext(ctx))

var apiErr *rest.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Code, apiErr.StatusCode) // e.g. APP_NOT_FOUND 404
}

if rest.IsRateLimit(err) {
	// back off and retry
}
```

## Contributing

Bug reports and PRs welcome at [`squarecloudofc/sdk-api-go`](https://github.com/squarecloudofc/sdk-api-go).

## Authors

- [@joaootavios](https://github.com/joaootavios)
- [@richaardev](https://github.com/richaardev)
