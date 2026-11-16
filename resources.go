package squarecloud

import (
	"context"
	"net/http"
	"net/url"
)

// AccountAPI reads the account that owns the API key.
type AccountAPI struct{ c *Client }

// Me returns your user, plan and memory usage, with the applications and
// databases you own. With an API key limited to some resources, the lists
// hold only those.
func (a AccountAPI) Me(ctx context.Context) (Account, error) {
	return call[Account](ctx, a.c, http.MethodGet, "/users/me", nil)
}

// Snapshots returns the snapshots of all your applications, or of all your
// databases when scope is [SnapshotScopeDatabases]. Pass "" for
// applications. It returns 402 UPGRADE_REQUIRED when the account has no active
// paid plan, and 429 RATE_LIMITED after 3 calls in a minute. It returns 503
// SNAPSHOTS_UNAVAILABLE while snapshot storage cannot answer, so try again in
// about 30 seconds.
func (a AccountAPI) Snapshots(ctx context.Context, scope SnapshotScope) ([]Snapshot, error) {
	p := "/users/snapshots"
	if scope != "" {
		p += "?scope=" + url.QueryEscape(string(scope))
	}
	return call[[]Snapshot](ctx, a.c, http.MethodGet, p, nil)
}

// ServiceAPI reports the status of the Square Cloud platform.
type ServiceAPI struct{ c *Client }

// Status returns the current status of the Square Cloud services and of the
// outside services they depend on, with the open incidents and maintenance
// windows. It works without an API key.
func (s ServiceAPI) Status(ctx context.Context) (ServiceStatus, error) {
	var out ServiceStatus
	err := s.c.do(ctx, http.MethodGet, "/service/status", nil, raw{&out})
	return out, err
}

// AIAPI sends chat requests to the Square Cloud AI, which uses the OpenAI
// chat completions format. An OpenAI client library pointed at
// [DefaultBaseURL] + "/ai" works too.
type AIAPI struct{ c *Client }

// Chat sends the conversation in req and returns the model's reply.
// Streaming is not supported. It needs an API key with the ai:chat scope and
// a Standard plan or higher. Other plans get 403 upgrade_required.
//
// The AI answers within 90 seconds, or returns 503 server_overloaded. That
// error is safe to try again, but the SDK does not retry it. Without a
// deadline on ctx, Chat waits at least 2 minutes. Errors from Chat use the
// OpenAI format, so their Code is lowercase, such as "rate_limit_exceeded".
// See [APIError].
func (a AIAPI) Chat(ctx context.Context, req ChatRequest) (ChatCompletion, error) {
	ctx, cancel := a.c.bound(ctx, aiFloor)
	defer cancel()
	var out ChatCompletion
	err := a.c.do(ctx, http.MethodPost, "/ai/chat/completions", req, raw{&out})
	return out, err
}

// DatabasesAPI creates and controls databases.
type DatabasesAPI struct {
	c *Client
	// Snapshots takes, lists and restores the database's snapshots.
	Snapshots SnapshotsAPI
}

func dbPath(id, sub string) string { return "/databases/" + url.PathEscape(id) + sub }

// Create creates a database and returns its connection details. Save
// Password from the result, because the API returns it only once.
// [DatabasesAPI.ResetCredentials] sets a new one. Creating a database can
// take up to 95 seconds, so without a deadline on ctx, Create waits at least 2
// minutes.
//
// Create returns 403 UPGRADE_REQUIRED on plans other than Standard, Pro and
// Enterprise, and 400 INSUFFICIENT_MEMORY when your plan does not have enough
// free memory.
func (d DatabasesAPI) Create(ctx context.Context, in DatabaseCreate) (DatabaseCreated, error) {
	var out DatabaseCreated
	err := held(ctx, d.c, http.MethodPost, "/databases", in, &out)
	return out, err
}

// Get returns the database's name, engine, memory, port and owner.
func (d DatabasesAPI) Get(ctx context.Context, id string) (Database, error) {
	return call[Database](ctx, d.c, http.MethodGet, dbPath(id, ""), nil)
}

// Update renames the database or changes its memory. Fields of in that are
// left at their zero value do not change.
func (d DatabasesAPI) Update(ctx context.Context, id string, in DatabaseUpdate) error {
	return d.c.do(ctx, http.MethodPatch, dbPath(id, ""), in, nil)
}

// Delete deletes the database. It returns 409 RESTORE_IN_PROGRESS while a
// snapshot is being restored, so try again when the restore ends.
func (d DatabasesAPI) Delete(ctx context.Context, id string) error {
	return d.c.do(ctx, http.MethodDelete, dbPath(id, ""), nil, nil)
}

// Start starts the database. Without a deadline on ctx, it waits at least 2
// minutes for the API to answer.
//
// When the database cannot start, Start returns a 409 error with no message
// and one of these codes: CONTAINER_ALREADY_STARTED,
// CONTAINER_ALREADY_STOPPED, CONTAINER_NOT_FOUND,
// CONTAINER_INSUFFICIENT_DISK_SPACE, CONTAINER_NETWORK_CONFLICT,
// ACTION_FAILED, or RESTORE_IN_PROGRESS while a snapshot is being restored.
// The "already" codes mean that the database is already in the state you
// asked for. They are still returned as errors, so treat them as success when
// that is what your code needs.
func (d DatabasesAPI) Start(ctx context.Context, id string) error {
	return held(ctx, d.c, http.MethodPost, dbPath(id, "/start"), nil, nil)
}

// Stop stops the database. Its time limit and errors are the same as for
// [DatabasesAPI.Start].
func (d DatabasesAPI) Stop(ctx context.Context, id string) error {
	return held(ctx, d.c, http.MethodPost, dbPath(id, "/stop"), nil, nil)
}

// Status returns the database's current CPU, memory, network and storage
// usage as formatted strings such as "120.4MB". Use [DatabasesAPI.StatusRaw]
// for numbers.
func (d DatabasesAPI) Status(ctx context.Context, id string) (RuntimeStats, error) {
	return call[RuntimeStats](ctx, d.c, http.MethodGet, dbPath(id, "/status"), nil)
}

// StatusRaw returns the same usage as [DatabasesAPI.Status] as numbers: CPU
// in percent, memory in MB, and storage and network traffic in bytes.
func (d DatabasesAPI) StatusRaw(ctx context.Context, id string) (RuntimeStatsRaw, error) {
	return call[RuntimeStatsRaw](ctx, d.c, http.MethodGet, dbPath(id, "/status?rawData=true"), nil)
}

// StatusAll reports whether each of your databases is running, with its CPU
// and memory usage.
func (d DatabasesAPI) StatusAll(ctx context.Context) ([]StatusListItem, error) {
	return call[[]StatusListItem](ctx, d.c, http.MethodGet, "/databases/status", nil)
}

// Metrics returns the database's CPU, memory, network and disk usage over
// time, newest point first. rng is "24h" (the default), with a point every 5
// minutes, or "7d", with a point every hour that holds the hour's average and
// its CPUMax and RAMMax peaks. "7d" needs a Pro or Enterprise plan, and other
// plans get the 24-hour answer without an error.
func (d DatabasesAPI) Metrics(ctx context.Context, id, rng string) ([]MetricPoint, error) {
	return call[[]MetricPoint](ctx, d.c, http.MethodGet, withQuery(dbPath(id, "/metrics"), "range", rng), nil)
}

// Certificate returns the database's TLS file, encoded in base64. It is a PEM
// file with the private key followed by the certificate. The database must be
// running.
func (d DatabasesAPI) Certificate(ctx context.Context, id string) (string, error) {
	r, err := call[struct {
		Certificate string `json:"certificate"`
	}](ctx, d.c, http.MethodGet, dbPath(id, "/credentials/certificate"), nil)
	return r.Certificate, err
}

// ResetCredentials replaces the database's password or TLS certificate, as
// reset selects. With [ResetPassword] it returns the new password, which the
// API shows only this once. With [ResetCertificate] it returns "", and
// [DatabasesAPI.Certificate] returns the new certificate. The database must
// be running.
func (d DatabasesAPI) ResetCredentials(ctx context.Context, id string, reset DatabaseReset) (string, error) {
	r, err := call[struct {
		Password string `json:"password"`
	}](ctx, d.c, http.MethodPost, dbPath(id, "/credentials/reset"), map[string]DatabaseReset{"reset": reset})
	return r.Password, err
}

// WorkspacesAPI creates workspaces, which let other Square Cloud accounts work
// on your applications.
type WorkspacesAPI struct {
	c *Client
	// Members adds, changes and removes the members of a workspace.
	Members WorkspaceMembersAPI
	// Apps shares applications in a workspace.
	Apps WorkspaceAppsAPI
}

// List returns the workspaces you own or belong to.
func (w WorkspacesAPI) List(ctx context.Context) ([]Workspace, error) {
	return call[[]Workspace](ctx, w.c, http.MethodGet, "/workspaces", nil)
}

// Get returns the workspace with its members and shared applications.
func (w WorkspacesAPI) Get(ctx context.Context, workspaceID string) (Workspace, error) {
	return call[Workspace](ctx, w.c, http.MethodGet, "/workspaces/"+url.PathEscape(workspaceID), nil)
}

// Create creates a workspace that you own and returns its id. It returns 403
// UPGRADE_REQUIRED on plans other than Standard, Pro and Enterprise, and 400
// WORKSPACE_LIMIT_REACHED when you already own as many workspaces as your
// plan allows.
func (w WorkspacesAPI) Create(ctx context.Context, name string) (WorkspaceCreated, error) {
	return call[WorkspaceCreated](ctx, w.c, http.MethodPost, "/workspaces", map[string]string{"name": name})
}

// Delete deletes the workspace. Only its owner can delete it.
func (w WorkspacesAPI) Delete(ctx context.Context, workspaceID string) error {
	return w.c.do(ctx, http.MethodDelete, "/workspaces", map[string]string{"workspaceId": workspaceID}, nil)
}

// Leave removes you from a workspace that you belong to. The owner cannot
// leave a workspace and must delete it instead.
func (w WorkspacesAPI) Leave(ctx context.Context, workspaceID string) error {
	return w.c.do(ctx, http.MethodDelete, "/workspaces/leave", map[string]string{"workspaceId": workspaceID}, nil)
}

// WorkspaceMembersAPI adds, changes and removes the members of a workspace.
type WorkspaceMembersAPI struct{ c *Client }

// Add adds a user to the workspace with the permissions of group. code is the
// invite code that the user got from [WorkspaceMembersAPI.InviteCode].
func (m WorkspaceMembersAPI) Add(ctx context.Context, workspaceID, code string, group WorkspaceGroup) error {
	return m.c.do(ctx, http.MethodPost, "/workspaces/members",
		map[string]string{"workspaceId": workspaceID, "code": code, "group": string(group)}, nil)
}

// Update changes the group, and so the permissions, of the member memberID.
func (m WorkspaceMembersAPI) Update(ctx context.Context, workspaceID, memberID string, group WorkspaceGroup) error {
	return m.c.do(ctx, http.MethodPatch, "/workspaces/members",
		map[string]string{"workspaceId": workspaceID, "memberId": memberID, "group": string(group)}, nil)
}

// Remove removes the member memberID from the workspace.
func (m WorkspaceMembersAPI) Remove(ctx context.Context, workspaceID, memberID string) error {
	return m.c.do(ctx, http.MethodDelete, "/workspaces/members",
		map[string]string{"workspaceId": workspaceID, "memberId": memberID}, nil)
}

// InviteCode returns an invite code for your own account. Give it to a
// workspace owner, who passes it to [WorkspaceMembersAPI.Add]. The code
// expires after 5 minutes, and you can get a new one every 5 minutes.
func (m WorkspaceMembersAPI) InviteCode(ctx context.Context) (string, error) {
	r, err := call[struct {
		Code string `json:"code"`
	}](ctx, m.c, http.MethodGet, "/workspaces/members/code", nil)
	return r.Code, err
}

// WorkspaceAppsAPI shares applications in a workspace.
type WorkspaceAppsAPI struct{ c *Client }

// Add shares your application appID in the workspace. Its members then reach
// it with the composite id described in [AppsAPI].
func (a WorkspaceAppsAPI) Add(ctx context.Context, workspaceID, appID string) error {
	return a.c.do(ctx, http.MethodPost, "/workspaces/applications", map[string]string{"workspaceId": workspaceID, "appId": appID}, nil)
}

// Remove stops sharing the application appID in the workspace. Only the
// workspace owner can do this.
func (a WorkspaceAppsAPI) Remove(ctx context.Context, workspaceID, appID string) error {
	return a.c.do(ctx, http.MethodDelete, "/workspaces/applications", map[string]string{"workspaceId": workspaceID, "appId": appID}, nil)
}
