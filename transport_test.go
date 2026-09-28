package squarecloud

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestErrorSquareDialect(t *testing.T) {
	c := testClient(t, reply(404, `{"status":"error","code":"APP_NOT_FOUND","message":"nope"}`))
	_, err := c.Apps.Get(context.Background(), "app1")
	e := asAPIError(t, err)
	if e.Status != 404 || e.Code != "APP_NOT_FOUND" || e.Message != "nope" || e.Method != "GET" || e.Path != "/v2/apps/app1" {
		t.Fatalf("%+v", e)
	}
	if got := err.Error(); got != "squarecloud: GET /v2/apps/app1: HTTP 404 APP_NOT_FOUND: nope" {
		t.Fatal(got)
	}
	// A code without a message keeps Message empty.
	c = testClient(t, reply(403, `{"status":"error","code":"MISSING_SCOPE"}`))
	_, err = c.Apps.Get(context.Background(), "app1")
	if e := asAPIError(t, err); e.Message != "" || err.Error() != "squarecloud: GET /v2/apps/app1: HTTP 403 MISSING_SCOPE" {
		t.Fatal(err)
	}
}

// An expired API key is 401 ACCESS_DENIED, like an unknown one.
func TestExpiredKeyIsAccessDenied(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "test-key" { // expired
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"status":"error","code":"ACCESS_DENIED"}`)
		}
	})
	_, err := c.Account.Me(context.Background())
	if e := asAPIError(t, err); e.Status != 401 || e.Code != CodeAccessDenied {
		t.Fatalf("%+v", e)
	}
}

func TestErrorOpenAIDialect(t *testing.T) {
	// Every AI route error is OpenAI-shaped with a lowercase code, auth, 429
	// and 503 included; none is retried (the route is a POST).
	for _, tc := range []struct {
		status int
		code   string
	}{
		{401, "access_denied"},
		{403, "missing_scope"},
		{429, "rate_limit_exceeded"},
		{429, "rate_limited"},
		{503, "database_unavailable"},
		{503, "server_overloaded"},
	} {
		c, n := retryServer(t, func(_ int32, w http.ResponseWriter) bool {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, `{"error":{"message":"m","type":"t","param":null,"code":"`+tc.code+`"}}`)
			return true
		})
		_, err := c.AI.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}})
		if e := asAPIError(t, err); e.Status != tc.status || e.Code != tc.code || e.Message != "m" || n.Load() != 1 {
			t.Fatalf("%+v, %d attempts", e, n.Load())
		}
	}
	// The gateway omits code when it has none; type is always set.
	c := testClient(t, reply(400, `{"error":{"message":"bad","type":"invalid_request_error"}}`))
	_, err := c.AI.Chat(context.Background(), ChatRequest{})
	if e := asAPIError(t, err); e.Code != "invalid_request_error" || e.Message != "bad" {
		t.Fatalf("%+v", e)
	}
	// An object in "error" wins over a top-level code.
	c = testClient(t, reply(400, `{"code":"OUTER","error":{"message":"inner","code":"inner_code"}}`))
	_, err = c.AI.Chat(context.Background(), ChatRequest{})
	if e := asAPIError(t, err); e.Code != "inner_code" || e.Message != "inner" {
		t.Fatalf("%+v", e)
	}
}

func TestChatSendsEmptyContent(t *testing.T) {
	var body string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = io.WriteString(w, `{"id":"x"}`)
	})
	_, err := c.AI.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "tool", ToolCallID: "t1"}}})
	if err != nil || body != `{"messages":[{"role":"tool","content":"","tool_call_id":"t1"}]}` {
		t.Fatal(body, err)
	}
}

func TestRejectsDotAndEmptyIDs(t *testing.T) {
	called := false
	c := testClient(t, func(http.ResponseWriter, *http.Request) { called = true })
	ctx := context.Background()
	for _, id := range []string{"", ".", ".."} {
		if err := c.Apps.Delete(ctx, id); err == nil {
			t.Fatalf("Apps.Delete(%q) was sent", id)
		}
		if _, err := c.Databases.Snapshots.List(ctx, id); err == nil {
			t.Fatalf("Databases.Snapshots.List(%q) was sent", id)
		}
		if _, err := c.Workspaces.Get(ctx, id); err == nil {
			t.Fatalf("Workspaces.Get(%q) was sent", id)
		}
	}
	if called {
		t.Fatal("a request reached the server")
	}
	if New("", WithHTTPClient(nil)).hc == nil {
		t.Fatal("WithHTTPClient(nil) must keep the default client")
	}
	if hc := (&http.Client{}); New("", WithHTTPClient(hc)).hc != hc {
		t.Fatal("WithHTTPClient ignored")
	}
}

func TestErrorNonJSONBody(t *testing.T) {
	c := testClient(t, reply(502, strings.Repeat("<html>bad gateway</html>", 10000)))
	err := c.Apps.Start(context.Background(), "app1")
	if e := asAPIError(t, err); e.Status != 502 || e.Code != CodeUnknown || e.Message != "HTTP 502" {
		t.Fatalf("%+v", e)
	}
	// A code-less body keeps its message.
	c = testClient(t, reply(500, `{"message":"boom"}`))
	if e := asAPIError(t, c.Apps.Start(context.Background(), "app1")); e.Code != CodeUnknown || e.Message != "boom" {
		t.Fatalf("%+v", e)
	}
}

// Database start/stop refusals are a 409 with a code and no message.
func TestDatabaseStartStop409(t *testing.T) {
	for _, code := range []string{CodeContainerAlreadyStarted, CodeContainerAlreadyStopped, CodeContainerTemporarilySuspended, CodeActionFailed} {
		c := testClient(t, reply(409, `{"status":"error","code":"`+code+`"}`))
		for _, err := range []error{c.Databases.Start(context.Background(), "db1"), c.Databases.Stop(context.Background(), "db1")} {
			if e := asAPIError(t, err); e.Status != 409 || e.Code != code || e.Message != "" {
				t.Fatalf("%+v", e)
			}
		}
	}
}

// A 2xx {"status":"error"} is still an error (a safety net: the cluster
// refusals of start/stop arrive as 409).
func TestErrorEnvelopeOn2xx(t *testing.T) {
	c := testClient(t, reply(200, `{"status":"error","code":"DATABASE_START_FAILED","message":"cluster"}`))
	e := asAPIError(t, c.Databases.Start(context.Background(), "db1"))
	if e.Status != 200 || e.Code != "DATABASE_START_FAILED" || e.Message != "cluster" || e.Path != "/v2/databases/db1/start" {
		t.Fatalf("%+v", e)
	}
	c = testClient(t, reply(200, `{"status":"error"}`))
	if _, err := c.Apps.Get(context.Background(), "app1"); asAPIError(t, err).Code != CodeUnknown || asAPIError(t, err).Message != "HTTP 200" {
		t.Fatal(err)
	}
	// A 2xx that is not JSON is typed too, with the decode error as cause.
	c = testClient(t, reply(200, `<html>`))
	_, err := c.Apps.Get(context.Background(), "app1")
	var se *json.SyntaxError
	if e := asAPIError(t, err); e.Status != 200 || e.Code != CodeUnknown || e.Message != "Invalid JSON in HTTP 200 response" || !errors.As(err, &se) {
		t.Fatalf("%+v", e)
	}
	if e := asAPIError(t, c.Apps.Delete(context.Background(), "app1")); e.Code != CodeUnknown {
		t.Fatalf("%+v", e)
	}
	// Empty and success bodies on void calls are fine.
	c = testClient(t, reply(200, ``))
	if err := c.Apps.Delete(context.Background(), "app1"); err != nil {
		t.Fatal(err)
	}
	// A 2xx without a value gives "" for string results.
	c = testClient(t, reply(200, `{"status":"success"}`))
	if url, err := c.Apps.Deploys.SetWebhook(context.Background(), "app1", "@"); err != nil || url != "" {
		t.Fatal(url, err)
	}
}

func TestLocalErrorsAreTyped(t *testing.T) {
	called := false
	c := testClient(t, func(http.ResponseWriter, *http.Request) { called = true })
	ctx := context.Background()
	e := asAPIError(t, c.Apps.Delete(ctx, ".."))
	if e.Status != 0 || e.Code != CodeInvalidID || e.Method != "DELETE" || e.Path != "/v2/apps/.." {
		t.Fatalf("%+v", e)
	}
	if got := e.Error(); got != `squarecloud: DELETE /v2/apps/..: INVALID_ID: Empty, "." and ".." are not valid ids` {
		t.Fatal(got)
	}
	e = asAPIError(t, c.Apps.Files.Write(ctx, "app1", "/big", make([]byte, maxFileSize+1)))
	if e.Code != CodeFileTooLarge || e.Path != "/v2/apps/app1/files" || e.Message != "File content is limited to 10 MB" {
		t.Fatalf("%+v", e)
	}
	_, err := c.Apps.Create(ctx, &hugeZip{})
	if e := asAPIError(t, err); e.Code != CodeFileTooLarge || e.Path != "/v2/apps" || e.Message != "Uploads are limited to 100 MB" {
		t.Fatalf("%+v", e)
	}
	// A nil upload reader is the caller's bug: a plain error, nothing sent.
	if err := c.Apps.Commit(ctx, "app1", nil, "", ""); err == nil || errors.As(err, new(*APIError)) {
		t.Fatal(err)
	}
	// Empty and whitespace-only keys fail locally, except on the public status.
	for _, key := range []string{"", " \t\n"} {
		anon := New(key, WithBaseURL(c.base))
		if _, err := anon.Apps.Get(ctx, "app1"); asAPIError(t, err).Code != CodeInvalidAPIKey {
			t.Fatal(err)
		}
	}
	if called {
		t.Fatal("a request reached the server")
	}
}

func TestSnapshotPending(t *testing.T) {
	c := testClient(t, reply(202, `{"status":"error","code":"SNAPSHOT_PROCESSING"}`))
	for _, s := range []SnapshotsAPI{c.Apps.Snapshots, c.Databases.Snapshots} {
		got, err := s.Create(context.Background(), "id1")
		if err != nil || !got.Pending || got.URL != "" {
			t.Fatalf("%+v %v", got, err)
		}
	}
	c = testClient(t, reply(200, `{"status":"success","response":{"url":"https://x/s.zip","key":"k"}}`))
	got, err := c.Apps.Snapshots.Create(context.Background(), "id1")
	if err != nil || got.Pending || got.URL != "https://x/s.zip" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestHeadersAndEscaping(t *testing.T) {
	var uri, auth, ua, ct, accept string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		uri, auth, ua, ct, accept = r.RequestURI, r.Header.Get("Authorization"), r.Header.Get("User-Agent"), r.Header.Get("Content-Type"), r.Header.Get("Accept")
		_, _ = io.WriteString(w, `{"status":"success"}`)
	})
	ctx := context.Background()
	if _, err := c.Apps.Logs(ctx, "a/b?c#d"); err != nil {
		t.Fatal(err)
	}
	if uri != "/v2/apps/a%2Fb%3Fc%23d/logs" || auth != "test-key" || ua != "squarecloud-sdk-go/"+Version || ct != "" || accept != "application/json" {
		t.Fatalf("uri=%q auth=%q ua=%q ct=%q accept=%q", uri, auth, ua, ct, accept)
	}
	// An error reports the path as sent (escaped).
	_, err := testClient(t, reply(404, `{"status":"error","code":"APP_NOT_FOUND"}`)).Apps.Get(ctx, "a/b")
	if e := asAPIError(t, err); e.Path != "/v2/apps/a%2Fb" {
		t.Fatal(e.Path)
	}
	// Composite <appId>-<workspaceId> ids pass through as one segment.
	if _, err := c.Apps.Logs(ctx, "0123456789abcdef0123456789abcdef-ws1"); err != nil || uri != "/v2/apps/0123456789abcdef0123456789abcdef-ws1/logs" {
		t.Fatal(uri, err)
	}
	if _, err := c.Apps.Files.List(ctx, "app1", "/a b&c=d"); err != nil {
		t.Fatal(err)
	}
	if uri != "/v2/apps/app1/files?path=%2Fa+b%26c%3Dd" {
		t.Fatal(uri)
	}
	if err := c.Apps.Files.Move(ctx, "app1", "/a", "/b"); err != nil || ct != "application/json" {
		t.Fatal(ct, err)
	}
	// Query rules: "" and false are dropped, dates are RFC 3339 UTC.
	if _, err := c.Apps.StatusAll(ctx, ""); err != nil || uri != "/v2/apps/status" {
		t.Fatal(uri, err)
	}
	if _, err := c.Account.Snapshots(ctx, ""); err != nil || uri != "/v2/users/snapshots" {
		t.Fatal(uri, err)
	}
	if _, err := c.Apps.Files.Read(ctx, "app1", ""); err != nil || uri != "/v2/apps/app1/files/content?encoding=base64" {
		t.Fatal(uri, err)
	}
	brt := time.FixedZone("BRT", -3*3600)
	if _, err := c.Apps.Network.Errors(ctx, "app1", time.Date(2026, 9, 1, 21, 0, 0, 0, brt), time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), false); err != nil ||
		uri != "/v2/apps/app1/network/errors?end=2026-09-02T00%3A00%3A00Z&start=2026-09-02T00%3A00%3A00Z" {
		t.Fatal(uri, err)
	}
	if _, err := c.Apps.Network.Analytics(ctx, "app1", time.Time{}, time.Time{}, AnalyticsFilters{Country: "BR"}); err != nil ||
		uri != "/v2/apps/app1/network/analytics?country=BR&end=0001-01-01T00%3A00%3A00Z&start=0001-01-01T00%3A00%3A00Z" {
		t.Fatal(uri, err)
	}
	// No key: no Authorization header (public routes). The override replaces
	// the whole User-Agent.
	anon := New("", WithBaseURL(c.base+"//"), WithUserAgent("cli"))
	if _, err := anon.Service.Status(ctx); err != nil || auth != "" || ua != "cli" || uri != "/v2/service/status" {
		t.Fatal(auth, ua, uri, err)
	}
}

// retryServer counts attempts; fail(n, w) answers attempt n when it returns true.
func retryServer(t *testing.T, fail func(n int32, w http.ResponseWriter) bool, opts ...Option) (*Client, *atomic.Int32) {
	var n atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if fail(n.Add(1), w) {
			return
		}
		_, _ = io.WriteString(w, `{"status":"success","response":{}}`)
	}, opts...)
	return c, &n
}

func busy(code string) func(int32, http.ResponseWriter) bool {
	return func(_ int32, w http.ResponseWriter) bool {
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"status":"error","code":"`+code+`"}`)
		return true
	}
}

func TestRetryPolicy(t *testing.T) {
	ctx := context.Background()
	firstHangsUp := func(n int32, w http.ResponseWriter) bool {
		if n == 1 {
			hangUp(w)
			return true
		}
		return false
	}

	t.Run("network error on GET is retried", func(t *testing.T) {
		c, n := retryServer(t, firstHangsUp)
		if _, err := c.Apps.Get(ctx, "app1"); err != nil || n.Load() != 2 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("network error on mutation is not retried", func(t *testing.T) {
		c, n := retryServer(t, firstHangsUp)
		err := c.Apps.Restart(ctx, "app1")
		if e := asAPIError(t, err); e.Code != CodeNetworkError || e.Status != 0 || e.Message == "" || e.Message != errors.Unwrap(err).Error() || n.Load() != 1 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("503 ANALYTICS_BUSY is retried up to maxRetries", func(t *testing.T) {
		c, n := retryServer(t, busy(CodeAnalyticsBusy))
		_, err := c.Apps.Network.DNS(ctx, "app1")
		if e := asAPIError(t, err); e.Code != CodeAnalyticsBusy || n.Load() != 3 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("503 UPLOAD_BUSY is retried on any method", func(t *testing.T) {
		c, n := retryServer(t, busy(CodeUploadBusy))
		if err := c.Apps.Files.Write(ctx, "app1", "/a", []byte("a")); asAPIError(t, err).Code != CodeUploadBusy || n.Load() != 3 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("WithMaxRetries(0) disables retries", func(t *testing.T) {
		c, n := retryServer(t, busy(CodeAnalyticsBusy), WithMaxRetries(0))
		if _, err := c.Apps.Network.DNS(ctx, "app1"); err == nil || n.Load() != 1 {
			t.Fatal(n.Load(), err)
		}
		c, n = retryServer(t, busy(CodeAnalyticsBusy), WithMaxRetries(-5))
		if _, err := c.Apps.Network.DNS(ctx, "app1"); err == nil || n.Load() != 1 {
			t.Fatal("negative counts as 0", n.Load(), err)
		}
	})
	t.Run("503 DATABASE_UNAVAILABLE only on GET", func(t *testing.T) {
		c, n := retryServer(t, busy(CodeDatabaseUnavailable))
		_, _ = c.Databases.Get(ctx, "db1")
		if n.Load() != 3 {
			t.Fatal("GET", n.Load())
		}
		n.Store(0)
		_ = c.Databases.Start(ctx, "db1")
		if n.Load() != 1 {
			t.Fatal("POST", n.Load())
		}
	})
	t.Run("429 is never retried", func(t *testing.T) {
		for _, code := range []string{CodeRateLimited, CodeRateLimit, CodeRateLimitExceeded, CodeKeepCalm} {
			c, n := retryServer(t, func(_ int32, w http.ResponseWriter) bool {
				w.WriteHeader(429)
				_, _ = io.WriteString(w, `{"status":"error","code":"`+code+`"}`)
				return true
			})
			if _, err := c.Apps.Get(ctx, "app1"); asAPIError(t, err).Status != 429 || n.Load() != 1 {
				t.Fatal(code, n.Load(), err)
			}
		}
	})
	t.Run("other 503s are not retried", func(t *testing.T) {
		c, n := retryServer(t, busy("MAINTENANCE"))
		if _, err := c.Apps.Get(ctx, "app1"); asAPIError(t, err).Status != 503 || n.Load() != 1 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("a timeout is never retried", func(t *testing.T) {
		c, n := retryServer(t, func(_ int32, w http.ResponseWriter) bool { time.Sleep(100 * time.Millisecond); return false },
			WithTimeout(20*time.Millisecond))
		if _, err := c.Apps.Get(ctx, "app1"); asAPIError(t, err).Code != CodeTimeout || n.Load() != 1 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("a transport timeout is never retried", func(t *testing.T) {
		tr := http.DefaultTransport.(loopbackOnly).rt.(*http.Transport).Clone()
		tr.ResponseHeaderTimeout = 20 * time.Millisecond
		c, n := retryServer(t, func(_ int32, w http.ResponseWriter) bool { time.Sleep(100 * time.Millisecond); return false },
			WithHTTPClient(&http.Client{Transport: loopbackOnly{tr}}))
		if _, err := c.Apps.Get(ctx, "app1"); asAPIError(t, err).Code != CodeTimeout || n.Load() != 1 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("503 UPLOAD_BUSY re-sends a seekable zip", func(t *testing.T) {
		var bodies []string
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			b, _ := io.ReadAll(f)
			bodies = append(bodies, string(b))
			if len(bodies) == 1 {
				busy(CodeUploadBusy)(0, w)
				return
			}
			_, _ = io.WriteString(w, `{"status":"success"}`)
		})
		if err := c.Apps.Commit(ctx, "app1", strings.NewReader("ZIPDATA"), "", ""); err != nil {
			t.Fatal(err)
		}
		if len(bodies) != 2 || bodies[0] != "ZIPDATA" || bodies[1] != "ZIPDATA" {
			t.Fatal(bodies)
		}
	})
	t.Run("503 UPLOAD_BUSY with a one-shot reader is not retried", func(t *testing.T) {
		c, n := retryServer(t, busy(CodeUploadBusy))
		err := c.Apps.Commit(ctx, "app1", io.MultiReader(strings.NewReader("ZIP")), "", "")
		if e := asAPIError(t, err); e.Code != CodeUploadBusy || n.Load() != 1 {
			t.Fatal(n.Load(), err)
		}
	})
}

func TestBackoffBounds(t *testing.T) {
	if shippedRetryBase != 500*time.Millisecond {
		t.Fatal(shippedRetryBase)
	}
	for attempt, want := range map[int]time.Duration{0: retryBase, 3: 8 * retryBase, 100: maxBackoff} {
		for range 50 {
			if d := backoff(attempt); d < want/2 || d > want {
				t.Fatalf("backoff(%d) = %v, want in [%v, %v]", attempt, d, want/2, want)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if start := time.Now(); wait(ctx, time.Hour) || time.Since(start) > time.Second {
		t.Fatal("cancellation must end the wait at once")
	}
}

func TestTimeoutFloors(t *testing.T) {
	ctx := context.Background()
	deadline := func(c *Client, floor time.Duration) time.Duration {
		bctx, cancel := c.bound(ctx, floor)
		defer cancel()
		d, ok := bctx.Deadline()
		if !ok {
			return 0
		}
		return time.Until(d).Round(time.Second)
	}
	for _, tc := range []struct {
		opts        []Option
		floor, want time.Duration
	}{
		{nil, 0, 30 * time.Second},
		{nil, heldFloor, 2 * time.Minute},
		{nil, aiFloor, 2 * time.Minute},
		{[]Option{WithTimeout(5 * time.Second)}, aiFloor, 2 * time.Minute},
		{[]Option{WithTimeout(5 * time.Second)}, heldFloor, 2 * time.Minute},
		{[]Option{WithTimeout(15 * time.Minute)}, aiFloor, 15 * time.Minute},
		{[]Option{WithTimeout(0)}, 0, 0},
		{[]Option{WithTimeout(0)}, heldFloor, 0}, // a disabled timeout disables the floors
		{[]Option{WithTimeout(-time.Second)}, aiFloor, 0},
	} {
		if got := deadline(New("k", tc.opts...), tc.floor); got != tc.want {
			t.Errorf("%d options, floor %v: deadline in %v, want %v", len(tc.opts), tc.floor, got, tc.want)
		}
	}
	// A caller deadline always wins.
	dctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	bctx, bcancel := New("k").bound(dctx, heldFloor)
	defer bcancel()
	if bctx != dctx {
		t.Fatal("the caller's ctx must be kept")
	}
}

func TestTimeouts(t *testing.T) {
	short := WithTimeout(50 * time.Millisecond)
	slow := func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}

	t.Run("JSON calls get a default deadline", func(t *testing.T) {
		c := testClient(t, slow, WithMaxRetries(0), short)
		err := c.Apps.Delete(context.Background(), "app1")
		if e := asAPIError(t, err); e.Code != CodeTimeout || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	})
	t.Run("WithTimeout(0) disables it", func(t *testing.T) {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			_, _ = io.WriteString(w, `{"status":"success"}`)
		}, WithTimeout(0))
		if err := c.Apps.Delete(context.Background(), "app1"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("the caller's deadline wins", func(t *testing.T) {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(150 * time.Millisecond)
			_, _ = io.WriteString(w, `{"status":"success"}`)
		}, short)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := c.Apps.Delete(ctx, "app1"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("uploads have no default deadline", func(t *testing.T) {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			time.Sleep(150 * time.Millisecond)
			_, _ = io.WriteString(w, `{"status":"success"}`)
		}, short)
		if err := c.Apps.Commit(context.Background(), "app1", strings.NewReader("z"), "", ""); err != nil {
			t.Fatal(err)
		}
		// A file write over 1 MiB of content is bounded only by ctx; one of
		// exactly 1 MiB is not.
		if err := c.Apps.Files.Write(context.Background(), "app1", "/big", bytes.Repeat([]byte("a"), largeFile+1)); err != nil {
			t.Fatal(err)
		}
		if err := c.Apps.Files.Write(context.Background(), "app1", "/small", bytes.Repeat([]byte("a"), largeFile)); asAPIError(t, err).Code != CodeTimeout {
			t.Fatal(err)
		}
	})
	t.Run("held operations outlive the default deadline", func(t *testing.T) {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(150 * time.Millisecond) // 3x the timeout
			if strings.HasSuffix(r.URL.Path, "/snapshots") {
				w.WriteHeader(202) // the API's answer after its ~90s soft deadline
			}
			_, _ = io.WriteString(w, `{"status":"success","response":{"password":"p"}}`)
		}, short)
		ctx := context.Background()
		db, err := c.Databases.Create(ctx, DatabaseCreate{Name: "d", Memory: 512, Type: DatabaseMongo, Version: "8"})
		if err != nil || db.Password != "p" {
			t.Fatal("Databases.Create", err)
		}
		if snap, err := c.Apps.Snapshots.Create(ctx, "app1"); err != nil || !snap.Pending {
			t.Fatal("Snapshots.Create", err)
		}
		for name, f := range map[string]func() error{
			"Apps.Start":        func() error { return c.Apps.Start(ctx, "app1") },
			"Apps.Stop":         func() error { return c.Apps.Stop(ctx, "app1") },
			"Apps.Restart":      func() error { return c.Apps.Restart(ctx, "app1") },
			"Databases.Start":   func() error { return c.Databases.Start(ctx, "db1") },
			"Databases.Stop":    func() error { return c.Databases.Stop(ctx, "db1") },
			"Snapshots.Restore": func() error { return c.Databases.Snapshots.Restore(ctx, "db1", "n", "v") },
			"AI.Chat":           func() error { _, err := c.AI.Chat(ctx, ChatRequest{}); return err },
		} {
			if err := f(); err != nil {
				t.Fatal(name, err)
			}
		}
	})
	// A body that stops mid-way is a network failure, not an undecodable body.
	t.Run("a deadline while reading the body is TIMEOUT", func(t *testing.T) {
		partial := func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"status":"success","response":{"encoding":"base64","data":"AAEC`)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
		c := testClient(t, partial, short)
		_, err := c.Apps.Files.Read(context.Background(), "app1", "/big.bin")
		if e := asAPIError(t, err); e.Status != 0 || e.Code != CodeTimeout || e.Path != "/v2/apps/app1/files/content" || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%+v", e)
		}
		srv := httptest.NewServer(http.HandlerFunc(partial))
		defer srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err = New("k").DownloadSnapshot(ctx, srv.URL+"/s.zip?Signature=secret", io.Discard)
		if e := asAPIError(t, err); e.Code != CodeTimeout || e.Path != "/s.zip" || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	})
	t.Run("a truncated body is NETWORK_ERROR, retried on GET", func(t *testing.T) {
		var n atomic.Int32
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			n.Add(1)
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, `{"status":"success","response":{"id":`)
			w.(http.Flusher).Flush()
			hangUp(w)
		})
		_, err := c.Apps.Get(context.Background(), "app1")
		if e := asAPIError(t, err); e.Status != 0 || e.Code != CodeNetworkError || !errors.Is(err, io.ErrUnexpectedEOF) || n.Load() != 1+defaultRetries {
			t.Fatalf("%+v %d", e, n.Load())
		}
	})
	t.Run("a ctx that ends during a backoff wait wins over the last error", func(t *testing.T) {
		old := retryBase
		retryBase = time.Hour
		t.Cleanup(func() { retryBase = old })
		c, _ := retryServer(t, busy(CodeUploadBusy))
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(20*time.Millisecond, cancel)
		if err := c.Apps.Commit(ctx, "app1", strings.NewReader("x"), "", ""); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := c.Apps.Commit(ctx, "app1", strings.NewReader("x"), "", "")
		if e := asAPIError(t, err); e.Code != CodeTimeout || e.Status != 0 || e.Path != "/v2/apps/app1/commit" || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	})
	// The deadline covers every attempt, so a timed-out call is not retried.
	t.Run("a spent ctx is not retried", func(t *testing.T) {
		var n atomic.Int32
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { n.Add(1); slow(w, r) })
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := c.Apps.Get(ctx, "app1")
		if e := asAPIError(t, err); e.Code != CodeTimeout || n.Load() != 1 {
			t.Fatal(n.Load(), err)
		}
	})
	t.Run("caller cancellation is not wrapped", func(t *testing.T) {
		c := testClient(t, slow)
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(20*time.Millisecond, cancel)
		if _, err := c.Apps.Get(ctx, "app1"); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
}

func TestFilesContent(t *testing.T) {
	var all [256]byte
	for i := range all {
		all[i] = byte(i)
	}
	text := []byte("olá, 世界\n\"x\"")
	var body, query string
	var serve []byte
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"status":"success","response":{"encoding":"base64","data":"`+base64.StdEncoding.EncodeToString(serve)+`"}}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = io.WriteString(w, `{"status":"success"}`)
	})
	ctx := context.Background()
	// Read always asks for base64 and decodes it: binary, UTF-8 and empty.
	for _, want := range [][]byte{all[:], text, {}} {
		serve = want
		got, err := c.Apps.Files.Read(ctx, "app1", "/f")
		if err != nil || !bytes.Equal(got, want) || query != "encoding=base64&path=%2Ff" {
			t.Fatal(got, query, err)
		}
	}
	// Write always sends base64, so binary and text round-trip byte for byte,
	// and empty content writes an empty file.
	for _, tc := range []struct {
		content []byte
		want    string
	}{
		{all[:], `{"path":"/f","content":"` + base64.StdEncoding.EncodeToString(all[:]) + `","encoding":"base64"}`},
		{text, `{"path":"/f","content":"b2zDoSwg5LiW55WMCiJ4Ig==","encoding":"base64"}`},
		{nil, `{"path":"/f","content":"","encoding":"base64"}`},
		{[]byte{}, `{"path":"/f","content":"","encoding":"base64"}`},
	} {
		if err := c.Apps.Files.Write(ctx, "app1", "/f", tc.content); err != nil || body != tc.want {
			t.Fatalf("%q %v", body, err)
		}
	}
	body = ""
	if err := c.Apps.Files.Write(ctx, "app1", "/big", make([]byte, maxFileSize+1)); err == nil || body != "" {
		t.Fatal("oversized write must fail before sending")
	}
	// A 10 MB binary is accepted (the cap is inclusive and on decoded bytes).
	if err := c.Apps.Files.Write(ctx, "app1", "/max", bytes.Repeat([]byte{0xff}, maxFileSize)); err != nil || !strings.HasPrefix(body, `{"path":"/max","content":"////`) {
		t.Fatal(len(body), err)
	}
	// Delete sends {path} as a JSON body (it used to be a query string).
	if err := c.Apps.Files.Delete(ctx, "app1", "/old.txt"); err != nil || body != `{"path":"/old.txt"}` {
		t.Fatalf("%q %v", body, err)
	}
	// Data that is not base64 is an undecodable body.
	c = testClient(t, reply(200, `{"status":"success","response":{"encoding":"base64","data":"!!"}}`))
	if _, err := c.Apps.Files.Read(ctx, "app1", "/f"); asAPIError(t, err).Code != CodeUnknown {
		t.Fatal(err)
	}
}

func TestFilesErrors(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		status int
		code   string
		call   func(*Client) error
	}{
		{404, CodeFileNotFound, func(c *Client) error { _, err := c.Apps.Files.List(ctx, "app1", "/missing"); return err }},
		{403, CodeBlockedPath, func(c *Client) error { _, err := c.Apps.Files.List(ctx, "app1", "/.git"); return err }},
		{400, CodeInvalidEncoding, func(c *Client) error { _, err := c.Apps.Files.Read(ctx, "app1", "/f"); return err }},
		{413, CodeFileTooLarge, func(c *Client) error { _, err := c.Apps.Files.Read(ctx, "app1", "/f"); return err }},
		{400, CodeInvalidContent, func(c *Client) error { return c.Apps.Files.Write(ctx, "app1", "/f", []byte("x")) }},
	} {
		c := testClient(t, reply(tc.status, `{"status":"error","code":"`+tc.code+`"}`))
		if e := asAPIError(t, tc.call(c)); e.Status != tc.status || e.Code != tc.code {
			t.Fatalf("%+v", e)
		}
	}
}

// A window with no traffic is {}: the result is nil, not a zero struct.
func TestEmptyNetworkWindowIsNil(t *testing.T) {
	ctx, t0 := context.Background(), time.Now()
	for _, body := range []string{`{"status":"success","response":{}}`, `{"status":"success","response":{ }}`, `{"status":"success"}`, `{"status":"success","response":null}`} {
		c := testClient(t, reply(200, body))
		a, err1 := c.Apps.Network.Analytics(ctx, "app1", t0, t0, AnalyticsFilters{})
		e, err2 := c.Apps.Network.Errors(ctx, "app1", t0, t0, false)
		p, err3 := c.Apps.Network.Performance(ctx, "app1", t0, t0)
		if a != nil || e != nil || p != nil || errors.Join(err1, err2, err3) != nil {
			t.Fatal(body, a, e, p, err1, err2, err3)
		}
	}
	c := testClient(t, reply(200, `{"status":"success","response":{"summary":{"total":3}}}`))
	if e, err := c.Apps.Network.Errors(ctx, "app1", t0, t0, true); err != nil || e == nil || e.Summary.Total != 3 {
		t.Fatal(e, err)
	}
	c = testClient(t, reply(200, `{"status":"success","response":{"summary":"x"}}`))
	if _, err := c.Apps.Network.Errors(ctx, "app1", t0, t0, true); asAPIError(t, err).Code != CodeUnknown {
		t.Fatal(err)
	}
}

// New response fields: snapshot version and URL, deploy error events,
// nullable member names and database certificates.
func TestResponseFields(t *testing.T) {
	ctx := context.Background()
	c := testClient(t, reply(200, `{"status":"success","response":[{"name":"n","size":1,"modified":"2026-01-01T00:00:00Z","key":"k","version_id":"3HL4kqtJlcpXroDTDmJ.rmSpXd3dIbrHY","url":"https://example.com/s.zip?sig"}]}`))
	snaps, err := c.Apps.Snapshots.List(ctx, "app1")
	if err != nil || len(snaps) != 1 || snaps[0].VersionID != "3HL4kqtJlcpXroDTDmJ.rmSpXd3dIbrHY" || snaps[0].URL != "https://example.com/s.zip?sig" {
		t.Fatal(snaps, err)
	}
	c = testClient(t, reply(200, `{"status":"success","response":[[{"id":"d1","state":"clone","date":"2026-01-01T00:00:00Z","source":"git","branch":"main"},`+
		`{"id":"d1","state":"error","date":"2026-01-01T00:00:01Z","source":"git","code":"CLONE_FAILED","message":"repository not found"}]]}`))
	deploys, err := c.Apps.Deploys.List(ctx, "app1")
	if err != nil || len(deploys) != 1 || len(deploys[0]) != 2 {
		t.Fatal(deploys, err)
	}
	if ev := deploys[0][1]; ev.State != DeployError || ev.Code != "CLONE_FAILED" || ev.Message != "repository not found" || ev.Source != "git" || deploys[0][0].Code != "" {
		t.Fatalf("%+v", deploys)
	}
	c = testClient(t, reply(200, `{"status":"success","response":{"id":"`+strings.Repeat("a", 40)+`","members":[{"id":"u","name":null,"group":"owner","joinedAt":"2026-01-01T00:00:00Z"}]}}`))
	ws, err := c.Workspaces.Get(ctx, "ws1")
	if err != nil || len(ws.Members) != 1 || ws.Members[0].Name != nil {
		t.Fatal(ws, err)
	}
	c = testClient(t, reply(200, `{"status":"success","response":{"id":"d","certificate":null}}`))
	db, err := c.Databases.Create(ctx, DatabaseCreate{Name: "d", Memory: 512, Type: DatabaseRedis, Version: "7"})
	if err != nil || db.Certificate != nil {
		t.Fatal(db, err)
	}
}

func TestEnvsDeleteReturnsRemaining(t *testing.T) {
	var body string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = io.WriteString(w, `{"status":"success","response":{"KEEP":"1"}}`)
	})
	left, err := c.Apps.Envs.Delete(context.Background(), "app1", "A", "B")
	if err != nil || left["KEEP"] != "1" || body != `{"envs":["A","B"]}` {
		t.Fatal(left, body, err)
	}
	if _, err := c.Apps.Envs.Delete(context.Background(), "app1"); err != nil || body != `{"envs":[]}` {
		t.Fatal(body, err) // never "envs": null
	}
}

func TestFractionalCPU(t *testing.T) {
	c := testClient(t, reply(200, `{"status":"success","response":{"id":"d","cpu":0.5,"password":"p"}}`))
	db, err := c.Databases.Create(context.Background(), DatabaseCreate{Name: "d", Memory: 512, Type: DatabaseMongo, Version: "8"})
	if err != nil || db.CPU != 0.5 || db.Password != "p" {
		t.Fatal(db, err)
	}
}

// Live bodies: lastModified can be fractional or null, and POST /apps sends
// the full domain and the cluster.
func TestLiveShapes(t *testing.T) {
	c := testClient(t, reply(200, `{"status":"success","response":[{"type":"file","name":"package-lock.json","size":90,"lastModified":1790520641499.565},{"type":"directory","name":"src","lastModified":null},{"type":"directory","name":"new"}]}`))
	files, err := c.Apps.Files.List(context.Background(), "app1", "")
	if err != nil || len(files) != 3 || *files[0].LastModified != 1790520641499.565 || files[1].LastModified != nil || files[2].Size != 0 {
		t.Fatal(files, err)
	}
	c = testClient(t, reply(200, `{"status":"success","response":{"id":"a","name":"n","domain":"n.squareweb.app","ram":512,"cpu":2,"language":{"name":"JavaScript","version":"recommended"},"cluster":"example-cluster"}}`))
	app, err := c.Apps.Create(context.Background(), strings.NewReader("PK\x03\x04"))
	if err != nil || app.Domain != "n.squareweb.app" || app.Cluster != "example-cluster" || app.CPU != 2 {
		t.Fatal(app, err)
	}
	// A window with no requests: null summary percentiles, not 0.
	none := `{"p50":null,"p95":null,"p99":null}`
	c = testClient(t, reply(200, `{"status":"success","response":{"summary":{"edge":`+none+`,"origin":{"p50":1.5,"p95":null,"p99":null},"requests":0},"timeseries":[],"countries":[],"colos":[],"slowest_paths":[]}}`))
	p, err := c.Apps.Network.Performance(context.Background(), "app1", time.Unix(0, 0), time.Unix(1, 0))
	if err != nil || p == nil || p.Summary.Edge.P50 != nil || p.Summary.Origin.P50 == nil || *p.Summary.Origin.P50 != 1.5 {
		t.Fatal(p, err)
	}
	// GET /apps/{id} drops custom (and domain on non-web apps) when unset.
	c = testClient(t, reply(200, `{"status":"success","response":{"id":"a","name":"n","owner":"u","cluster":"c","ram":256,"language":"javascript","created_at":"2026-01-01T00:00:00Z"}}`))
	got, err := c.Apps.Get(context.Background(), "a")
	if err != nil || got.Custom != nil || got.Domain != nil {
		t.Fatal(got, err)
	}
}
