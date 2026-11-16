package squarecloud_test

// go test compiles and vets these examples but never runs them, because they
// have no "Output:" comment. They never call the real API.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/squarecloudofc/sdk-api-go/v4"
)

func Example() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))

	me, err := c.Account.Me(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(me.User.Name, me.User.Plan.Name, len(me.Applications))

	// For an application shared with you through a workspace, use its
	// composite id instead (see AppsAPI).
	appID := "a1b2c3d4e5f60718293a4b5c6d7e8f90"
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

	// The zip is streamed as it is uploaded, never loaded into memory.
	f, err := os.Open("app.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	app, err := c.Apps.Create(ctx, f)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(app.ID, app.Language.Name)

	// Commit a single file. A file that is not a zip lands at
	// <path>/<filename>, so this one becomes src/index.js.
	src, err := os.Open("index.js")
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	if err := c.Apps.Commit(ctx, app.ID, src, "src", ""); err != nil {
		log.Fatal(err)
	}

	// Deploy on every push to main through the Square Cloud GitHub App. The
	// API key needs the apps:deploy scope.
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
			return // The error is io.EOF when the stream ends normally.
		}
		switch ev.Event {
		case "logs":
			fmt.Printf("[%s] %s\n", ev.Stream, ev.Line)
		case "status": // Status is never nil on a status event.
			fmt.Printf("cpu %.1f%% ram %.0f/%.0f MB\n", ev.Status.CPU, ev.Status.RAM[0], ev.Status.RAM[1])
		default: // A "system" or "error" event.
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
		fmt.Println("The snapshot is still being taken. List the snapshots in a few minutes.")
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
}

func ExampleSnapshotsAPI_Restore() {
	ctx := context.Background()
	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))
	appID := "a1b2c3d4e5f60718293a4b5c6d7e8f90"

	// Restore the first listed snapshot. Restore takes its Name and
	// VersionID.
	snaps, err := c.Apps.Snapshots.List(ctx, appID)
	if err != nil || len(snaps) == 0 {
		log.Fatal("no snapshots", err)
	}
	res, err := c.Apps.Snapshots.Restore(ctx, appID, snaps[0].Name, snaps[0].VersionID)
	if err != nil {
		log.Fatal(err)
	}
	// A large restore keeps running after Restore returns. Follow it with
	// RestoreState until it is no longer queued or running, and never call
	// Restore again.
	st := squarecloud.SnapshotRestoreState{State: "success"}
	for res.Pending {
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
	// Save Password, Certificate and ConnectionURL now. The API never shows
	// them again.
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
