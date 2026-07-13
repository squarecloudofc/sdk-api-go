package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

type capturedRequest struct {
	Method string
	Path   string
	Query  string
	Body   []byte
}

// newTestRest spins up an httptest server that captures the request and
// replies with responseBody, returning a Rest wired to it.
func newTestRest(t *testing.T, statusCode int, responseBody string) (Rest, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Query = r.URL.RawQuery
		captured.Body, _ = io.ReadAll(r.Body)

		if r.Header.Get("Authorization") != "test-token" {
			t.Errorf("missing Authorization header")
		}

		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)

	client := NewClient("test-token", WithURL(server.URL))
	t.Cleanup(client.Close)

	return New(client), captured
}

func envelope(response string) string {
	return `{"status":"success","response":` + response + `}`
}

func TestUserAndService(t *testing.T) {
	t.Run("SelfUser", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"user":{"id":"u1","name":"joao","email":"a@b.c","plan":{"name":"pro-12","memory":{"limit":12288,"available":100,"used":50},"duration":null},"created_at":"2024-01-01T00:00:00Z"},"applications":[],"databases":[]}`))

		user, err := rest.SelfUser()
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "GET" || req.Path != "/users/me" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
		if user.ID != "u1" || user.Plan.Name != "pro-12" {
			t.Errorf("unexpected user: %+v", user)
		}
	})

	t.Run("UserSnapshots", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`[{"name":"app1","size":123,"modified":"2026-01-01T00:00:00Z","key":"k1"}]`))

		snaps, err := rest.UserSnapshots(squarecloud.SnapshotScopeDatabases)
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/users/snapshots" || req.Query != "scope=databases" {
			t.Errorf("got %s?%s", req.Path, req.Query)
		}
		if len(snaps) != 1 || snaps[0].Key != "k1" {
			t.Errorf("unexpected snapshots: %+v", snaps)
		}
	})

	t.Run("ServiceStatus un-enveloped", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"ok","message":"All systems operational"}`)

		status, err := rest.ServiceStatus()
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/service/status" {
			t.Errorf("got %s", req.Path)
		}
		if status.Message != "All systems operational" {
			t.Errorf("unexpected status: %+v", status)
		}
	})
}

func TestApplications(t *testing.T) {
	t.Run("GetApplications via users/me", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"user":{},"applications":[{"id":"a1","name":"api","lang":"go","cluster":"fl-1","ram":512,"domain":null,"custom":null,"created_at":"2026-01-01T00:00:00Z"}],"databases":[]}`))

		apps, err := rest.GetApplications()
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/users/me" {
			t.Errorf("got %s", req.Path)
		}
		if len(apps) != 1 || apps[0].ID != "a1" || apps[0].Domain != nil {
			t.Errorf("unexpected apps: %+v", apps)
		}
	})

	t.Run("GetApplicationStatus", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"cpu":"0.50%","ram":"120/512MB","status":"running","running":true,"storage":"10MB","network":{"total":"1KB ↑ 2KB ↓","now":"0B ↑ 0B ↓"},"uptime":1700000000000}`))

		status, err := rest.GetApplicationStatus("a1")
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/apps/a1/status" {
			t.Errorf("got %s", req.Path)
		}
		if !status.Running || status.Uptime == nil || *status.Uptime != 1700000000000 {
			t.Errorf("unexpected status: %+v", status)
		}
	})

	t.Run("GetApplicationStatusRaw", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"cpu":0.5,"ram":120,"status":"running","running":true,"storage":10485760,"network":{"total":[1024,2048],"now":[0,0]},"uptime":null}`))

		status, err := rest.GetApplicationStatusRaw("a1")
		if err != nil {
			t.Fatal(err)
		}
		if req.Query != "rawData=true" {
			t.Errorf("got query %s", req.Query)
		}
		if status.Network.Total != [2]int64{1024, 2048} || status.Uptime != nil {
			t.Errorf("unexpected status: %+v", status)
		}
	})

	t.Run("PostApplicationCommit with path", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"success"}`)

		err := rest.PostApplicationCommit("a1", strings.NewReader("zipdata"), WithQueryParam("path", "src/sub dir"))
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "POST" || req.Path != "/apps/a1/commit" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
		if req.Query != "path=src%2Fsub+dir" {
			t.Errorf("path not encoded: %s", req.Query)
		}
		if !strings.Contains(string(req.Body), "zipdata") {
			t.Error("multipart body missing file content")
		}
	})

	t.Run("RestoreApplicationSnapshot", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"success"}`)

		err := rest.RestoreApplicationSnapshot("a1", "snap-id", "ver-id")
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "POST" || req.Path != "/apps/a1/snapshots/restore" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}

		var body map[string]string
		_ = json.Unmarshal(req.Body, &body)
		if body["snapshotId"] != "snap-id" || body["versionId"] != "ver-id" {
			t.Errorf("unexpected body: %s", req.Body)
		}
	})

	t.Run("GetApplicationDomains", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`[{"app_id":"a1","hostname":"foo.dev","type":"custom"}]`))

		domains, err := rest.GetApplicationDomains()
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/apps/domains" {
			t.Errorf("got %s", req.Path)
		}
		if len(domains) != 1 || domains[0].Type != squarecloud.AppDomainTypeCustom {
			t.Errorf("unexpected domains: %+v", domains)
		}
	})

	t.Run("GetLoadBalancers", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"limit":5,"balancers":[{"hostname":"foo.dev","apps":[{"id":"a1","name":"api"}]}]}`))

		lbs, err := rest.GetLoadBalancers()
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/apps/load-balancers" {
			t.Errorf("got %s", req.Path)
		}
		if lbs.Limit != 5 || len(lbs.Balancers) != 1 {
			t.Errorf("unexpected lbs: %+v", lbs)
		}
	})
}

func TestEnvsAndFiles(t *testing.T) {
	t.Run("SetApplicationEnvs", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"KEY":"value","OLD":"kept"}`))

		envs, err := rest.SetApplicationEnvs("a1", squarecloud.EnvVars{"KEY": "value"})
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "POST" || req.Path != "/apps/a1/envs" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
		if !strings.Contains(string(req.Body), `"envs":{"KEY":"value"}`) {
			t.Errorf("unexpected body: %s", req.Body)
		}
		if envs["OLD"] != "kept" {
			t.Errorf("unexpected envs: %+v", envs)
		}
	})

	t.Run("DeleteApplicationEnvs", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"success"}`)

		err := rest.DeleteApplicationEnvs("a1", []string{"KEY"})
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "DELETE" || !strings.Contains(string(req.Body), `"envs":["KEY"]`) {
			t.Errorf("got %s body %s", req.Method, req.Body)
		}
	})

	t.Run("ReadApplicationFile decodes Buffer", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"type":"Buffer","data":[104,105]}`))

		content, err := rest.ReadApplicationFile("a1", "/main.go")
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/apps/a1/files/content" || req.Query != "path=%2Fmain.go" {
			t.Errorf("got %s?%s", req.Path, req.Query)
		}
		if string(content.Data) != "hi" {
			t.Errorf("unexpected content: %q", content.Data)
		}
	})

	t.Run("MoveApplicationFile", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"success"}`)

		err := rest.MoveApplicationFile("a1", "/a.txt", "/b.txt")
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "PATCH" || req.Path != "/apps/a1/files" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
	})
}

func TestDeploysAndNetwork(t *testing.T) {
	t.Run("GetApplicationDeployments nested", func(t *testing.T) {
		rest, _ := newTestRest(t, 200, envelope(`[[{"id":"abc","date":"2026-01-01T00:00:00Z","state":"clone","branch":"main"},{"id":"abc","date":"2026-01-01T00:01:00Z","state":"success"}]]`))

		deploys, err := rest.GetApplicationDeployments("a1")
		if err != nil {
			t.Fatal(err)
		}
		if len(deploys) != 1 || len(deploys[0]) != 2 || deploys[0][0].Branch != "main" {
			t.Errorf("unexpected deploys: %+v", deploys)
		}
	})

	t.Run("GetApplicationDNS array shape", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`[{"type":"cname","name":"foo.dev","value":"cname.squareweb.app","status":"active"}]`))

		records, err := rest.GetApplicationDNS("a1")
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/apps/a1/network/dns" {
			t.Errorf("got %s", req.Path)
		}
		if len(records) != 1 || records[0].Value != "cname.squareweb.app" {
			t.Errorf("unexpected records: %+v", records)
		}
	})

	t.Run("GetApplicationAnalytics sends range and filters", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"visits":[{"date":"2026-01-01T00:00:00Z","visits":10,"requests":20,"bytes":300}]}`))

		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.Add(24 * time.Hour)

		analytics, err := rest.GetApplicationAnalytics("a1", start, end, WithQueryParam("country", "BR"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"start=2026-01-01T00%3A00%3A00Z", "end=2026-01-02T00%3A00%3A00Z", "country=BR"} {
			if !strings.Contains(req.Query, want) {
				t.Errorf("query %q missing %q", req.Query, want)
			}
		}
		if len(analytics.Visits) != 1 || analytics.Visits[0].Requests != 20 {
			t.Errorf("unexpected analytics: %+v", analytics)
		}
	})

	t.Run("PurgeApplicationCache no body", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"success"}`)

		if err := rest.PurgeApplicationCache("a1"); err != nil {
			t.Fatal(err)
		}
		if req.Method != "POST" || req.Path != "/apps/a1/network/purge_cache" || len(req.Body) != 0 {
			t.Errorf("got %s %s body=%q", req.Method, req.Path, req.Body)
		}
	})
}

func TestDatabases(t *testing.T) {
	t.Run("CreateDatabase", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"id":"db1","name":"main","memory":1024,"cpu":1,"type":"postgres","password":"secret","connection_url":"postgres://...","cluster":"fl-1"}`))

		created, err := rest.CreateDatabase(squarecloud.DatabaseCreateOptions{
			Name: "main", Memory: 1024, Type: squarecloud.DatabaseTypePostgres, Version: "16",
		})
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "POST" || req.Path != "/databases" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
		if !strings.Contains(string(req.Body), `"type":"postgres"`) {
			t.Errorf("unexpected body: %s", req.Body)
		}
		if created.Password != "secret" {
			t.Errorf("unexpected created: %+v", created)
		}
	})

	t.Run("ResetDatabaseCredentials", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"password":"newpass"}`))

		reset, err := rest.ResetDatabaseCredentials("db1", squarecloud.DatabaseResetPassword)
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/databases/db1/credentials/reset" || !strings.Contains(string(req.Body), `"reset":"password"`) {
			t.Errorf("got %s body %s", req.Path, req.Body)
		}
		if reset.Password != "newpass" {
			t.Errorf("unexpected reset: %+v", reset)
		}
	})

	t.Run("UpdateDatabase PATCH", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"success"}`)

		err := rest.UpdateDatabase("db1", squarecloud.DatabaseUpdateOptions{RAM: 2048})
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "PATCH" || req.Path != "/databases/db1" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
	})
}

func TestWorkspaces(t *testing.T) {
	t.Run("CreateWorkspace", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"id":"ws1","name":"team"}`))

		created, err := rest.CreateWorkspace("team")
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "POST" || req.Path != "/workspaces" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
		if created.ID != "ws1" {
			t.Errorf("unexpected created: %+v", created)
		}
	})

	t.Run("RemoveWorkspaceMember DELETE with body", func(t *testing.T) {
		rest, req := newTestRest(t, 200, `{"status":"success"}`)

		err := rest.RemoveWorkspaceMember("ws1", "u2")
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != "DELETE" || req.Path != "/workspaces/members" {
			t.Errorf("got %s %s", req.Method, req.Path)
		}
		if !strings.Contains(string(req.Body), `"memberId":"u2"`) {
			t.Errorf("unexpected body: %s", req.Body)
		}
	})

	t.Run("GetWorkspaceInviteCode", func(t *testing.T) {
		rest, req := newTestRest(t, 200, envelope(`{"code":"deadbeef"}`))

		code, err := rest.GetWorkspaceInviteCode()
		if err != nil {
			t.Fatal(err)
		}
		if req.Path != "/workspaces/members/code" {
			t.Errorf("got %s", req.Path)
		}
		if code.Code != "deadbeef" {
			t.Errorf("unexpected code: %+v", code)
		}
	})
}

func TestErrorsAndContext(t *testing.T) {
	t.Run("APIError parsing", func(t *testing.T) {
		rest, _ := newTestRest(t, 404, `{"status":"error","code":"APP_NOT_FOUND","message":"app not found"}`)

		_, err := rest.GetApplication("nope")
		if err == nil {
			t.Fatal("expected error")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.Code != "APP_NOT_FOUND" || apiErr.StatusCode != 404 {
			t.Errorf("unexpected error: %+v", apiErr)
		}
		if ErrorCode(err) != "APP_NOT_FOUND" {
			t.Errorf("ErrorCode mismatch")
		}
	})

	t.Run("IsRateLimit", func(t *testing.T) {
		err := &APIError{StatusCode: 429, Code: "RATE_LIMIT_EXCEEDED"}
		if !IsRateLimit(err) {
			t.Error("expected rate limit")
		}
		if IsRateLimit(&APIError{Code: "APP_NOT_FOUND"}) {
			t.Error("false positive")
		}
	})

	t.Run("WithContext cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		client := NewClient("test-token", WithURL(server.URL))
		defer client.Close()
		rest := New(client)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := rest.SelfUser(WithContext(ctx))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline exceeded, got %v", err)
		}
	})
}

func TestRealtimeStream(t *testing.T) {
	sse := "event: REALTIME_CONNECTING\ndata: {}\n\nevent: message\ndata: line1\ndata: line2\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/a1/realtime" {
			t.Errorf("got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer server.Close()

	client := NewClient("test-token", WithURL(server.URL))
	defer client.Close()
	rest := New(client)

	stream, err := rest.ApplicationRealtime("a1")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	first, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if first.Event != "REALTIME_CONNECTING" {
		t.Errorf("unexpected first event: %+v", first)
	}

	second, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if second.Event != "message" || second.Data != "line1\nline2" {
		t.Errorf("unexpected second event: %+v", second)
	}

	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}
