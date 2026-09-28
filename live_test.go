//go:build live

package squarecloud

// The live suite (SDK-STANDARD 10.1) runs every operation against the real
// API with the key in SQUARECLOUD_API_KEY, creating and deleting real
// resources named sdk-live-go-<base36 ms>. It is skipped without the key and
// never runs in CI:
//
//	SQUARECLOUD_API_KEY=... go test -tags live -run TestLive -count=1 -timeout 30m .
//
// The key, database passwords and certificates are never logged.

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// livePace spaces every request: the API allows an API key 30 calls per 60s
// on the smallest paid plan (a 30-minute block past it).
const livePace = 2100 * time.Millisecond

// paced is a RoundTripper that sends at most one request per livePace and
// keeps the last 2xx JSON body for the shape checks.
type paced struct {
	rt   http.RoundTripper
	mu   sync.Mutex
	next time.Time
	last []byte
}

func (p *paced) RoundTrip(r *http.Request) (*http.Response, error) {
	p.mu.Lock()
	d := time.Until(p.next)
	p.next = time.Now().Add(max(d, 0) + livePace)
	p.mu.Unlock()
	if d > 0 && !wait(r.Context(), d) {
		return nil, r.Context().Err()
	}
	res, err := p.rt.RoundTrip(r)
	if err != nil || res.StatusCode/100 != 2 || !strings.Contains(res.Header.Get("Content-Type"), "json") {
		return res, err
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	p.mu.Lock()
	p.last = body
	p.mu.Unlock()
	return res, nil
}

type liveRun struct {
	t       *testing.T
	ctx     context.Context
	c       *Client
	p       *paced
	touched map[string]bool
}

// calm runs f, waiting 10s and retrying (test only: the SDK never retries
// these) on 429 KEEP_CALM at most 7 times (the github-app and webhook
// windows are 60s per user, and another SDK's suite may have just used them),
// and on 403 RESTORE_IN_PROGRESS for up to 130s (the API blocks deletes for
// 120s after a restore).
func calm(ctx context.Context, f func() error) error {
	restoring := time.Now().Add(130 * time.Second)
	for calms := 0; ; {
		err := f()
		var e *APIError
		switch {
		case !errors.As(err, &e):
			return err
		case e.Status == 429 && e.Code == CodeKeepCalm && calms < 7:
			calms++
		case e.Status == 403 && e.Code == "RESTORE_IN_PROGRESS" && time.Now().Before(restoring):
		default:
			return err
		}
		if !wait(ctx, 10*time.Second) {
			return ctx.Err()
		}
	}
}

// do runs op through calm.
func (l *liveRun) do(op string, f func() error) error {
	l.touched[op] = true
	return calm(l.ctx, f)
}

// must requires success.
func (l *liveRun) must(op string, f func() error) bool {
	if err := l.do(op, f); err != nil {
		if cause := errors.Unwrap(err); cause != nil {
			err = fmt.Errorf("%w (%v)", err, cause) // e.g. the JSON type error behind "Invalid JSON"
		}
		l.t.Errorf("%s: %v", op, err)
		return false
	}
	return true
}

// shape strictly decodes the last 2xx JSON body (its response field when
// enveloped) into the type of got: a field the API sends that the SDK type
// lacks, or a type the SDK gets wrong, is a shape mismatch. The decoder's
// errors name fields and types, never values, so secrets stay out of the log.
func (l *liveRun) shape(op string, got any) {
	typ := reflect.TypeOf(got)
	if typ == nil || typ.Kind() == reflect.String || typ == reflect.TypeFor[[]byte]() {
		return // void, or a single field unwrapped from an object
	}
	l.p.mu.Lock()
	body := l.p.last
	l.p.mu.Unlock()
	var env struct {
		Status   string          `json:"status"`
		Response json.RawMessage `json:"response"`
	}
	if json.Unmarshal(body, &env) == nil && env.Status == "success" {
		body = env.Response
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(reflect.New(typ).Interface()); err != nil && strings.HasSuffix(err.Error(), `"unhydratable"`) {
		// ponytail: the API leaks its internal workspace hydration flag
		// on uncached reads (libs/data/Workspace.js); it is not in the spec.
		l.t.Logf("%s: API sends the undocumented internal field unhydratable", op)
	} else if err != nil {
		l.t.Errorf("%s: response does not strictly decode into %v: %v", op, typ, err)
	}
}

// val is must for an operation with a result, which it shape-checks.
func val[T any](l *liveRun, op string, f func() (T, error)) (T, bool) {
	var x T
	ok := l.must(op, func() (err error) { x, err = f(); return err })
	if ok {
		l.shape(op, x)
	}
	return x, ok
}

// gatedCodes are the documented plan, quota and scope refusals. They are
// matched ignoring case: AI.Chat answers in the OpenAI dialect
// (upgrade_required).
var gatedCodes = []string{"UPGRADE_REQUIRED", "INSUFFICIENT_MEMORY", CodeMissingScope,
	"DAILY_SNAPSHOTS_LIMIT_REACHED", "WORKSPACE_LIMIT_REACHED"}

// gated accepts success or a documented plan, quota or scope error,
// reported as a skip. It reports whether op succeeded.
func (l *liveRun) gated(op string, f func() error) bool {
	return l.gatedOn(op, gatedCodes, f)
}

// gatedOn is gated with its own list of accepted codes.
func (l *liveRun) gatedOn(op string, codes []string, f func() error) bool {
	err := l.do(op, f)
	var e *APIError
	switch {
	case err == nil:
		return true
	case errors.As(err, &e) && slices.ContainsFunc(codes, func(c string) bool { return strings.EqualFold(c, e.Code) }):
		l.t.Logf("skip %s: %s", op, e.Code)
	default:
		l.t.Errorf("%s: %v", op, err)
	}
	return false
}

// gatedVal is gated for an operation with a result, which it shape-checks.
func gatedVal[T any](l *liveRun, op string, f func() (T, error)) (T, bool) {
	var x T
	ok := l.gated(op, func() (err error) { x, err = f(); return err })
	if ok {
		l.shape(op, x)
	}
	return x, ok
}

// fails requires an HTTP 4xx other than 429, with code when code is not "".
func (l *liveRun) fails(op, code string, f func() error) {
	l.failsOr(op, code, nil, f)
}

// failsOr is fails that also accepts a 5xx whose code is in upstream: a
// failure of the service the route calls, not of the SDK.
func (l *liveRun) failsOr(op, code string, upstream []string, f func() error) {
	err := l.do(op, f)
	var e *APIError
	switch {
	case !errors.As(err, &e):
	case e.Status >= 400 && e.Status <= 499 && e.Status != 429 && (code == "" || e.Code == code):
		l.t.Logf("%s: HTTP %d %s (expected)", op, e.Status, e.Code)
		return
	case e.Status >= 500 && slices.Contains(upstream, e.Code):
		l.t.Logf("%s: HTTP %d %s", op, e.Status, e.Code)
		return
	}
	l.t.Errorf("%s: want HTTP 4xx %s, got %v", op, code, err)
}

// okOr4xx accepts success or any HTTP 4xx (e.g. DNS without a custom domain).
func (l *liveRun) okOr4xx(op string, f func() error) {
	err := l.do(op, f)
	var e *APIError
	if err != nil && (!errors.As(err, &e) || e.Status < 400 || e.Status > 499) {
		l.t.Errorf("%s: %v", op, err)
	} else if err != nil {
		l.t.Logf("%s: HTTP %d %s", op, e.Status, e.Code)
	}
}

// isLocal reports whether err is a local check (Status 0) with code.
func isLocal(err error, code string) bool {
	var e *APIError
	return errors.As(err, &e) && e.Status == 0 && e.Code == code
}

// until polls cond every 5s (through the pacer) for up to limit.
func (l *liveRun) until(what string, limit time.Duration, cond func() bool) bool {
	for end := time.Now().Add(limit); ; {
		if cond() {
			return true
		}
		if time.Now().After(end) || !wait(l.ctx, 5*time.Second) {
			l.t.Logf("%s: gave up after %v", what, limit)
			return false
		}
	}
}

// cleanup deletes one resource through calm (l.ctx is already canceled
// when cleanups run); a 404 is fine and a failure never stops the other
// deletes. op counts as touched: the cleanup asserts its result.
func (l *liveRun) cleanup(op, what string, f func(context.Context) error) {
	l.touched[op] = true
	l.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := calm(ctx, func() error { return f(ctx) })
		var e *APIError
		if err != nil && !(errors.As(err, &e) && e.Status == 404) {
			l.t.Errorf("cleanup %s %s: %v", op, what, err)
		}
	})
}

func TestLive(t *testing.T) {
	key := os.Getenv("SQUARECLOUD_API_KEY")
	if key == "" {
		t.Skip("SQUARECLOUD_API_KEY is not set")
	}
	// Undo the offline suite's test settings.
	retryBase, realtimePace = shippedRetryBase, shippedPace
	// No deadline here: each call keeps the SDK's own timeouts.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &paced{rt: &http.Transport{Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true}}
	c := New(key, WithHTTPClient(&http.Client{Transport: p}))
	l := &liveRun{t: t, ctx: ctx, c: c, p: p, touched: map[string]bool{}}
	prefix := "sdk-live-go-"
	name := prefix + strconv.FormatInt(time.Now().UnixMilli(), 36)
	// Registered first, so it runs after every delete: nothing with the
	// prefix may remain on the account (as in the JS and PY suites).
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		var me Account
		var ws []Workspace
		err := calm(ctx, func() (err error) { me, err = c.Account.Me(ctx); return err })
		if err == nil {
			err = calm(ctx, func() (err error) { ws, err = c.Workspaces.List(ctx); return err })
		}
		if err != nil {
			t.Errorf("final account listing: %v", err)
			return
		}
		var left []string
		for _, a := range me.Applications {
			if strings.HasPrefix(a.Name, prefix) {
				left = append(left, "app "+a.ID)
			}
		}
		for _, d := range me.Databases {
			if strings.HasPrefix(d.Name, prefix) {
				left = append(left, "database "+d.ID)
			}
		}
		for _, w := range ws {
			if strings.HasPrefix(w.Name, prefix) {
				left = append(left, "workspace "+w.ID)
			}
		}
		if len(left) > 0 {
			t.Errorf("left on the account: %v", left)
		} else {
			t.Log("account is left clean")
		}
	})

	// 1. Read-only (the account read also feeds the stale sweep).
	val(l, "Service.Status", func() (ServiceStatus, error) { return c.Service.Status(ctx) })
	me, ok := val(l, "Account.Me", func() (Account, error) { return c.Account.Me(ctx) })
	if !ok {
		t.FailNow()
	}
	for _, scope := range []SnapshotScope{SnapshotScopeApplications, SnapshotScopeDatabases} {
		gatedVal(l, "Account.Snapshots", func() ([]Snapshot, error) { return c.Account.Snapshots(ctx, scope) })
	}
	val(l, "Apps.StatusAll", func() ([]StatusListItem, error) { return c.Apps.StatusAll(ctx, "") })
	val(l, "Apps.Domains", func() ([]AppDomain, error) { return c.Apps.Domains(ctx) })
	val(l, "Apps.LoadBalancers", func() (LoadBalancers, error) { return c.Apps.LoadBalancers(ctx) })
	val(l, "Databases.StatusAll", func() ([]StatusListItem, error) { return c.Databases.StatusAll(ctx) })
	workspaces, _ := val(l, "Workspaces.List", func() ([]Workspace, error) { return c.Workspaces.List(ctx) })
	// One call per 5 minutes per user: another SDK's suite may have just
	// used it, which outlasts the KEEP_CALM retries.
	l.gatedOn("Workspaces.Members.InviteCode", []string{CodeKeepCalm}, func() error {
		code, err := c.Workspaces.Members.InviteCode(ctx)
		if err == nil && code == "" {
			err = errors.New("empty invite code")
		}
		return err
	})

	// Sweep resources left by earlier runs of this suite, and nothing else.
	// Deletes go through calm (1 per 3s per user); "sweep" is not an
	// operation name, so it does not count as touched.
	sweep := func(kind, id string, del func(context.Context, string) error) {
		t.Logf("sweep %s %s", kind, id)
		if err := l.do("sweep", func() error { return del(ctx, id) }); err != nil {
			t.Logf("sweep %s %s: %v", kind, id, err)
		}
	}
	for _, a := range me.Applications {
		if strings.HasPrefix(a.Name, prefix) {
			sweep("app", a.ID, c.Apps.Delete)
		}
	}
	for _, d := range me.Databases {
		if strings.HasPrefix(d.Name, prefix) {
			sweep("database", d.ID, c.Databases.Delete)
		}
	}
	for _, w := range workspaces {
		if strings.HasPrefix(w.Name, prefix) && w.Owner == me.User.ID {
			sweep("workspace", w.ID, c.Workspaces.Delete)
		}
	}

	// 2. Error paths.
	if _, err := c.Apps.Get(ctx, ".."); !isLocal(err, CodeInvalidID) {
		t.Errorf(`Apps.Get(".."): want a local INVALID_ID, got %v`, err)
	}
	l.fails("Apps.Get", "APP_NOT_FOUND", func() error { _, err := c.Apps.Get(ctx, strings.Repeat("0", 32)); return err })

	// 3. App.
	web := me.User.Plan.Memory.Available >= 512
	appID := liveApp(l, name, web)
	if appID != "" {
		liveNetwork(l, appID, web)
		liveSnapshots(l, appID)
	}
	liveWorkspace(l, name, appID)
	liveDatabase(l, name)

	// 8. AI: one short request.
	gatedVal(l, "AI.Chat", func() (ChatCompletion, error) {
		return c.AI.Chat(ctx, ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "ping"}}, MaxTokens: 5})
	})

	// Every public method was touched, by success or by an asserted error.
	var missing []string
	for _, tc := range confCases() {
		if !l.touched[tc.name] {
			missing = append(missing, tc.name)
		}
	}
	if len(missing) > 0 && appID != "" {
		t.Errorf("operations not exercised: %v", missing)
	}
	t.Logf("operations touched: %d", len(l.touched)-btoi(l.touched["sweep"]))
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func liveApp(l *liveRun, name string, web bool) string {
	ctx, c := l.ctx, l.c
	config := "DISPLAY_NAME=" + name + "\nMAIN=index.js\nVERSION=recommended\n"
	index := "setInterval(() => console.log('sdk-live tick', Date.now()), 1000);\n"
	if web {
		config += "MEMORY=512\nSUBDOMAIN=" + name + "\n"
		index = "require('http').createServer((q, s) => s.end('ok')).listen(process.env.PORT || 80);\n" + index
	} else {
		config += "MEMORY=256\n"
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// package.json is required for Node (400 INVALID_DEPENDENCY without it).
	for file, body := range map[string]string{"squarecloud.app": config, "package.json": "{}\n", "index.js": index} {
		w, _ := zw.Create(file)
		_, _ = io.WriteString(w, body)
	}
	_ = zw.Close()

	app, ok := val(l, "Apps.Create", func() (AppCreated, error) { return c.Apps.Create(ctx, bytes.NewReader(buf.Bytes())) })
	if !ok {
		return ""
	}
	id := app.ID
	l.cleanup("Apps.Delete", id, func(ctx context.Context) error { return c.Apps.Delete(ctx, id) })
	if app.Name != name || app.RAM == 0 || app.Language.Name == "" {
		l.t.Errorf("Apps.Create: unexpected %+v", app)
	}

	if a, ok := val(l, "Apps.Get", func() (App, error) { return c.Apps.Get(ctx, id) }); ok && (a.ID != id || a.Name != name) {
		l.t.Errorf("Apps.Get: got %s %q", a.ID, a.Name)
	}
	val(l, "Apps.Status", func() (RuntimeStats, error) { return c.Apps.Status(ctx, id) })
	val(l, "Apps.StatusRaw", func() (RuntimeStatsRaw, error) { return c.Apps.StatusRaw(ctx, id) })
	val(l, "Apps.Logs", func() (string, error) { return c.Apps.Logs(ctx, id) })
	val(l, "Apps.Metrics", func() ([]MetricPoint, error) { return c.Apps.Metrics(ctx, id) })
	l.must("Apps.Stop", func() error { return c.Apps.Stop(ctx, id) })
	l.must("Apps.Start", func() error { return c.Apps.Start(ctx, id) })
	liveRealtime(l, id)

	if all, ok := val(l, "Apps.Envs.Set", func() (EnvVars, error) { return c.Apps.Envs.Set(ctx, id, EnvVars{"SDK_LIVE": "1"}) }); ok && all["SDK_LIVE"] != "1" {
		l.t.Errorf("Apps.Envs.Set did not return the variable")
	}
	if all, ok := val(l, "Apps.Envs.Get", func() (EnvVars, error) { return c.Apps.Envs.Get(ctx, id) }); ok && all["SDK_LIVE"] != "1" {
		l.t.Errorf("Apps.Envs.Get misses the variable")
	}
	want := EnvVars{"SDK_LIVE": "2", "OTHER": "x"}
	if all, ok := val(l, "Apps.Envs.Replace", func() (EnvVars, error) { return c.Apps.Envs.Replace(ctx, id, want) }); ok && !maps.Equal(all, want) {
		l.t.Errorf("Apps.Envs.Replace: got %v, want %v", all, want)
	}
	if left, ok := val(l, "Apps.Envs.Delete", func() (EnvVars, error) { return c.Apps.Envs.Delete(ctx, id, "SDK_LIVE") }); ok && !maps.Equal(left, EnvVars{"OTHER": "x"}) {
		l.t.Errorf("Apps.Envs.Delete: left %v", left)
	}

	// Round trips: every byte value, UTF-8 text and an empty file.
	var bin [256]byte
	for i := range bin {
		bin[i] = byte(i)
	}
	text := []byte("olá, 世界 sdk-live\n")
	readBack := func(path string, want []byte) {
		if got, ok := val(l, "Apps.Files.Read", func() ([]byte, error) { return c.Apps.Files.Read(ctx, id, path) }); ok && !bytes.Equal(got, want) {
			l.t.Errorf("Apps.Files.Read %s: got %q, want %q", path, got, want)
		}
	}
	for path, content := range map[string][]byte{"/sdk-live.txt": text, "/sdk-live.bin": bin[:], "/sdk-live-empty.txt": {}} {
		if l.must("Apps.Files.Write", func() error { return c.Apps.Files.Write(ctx, id, path, content) }) {
			readBack(path, content)
		}
	}
	l.must("Apps.Files.Delete", func() error { return c.Apps.Files.Delete(ctx, id, "/sdk-live-empty.txt") })
	l.fails("Apps.Files.List", CodeFileNotFound, func() error { _, err := c.Apps.Files.List(ctx, id, "/sdk-live-missing-dir"); return err })
	if entries, ok := val(l, "Apps.Files.List", func() ([]FileEntry, error) { return c.Apps.Files.List(ctx, id, "") }); ok &&
		!slices.ContainsFunc(entries, func(e FileEntry) bool { return e.Name == "sdk-live.txt" && e.Type == "file" }) {
		l.t.Errorf("Apps.Files.List misses sdk-live.txt: %+v", entries)
	}
	l.must("Apps.Files.Move", func() error { return c.Apps.Files.Move(ctx, id, "/sdk-live.txt", "/sdk-live-moved.txt") })
	readBack("/sdk-live-moved.txt", text)
	l.must("Apps.Files.Delete", func() error { return c.Apps.Files.Delete(ctx, id, "/sdk-live-moved.txt") })
	l.must("Apps.Files.Delete", func() error { return c.Apps.Files.Delete(ctx, id, "/sdk-live.bin") })
	if entries, ok := val(l, "Apps.Files.List", func() ([]FileEntry, error) { return c.Apps.Files.List(ctx, id, "/") }); ok &&
		slices.ContainsFunc(entries, func(e FileEntry) bool { return strings.HasPrefix(e.Name, "sdk-live") }) {
		l.t.Errorf("Apps.Files.Delete left files: %+v", entries)
	}

	// Commit a non-zip file at the root and into a directory, then read both.
	for _, path := range []string{"", "sdk-live-dir"} {
		body := "// sdk-live commit " + path + "\n"
		if l.must("Apps.Commit", func() error { return c.Apps.Commit(ctx, id, strings.NewReader(body), path, "sdk-live-commit.js") }) {
			readBack(strings.TrimSuffix("/"+path, "/")+"/sdk-live-commit.js", []byte(body))
		}
	}

	val(l, "Apps.Deploys.List", func() ([][]DeployEvent, error) { return c.Apps.Deploys.List(ctx, id) })
	val(l, "Apps.Deploys.Current", func() (DeployCurrent, error) { return c.Apps.Deploys.Current(ctx, id) })
	l.fails("Apps.Deploys.SetWebhook", "", func() error { _, err := c.Apps.Deploys.SetWebhook(ctx, id, "invalid"); return err })
	// The test account has no GitHub App installation.
	l.fails("Apps.Deploys.LinkGithubApp", CodeGithubNotConnected, func() error {
		_, err := c.Apps.Deploys.LinkGithubApp(ctx, id, "squarecloudofc/does-not-exist", "main")
		return err
	})
	l.fails("Apps.Deploys.UnlinkGithubApp", CodeGitNotConfigured, func() error { return c.Apps.Deploys.UnlinkGithubApp(ctx, id) })
	return id
}

// liveRealtime opens the stream, restarts the app while it is open, and reads
// until a logs or status event arrives after the restart (or 90s pass).
func liveRealtime(l *liveRun, id string) {
	ctx, c := l.ctx, l.c
	l.touched["Apps.Restart"] = true
	l.must("Apps.Realtime", func() error {
		rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		s, err := c.Apps.Realtime(rctx, id)
		if err != nil {
			return err
		}
		defer s.Close()
		restarted := make(chan error, 1)
		go func() { restarted <- calm(ctx, func() error { return c.Apps.Restart(ctx, id) }) }()
		var restart error
		done, counts := false, map[string]int{}
		defer func() { l.t.Logf("Apps.Realtime during restart: %v", counts) }()
		for {
			ev, err := s.Next()
			if !done {
				select {
				case restart = <-restarted:
					done = true
					if restart != nil {
						l.t.Errorf("Apps.Restart: %v", restart)
					}
				default:
				}
			}
			switch {
			case err != nil && (rctx.Err() != nil || errors.Is(err, io.EOF)):
				if !done {
					if restart = <-restarted; restart != nil {
						l.t.Errorf("Apps.Restart: %v", restart)
					}
				}
				if counts["logs"]+counts["status"] == 0 {
					return errors.New("no logs or status event in 90s")
				}
				return nil
			case err != nil:
				return err
			case ev.Event == "status" && ev.Status == nil:
				return errors.New("status event without Status")
			}
			counts[ev.Event]++
			if done && (ev.Event == "logs" || ev.Event == "status") {
				return nil
			}
		}
	})
}

// 4. Network: a website answers; any other app gets the documented 4xx, or
// the edge query's 500 UNABLE_TO_FETCH_*. Logs and Performance also need Pro
// or Enterprise (403 UPGRADE_REQUIRED).
func liveNetwork(l *liveRun, id string, web bool) {
	ctx, c := l.ctx, l.c
	end := time.Now()
	start := end.Add(-24 * time.Hour)
	if web {
		val(l, "Apps.Network.Analytics", func() (*NetworkAnalytics, error) {
			return c.Apps.Network.Analytics(ctx, id, start, end, AnalyticsFilters{})
		})
		val(l, "Apps.Network.Errors", func() (*NetworkErrors, error) { return c.Apps.Network.Errors(ctx, id, start, end, true) })
		gatedVal(l, "Apps.Network.Logs", func() ([]NetworkLog, error) { return c.Apps.Network.Logs(ctx, id, start, end) })
		gatedVal(l, "Apps.Network.Performance", func() (*NetworkPerformance, error) { return c.Apps.Network.Performance(ctx, id, start, end) })
	} else {
		noDomain := []string{"UNABLE_TO_FETCH_ANALYTICS", "UNABLE_TO_FETCH_ERRORS", "UNABLE_TO_FETCH_PERFORMANCE"}
		l.failsOr("Apps.Network.Analytics", "", noDomain, func() error {
			_, err := c.Apps.Network.Analytics(ctx, id, start, end, AnalyticsFilters{})
			return err
		})
		l.failsOr("Apps.Network.Errors", "", noDomain, func() error { _, err := c.Apps.Network.Errors(ctx, id, start, end, false); return err })
		l.failsOr("Apps.Network.Logs", "", noDomain, func() error { _, err := c.Apps.Network.Logs(ctx, id, start, end); return err })
		l.failsOr("Apps.Network.Performance", "", noDomain, func() error { _, err := c.Apps.Network.Performance(ctx, id, start, end); return err })
	}
	l.okOr4xx("Apps.Network.DNS", func() error {
		recs, err := c.Apps.Network.DNS(ctx, id)
		if err == nil {
			l.shape("Apps.Network.DNS", recs)
		}
		return err
	})
	l.okOr4xx("Apps.Network.PurgeCache", func() error { return c.Apps.Network.PurgeCache(ctx, id) })
	// Never a well-formed domain: on a paid plan it is registered at the
	// CDN. A malformed one is refused first on every plan, and "@" on an app
	// without a custom domain only clears the field (as in JS and PY).
	l.fails("Apps.Network.SetDomain", "INVALID_DOMAIN", func() error { return c.Apps.Network.SetDomain(ctx, id, "not a domain") })
	l.gated("Apps.Network.SetDomain", func() error { return c.Apps.Network.SetDomain(ctx, id, "@") })
}

// 5. Snapshots: create, download (zip magic), list and restore the latest.
// A 202 (still generating) is followed by a few list polls.
func liveSnapshots(l *liveRun, id string) {
	ctx, c := l.ctx, l.c
	snap, created := gatedVal(l, "Apps.Snapshots.Create", func() (SnapshotCreated, error) { return c.Apps.Snapshots.Create(ctx, id) })
	if created && !snap.Pending {
		var head bytes.Buffer
		if err := c.DownloadSnapshot(ctx, snap.URL, &head); err != nil || !bytes.HasPrefix(head.Bytes(), []byte("PK\x03\x04")) {
			l.t.Errorf("DownloadSnapshot: %v (want a zip, got %d bytes)", err, head.Len())
		} else {
			l.t.Logf("DownloadSnapshot: %d bytes, zip header ok", head.Len())
		}
	}
	snaps := listSnapshots(l, "Apps.Snapshots.List", created && snap.Pending, func() ([]Snapshot, error) { return c.Apps.Snapshots.List(ctx, id) })
	if len(snaps) > 0 {
		l.must("Apps.Snapshots.Restore", func() error { return c.Apps.Snapshots.Restore(ctx, id, snaps[0].Name, snaps[0].VersionID) })
	} else {
		l.fails("Apps.Snapshots.Restore", "", func() error { return c.Apps.Snapshots.Restore(ctx, id, "sdk-live-missing", "0") })
	}
}

// listSnapshots lists (polling every 30s for up to 3 minutes while a created
// snapshot is pending) and checks every entry has a VersionID and a URL.
func listSnapshots(l *liveRun, op string, pending bool, f func() ([]Snapshot, error)) []Snapshot {
	var snaps []Snapshot
	limit := time.Duration(0)
	if pending {
		limit = 3 * time.Minute
	}
	l.until(op, limit, func() bool {
		var ok bool
		snaps, ok = val(l, op, f)
		if !pending || !ok || len(snaps) > 0 {
			return true
		}
		wait(l.ctx, 25*time.Second)
		return false
	})
	for _, s := range snaps {
		if s.Name == "" || s.VersionID == "" || s.URL == "" {
			l.t.Errorf("%s: snapshot without a name, version_id or url", op)
		}
	}
	return snaps
}

// 6. Workspace. Without the plan for it, every call runs against a missing
// workspace and must fail with a 4xx.
func liveWorkspace(l *liveRun, name, appID string) {
	ctx, c := l.ctx, l.c
	ws, created := gatedVal(l, "Workspaces.Create", func() (WorkspaceCreated, error) { return c.Workspaces.Create(ctx, name) })
	wsID := ws.ID
	if created {
		l.cleanup("Workspaces.Delete", wsID, func(ctx context.Context) error { return c.Workspaces.Delete(ctx, wsID) })
	} else {
		wsID = strings.Repeat("0", 32)
	}
	either := l.must
	if !created {
		either = func(op string, f func() error) bool { l.fails(op, "", f); return true }
	}
	get := func(check func(Workspace) error) {
		either("Workspaces.Get", func() error {
			w, err := c.Workspaces.Get(ctx, wsID)
			if err == nil {
				l.shape("Workspaces.Get", w)
				err = check(w)
			}
			return err
		})
	}
	get(func(w Workspace) error {
		if w.ID != wsID || w.Name != name {
			return fmt.Errorf("got %s %q", w.ID, w.Name)
		}
		return nil
	})
	if appID != "" {
		either("Workspaces.Apps.Add", func() error { return c.Workspaces.Apps.Add(ctx, wsID, appID) })
		if created {
			get(func(w Workspace) error {
				if !slices.ContainsFunc(w.Applications, func(a WorkspaceApp) bool { return a.ID == appID }) {
					return errors.New("the added app is missing")
				}
				return nil
			})
		}
		either("Workspaces.Apps.Remove", func() error { return c.Workspaces.Apps.Remove(ctx, wsID, appID) })
	}
	l.fails("Workspaces.Members.Add", "", func() error { return c.Workspaces.Members.Add(ctx, wsID, "invalid", GroupView) })
	// An unknown member is 404 MEMBER_NOT_FOUND in an existing workspace.
	member := ""
	if created {
		member = CodeMemberNotFound
	}
	l.fails("Workspaces.Members.Update", member, func() error { return c.Workspaces.Members.Update(ctx, wsID, "0", GroupView) })
	l.fails("Workspaces.Members.Remove", member, func() error { return c.Workspaces.Members.Remove(ctx, wsID, "0") })
	l.fails("Workspaces.Leave", "", func() error { return c.Workspaces.Leave(ctx, wsID) }) // the owner cannot leave
	either("Workspaces.Delete", func() error { return c.Workspaces.Delete(ctx, wsID) })
}

// 7. Database (the smallest engine: redis, 512 MB). Without the plan for it,
// every call runs against a missing database and must fail with a 4xx.
func liveDatabase(l *liveRun, name string) {
	ctx, c := l.ctx, l.c
	db, created := gatedVal(l, "Databases.Create", func() (DatabaseCreated, error) {
		return c.Databases.Create(ctx, DatabaseCreate{Name: name, Memory: 512, Type: DatabaseRedis, Version: "8"})
	})
	id := db.ID
	if !created {
		id = strings.Repeat("0", 32)
		either := func(op string, f func() error) { l.fails(op, "", f) }
		for op, f := range map[string]func() error{
			"Databases.Get":              func() error { _, err := c.Databases.Get(ctx, id); return err },
			"Databases.Status":           func() error { _, err := c.Databases.Status(ctx, id); return err },
			"Databases.StatusRaw":        func() error { _, err := c.Databases.StatusRaw(ctx, id); return err },
			"Databases.Metrics":          func() error { _, err := c.Databases.Metrics(ctx, id); return err },
			"Databases.Update":           func() error { return c.Databases.Update(ctx, id, DatabaseUpdate{Name: name + "-u"}) },
			"Databases.Certificate":      func() error { _, err := c.Databases.Certificate(ctx, id); return err },
			"Databases.ResetCredentials": func() error { _, err := c.Databases.ResetCredentials(ctx, id, ResetPassword); return err },
			"Databases.Stop":             func() error { return c.Databases.Stop(ctx, id) },
			"Databases.Start":            func() error { return c.Databases.Start(ctx, id) },
			"Databases.Snapshots.Create": func() error { _, err := c.Databases.Snapshots.Create(ctx, id); return err },
			"Databases.Snapshots.List":   func() error { _, err := c.Databases.Snapshots.List(ctx, id); return err },
			"Databases.Snapshots.Restore": func() error {
				return c.Databases.Snapshots.Restore(ctx, id, "sdk-live-missing", "0")
			},
			"Databases.Delete": func() error { return c.Databases.Delete(ctx, id) },
		} {
			either(op, f)
		}
		return
	}
	l.cleanup("Databases.Delete", id, func(ctx context.Context) error { return c.Databases.Delete(ctx, id) })
	if db.Name != name || db.Type != DatabaseRedis || db.Password == "" || db.ConnectionURL == "" {
		l.t.Errorf("Databases.Create: unexpected name %q, type %q, or an empty password or connection URL", db.Name, db.Type)
	}
	running := func() bool {
		st, ok := val(l, "Databases.Status", func() (RuntimeStats, error) { return c.Databases.Status(ctx, id) })
		return ok && st.Running
	}
	l.until("database running", 2*time.Minute, running)
	if d, ok := val(l, "Databases.Get", func() (Database, error) { return c.Databases.Get(ctx, id) }); ok && (d.ID != id || d.Type != DatabaseRedis) {
		l.t.Errorf("Databases.Get: got %s %q", d.ID, d.Type)
	}
	val(l, "Databases.StatusRaw", func() (RuntimeStatsRaw, error) { return c.Databases.StatusRaw(ctx, id) })
	val(l, "Databases.Metrics", func() ([]MetricPoint, error) { return c.Databases.Metrics(ctx, id) })
	l.must("Databases.Update", func() error { return c.Databases.Update(ctx, id, DatabaseUpdate{Name: name + "-u"}) })
	if d, ok := val(l, "Databases.Get", func() (Database, error) { return c.Databases.Get(ctx, id) }); ok && d.Name != name+"-u" {
		l.t.Errorf("Databases.Update: name is %q", d.Name)
	}
	// Certificate and password are never logged, only checked for presence.
	if cert, ok := val(l, "Databases.Certificate", func() (string, error) { return c.Databases.Certificate(ctx, id) }); ok && cert == "" {
		l.t.Errorf("Databases.Certificate: empty")
	}
	if pw, ok := val(l, "Databases.ResetCredentials", func() (string, error) { return c.Databases.ResetCredentials(ctx, id, ResetPassword) }); ok && pw == "" {
		l.t.Errorf("Databases.ResetCredentials: empty password")
	}
	l.must("Databases.Stop", func() error { return c.Databases.Stop(ctx, id) })
	l.must("Databases.Start", func() error { return c.Databases.Start(ctx, id) })
	// Starting a started database is a success or a typed 409.
	if err := l.do("Databases.Start", func() error { return c.Databases.Start(ctx, id) }); err == nil {
		l.t.Log("Databases.Start again: success")
	} else if e := (*APIError)(nil); errors.As(err, &e) && e.Status == 409 && (e.Code == CodeContainerAlreadyStarted || e.Code == CodeActionFailed) {
		l.t.Logf("Databases.Start again: HTTP 409 %s", e.Code)
	} else {
		l.t.Errorf("Databases.Start again: %v", err)
	}
	l.until("database running", 2*time.Minute, running)
	snap, snapped := gatedVal(l, "Databases.Snapshots.Create", func() (SnapshotCreated, error) { return c.Databases.Snapshots.Create(ctx, id) })
	if snapped && !snap.Pending {
		var head bytes.Buffer
		if err := c.DownloadSnapshot(ctx, snap.URL, &head); err != nil || head.Len() == 0 {
			l.t.Errorf("DownloadSnapshot (database): %v (%d bytes)", err, head.Len())
		} else {
			l.t.Logf("DownloadSnapshot (database): %d bytes, header %x", head.Len(), head.Bytes()[:min(4, head.Len())])
		}
	}
	snaps := listSnapshots(l, "Databases.Snapshots.List", snapped && snap.Pending, func() ([]Snapshot, error) { return c.Databases.Snapshots.List(ctx, id) })
	if len(snaps) > 0 {
		l.must("Databases.Snapshots.Restore", func() error {
			return c.Databases.Snapshots.Restore(ctx, id, snaps[0].Name, snaps[0].VersionID)
		})
	} else {
		l.fails("Databases.Snapshots.Restore", "", func() error { return c.Databases.Snapshots.Restore(ctx, id, "sdk-live-missing", "0") })
	}
	l.must("Databases.Delete", func() error { return c.Databases.Delete(ctx, id) })
}
