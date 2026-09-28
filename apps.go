package squarecloud

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

// appPath escapes appID so "/", "?" or "#" cannot change the route.
func appPath(appID, sub string) string { return "/apps/" + url.PathEscape(appID) + sub }

// AppsAPI covers /apps. Every appID may be "<appId>-<workspaceId>" for an app
// shared through a workspace.
type AppsAPI struct {
	c         *Client
	Files     AppFilesAPI
	Envs      AppEnvsAPI
	Snapshots SnapshotsAPI
	Network   AppNetworkAPI
	Deploys   AppDeploysAPI
}

// Create uploads a zip (at most 100 MB) and deploys a new application. The
// zip is streamed; any io.Seeker gets a Content-Length, and an *os.File (or
// any io.ReaderAt + io.Seeker) can also be retried on 503 UPLOAD_BUSY. The
// part is named after an *os.File, else "app.zip".
func (a AppsAPI) Create(ctx context.Context, zip io.Reader) (AppCreated, error) {
	u, err := newUpload(zip, "", "app.zip")
	if err != nil {
		return AppCreated{}, err
	}
	return call[AppCreated](ctx, a.c, http.MethodPost, "/apps", u)
}

// Commit uploads a file into an existing application. path is the
// destination directory inside the app ("" for the root). A .zip is unpacked
// there; any other file lands at path/<filename>. filename ("" for the
// default) overrides r's own name (for an *os.File, the base of its Name()),
// which overrides "commit.zip".
func (a AppsAPI) Commit(ctx context.Context, appID string, r io.Reader, path, filename string) error {
	u, err := newUpload(r, filename, "commit.zip")
	if err != nil {
		return err
	}
	return a.c.do(ctx, http.MethodPost, withPath(appPath(appID, "/commit"), path), u, nil)
}

// Get returns an application.
func (a AppsAPI) Get(ctx context.Context, appID string) (App, error) {
	return call[App](ctx, a.c, http.MethodGet, appPath(appID, ""), nil)
}

// Delete deletes an application.
func (a AppsAPI) Delete(ctx context.Context, appID string) error {
	return a.c.do(ctx, http.MethodDelete, appPath(appID, ""), nil, nil)
}

// held sends a call the API holds for up to 95s (timeout floor 2 minutes).
func held(ctx context.Context, c *Client, method, path string, in, out any) error {
	ctx, cancel := c.bound(ctx, heldFloor)
	defer cancel()
	return c.do(ctx, method, path, in, out)
}

// Start starts an application. Without a ctx deadline it gets at least 2
// minutes. A cluster refusal is 409 with a code and no message:
// CONTAINER_ALREADY_STARTED, CONTAINER_ALREADY_STOPPED,
// CONTAINER_TEMPORARILY_SUSPENDED, CONTAINER_NOT_FOUND,
// CONTAINER_INSUFFICIENT_DISK_SPACE, CONTAINER_NETWORK_CONFLICT or
// ACTION_FAILED. An "already" code means the app is in the wanted state; the
// SDK returns it as an error and leaves that call to the caller.
func (a AppsAPI) Start(ctx context.Context, appID string) error {
	return held(ctx, a.c, http.MethodPost, appPath(appID, "/start"), nil, nil)
}

// Stop stops an application; timeout and errors as in Start.
func (a AppsAPI) Stop(ctx context.Context, appID string) error {
	return held(ctx, a.c, http.MethodPost, appPath(appID, "/stop"), nil, nil)
}

// Restart restarts an application; timeout and errors as in Start.
func (a AppsAPI) Restart(ctx context.Context, appID string) error {
	return held(ctx, a.c, http.MethodPost, appPath(appID, "/restart"), nil, nil)
}

// Status returns formatted runtime stats. See StatusRaw for numbers.
func (a AppsAPI) Status(ctx context.Context, appID string) (RuntimeStats, error) {
	return call[RuntimeStats](ctx, a.c, http.MethodGet, appPath(appID, "/status"), nil)
}

// StatusRaw is Status with ?rawData=true.
func (a AppsAPI) StatusRaw(ctx context.Context, appID string) (RuntimeStatsRaw, error) {
	return call[RuntimeStatsRaw](ctx, a.c, http.MethodGet, appPath(appID, "/status?rawData=true"), nil)
}

// StatusAll lists every app's status. workspaceID ("" for none) limits the
// listing to the apps shared in that workspace.
func (a AppsAPI) StatusAll(ctx context.Context, workspaceID string) ([]StatusListItem, error) {
	p := "/apps/status"
	if workspaceID != "" {
		p += "?" + url.Values{"workspaceId": {workspaceID}}.Encode()
	}
	return call[[]StatusListItem](ctx, a.c, http.MethodGet, p, nil)
}

// Logs returns the latest log output ("" when there is none).
func (a AppsAPI) Logs(ctx context.Context, appID string) (string, error) {
	r, err := call[struct {
		Logs string `json:"logs"`
	}](ctx, a.c, http.MethodGet, appPath(appID, "/logs"), nil)
	return r.Logs, err
}

// Metrics returns the 5-minute samples of the last 24 hours, newest first.
func (a AppsAPI) Metrics(ctx context.Context, appID string) ([]MetricPoint, error) {
	return call[[]MetricPoint](ctx, a.c, http.MethodGet, appPath(appID, "/metrics"), nil)
}

// Domains lists every hostname of the account's apps (custom first).
func (a AppsAPI) Domains(ctx context.Context) ([]AppDomain, error) {
	return call[[]AppDomain](ctx, a.c, http.MethodGet, "/apps/domains", nil)
}

// LoadBalancers lists the account's load balancers and their limit.
func (a AppsAPI) LoadBalancers(ctx context.Context) (LoadBalancers, error) {
	return call[LoadBalancers](ctx, a.c, http.MethodGet, "/apps/load-balancers", nil)
}

// AppDeploysAPI covers /apps/{id}/deploy* and /deployments.
type AppDeploysAPI struct{ c *Client }

// List returns the recent deploys, each as its timeline of events.
func (d AppDeploysAPI) List(ctx context.Context, appID string) ([][]DeployEvent, error) {
	return call[[][]DeployEvent](ctx, d.c, http.MethodGet, appPath(appID, "/deployments"), nil)
}

// Current returns the linked GitHub App repository and the webhook URL.
func (d AppDeploysAPI) Current(ctx context.Context, appID string) (DeployCurrent, error) {
	return call[DeployCurrent](ctx, d.c, http.MethodGet, appPath(appID, "/deployments/current"), nil)
}

// SetWebhook enrolls a GitHub token (ghp_* / github_pat_*) for push deploys
// and returns the webhook URL to configure on GitHub. accessToken "@" removes
// the webhook (the returned URL is then "").
func (d AppDeploysAPI) SetWebhook(ctx context.Context, appID, accessToken string) (string, error) {
	r, err := call[struct {
		Webhook string `json:"webhook"`
	}](ctx, d.c, http.MethodPost, appPath(appID, "/deploy/webhook"), map[string]string{"access_token": accessToken})
	return r.Webhook, err
}

// LinkGithubApp links repository ("owner/name") at branch through the Square
// Cloud GitHub App (scope apps:deploy; 3 calls per 60s). The caller's GitHub
// account must hold an installation that covers the repository (else 403
// REPOSITORY_NOT_AVAILABLE) and write access to it (else 403
// REPOSITORY_PERMISSION_REQUIRED). 502 FAILED_TO_FETCH means GitHub did not
// confirm the branch; it is safe to retry. 409
// REPOSITORY_BRANCH_ALREADY_CONFIGURED means another app of any account has
// the same repository and branch; Message names that app only when it is the
// caller's own. Re-linking needs UnlinkGithubApp first (else 400
// GIT_ALREADY_CONFIGURED).
func (d AppDeploysAPI) LinkGithubApp(ctx context.Context, appID, repository, branch string) (LinkedRepository, error) {
	r, err := call[struct {
		Repository LinkedRepository `json:"repository"`
	}](ctx, d.c, http.MethodPost, appPath(appID, "/deploy/github-app"),
		map[string]string{"repositoryName": repository, "repositoryBranch": branch})
	return r.Repository, err
}

// UnlinkGithubApp removes the GitHub App link (400 GIT_NOT_CONFIGURED when
// there is none).
func (d AppDeploysAPI) UnlinkGithubApp(ctx context.Context, appID string) error {
	return d.c.do(ctx, http.MethodDelete, appPath(appID, "/deploy/github-app"), nil, nil)
}

// AppEnvsAPI covers /apps/{id}/envs. Every call returns the resulting set.
type AppEnvsAPI struct{ c *Client }

// Get returns the environment variables.
func (e AppEnvsAPI) Get(ctx context.Context, appID string) (EnvVars, error) {
	return call[EnvVars](ctx, e.c, http.MethodGet, appPath(appID, "/envs"), nil)
}

// Set merges envs into the existing variables and returns all of them.
func (e AppEnvsAPI) Set(ctx context.Context, appID string, envs EnvVars) (EnvVars, error) {
	return call[EnvVars](ctx, e.c, http.MethodPost, appPath(appID, "/envs"), map[string]EnvVars{"envs": envs})
}

// Replace overwrites all variables with envs.
func (e AppEnvsAPI) Replace(ctx context.Context, appID string, envs EnvVars) (EnvVars, error) {
	return call[EnvVars](ctx, e.c, http.MethodPut, appPath(appID, "/envs"), map[string]EnvVars{"envs": envs})
}

// Delete removes the named variables and returns the remaining ones.
func (e AppEnvsAPI) Delete(ctx context.Context, appID string, keys ...string) (EnvVars, error) {
	if keys == nil {
		keys = []string{} // "envs": [] rather than null
	}
	return call[EnvVars](ctx, e.c, http.MethodDelete, appPath(appID, "/envs"), map[string][]string{"envs": keys})
}

// AppFilesAPI covers /apps/{id}/files. Paths are absolute inside the app.
type AppFilesAPI struct{ c *Client }

// List lists a directory ("" for the root). A missing directory is 404
// FILE_NOT_FOUND and a protected one 403 BLOCKED_PATH; paths are at most 256
// characters.
func (f AppFilesAPI) List(ctx context.Context, appID, path string) ([]FileEntry, error) {
	return call[[]FileEntry](ctx, f.c, http.MethodGet, withPath(appPath(appID, "/files"), path), nil)
}

// withPath appends the path query parameter, dropped when "".
func withPath(p, path string) string {
	if path != "" {
		p += "?" + url.Values{"path": {path}}.Encode()
	}
	return p
}

// Read returns a file's bytes (at most 10 MB, else 413 FILE_TOO_LARGE). It
// always sends ?encoding=base64 and decodes the base64 data; a missing file
// is 404 FILE_NOT_FOUND.
func (f AppFilesAPI) Read(ctx context.Context, appID, path string) ([]byte, error) {
	q := url.Values{"encoding": {"base64"}}
	if path != "" {
		q.Set("path", path)
	}
	// encoding/json decodes a JSON string into []byte as standard base64.
	r, err := call[struct {
		Data []byte `json:"data"`
	}](ctx, f.c, http.MethodGet, appPath(appID, "/files/content?"+q.Encode()), nil)
	return r.Data, err
}

// Write creates or overwrites a file (at most 10 MB, checked locally). The
// content is always sent base64-encoded ({path, content, encoding:
// "base64"}), so text and binary files round-trip byte for byte; the wire
// is about 1.33x the content. Empty content writes an empty file. The API
// answers 400 INVALID_CONTENT for content it cannot decode and 413 above
// 10 MB. Content above 1 MiB gets no default deadline: only ctx bounds it.
func (f AppFilesAPI) Write(ctx context.Context, appID, path string, content []byte) error {
	if len(content) > maxFileSize {
		return f.c.localError(http.MethodPut, appPath(appID, "/files"), CodeFileTooLarge, msgFileTooLarge)
	}
	if len(content) <= largeFile {
		var cancel context.CancelFunc
		ctx, cancel = f.c.bound(ctx, 0)
		defer cancel()
	}
	b, _ := json.Marshal(struct { // strings cannot fail
		Path     string `json:"path"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}{path, base64.StdEncoding.EncodeToString(content), "base64"})
	return f.c.do(ctx, http.MethodPut, appPath(appID, "/files"), json.RawMessage(b), nil)
}

// Move moves or renames the file at path to to.
func (f AppFilesAPI) Move(ctx context.Context, appID, path, to string) error {
	return f.c.do(ctx, http.MethodPatch, appPath(appID, "/files"), map[string]string{"path": path, "to": to}, nil)
}

// Delete deletes the file or directory at path.
func (f AppFilesAPI) Delete(ctx context.Context, appID, path string) error {
	return f.c.do(ctx, http.MethodDelete, appPath(appID, "/files"), map[string]string{"path": path}, nil)
}

// SnapshotsAPI covers /apps/{id}/snapshots and /databases/{id}/snapshots.
type SnapshotsAPI struct {
	c    *Client
	base string
}

// List lists the snapshots of the application or database id.
func (s SnapshotsAPI) List(ctx context.Context, id string) ([]Snapshot, error) {
	return call[[]Snapshot](ctx, s.c, http.MethodGet, s.base+url.PathEscape(id)+"/snapshots", nil)
}

// Create takes a snapshot. A large one may still be processing: the result
// then has Pending set (HTTP 202) and no URL; it is not an error, and it shows
// up in List within about 2 minutes (do not call Create again). URL is a
// presigned download link valid for 30 days (see Client.DownloadSnapshot).
func (s SnapshotsAPI) Create(ctx context.Context, id string) (SnapshotCreated, error) {
	var out SnapshotCreated
	err := held(ctx, s.c, http.MethodPost, s.base+url.PathEscape(id)+"/snapshots", nil, &out)
	return out, err
}

// Restore restores the snapshot named name (Snapshot.Name) at versionID
// (Snapshot.VersionID). A failed restore is 404 SNAPSHOT_RESTORE_FAILED.
func (s SnapshotsAPI) Restore(ctx context.Context, id, name, versionID string) error {
	return held(ctx, s.c, http.MethodPost, s.base+url.PathEscape(id)+"/snapshots/restore",
		map[string]string{"snapshotId": name, "versionId": versionID}, nil)
}

// AppNetworkAPI covers /apps/{id}/network (websites only). Analytics, Errors
// and Performance return nil when the window has no traffic (the API answers
// {}).
type AppNetworkAPI struct{ c *Client }

func window(start, end time.Time) url.Values {
	return url.Values{"start": {start.UTC().Format(time.RFC3339)}, "end": {end.UTC().Format(time.RFC3339)}}
}

// orNil decodes to nil when the API answers {} (a window with no traffic).
type orNil[T any] struct{ v *T }

func (o *orNil[T]) UnmarshalJSON(b []byte) error {
	// b is one valid JSON value, so "{", blanks, "}" is an empty object.
	if string(b) == "null" || b[0] == '{' && len(bytes.TrimSpace(b[1:len(b)-1])) == 0 {
		return nil
	}
	o.v = new(T)
	return json.Unmarshal(b, o.v)
}

func networkWindow[T any](ctx context.Context, c *Client, path string) (*T, error) {
	r, err := call[orNil[T]](ctx, c, http.MethodGet, path, nil)
	return r.v, err
}

// Analytics returns edge analytics for start..end, narrowed by the optional
// filters f.
func (n AppNetworkAPI) Analytics(ctx context.Context, appID string, start, end time.Time, f AnalyticsFilters) (*NetworkAnalytics, error) {
	v := window(start, end)
	for _, kv := range [...][2]string{
		{"country", f.Country}, {"ip", f.IP}, {"path", f.Path}, {"status", f.Status}, {"os", f.OS},
		{"browser", f.Browser}, {"protocol", f.Protocol}, {"referer", f.Referer}, {"provider", f.Provider},
		{"content_type", f.ContentType}, {"bot", f.Bot},
	} {
		if kv[1] != "" {
			v.Set(kv[0], kv[1])
		}
	}
	return networkWindow[NetworkAnalytics](ctx, n.c, appPath(appID, "/network/analytics?"+v.Encode()))
}

// Errors returns edge errors (5xx only unless include4xx).
func (n AppNetworkAPI) Errors(ctx context.Context, appID string, start, end time.Time, include4xx bool) (*NetworkErrors, error) {
	v := window(start, end)
	if include4xx {
		v.Set("include_4xx", "true")
	}
	return networkWindow[NetworkErrors](ctx, n.c, appPath(appID, "/network/errors?"+v.Encode()))
}

// Logs returns the edge request logs for start..end.
func (n AppNetworkAPI) Logs(ctx context.Context, appID string, start, end time.Time) ([]NetworkLog, error) {
	return call[[]NetworkLog](ctx, n.c, http.MethodGet, appPath(appID, "/network/logs?"+window(start, end).Encode()), nil)
}

// Performance returns edge and origin latency percentiles for start..end.
func (n AppNetworkAPI) Performance(ctx context.Context, appID string, start, end time.Time) (*NetworkPerformance, error) {
	return networkWindow[NetworkPerformance](ctx, n.c, appPath(appID, "/network/performance?"+window(start, end).Encode()))
}

// DNS returns the records to configure for the custom domain.
func (n AppNetworkAPI) DNS(ctx context.Context, appID string) ([]DNSRecord, error) {
	return call[[]DNSRecord](ctx, n.c, http.MethodGet, appPath(appID, "/network/dns"), nil)
}

// SetDomain attaches a custom domain ("@" removes it).
func (n AppNetworkAPI) SetDomain(ctx context.Context, appID, domain string) error {
	return n.c.do(ctx, http.MethodPost, appPath(appID, "/network/custom"), map[string]string{"custom": domain}, nil)
}

// PurgeCache purges the edge cache.
func (n AppNetworkAPI) PurgeCache(ctx context.Context, appID string) error {
	return n.c.do(ctx, http.MethodPost, appPath(appID, "/network/purge_cache"), nil, nil)
}
