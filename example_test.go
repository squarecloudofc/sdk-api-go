package squarecloud_test

// These examples are compiled and vetted by `go test` but never executed (no
// "Output:" comment), so they never call the real API.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/squarecloudofc/sdk-api-go/v3"
)

func Example() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	me, err := c.Account.Me(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(me.User.Name, me.User.Plan.Name, len(me.Applications))

	appID := "a1b2c3d4e5f60718293a4b5c6d7e8f90" // or "<appId>-<workspaceId>" for a shared app
	if err := c.Apps.Restart(ctx, appID); err != nil {
		var apiErr *squarecloud.APIError
		if errors.As(err, &apiErr) {
			log.Fatalf("%s (HTTP %d): %s", apiErr.Code, apiErr.Status, apiErr.Message)
		}
		log.Fatal(err)
	}
}

func ExampleAppsAPI_Create() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	f, err := os.Open("app.zip") // streamed, never loaded in memory
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	app, err := c.Apps.Create(ctx, f)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(app.ID, app.Language.Name)

	// Commit one file: a non-zip lands at <path>/<filename>.
	src, err := os.Open("index.js")
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	if err := c.Apps.Commit(ctx, app.ID, src, "src", ""); err != nil {
		log.Fatal(err)
	}

	// Deploy on push through the Square Cloud GitHub App (scope apps:deploy).
	repo, err := c.Apps.Deploys.LinkGithubApp(ctx, app.ID, "octocat/hello-world", "main")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(repo.FullName, repo.Branch)
	if err := c.Apps.Deploys.UnlinkGithubApp(ctx, app.ID); err != nil {
		log.Fatal(err)
	}
}

func ExampleAppsAPI_Realtime() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	s, err := c.Apps.Realtime(ctx, "a1b2c3d4e5f60718293a4b5c6d7e8f90")
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	for {
		ev, err := s.Next()
		if err != nil {
			return // io.EOF when the server ends the stream
		}
		switch ev.Event {
		case "logs":
			fmt.Printf("[%s] %s\n", ev.Stream, ev.Line)
		case "status": // Status is never nil on a status event
			fmt.Printf("cpu %.1f%% ram %.0f/%.0f MB\n", ev.Status.CPU, ev.Status.RAM[0], ev.Status.RAM[1])
		default: // system, error
			fmt.Println(ev.Event, ev.Data)
		}
	}
}

func ExampleSnapshotsAPI_Create() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))
	appID := "a1b2c3d4e5f60718293a4b5c6d7e8f90"

	snap, err := c.Apps.Snapshots.Create(ctx, appID)
	if err != nil {
		log.Fatal(err)
	}
	if snap.Pending {
		fmt.Println("still processing, list the snapshots later")
		return
	}
	out, err := os.Create("snapshot.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if err := c.DownloadSnapshot(ctx, snap.URL, out); err != nil {
		log.Fatal(err)
	}

	// Restore the latest one: Name and VersionID of a listed snapshot (its URL
	// downloads it too).
	snaps, err := c.Apps.Snapshots.List(ctx, appID)
	if err != nil || len(snaps) == 0 {
		log.Fatal("no snapshots", err)
	}
	if err := c.Apps.Snapshots.Restore(ctx, appID, snaps[0].Name, snaps[0].VersionID); err != nil {
		log.Fatal(err)
	}
}

func ExampleDatabasesAPI_Create() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	db, err := c.Databases.Create(ctx, squarecloud.DatabaseCreate{
		Name: "cache", Memory: 512, Type: squarecloud.DatabaseRedis, Version: "8",
	})
	if err != nil {
		log.Fatal(err)
	}
	// Password, Certificate and ConnectionURL are shown only once.
	fmt.Println(db.ID, db.ConnectionURL != "")
}

func ExampleWorkspacesAPI_Create() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	ws, err := c.Workspaces.Create(ctx, "Acme")
	if err != nil {
		log.Fatal(err)
	}
	if err := c.Workspaces.Apps.Add(ctx, ws.ID, "a1b2c3d4e5f60718293a4b5c6d7e8f90"); err != nil {
		log.Fatal(err)
	}
	fmt.Println(ws.ID, ws.Name)
}

func ExampleAIAPI_Chat() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	res, err := c.AI.Chat(ctx, squarecloud.ChatRequest{
		Messages: []squarecloud.ChatMessage{{Role: "user", Content: "Say hi"}},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Choices[0].Message.Content)
}
