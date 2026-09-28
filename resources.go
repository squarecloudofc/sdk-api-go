package squarecloud

import (
	"context"
	"net/http"
	"net/url"
)

// AccountAPI covers /users.
type AccountAPI struct{ c *Client }

// Me returns the account with its applications and databases in one call.
func (a AccountAPI) Me(ctx context.Context) (Account, error) {
	return call[Account](ctx, a.c, http.MethodGet, "/users/me", nil)
}

// Snapshots lists every snapshot of the account ("" scope = applications).
func (a AccountAPI) Snapshots(ctx context.Context, scope SnapshotScope) ([]Snapshot, error) {
	p := "/users/snapshots"
	if scope != "" {
		p += "?scope=" + url.QueryEscape(string(scope))
	}
	return call[[]Snapshot](ctx, a.c, http.MethodGet, p, nil)
}

// ServiceAPI covers /service.
type ServiceAPI struct{ c *Client }

// Status is the public platform health (works without an API key).
func (s ServiceAPI) Status(ctx context.Context) (ServiceStatus, error) {
	var out ServiceStatus
	err := s.c.do(ctx, http.MethodGet, "/service/status", nil, raw{&out})
	return out, err
}

// AIAPI covers /ai. Any OpenAI SDK pointed at DefaultBaseURL+"/ai" works too.
type AIAPI struct{ c *Client }

// Chat sends a non-streaming chat completion (scope ai:chat, Standard plan
// and up). Route errors use the OpenAI dialect; both dialects become *APIError.
// The gateway gives the whole request one 90s deadline and answers 503
// server_overloaded past it; that is safe for the caller to retry, but the SDK
// does not. Without a ctx deadline it gets at least 2 minutes, not the usual
// 30s.
func (a AIAPI) Chat(ctx context.Context, req ChatRequest) (ChatCompletion, error) {
	ctx, cancel := a.c.bound(ctx, aiFloor)
	defer cancel()
	var out ChatCompletion
	err := a.c.do(ctx, http.MethodPost, "/ai/chat/completions", req, raw{&out})
	return out, err
}

// DatabasesAPI covers /databases.
type DatabasesAPI struct {
	c         *Client
	Snapshots SnapshotsAPI
}

func dbPath(id, sub string) string { return "/databases/" + url.PathEscape(id) + sub }

// Create provisions a database. Keep the result: Password, Certificate and
// ConnectionURL are never shown again. Provisioning can take up to 95s, so
// without a ctx deadline it gets at least 2 minutes, not the usual 30s.
func (d DatabasesAPI) Create(ctx context.Context, in DatabaseCreate) (DatabaseCreated, error) {
	var out DatabaseCreated
	err := held(ctx, d.c, http.MethodPost, "/databases", in, &out)
	return out, err
}

// Get returns a database.
func (d DatabasesAPI) Get(ctx context.Context, id string) (Database, error) {
	return call[Database](ctx, d.c, http.MethodGet, dbPath(id, ""), nil)
}

// Update renames or resizes a database; zero fields are left unchanged.
func (d DatabasesAPI) Update(ctx context.Context, id string, in DatabaseUpdate) error {
	return d.c.do(ctx, http.MethodPatch, dbPath(id, ""), in, nil)
}

// Delete deletes a database.
func (d DatabasesAPI) Delete(ctx context.Context, id string) error {
	return d.c.do(ctx, http.MethodDelete, dbPath(id, ""), nil, nil)
}

// Start starts a database. A cluster refusal is 409 with a code and no
// message: CONTAINER_ALREADY_STARTED, CONTAINER_ALREADY_STOPPED,
// CONTAINER_NOT_FOUND, CONTAINER_INSUFFICIENT_DISK_SPACE,
// CONTAINER_NETWORK_CONFLICT or ACTION_FAILED. An "already" code means the
// database is in the wanted state; the SDK returns it as an error and leaves
// that call to the caller. 403 RESTORE_IN_PROGRESS while a snapshot restores.
func (d DatabasesAPI) Start(ctx context.Context, id string) error {
	return held(ctx, d.c, http.MethodPost, dbPath(id, "/start"), nil, nil)
}

// Stop stops a database; errors as in Start.
func (d DatabasesAPI) Stop(ctx context.Context, id string) error {
	return held(ctx, d.c, http.MethodPost, dbPath(id, "/stop"), nil, nil)
}

// Status returns formatted runtime stats. See StatusRaw for numbers.
func (d DatabasesAPI) Status(ctx context.Context, id string) (RuntimeStats, error) {
	return call[RuntimeStats](ctx, d.c, http.MethodGet, dbPath(id, "/status"), nil)
}

// StatusRaw is Status with ?rawData=true.
func (d DatabasesAPI) StatusRaw(ctx context.Context, id string) (RuntimeStatsRaw, error) {
	return call[RuntimeStatsRaw](ctx, d.c, http.MethodGet, dbPath(id, "/status?rawData=true"), nil)
}

// StatusAll lists every database's status.
func (d DatabasesAPI) StatusAll(ctx context.Context) ([]StatusListItem, error) {
	return call[[]StatusListItem](ctx, d.c, http.MethodGet, "/databases/status", nil)
}

// Metrics returns the 5-minute samples of the last 24 hours, newest first.
func (d DatabasesAPI) Metrics(ctx context.Context, id string) ([]MetricPoint, error) {
	return call[[]MetricPoint](ctx, d.c, http.MethodGet, dbPath(id, "/metrics"), nil)
}

// Certificate returns the TLS certificate as base64-encoded PEM.
func (d DatabasesAPI) Certificate(ctx context.Context, id string) (string, error) {
	r, err := call[struct {
		Certificate string `json:"certificate"`
	}](ctx, d.c, http.MethodGet, dbPath(id, "/credentials/certificate"), nil)
	return r.Certificate, err
}

// ResetCredentials rotates the password or certificate. The new password is
// returned (once) for ResetPassword and is empty for ResetCertificate.
func (d DatabasesAPI) ResetCredentials(ctx context.Context, id string, reset DatabaseReset) (string, error) {
	r, err := call[struct {
		Password string `json:"password"`
	}](ctx, d.c, http.MethodPost, dbPath(id, "/credentials/reset"), map[string]DatabaseReset{"reset": reset})
	return r.Password, err
}

// WorkspacesAPI covers /workspaces.
type WorkspacesAPI struct {
	c       *Client
	Members WorkspaceMembersAPI
	Apps    WorkspaceAppsAPI
}

// List lists the workspaces the account owns or belongs to.
func (w WorkspacesAPI) List(ctx context.Context) ([]Workspace, error) {
	return call[[]Workspace](ctx, w.c, http.MethodGet, "/workspaces", nil)
}

// Get returns a workspace with its members and applications.
func (w WorkspacesAPI) Get(ctx context.Context, workspaceID string) (Workspace, error) {
	return call[Workspace](ctx, w.c, http.MethodGet, "/workspaces/"+url.PathEscape(workspaceID), nil)
}

// Create creates a workspace owned by the account.
func (w WorkspacesAPI) Create(ctx context.Context, name string) (WorkspaceCreated, error) {
	return call[WorkspaceCreated](ctx, w.c, http.MethodPost, "/workspaces", map[string]string{"name": name})
}

// Delete deletes a workspace (owner only).
func (w WorkspacesAPI) Delete(ctx context.Context, workspaceID string) error {
	return w.c.do(ctx, http.MethodDelete, "/workspaces", map[string]string{"workspaceId": workspaceID}, nil)
}

// Leave removes the account from a workspace it does not own.
func (w WorkspacesAPI) Leave(ctx context.Context, workspaceID string) error {
	return w.c.do(ctx, http.MethodDelete, "/workspaces/leave", map[string]string{"workspaceId": workspaceID}, nil)
}

// WorkspaceMembersAPI covers /workspaces/members.
type WorkspaceMembersAPI struct{ c *Client }

// Add adds the user who generated code (their InviteCode) with group.
func (m WorkspaceMembersAPI) Add(ctx context.Context, workspaceID, code string, group WorkspaceGroup) error {
	return m.c.do(ctx, http.MethodPost, "/workspaces/members",
		map[string]string{"workspaceId": workspaceID, "code": code, "group": string(group)}, nil)
}

// Update changes a member's group.
func (m WorkspaceMembersAPI) Update(ctx context.Context, workspaceID, memberID string, group WorkspaceGroup) error {
	return m.c.do(ctx, http.MethodPatch, "/workspaces/members",
		map[string]string{"workspaceId": workspaceID, "memberId": memberID, "group": string(group)}, nil)
}

// Remove removes a member from the workspace.
func (m WorkspaceMembersAPI) Remove(ctx context.Context, workspaceID, memberID string) error {
	return m.c.do(ctx, http.MethodDelete, "/workspaces/members",
		map[string]string{"workspaceId": workspaceID, "memberId": memberID}, nil)
}

// InviteCode generates the caller's single-use invite code (valid 5 minutes)
// for a workspace owner to pass to Add.
func (m WorkspaceMembersAPI) InviteCode(ctx context.Context) (string, error) {
	r, err := call[struct {
		Code string `json:"code"`
	}](ctx, m.c, http.MethodGet, "/workspaces/members/code", nil)
	return r.Code, err
}

// WorkspaceAppsAPI covers /workspaces/applications.
type WorkspaceAppsAPI struct{ c *Client }

// Add shares the application appID in the workspace.
func (a WorkspaceAppsAPI) Add(ctx context.Context, workspaceID, appID string) error {
	return a.c.do(ctx, http.MethodPost, "/workspaces/applications", map[string]string{"workspaceId": workspaceID, "appId": appID}, nil)
}

// Remove stops sharing the application appID in the workspace.
func (a WorkspaceAppsAPI) Remove(ctx context.Context, workspaceID, appID string) error {
	return a.c.do(ctx, http.MethodDelete, "/workspaces/applications", map[string]string{"workspaceId": workspaceID, "appId": appID}, nil)
}
