package squarecloud

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// The conformance test calls every public operation against a mock server
// and checks each request against spec/openapi.json (the pinned API
// contract): the method and path template exist (every ID contains a "/", so
// a method that stops escaping matches no template), query keys are declared
// with values in their enums, JSON body keys are declared properties whose
// values match the schema (type, items, oneOf, enum; required keys present),
// and the exact query and body are pinned per case. Every 2xx example
// (alternates and 202 included) must decode into the SDK result with no
// unknown fields, and every 4xx/5xx example must come back as *APIError with
// the example's status and code.

const specVersion, specOps = "14.54.0", 69

// Out of SDK scope: codegen and GitHub's webhook.
var notSDK = map[string]bool{"GET /v2/openapi.json": true, "POST /v2/git/webhook/{webhook}": true}

type schema struct {
	Ref        string             `json:"$ref"`
	Type       any                `json:"type"`
	Required   []string           `json:"required"`
	Properties map[string]*schema `json:"properties"`
	Enum       []any              `json:"enum"`
	OneOf      []*schema          `json:"oneOf"`
	Items      *schema            `json:"items"`
}

// matches reports whether a decoded JSON value fits s (nil s: anything).
func matches(s *schema, v any) bool {
	if s == nil {
		return true
	}
	if len(s.OneOf) > 0 {
		return slices.ContainsFunc(s.OneOf, func(b *schema) bool { return matches(b, v) })
	}
	if len(s.Enum) > 0 && !slices.Contains(s.Enum, v) {
		return false
	}
	is := func(typ string) bool { return s.Type == nil || s.Type == typ }
	switch x := v.(type) {
	case string:
		return is("string")
	case bool:
		return is("boolean")
	case float64:
		return is("number") || (s.Type == "integer" && x == math.Trunc(x))
	case []any:
		for _, e := range x {
			if !matches(s.Items, e) {
				return false
			}
		}
		return is("array")
	case map[string]any:
		for k, e := range x {
			if !matches(s.Properties[k], e) {
				return false
			}
		}
		return is("object")
	}
	return s.Type == nil // null
}

type example struct {
	status int
	name   string
	body   json.RawMessage
}

type specOp struct {
	key     string // "METHOD /v2/template"
	re      *regexp.Regexp
	params  int
	query   map[string]param   // by name
	body    map[string]*schema // declared body properties (nil: no body)
	bodyReq []string
	ok      []example // every 2xx example
	fail    []example // every 4xx/5xx example
}

type param struct {
	required bool
	enum     []any
}

type rawSpec struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Responses map[string]json.RawMessage `json:"responses"`
		Schemas   map[string]*schema         `json:"schemas"`
	} `json:"components"`
}

func loadSpec(t *testing.T) (string, []*specOp) {
	t.Helper()
	b, err := os.ReadFile("spec/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec rawSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	resolve := func(raw json.RawMessage, into any) {
		var ref struct {
			Ref string `json:"$ref"`
		}
		_ = json.Unmarshal(raw, &ref)
		if name, ok := strings.CutPrefix(ref.Ref, "#/components/responses/"); ok {
			raw = spec.Components.Responses[name]
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatal(err)
		}
	}
	type content map[string]struct {
		Schema   *schema                                    `json:"schema"`
		Example  json.RawMessage                            `json:"example"`
		Examples map[string]struct{ Value json.RawMessage } `json:"examples"`
	}
	var ops []*specOp
	for path, methods := range spec.Paths {
		for method, raw := range methods {
			var op struct {
				Parameters []struct {
					Name     string `json:"name"`
					In       string `json:"in"`
					Required bool   `json:"required"`
					Schema   schema `json:"schema"`
				} `json:"parameters"`
				RequestBody *struct {
					Content content `json:"content"`
				} `json:"requestBody"`
				Responses map[string]json.RawMessage `json:"responses"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				continue // not an operation (e.g. path-level parameters)
			}
			o := &specOp{key: strings.ToUpper(method) + " " + path, query: map[string]param{}}
			o.params = strings.Count(path, "{")
			// QuoteMeta escapes the braces, so match the escaped placeholder.
			o.re = regexp.MustCompile("^" + regexp.MustCompile(`\\\{[^}]+\\\}`).ReplaceAllString(regexp.QuoteMeta(path), `[^/]+`) + "$")
			for _, p := range op.Parameters {
				if p.In == "query" {
					o.query[p.Name] = param{p.Required, p.Schema.Enum}
				}
			}
			if op.RequestBody != nil {
				for ct, c := range op.RequestBody.Content {
					if ct == "application/json" {
						s := c.Schema
						if s.Ref != "" {
							s = spec.Components.Schemas[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
						}
						o.body, o.bodyReq = s.Properties, s.Required
						if o.body == nil {
							o.body = map[string]*schema{}
						}
					} else {
						o.body, o.bodyReq = map[string]*schema{"file": {}}, []string{"file"}
					}
				}
			}
			for code, r := range op.Responses {
				var st int
				if _, err := fmt.Sscan(code, &st); err != nil {
					continue
				}
				var resp struct {
					Content content `json:"content"`
				}
				resolve(r, &resp)
				for _, c := range resp.Content {
					exs := []example{}
					if len(c.Example) > 0 {
						exs = append(exs, example{st, "example", c.Example})
					}
					for name, e := range c.Examples {
						exs = append(exs, example{st, name, e.Value})
					}
					if st < 300 {
						o.ok = append(o.ok, exs...)
					} else {
						o.fail = append(o.fail, exs...)
					}
				}
			}
			byStatus := func(a, b example) int {
				return cmp.Or(cmp.Compare(a.status, b.status), strings.Compare(a.name, b.name))
			}
			slices.SortFunc(o.ok, byStatus)
			slices.SortFunc(o.fail, byStatus)
			ops = append(ops, o)
		}
	}
	// Literal segments win over placeholders (/apps/status vs /apps/{appId}).
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].params < ops[j].params })
	return spec.Info.Version, ops
}

type confCase struct {
	name string
	q    string // the exact query string sent
	b    string // the exact JSON body sent ("" for none; multipart is checked apart)
	body string // response override when the spec example does not apply
	call func(context.Context, *Client) (any, error)
}

var (
	t0 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	t1 = t0.Add(24 * time.Hour)
)

// v adapts a (T, error) call to the table.
func v[T any](x T, err error) (any, error) { return x, err }

// id is used for every path ID: a method that does not escape it produces an
// extra segment and matches no spec template.
const id = "a/b"

const (
	version = "3sL4kqtJlcpXroDTDmJ+rmSpXd3dIbrHY"
	window1 = "end=2026-09-02T00%3A00%3A00Z&start=2026-09-01T00%3A00%3A00Z"
)

func confCases() []confCase {
	zip := func() io.Reader { return strings.NewReader("PK\x03\x04zip") }
	rawStats := `{"status":"success","response":{"cpu":0.5,"ram":120.4,"status":"running","running":true,"storage":12345,"network":{"total":[1,2],"now":[3,4]},"uptime":1716000000000}}`
	return []confCase{
		{name: "Account.Me", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Account.Me(ctx)) }},
		{name: "Account.Snapshots", q: "scope=databases", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Account.Snapshots(ctx, SnapshotScopeDatabases))
		}},
		{name: "Service.Status", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Service.Status(ctx)) }},
		{name: "AI.Chat", b: `{"messages":[{"role":"user","content":"hi"}],"tool_choice":"auto","max_tokens":10,"max_completion_tokens":20,"temperature":0.2}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				temp := 0.2
				return v(c.AI.Chat(ctx, ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}, MaxTokens: 10, MaxCompletionTokens: 20,
					Temperature: &temp, ToolChoice: "auto"}))
			}},
		{name: "Apps.Create", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Create(ctx, zip())) }},
		{name: "Apps.Commit", q: "path=src", call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Apps.Commit(ctx, id, zip(), "src", "")
		}},
		{name: "Apps.Get", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Get(ctx, id)) }},
		{name: "Apps.Delete", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Apps.Delete(ctx, id) }},
		{name: "Apps.Start", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Apps.Start(ctx, id) }},
		{name: "Apps.Stop", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Apps.Stop(ctx, id) }},
		{name: "Apps.Restart", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Apps.Restart(ctx, id) }},
		{name: "Apps.Status", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Status(ctx, id)) }},
		{name: "Apps.StatusRaw", q: "rawData=true", body: rawStats, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.StatusRaw(ctx, id))
		}},
		{name: "Apps.StatusAll", q: "workspaceId=0123456789ab4def8123456789abcdef", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.StatusAll(ctx, "0123456789ab4def8123456789abcdef"))
		}},
		{name: "Apps.Logs", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Logs(ctx, id)) }},
		{name: "Apps.Metrics", q: "range=7d", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Metrics(ctx, id, "7d")) }},
		{name: "Apps.Realtime", call: func(ctx context.Context, c *Client) (any, error) {
			s, err := c.Apps.Realtime(ctx, id)
			if err != nil {
				return nil, err
			}
			defer s.Close()
			var evs []RealtimeEvent
			for {
				ev, err := s.Next()
				if err == io.EOF {
					return evs, nil
				} else if err != nil {
					return nil, err
				}
				evs = append(evs, ev)
			}
		}},
		{name: "Apps.Domains", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Domains(ctx)) }},
		{name: "Apps.LoadBalancers", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.LoadBalancers(ctx)) }},
		{name: "Apps.Deploys.List", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Deploys.List(ctx, id)) }},
		{name: "Apps.Deploys.Current", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Deploys.Current(ctx, id)) }},
		{name: "Apps.Deploys.SetWebhook", b: `{"access_token":"ghp_x"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Deploys.SetWebhook(ctx, id, "ghp_x"))
		}},
		{name: "Apps.Deploys.LinkGithubApp", b: `{"repositoryBranch":"main","repositoryName":"octocat/hello-world"}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return v(c.Apps.Deploys.LinkGithubApp(ctx, id, "octocat/hello-world", "main"))
			}},
		{name: "Apps.Deploys.UnlinkGithubApp", call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Apps.Deploys.UnlinkGithubApp(ctx, id)
		}},
		{name: "Apps.Envs.Get", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Envs.Get(ctx, id)) }},
		{name: "Apps.Envs.Set", b: `{"envs":{"A":"1"}}`, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Envs.Set(ctx, id, EnvVars{"A": "1"}))
		}},
		{name: "Apps.Envs.Replace", b: `{"envs":{"A":"1"}}`, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Envs.Replace(ctx, id, EnvVars{"A": "1"}))
		}},
		{name: "Apps.Envs.Delete", b: `{"envs":["A"]}`, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Envs.Delete(ctx, id, "A"))
		}},
		{name: "Apps.Files.List", q: "path=%2Fsrc", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Files.List(ctx, id, "/src"))
		}},
		{name: "Apps.Files.Read", q: "encoding=base64&path=%2Fa.js", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Files.Read(ctx, id, "/a.js"))
		}},
		{name: "Apps.Files.Write", b: `{"path":"/a.bin","content":"/wA=","encoding":"base64"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Apps.Files.Write(ctx, id, "/a.bin", []byte{0xff, 0})
		}},
		{name: "Apps.Files.Move", b: `{"path":"/a","to":"/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Apps.Files.Move(ctx, id, "/a", "/b")
		}},
		{name: "Apps.Files.Delete", b: `{"path":"/a"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Apps.Files.Delete(ctx, id, "/a")
		}},
		{name: "Apps.Snapshots.List", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Snapshots.List(ctx, id)) }},
		{name: "Apps.Snapshots.Create", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Snapshots.Create(ctx, id)) }},
		{name: "Apps.Snapshots.Restore", b: `{"snapshotId":"app1_javascript_manual","versionId":"` + version + `"}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return v(c.Apps.Snapshots.Restore(ctx, id, "app1_javascript_manual", version))
			}},
		{name: "Apps.Snapshots.RestoreState", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Snapshots.RestoreState(ctx, id))
		}},
		{name: "Apps.Network.Analytics",
			q: "bot=Unverified&browser=Chrome&content_type=json&country=BR&end=2026-09-02T00%3A00%3A00Z&ip=1.1.1.1&os=Windows&path=%2Fapi" +
				"&protocol=HTTP%2F2&provider=GOOGLE+%2815169%29&referer=google.com&start=2026-09-01T00%3A00%3A00Z&status=404",
			call: func(ctx context.Context, c *Client) (any, error) {
				return v(c.Apps.Network.Analytics(ctx, id, t0, t1, AnalyticsFilters{Country: "BR", IP: "1.1.1.1", Path: "/api",
					Status: "404", OS: "Windows", Browser: "Chrome", Protocol: "HTTP/2", Referer: "google.com", Provider: "GOOGLE (15169)",
					ContentType: "json", Bot: "Unverified"}))
			}},
		{name: "Apps.Network.Errors", q: "end=2026-09-02T00%3A00%3A00Z&include_4xx=true&start=2026-09-01T00%3A00%3A00Z",
			call: func(ctx context.Context, c *Client) (any, error) {
				return v(c.Apps.Network.Errors(ctx, id, t0, t1, true))
			}},
		{name: "Apps.Network.Logs", q: window1, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Network.Logs(ctx, id, t0, t1))
		}},
		{name: "Apps.Network.Performance", q: window1, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Apps.Network.Performance(ctx, id, t0, t1))
		}},
		{name: "Apps.Network.DNS", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Apps.Network.DNS(ctx, id)) }},
		{name: "Apps.Network.SetDomain", b: `{"custom":"example.com"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Apps.Network.SetDomain(ctx, id, "example.com")
		}},
		{name: "Apps.Network.PurgeCache", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Apps.Network.PurgeCache(ctx, id) }},
		{name: "Databases.Create", b: `{"name":"db","memory":512,"type":"mongo","version":"8"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Databases.Create(ctx, DatabaseCreate{Name: "db", Memory: 512, Type: DatabaseMongo, Version: "8"}))
		}},
		{name: "Databases.Get", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Databases.Get(ctx, id)) }},
		{name: "Databases.Update", b: `{"name":"db","ram":1024}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Databases.Update(ctx, id, DatabaseUpdate{Name: "db", RAM: 1024})
		}},
		{name: "Databases.Delete", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Databases.Delete(ctx, id) }},
		{name: "Databases.Start", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Databases.Start(ctx, id) }},
		{name: "Databases.Stop", call: func(ctx context.Context, c *Client) (any, error) { return nil, c.Databases.Stop(ctx, id) }},
		{name: "Databases.Status", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Databases.Status(ctx, id)) }},
		{name: "Databases.StatusRaw", q: "rawData=true", body: rawStats, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Databases.StatusRaw(ctx, id))
		}},
		{name: "Databases.StatusAll", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Databases.StatusAll(ctx)) }},
		{name: "Databases.Metrics", q: "range=7d", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Databases.Metrics(ctx, id, "7d"))
		}},
		{name: "Databases.Certificate", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Databases.Certificate(ctx, id)) }},
		{name: "Databases.ResetCredentials", b: `{"reset":"password"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Databases.ResetCredentials(ctx, id, ResetPassword))
		}},
		{name: "Databases.Snapshots.List", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Databases.Snapshots.List(ctx, id)) }},
		{name: "Databases.Snapshots.Create", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Databases.Snapshots.Create(ctx, id))
		}},
		{name: "Databases.Snapshots.Restore", b: `{"snapshotId":"db1_mongo","versionId":"` + version + `"}`,
			call: func(ctx context.Context, c *Client) (any, error) {
				return v(c.Databases.Snapshots.Restore(ctx, id, "db1_mongo", version))
			}},
		{name: "Databases.Snapshots.RestoreState", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Databases.Snapshots.RestoreState(ctx, id))
		}},
		{name: "Workspaces.List", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Workspaces.List(ctx)) }},
		{name: "Workspaces.Get", call: func(ctx context.Context, c *Client) (any, error) { return v(c.Workspaces.Get(ctx, id)) }},
		{name: "Workspaces.Create", b: `{"name":"Acme"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Workspaces.Create(ctx, "Acme"))
		}},
		{name: "Workspaces.Delete", b: `{"workspaceId":"a/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Workspaces.Delete(ctx, id)
		}},
		{name: "Workspaces.Leave", b: `{"workspaceId":"a/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Workspaces.Leave(ctx, id)
		}},
		{name: "Workspaces.Members.Add", b: `{"code":"code1","group":"manager","workspaceId":"a/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Workspaces.Members.Add(ctx, id, "code1", GroupManager)
		}},
		{name: "Workspaces.Members.Update", b: `{"group":"view","memberId":"u1","workspaceId":"a/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Workspaces.Members.Update(ctx, id, "u1", GroupView)
		}},
		{name: "Workspaces.Members.Remove", b: `{"memberId":"u1","workspaceId":"a/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Workspaces.Members.Remove(ctx, id, "u1")
		}},
		{name: "Workspaces.Members.InviteCode", call: func(ctx context.Context, c *Client) (any, error) {
			return v(c.Workspaces.Members.InviteCode(ctx))
		}},
		{name: "Workspaces.Apps.Add", b: `{"appId":"a/b","workspaceId":"a/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Workspaces.Apps.Add(ctx, id, id)
		}},
		{name: "Workspaces.Apps.Remove", b: `{"appId":"a/b","workspaceId":"a/b"}`, call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Workspaces.Apps.Remove(ctx, id, id)
		}},
	}
}

func TestConformance(t *testing.T) {
	version, ops := loadSpec(t)
	if version != specVersion {
		t.Fatalf("spec info.version is %q, want %q", version, specVersion)
	}
	var (
		cur     confCase
		serve   example // what the mock answers
		matched *specOp
		covered = map[string]bool{}
		fail    []string
	)
	check := func(ok bool, format string, args ...any) {
		if !ok {
			fail = append(fail, cur.name+": "+fmt.Sprintf(format, args...))
		}
	}
	srv := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		matched = nil
		for _, o := range ops {
			if strings.HasPrefix(o.key, r.Method+" ") && o.re.MatchString(r.URL.EscapedPath()) {
				matched = o
				break
			}
		}
		if matched == nil {
			check(false, "no spec operation for %s %s", r.Method, r.URL.EscapedPath())
			w.WriteHeader(404)
			return
		}
		covered[matched.key] = true
		check(r.Header.Get("Authorization") == "test-key", "missing Authorization")
		q := r.URL.Query()
		for k := range q {
			p, ok := matched.query[k]
			check(ok, "query %q is not declared by %s", k, matched.key)
			check(len(p.enum) == 0 || slices.Contains(p.enum, any(q.Get(k))), "query %s=%q not in enum %v", k, q.Get(k), p.enum)
		}
		for k, p := range matched.query {
			check(!p.required || q.Has(k), "required query %q missing", k)
		}
		check(r.URL.RawQuery == cur.q, "query %q, want %q", r.URL.RawQuery, cur.q)
		checkBody(r, matched, cur.b, check)

		ex := serve
		if ex.body == nil { // first call of a case: the op is known only now
			ex = example{200, "override", json.RawMessage(cur.body)}
			if cur.body == "" && len(matched.ok) > 0 {
				ex = matched.ok[0]
			}
		}
		check(len(ex.body) > 0, "no 2xx example for %s", matched.key)
		if matched.key == "GET /v2/apps/{appId}/realtime" && ex.status < 300 {
			// The spec example spells the stream-id bytes as "\u0001" text.
			var s string
			_ = json.Unmarshal(ex.body, &s)
			s = strings.NewReplacer(`\u0001`, "\x01", `\u0002`, "\x02").Replace(s)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, s)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ex.status)
		_, _ = w.Write(ex.body)
	})
	c := New("test-key", WithBaseURL(srv.base), WithMaxRetries(0))

	ctx := context.Background()
	for _, tc := range confCases() {
		cur, serve = tc, example{}
		got, err := tc.call(ctx, c)
		if matched == nil {
			continue
		}
		op := matched
		exs := op.ok
		if tc.body != "" {
			exs = []example{{200, "override", json.RawMessage(tc.body)}}
		}
		for i, ex := range exs {
			if i > 0 {
				serve = ex
				got, err = tc.call(ctx, c)
			}
			if err != nil {
				check(false, "%d %s: call failed: %v", ex.status, ex.name, err)
				continue
			}
			checkDecoded(got, ex, check)
		}
		for _, ex := range op.fail {
			serve = ex
			_, err := tc.call(ctx, c)
			e, ok := err.(*APIError)
			if !ok {
				check(false, "%d %s: want *APIError, got %T %v", ex.status, ex.name, err, err)
				continue
			}
			want := errorCode(ex.body)
			check(e.Status == ex.status && e.Code == want, "%d %s: got HTTP %d %s, want %d %s", ex.status, ex.name, e.Status, e.Code, ex.status, want)
		}
	}

	var want []string
	for _, o := range ops {
		if !notSDK[o.key] {
			want = append(want, o.key)
		}
	}
	var have []string
	for k := range covered {
		have = append(have, k)
	}
	sort.Strings(want)
	sort.Strings(have)
	if len(want) != specOps {
		t.Errorf("spec has %d SDK operations, want %d", len(want), specOps)
	}
	if !slices.Equal(want, have) {
		t.Errorf("covered ops differ from the spec:\nmissing: %v\nextra: %v", diff(want, have), diff(have, want))
	}
	for _, f := range fail {
		t.Error(f)
	}
}

// errorCode is the code an error example must produce, in either dialect.
func errorCode(body []byte) string {
	var b struct {
		Code  string `json:"code"`
		Error *struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &b)
	if b.Error != nil && b.Code == "" {
		return cmp.Or(b.Error.Code, b.Error.Type)
	}
	return cmp.Or(b.Code, CodeUnknown)
}

func diff(a, b []string) (out []string) {
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func checkBody(r *http.Request, o *specOp, want string, check func(bool, string, ...any)) {
	raw, _ := io.ReadAll(r.Body)
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch {
	case o.body == nil:
		check(len(raw) == 0, "%s takes no body, got %q", o.key, raw)
	case ct == "multipart/form-data":
		r.Body = io.NopCloser(bytes.NewReader(raw))
		f, h, err := r.FormFile("file")
		check(err == nil && h.Filename != "", "multipart without a file part: %v", err)
		if f != nil {
			_ = f.Close()
		}
		return
	default:
		check(ct == "application/json", "content type %q", ct)
		var m map[string]any
		check(json.Unmarshal(raw, &m) == nil, "body is not a JSON object: %q", raw)
		for k, val := range m {
			prop, ok := o.body[k]
			check(ok, "body key %q is not declared by %s", k, o.key)
			check(!ok || matches(prop, val), "body %q=%#v does not match its schema in %s", k, val, o.key)
		}
		for _, k := range o.bodyReq {
			_, ok := m[k]
			check(ok, "required body key %q missing", k)
		}
	}
	check(string(raw) == want, "body %s, want %s", raw, want)
}

// checkDecoded asserts the SDK value matches the example: strings and bytes
// equal the example's value, a nil pointer means the example was {}, and any
// other result's type knows every field of the example (DisallowUnknownFields).
func checkDecoded(got any, ex example, check func(bool, string, ...any)) {
	payload := []byte(ex.body)
	var env struct {
		Status   string          `json:"status"`
		Response json.RawMessage `json:"response"`
	}
	if json.Unmarshal(payload, &env) == nil && env.Status == "success" {
		payload = env.Response
	}
	switch x := got.(type) {
	case nil:
		return // void
	case string:
		var m map[string]string
		_ = json.Unmarshal(payload, &m)
		want := ""
		for _, s := range m {
			want = s
		}
		check(len(m) <= 1 && x == want, "%d %s: got %q, want %q", ex.status, ex.name, x, want)
		return
	case []byte:
		var b struct{ Data []byte } // base64
		_ = json.Unmarshal(payload, &b)
		check(len(x) > 0 && bytes.Equal(x, b.Data), "%d %s: decoded bytes %v", ex.status, ex.name, x)
		return
	case []RealtimeEvent:
		check(len(x) == 5 && x[2].Status != nil && x[2].Status.CPULimit == 1 && x[3].Stream == "stdout" && x[4].Stream == "stderr",
			"realtime example decoded wrong: %+v", x)
		return
	case SnapshotCreated:
		check(x.Pending == (ex.status == http.StatusAccepted), "%d %s: Pending %v", ex.status, ex.name, x.Pending)
		if x.Pending {
			return
		}
	case SnapshotRestored: // 200 {"status":"success"} or 202 SNAPSHOT_RESTORE_PROCESSING
		check(x.Pending == (ex.status == http.StatusAccepted), "%d %s: Pending %v", ex.status, ex.name, x.Pending)
		return
	case LinkedRepository:
		var r struct{ Repository json.RawMessage }
		_ = json.Unmarshal(payload, &r)
		payload = r.Repository
	}
	rv := reflect.ValueOf(got)
	typ := rv.Type()
	if typ.Kind() == reflect.Pointer {
		empty := len(bytes.Trim(payload, "{} \n")) == 0
		check(rv.IsNil() == empty, "%d %s: nil result %v for payload %s", ex.status, ex.name, rv.IsNil(), payload)
		if rv.IsNil() {
			return
		}
		typ = typ.Elem()
	}
	check(!rv.IsZero(), "%d %s: decoded a zero %T", ex.status, ex.name, got)
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	err := dec.Decode(reflect.New(typ).Interface())
	check(err == nil, "%d %s: example does not strictly decode into %v: %v", ex.status, ex.name, typ, err)
}

// SDK-STANDARD 3, rule 3: a key without snapshots:read gets a 200 snapshot
// create with an empty response (no spec example shows it). It is done, not
// pending, with URL and Key empty, the same in all three SDKs.
func TestSnapshotCreateEmptyResponse(t *testing.T) {
	for _, body := range []string{`{"status":"success"}`, `{"status":"success","response":{}}`} {
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) })
		for _, s := range []SnapshotsAPI{c.Apps.Snapshots, c.Databases.Snapshots} {
			got, err := s.Create(context.Background(), "id1")
			if err != nil || got != (SnapshotCreated{Pending: false, URL: "", Key: ""}) {
				t.Fatalf("%s: %+v %v", body, got, err)
			}
		}
	}
}

// The public surface is exactly the case table: every exported method of
// every group (nested groups included), plus the DownloadSnapshot helper.
func TestSurface(t *testing.T) {
	var want, have []string
	for _, tc := range confCases() {
		want = append(want, tc.name)
	}
	var walk func(prefix string, v reflect.Value)
	walk = func(prefix string, v reflect.Value) {
		typ := v.Type()
		for i := range typ.NumMethod() {
			have = append(have, prefix+typ.Method(i).Name)
		}
		for i := range typ.NumField() {
			if f := typ.Field(i); f.IsExported() {
				walk(prefix+f.Name+".", v.Field(i))
			}
		}
	}
	walk("", reflect.ValueOf(*New("k")))
	ptr := reflect.TypeOf(&Client{})
	for i := range ptr.NumMethod() {
		if name := ptr.Method(i).Name; name != "DownloadSnapshot" {
			have = append(have, name)
		}
	}
	slices.Sort(want)
	slices.Sort(have)
	if !slices.Equal(want, have) {
		t.Errorf("public methods differ from the case table:\nuntested: %v\nmissing: %v", diff(have, want), diff(want, have))
	}
}

// Every code of the spec's ErrorCode enum has a Code* constant in codes.go,
// and every constant there is either a spec code or one of the SDK's own.
func TestErrorCodeConstants(t *testing.T) {
	b, err := os.ReadFile("spec/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec rawSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("codes.go")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*Code\w+\s*=\s*"([^"]+)"`).FindAllSubmatch(src, -1) {
		have[string(m[1])] = true
	}
	enum := spec.Components.Schemas["ErrorCode"].Enum
	if len(enum) == 0 {
		t.Fatal("spec has no ErrorCode enum")
	}
	sdkOnly := map[string]bool{CodeNetworkError: true, CodeTimeout: true, CodeUnknown: true, CodeInvalidAPIKey: true,
		CodeRateLimit: true, CodeRateLimitExceeded: true, CodeMetricsNotSupported: true} // the last three are deprecated
	// Listed in the enum, but only an internal (non-public) route sends them.
	internal := map[string]bool{"SUBDOMAIN_TAKEN": true, "DEPLOY_FAILED": true}
	for _, c := range enum {
		if internal[c.(string)] {
			if have[c.(string)] {
				t.Errorf("internal code %s has a Code* constant", c)
			}
			continue
		}
		if !have[c.(string)] {
			t.Errorf("spec code %s has no Code* constant", c)
		}
		delete(have, c.(string))
	}
	for c := range have {
		if !sdkOnly[c] {
			t.Errorf("constant %s is not a spec code", c)
		}
	}
}

// Input enum constants no case sends must still match the spec.
func TestInputEnums(t *testing.T) {
	ops := map[string]*specOp{}
	_, list := loadSpec(t)
	for _, o := range list {
		ops[o.key] = o
	}
	for _, tc := range []struct {
		enum []any
		sdk  []string
	}{
		{ops["POST /v2/workspaces/members"].body["group"].Enum, []string{string(GroupView), string(GroupManager), string(GroupMaintain), string(GroupAdmin)}},
		{ops["PATCH /v2/workspaces/members"].body["group"].Enum, []string{string(GroupView), string(GroupManager), string(GroupMaintain), string(GroupAdmin)}},
		{ops["POST /v2/databases/{databaseId}/credentials/reset"].body["reset"].Enum, []string{string(ResetPassword), string(ResetCertificate)}},
		{ops["POST /v2/databases"].body["type"].Enum, []string{string(DatabaseMongo), string(DatabaseMySQL), string(DatabaseRedis), string(DatabasePostgres)}},
		{ops["GET /v2/users/snapshots"].query["scope"].enum, []string{string(SnapshotScopeApplications), string(SnapshotScopeDatabases)}},
	} {
		var spec []string
		for _, v := range tc.enum {
			spec = append(spec, fmt.Sprint(v))
		}
		slices.Sort(spec)
		slices.Sort(tc.sdk)
		if !slices.Equal(spec, tc.sdk) {
			t.Errorf("spec enum %v, SDK constants %v", spec, tc.sdk)
		}
	}
}
