package rest

import (
	"net/http"

	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

var _ Rest = (*restImpl)(nil)

type Rest interface {
	SelfUser(opts ...RequestOpt) (squarecloud.User, error)
	UserSnapshots(scope squarecloud.SnapshotScope, opts ...RequestOpt) ([]squarecloud.Snapshot, error)
	ServiceStatus(opts ...RequestOpt) (squarecloud.ServiceStatus, error)

	Applications
	Databases
	Workspaces
}

type restImpl struct {
	Client
	Applications
	Databases
	Workspaces
}

func New(client Client) Rest {
	return &restImpl{
		Client:       client,
		Applications: NewApplications(client),
		Databases:    NewDatabases(client),
		Workspaces:   NewWorkspaces(client),
	}
}

type responseUser struct {
	User         squarecloud.User              `json:"user"`
	Applications []squarecloud.UserApplication `json:"applications"`
	Databases    []squarecloud.UserDatabase    `json:"databases"`
}

func (s *restImpl) SelfUser(opts ...RequestOpt) (squarecloud.User, error) {
	var r squarecloud.APIResponse[responseUser]
	err := s.Request(http.MethodGet, EndpointUser(), nil, &r, opts...)

	return r.Response.User, err
}

func (s *restImpl) UserSnapshots(scope squarecloud.SnapshotScope, opts ...RequestOpt) ([]squarecloud.Snapshot, error) {
	opts = append(opts, WithQueryParam("scope", string(scope)))

	var r squarecloud.APIResponse[[]squarecloud.Snapshot]
	err := s.Request(http.MethodGet, EndpointUserSnapshots(), nil, &r, opts...)

	return r.Response, err
}

// ServiceStatus returns the platform health. This endpoint responds with
// {status, message} directly, without the usual response envelope.
func (s *restImpl) ServiceStatus(opts ...RequestOpt) (squarecloud.ServiceStatus, error) {
	var r squarecloud.ServiceStatus
	err := s.Request(http.MethodGet, EndpointServiceStatus(), nil, &r, opts...)

	return r, err
}
