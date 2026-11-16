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

// AppsAPI deploys and controls applications. Its fields group the calls that
// work on one application's files, environment variables, snapshots, network
// and deploys.
//
// Every application id that this group and its fields take is normally the
// application's own id. To act on an application that another account shares
// with you through a workspace, pass the composite id
// "<appId>-<workspaceId>" instead.
type AppsAPI struct {
	c *Client
	// Files reads and writes the application's files.
	Files AppFilesAPI
	// Envs reads and changes the application's environment variables.
	Envs AppEnvsAPI
	// Snapshots takes, lists and restores the application's snapshots.
	Snapshots SnapshotsAPI
	// Network reports a website's traffic and sets its custom domain.
	Network AppNetworkAPI
	// Deploys sets up deploys from GitHub and lists past deploys.
	Deploys AppDeploysAPI
}

// Create deploys a new application from a zip archive and returns its id and
// settings. The zip can be at most 100 MB.
//
// The zip is streamed as it is read and never held in memory. Pass an
// [*os.File] when you can. Its size is then known before the upload starts,
// its name becomes the name of the uploaded file, and the upload can be
// repeated after 503 UPLOAD_BUSY. A reader without a name is uploaded as
// "app.zip". A zip over 100 MB returns [CodeFileTooLarge], before anything is
// sent when its size is known, or as soon as the stream passes 100 MB. An
// upload has no time limit, so cancel ctx to stop it. The API returns 400
// INSUFFICIENT_MEMORY when your plan does not have enough free memory for the
// application.
func (a AppsAPI) Create(ctx context.Context, zip io.Reader) (AppCreated, error) {
	u, err := newUpload(zip, "", "app.zip")
	if err != nil {
		return AppCreated{}, err
	}
	return call[AppCreated](ctx, a.c, http.MethodPost, "/apps", u)
}

// Commit uploads a file into an existing application. A .zip archive is
// unpacked into the directory path, and any other file is written to
// path/<filename>. Pass "" as path for the application's root directory. See
// [AppsAPI] for workspace applications.
//
// The uploaded file is named filename. When filename is "", it takes the base
// name of an [*os.File], or "commit.zip" for other readers. The upload
// streams like the one of [AppsAPI.Create], with the same 100 MB limit and no
// time limit. The API returns 403 BLOCKED_PATH when a workspace Maintainer
// sends one of the application's restricted files, such as an .env file or the
// configuration file.
func (a AppsAPI) Commit(ctx context.Context, appID string, r io.Reader, path, filename string) error {
	u, err := newUpload(r, filename, "commit.zip")
	if err != nil {
		return err
	}
	return a.c.do(ctx, http.MethodPost, withQuery(appPath(appID, "/commit"), "path", path), u, nil)
}

// Get returns the application's name, language, memory, domains and owner.
// See [AppsAPI] for workspace applications.
func (a AppsAPI) Get(ctx context.Context, appID string) (App, error) {
	return call[App](ctx, a.c, http.MethodGet, appPath(appID, ""), nil)
}

// Delete deletes the application. It returns 409 RESTORE_IN_PROGRESS while a
// snapshot is being restored, so try again when the restore ends. See
// [AppsAPI] for workspace applications.
func (a AppsAPI) Delete(ctx context.Context, appID string) error {
	return a.c.do(ctx, http.MethodDelete, appPath(appID, ""), nil, nil)
}

// held sends a call that the API can hold open for up to 95 seconds, with a
// time limit of at least 2 minutes.
func held(ctx context.Context, c *Client, method, path string, in, out any) error {
	ctx, cancel := c.bound(ctx, heldFloor)
	defer cancel()
	return c.do(ctx, method, path, in, out)
}

// Start starts the application. Without a deadline on ctx, it waits at least
// 2 minutes for the API to answer. See [AppsAPI] for workspace applications.
//
// When the application cannot start, Start returns a 409 error with no
// message and one of these codes: CONTAINER_ALREADY_STARTED,
// CONTAINER_ALREADY_STOPPED, CONTAINER_TEMPORARILY_SUSPENDED,
// CONTAINER_NOT_FOUND, CONTAINER_INSUFFICIENT_DISK_SPACE,
// CONTAINER_NETWORK_CONFLICT, ACTION_FAILED, or RESTORE_IN_PROGRESS while a
// snapshot is being restored. The "already" codes mean that the application
// is already in the state you asked for. They are still returned as errors,
// so treat them as success when that is what your code needs.
func (a AppsAPI) Start(ctx context.Context, appID string) error {
	return held(ctx, a.c, http.MethodPost, appPath(appID, "/start"), nil, nil)
}

// Stop stops the application. Its time limit and errors are the same as for
// [AppsAPI.Start]. See [AppsAPI] for workspace applications.
func (a AppsAPI) Stop(ctx context.Context, appID string) error {
	return held(ctx, a.c, http.MethodPost, appPath(appID, "/stop"), nil, nil)
}

// Restart restarts the application. Its time limit and errors are the same as
// for [AppsAPI.Start]. See [AppsAPI] for workspace applications.
func (a AppsAPI) Restart(ctx context.Context, appID string) error {
	return held(ctx, a.c, http.MethodPost, appPath(appID, "/restart"), nil, nil)
}

// Status returns the application's current CPU, memory, network and storage
// usage as formatted strings such as "120.4MB". Use [AppsAPI.StatusRaw] for
// numbers. See [AppsAPI] for workspace applications.
func (a AppsAPI) Status(ctx context.Context, appID string) (RuntimeStats, error) {
	return call[RuntimeStats](ctx, a.c, http.MethodGet, appPath(appID, "/status"), nil)
}

// StatusRaw returns the same usage as [AppsAPI.Status] as numbers: CPU in
// percent, memory in MB, and storage and network traffic in bytes. See
// [AppsAPI] for workspace applications.
func (a AppsAPI) StatusRaw(ctx context.Context, appID string) (RuntimeStatsRaw, error) {
	return call[RuntimeStatsRaw](ctx, a.c, http.MethodGet, appPath(appID, "/status?rawData=true"), nil)
}

// StatusAll reports whether each of your applications is running, with its
// CPU and memory usage. Pass a workspaceID to list the applications shared in
// that workspace instead, or "" for your own.
func (a AppsAPI) StatusAll(ctx context.Context, workspaceID string) ([]StatusListItem, error) {
	p := "/apps/status"
	if workspaceID != "" {
		p += "?" + url.Values{"workspaceId": {workspaceID}}.Encode()
	}
	return call[[]StatusListItem](ctx, a.c, http.MethodGet, p, nil)
}

// Logs returns the application's most recent output, with standard output
// and standard error together in the order they were written. It returns ""
// when there is no output. Use [AppsAPI.Realtime] to follow the logs live.
// See [AppsAPI] for workspace applications.
func (a AppsAPI) Logs(ctx context.Context, appID string) (string, error) {
	r, err := call[struct {
		Logs string `json:"logs"`
	}](ctx, a.c, http.MethodGet, appPath(appID, "/logs"), nil)
	return r.Logs, err
}

// Metrics returns the application's CPU, memory, network and disk usage over
// time, newest point first. See [AppsAPI] for workspace applications.
//
// rng selects the period. With "24h", the default that "" also gets, there is
// a point every 5 minutes, or every 10 minutes for applications with less
// than 512 MB. With "7d" there is a point every hour, where CPU and RAM are
// the hour's average and [MetricPoint.CPUMax] and [MetricPoint.RAMMax] its
// peaks. "7d" works on the Pro and Enterprise plans for applications with
// 512 MB or more. Other plans and smaller applications get the 24-hour answer
// without an error, so check the points you get instead of assuming 7 days.
// Any other rng returns 400 INVALID_PARAMETERS.
func (a AppsAPI) Metrics(ctx context.Context, appID, rng string) ([]MetricPoint, error) {
	return call[[]MetricPoint](ctx, a.c, http.MethodGet, withQuery(appPath(appID, "/metrics"), "range", rng), nil)
}

// Domains returns every hostname of your applications, both the default
// subdomains and the custom domains. Custom domains come first.
func (a AppsAPI) Domains(ctx context.Context) ([]AppDomain, error) {
	return call[[]AppDomain](ctx, a.c, http.MethodGet, "/apps/domains", nil)
}

// LoadBalancers groups your applications by custom domain. A domain that two
// or more applications serve is a load balancer, which spreads the traffic
// across them. The result also says how many applications your plan lets
// share one domain.
func (a AppsAPI) LoadBalancers(ctx context.Context) (LoadBalancers, error) {
	return call[LoadBalancers](ctx, a.c, http.MethodGet, "/apps/load-balancers", nil)
}

// AppDeploysAPI sets up deploys from GitHub and lists an application's past
// deploys.
type AppDeploysAPI struct{ c *Client }

// List returns the application's recent deploys. Each deploy is a list of
// [DeployEvent] values that goes from "pending" to "success" or "error". See
// [AppsAPI] for workspace applications.
func (d AppDeploysAPI) List(ctx context.Context, appID string) ([][]DeployEvent, error) {
	return call[[][]DeployEvent](ctx, d.c, http.MethodGet, appPath(appID, "/deployments"), nil)
}

// Current returns the repository linked through the GitHub App and the
// deploy webhook URL. Each part is empty when that kind of deploy is not set
// up. See [AppsAPI] for workspace applications.
func (d AppDeploysAPI) Current(ctx context.Context, appID string) (DeployCurrent, error) {
	return call[DeployCurrent](ctx, d.c, http.MethodGet, appPath(appID, "/deployments/current"), nil)
}

// SetWebhook turns on deploys from a GitHub webhook and returns the webhook
// URL to add to your repository on GitHub. accessToken is a GitHub personal
// access token, which starts with "ghp_" or "github_pat_". Pass "@" to remove
// the webhook, in which case the returned URL is "". Any other token returns
// 400 INVALID_ACCESS_TOKEN. See [AppsAPI] for workspace applications.
func (d AppDeploysAPI) SetWebhook(ctx context.Context, appID, accessToken string) (string, error) {
	r, err := call[struct {
		Webhook string `json:"webhook"`
	}](ctx, d.c, http.MethodPost, appPath(appID, "/deploy/webhook"), map[string]string{"access_token": accessToken})
	return r.Webhook, err
}

// LinkGithubApp links the application to a branch of a GitHub repository
// through the Square Cloud GitHub App, and returns the linked repository.
// After that, a push to the branch deploys the application. repository is
// written as "owner/name". See [AppsAPI] for workspace applications.
//
// Linking and unlinking each need an API key with the apps:deploy scope, and
// they share a limit of 3 calls a minute. Your GitHub account must be connected to Square
// Cloud, or the call returns 403 GITHUB_NOT_CONNECTED. The account must have
// the GitHub App installed on the repository, or the call returns 403
// REPOSITORY_NOT_AVAILABLE. It must also be able to write to the repository,
// or the call returns 403 REPOSITORY_PERMISSION_REQUIRED.
//
// A 502 FAILED_TO_FETCH means that GitHub did not confirm the branch, and it
// is safe to try again. A 409 REPOSITORY_BRANCH_ALREADY_CONFIGURED means that
// another application, of any account, already deploys from that repository
// and branch. Its Message names that application only when it is yours. To
// link a different repository, call [AppDeploysAPI.UnlinkGithubApp] first, or
// the call returns 400 GIT_ALREADY_CONFIGURED.
func (d AppDeploysAPI) LinkGithubApp(ctx context.Context, appID, repository, branch string) (LinkedRepository, error) {
	r, err := call[struct {
		Repository LinkedRepository `json:"repository"`
	}](ctx, d.c, http.MethodPost, appPath(appID, "/deploy/github-app"),
		map[string]string{"repositoryName": repository, "repositoryBranch": branch})
	return r.Repository, err
}

// UnlinkGithubApp removes the link that [AppDeploysAPI.LinkGithubApp]
// created. It returns 400 GIT_NOT_CONFIGURED when the application has no
// link. It needs the apps:deploy scope and shares the limit of 3 calls a
// minute with [AppDeploysAPI.LinkGithubApp]. See [AppsAPI] for workspace
// applications.
func (d AppDeploysAPI) UnlinkGithubApp(ctx context.Context, appID string) error {
	return d.c.do(ctx, http.MethodDelete, appPath(appID, "/deploy/github-app"), nil, nil)
}

// AppEnvsAPI reads and changes an application's environment variables.
//
// Every method returns the variables as they are after the call. With an API
// key that lacks the envs:read scope, Set returns only the variables you sent
// and Delete returns an empty set. An application can have at most 256
// variables, and a write past that returns 400 TOO_MANY_ENV_VARS. When two
// writes to the same application happen at the same time, one of them fails
// with 409 ENV_BUSY. The SDK does not retry it, so try again yourself. Static
// applications have no environment variables, so every method returns 400
// STATIC_APP_ENV_NOT_SUPPORTED for them.
type AppEnvsAPI struct{ c *Client }

// Get returns the application's environment variables. See [AppsAPI] for
// workspace applications.
func (e AppEnvsAPI) Get(ctx context.Context, appID string) (EnvVars, error) {
	return call[EnvVars](ctx, e.c, http.MethodGet, appPath(appID, "/envs"), nil)
}

// Set adds envs to the application's variables, replacing the value of any
// name that already exists, and returns all of them. See [AppsAPI] for
// workspace applications.
func (e AppEnvsAPI) Set(ctx context.Context, appID string, envs EnvVars) (EnvVars, error) {
	return call[EnvVars](ctx, e.c, http.MethodPost, appPath(appID, "/envs"), map[string]EnvVars{"envs": envs})
}

// Replace replaces all of the application's variables with envs and returns
// the result. See [AppsAPI] for workspace applications.
func (e AppEnvsAPI) Replace(ctx context.Context, appID string, envs EnvVars) (EnvVars, error) {
	return call[EnvVars](ctx, e.c, http.MethodPut, appPath(appID, "/envs"), map[string]EnvVars{"envs": envs})
}

// Delete removes the variables named in keys and returns the ones that
// remain. See [AppsAPI] for workspace applications.
func (e AppEnvsAPI) Delete(ctx context.Context, appID string, keys ...string) (EnvVars, error) {
	if keys == nil {
		keys = []string{} // "envs": [] rather than null
	}
	return call[EnvVars](ctx, e.c, http.MethodDelete, appPath(appID, "/envs"), map[string][]string{"envs": keys})
}

// AppFilesAPI reads and writes an application's files. Paths start at the
// application's root directory, such as "/src/index.js", and are at most 256
// characters long.
type AppFilesAPI struct{ c *Client }

// List returns the entries of the directory path, or of the root directory
// when path is "". A missing directory returns 404 FILE_NOT_FOUND, and a
// directory that the API protects returns 403 BLOCKED_PATH. See [AppsAPI]
// for workspace applications.
func (f AppFilesAPI) List(ctx context.Context, appID, path string) ([]FileEntry, error) {
	return call[[]FileEntry](ctx, f.c, http.MethodGet, withQuery(appPath(appID, "/files"), "path", path), nil)
}

// withQuery appends the query parameter k=v, dropped when v is "".
func withQuery(p, k, v string) string {
	if v != "" {
		p += "?" + url.Values{k: {v}}.Encode()
	}
	return p
}

// Read returns the content of the file at path, byte for byte. A file over
// 10 MB returns 413 FILE_TOO_LARGE, a missing file returns 404
// FILE_NOT_FOUND, and a file the server cannot read returns 400 READ_FAILED.
// See [AppsAPI] for workspace applications.
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

// Write creates or replaces the file at path with content. Text and binary
// content arrive byte for byte, and empty content creates an empty file. See
// [AppsAPI] for workspace applications.
//
// Content over 10 MB returns [CodeFileTooLarge] without sending anything.
// Upload larger files with [AppsAPI.Commit] instead. The API returns 400
// WRITE_FAILED when it cannot write the file. Content over 1 MiB has no time
// limit, so only ctx stops it.
func (f AppFilesAPI) Write(ctx context.Context, appID, path string, content []byte) error {
	if len(content) > maxFileSize {
		return f.c.localError(http.MethodPut, appPath(appID, "/files"), CodeFileTooLarge, msgFileTooLarge)
	}
	if len(content) <= largeFile {
		var cancel context.CancelFunc
		ctx, cancel = f.c.bound(ctx, 0)
		defer cancel()
	}
	// The content is always sent as base64, so binary files arrive intact.
	// The request is about 1.33 times the size of the content.
	b, _ := json.Marshal(struct { // strings cannot fail
		Path     string `json:"path"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}{path, base64.StdEncoding.EncodeToString(content), "base64"})
	return f.c.do(ctx, http.MethodPut, appPath(appID, "/files"), json.RawMessage(b), nil)
}

// Move moves or renames the file at path to the new path to. See [AppsAPI]
// for workspace applications.
func (f AppFilesAPI) Move(ctx context.Context, appID, path, to string) error {
	return f.c.do(ctx, http.MethodPatch, appPath(appID, "/files"), map[string]string{"path": path, "to": to}, nil)
}

// Delete deletes the file at path. See [AppsAPI] for workspace applications.
func (f AppFilesAPI) Delete(ctx context.Context, appID, path string) error {
	return f.c.do(ctx, http.MethodDelete, appPath(appID, "/files"), map[string]string{"path": path}, nil)
}

// SnapshotsAPI takes, lists and restores the snapshots of one application or
// database. [AppsAPI.Snapshots] works on applications and
// [DatabasesAPI.Snapshots] on databases. Every method takes the id of the
// application or database.
type SnapshotsAPI struct {
	c    *Client
	base string
}

// List returns the snapshots of the application or database id, each with a
// download link. It returns 503 SNAPSHOTS_UNAVAILABLE while snapshot storage
// cannot answer. The SDK does not retry that error, and the API suggests
// waiting about 30 seconds. See [AppsAPI] for workspace applications.
func (s SnapshotsAPI) List(ctx context.Context, id string) ([]Snapshot, error) {
	return call[[]Snapshot](ctx, s.c, http.MethodGet, s.base+url.PathEscape(id)+"/snapshots", nil)
}

// Create takes a snapshot of the application or database id and returns its
// download link, which is valid for 1 hour. Use [Client.DownloadSnapshot] to
// save it. Without a deadline on ctx, Create waits at least 2 minutes. See
// [AppsAPI] for workspace applications.
//
// A large snapshot can take longer than the API waits. Create then returns a
// result with Pending set and no URL, which is not an error. The snapshot
// appears in [SnapshotsAPI.List] within about 2 minutes. Do not call Create
// again, because it allows one call every 3 minutes and each call counts
// against your plan's daily snapshot quota. With an API key that lacks the
// snapshots:read scope, a finished snapshot comes back with URL and Key
// empty.
//
// Create returns 409 RESTORE_IN_PROGRESS while a snapshot is being restored,
// so try again when the restore ends. It returns 429
// DAILY_SNAPSHOTS_LIMIT_REACHED when you have used your plan's daily
// snapshots, and 503 CLUSTER_MAINTENANCE_TRY_LATER during maintenance, so try
// again in a few minutes.
func (s SnapshotsAPI) Create(ctx context.Context, id string) (SnapshotCreated, error) {
	var out SnapshotCreated
	err := held(ctx, s.c, http.MethodPost, s.base+url.PathEscape(id)+"/snapshots", nil, &out)
	return out, err
}

// Restore restores the application or database id from a snapshot. name and
// versionID are the Name and VersionID fields of a [Snapshot]. Without a
// deadline on ctx, Restore waits at least 2 minutes. See [AppsAPI] for
// workspace applications.
//
// A restore that takes longer than about 90 seconds keeps running after the
// call returns. Restore then returns a result with Pending set, which is not
// an error. Follow the restore with [SnapshotsAPI.RestoreState], and do not
// call Restore again.
//
// A restore can fail with these errors:
//   - 400 MISSING_PARAMETERS, INVALID_SNAPSHOT_ID or INVALID_VERSION_ID, and
//     SNAPSHOT_DATABASE_MISMATCH for databases.
//   - 404 APP_NOT_FOUND or DATABASE_NOT_FOUND, SNAPSHOT_NOT_FOUND,
//     SNAPSHOT_RESTORE_FAILED, SNAPSHOT_CORRUPT, RESTORE_INTERRUPTED or
//     CONTAINER_NOT_FOUND.
//   - 409 RESTORE_IN_PROGRESS or SNAPSHOT_IN_PROGRESS, while another restore
//     or a snapshot is running.
//   - 413 PAYLOAD_TOO_LARGE.
//   - 503 CLUSTER_MAINTENANCE_TRY_LATER, during maintenance.
func (s SnapshotsAPI) Restore(ctx context.Context, id, name, versionID string) (SnapshotRestored, error) {
	var out SnapshotRestored
	err := held(ctx, s.c, http.MethodPost, s.base+url.PathEscape(id)+"/snapshots/restore",
		map[string]string{"snapshotId": name, "versionId": versionID}, &out)
	return out, err
}

// RestoreState reports the progress of the last restore of the application
// or database id. State is "queued" or "running" while the restore runs, and
// Step names the current step. When the restore ends, State becomes
// "success" or "error", and Code says why it failed. That result stays for 1
// hour. After that, or when there was no recent restore, State is "none".
// See [AppsAPI] for workspace applications.
//
// RestoreState allows up to 30 calls a minute, so poll it every few seconds.
func (s SnapshotsAPI) RestoreState(ctx context.Context, id string) (SnapshotRestoreState, error) {
	return call[SnapshotRestoreState](ctx, s.c, http.MethodGet, s.base+url.PathEscape(id)+"/snapshots/restore", nil)
}

// AppNetworkAPI reports the traffic of a website and sets its custom domain.
// Its methods work only for websites.
//
// Analytics, Errors, Logs and Performance cover the traffic from start to
// end. The API keeps 7 days of traffic, so start reaches back at most 7 days.
// Analytics, Errors and Performance return nil when nothing was served in that
// period. The four share a limit of 20 calls a minute for data that is not
// cached, and past it they return 429 RATE_LIMITED.
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

// Analytics returns the visits, requests and bandwidth of the application's
// domains, broken down by country, device, path and more. f narrows the
// results, and its zero value applies no filter. See [AppsAPI] for
// workspace applications.
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

// Errors returns the error responses served from the application's domains,
// by status, path and method and over time. It counts only 5xx responses,
// unless include4xx is true. See [AppsAPI] for workspace applications.
func (n AppNetworkAPI) Errors(ctx context.Context, appID string, start, end time.Time, include4xx bool) (*NetworkErrors, error) {
	v := window(start, end)
	if include4xx {
		v.Set("include_4xx", "true")
	}
	return networkWindow[NetworkErrors](ctx, n.c, appPath(appID, "/network/errors?"+v.Encode()))
}

// Logs returns one entry for each request served from the application's
// domains. It returns 403 UPGRADE_REQUIRED on plans other than Pro and
// Enterprise. See [AppsAPI] for workspace applications.
func (n AppNetworkAPI) Logs(ctx context.Context, appID string, start, end time.Time) ([]NetworkLog, error) {
	return call[[]NetworkLog](ctx, n.c, http.MethodGet, appPath(appID, "/network/logs?"+window(start, end).Encode()), nil)
}

// Performance returns how fast the application's domains answered, as
// latency percentiles at the edge and at your application. It breaks them
// down over time, by country and by edge location, and lists the slowest
// paths. It returns 403 UPGRADE_REQUIRED on plans other than Pro and
// Enterprise. See [AppsAPI] for workspace applications.
func (n AppNetworkAPI) Performance(ctx context.Context, appID string, start, end time.Time) (*NetworkPerformance, error) {
	return networkWindow[NetworkPerformance](ctx, n.c, appPath(appID, "/network/performance?"+window(start, end).Encode()))
}

// DNS returns the records to create at your DNS provider for the custom
// domain, with the validation status of each. It returns 400 NO_CUSTOM_DOMAIN
// when the application has no custom domain. See [AppsAPI] for workspace
// applications.
func (n AppNetworkAPI) DNS(ctx context.Context, appID string) ([]DNSRecord, error) {
	return call[[]DNSRecord](ctx, n.c, http.MethodGet, appPath(appID, "/network/dns"), nil)
}

// SetDomain sets the application's custom domain to domain, replacing any
// previous one. Pass "@" to remove the custom domain. See [AppsAPI] for
// workspace applications.
//
// It returns 403 UPGRADE_REQUIRED on plans other than Standard, Pro and
// Enterprise, and 400 RESERVED_DOMAIN for a Square Cloud domain. It returns
// 409 DOMAIN_ALREADY_EXISTS when another account uses the domain, and 403
// LOAD_BALANCER_LIMIT_REACHED when the domain is already on as many of your
// applications as your plan allows.
func (n AppNetworkAPI) SetDomain(ctx context.Context, appID, domain string) error {
	return n.c.do(ctx, http.MethodPost, appPath(appID, "/network/custom"), map[string]string{"custom": domain}, nil)
}

// PurgeCache clears the responses cached at the edge for the application's
// domains. See [AppsAPI] for workspace applications.
func (n AppNetworkAPI) PurgeCache(ctx context.Context, appID string) error {
	return n.c.do(ctx, http.MethodPost, appPath(appID, "/network/purge_cache"), nil, nil)
}
