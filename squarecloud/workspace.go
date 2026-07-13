package squarecloud

import "time"

// WorkspaceMemberGroup is a workspace member permission tier. "owner" and
// "admin" have full management; "maintain" adds operational access;
// "manager" covers monitoring and lifecycle control; "view" is read-only.
type WorkspaceMemberGroup string

const (
	WorkspaceGroupOwner    WorkspaceMemberGroup = "owner"
	WorkspaceGroupAdmin    WorkspaceMemberGroup = "admin"
	WorkspaceGroupMaintain WorkspaceMemberGroup = "maintain"
	WorkspaceGroupManager  WorkspaceMemberGroup = "manager"
	WorkspaceGroupView     WorkspaceMemberGroup = "view"
)

// Workspace is the full descriptor returned by GET /workspaces/{id} and the
// listing of GET /workspaces.
type Workspace struct {
	// ID is a UUID v4 without hyphens (32 hex chars).
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Owner        string            `json:"owner"`
	Members      []WorkspaceMember `json:"members"`
	Applications []WorkspaceApp    `json:"applications"`
	CreatedAt    time.Time         `json:"createdAt"`
}

type WorkspaceMember struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Group    WorkspaceMemberGroup `json:"group"`
	JoinedAt time.Time            `json:"joinedAt"`
}

// WorkspaceApp is an application descriptor as embedded in a workspace
// listing. Description is always present but may be nil when unset.
type WorkspaceApp struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"desc"`
	RAM         int     `json:"ram"`
	Lang        string  `json:"lang"`
	Domain      *string `json:"domain"`
	Custom      *string `json:"custom"`
}

// WorkspaceCreated is the response of POST /workspaces.
type WorkspaceCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkspaceInviteCode is the response of GET /workspaces/members/code. The
// code is single-use and valid for 5 minutes.
type WorkspaceInviteCode struct {
	Code string `json:"code"`
}
