package rest

import (
	"net/http"

	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

var _ Databases = (*databasesImpl)(nil)

func NewDatabases(client Client) Databases {
	return &databasesImpl{client: client}
}

type Databases interface {
	GetDatabases(options ...RequestOpt) ([]squarecloud.UserDatabase, error)
	GetDatabaseListStatus(options ...RequestOpt) ([]squarecloud.DatabaseStatusListItem, error)
	CreateDatabase(create squarecloud.DatabaseCreateOptions, options ...RequestOpt) (squarecloud.DatabaseCreated, error)

	GetDatabase(dbID string, options ...RequestOpt) (squarecloud.Database, error)
	UpdateDatabase(dbID string, update squarecloud.DatabaseUpdateOptions, options ...RequestOpt) error
	DeleteDatabase(dbID string, options ...RequestOpt) error
	GetDatabaseStatus(dbID string, options ...RequestOpt) (squarecloud.DatabaseStatus, error)
	GetDatabaseStatusRaw(dbID string, options ...RequestOpt) (squarecloud.DatabaseStatusRaw, error)
	GetDatabaseMetrics(dbID string, options ...RequestOpt) ([]squarecloud.MetricPoint, error)
	StartDatabase(dbID string, options ...RequestOpt) error
	StopDatabase(dbID string, options ...RequestOpt) error

	GetDatabaseCertificate(dbID string, options ...RequestOpt) (squarecloud.DatabaseCertificate, error)
	// ResetDatabaseCredentials rotates a credential. The returned password is
	// non-empty only for reset type "password" and is shown only once.
	ResetDatabaseCredentials(dbID string, reset squarecloud.DatabaseResetType, options ...RequestOpt) (squarecloud.DatabasePasswordReset, error)

	GetDatabaseSnapshots(dbID string, options ...RequestOpt) ([]squarecloud.Snapshot, error)
	CreateDatabaseSnapshot(dbID string, options ...RequestOpt) (squarecloud.SnapshotCreated, error)
	RestoreDatabaseSnapshot(dbID, snapshotID, versionID string, options ...RequestOpt) error
}

type databasesImpl struct {
	client Client
}

func (s *databasesImpl) GetDatabases(opts ...RequestOpt) ([]squarecloud.UserDatabase, error) {
	var r squarecloud.APIResponse[responseUser]
	err := s.client.Request(http.MethodGet, EndpointUser(), nil, &r, opts...)

	return r.Response.Databases, err
}

func (s *databasesImpl) GetDatabaseListStatus(opts ...RequestOpt) ([]squarecloud.DatabaseStatusListItem, error) {
	var r squarecloud.APIResponse[[]squarecloud.DatabaseStatusListItem]
	err := s.client.Request(http.MethodGet, EndpointDatabaseListStatus(), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) CreateDatabase(create squarecloud.DatabaseCreateOptions, opts ...RequestOpt) (squarecloud.DatabaseCreated, error) {
	body, err := jsonBody(create)
	if err != nil {
		return squarecloud.DatabaseCreated{}, err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[squarecloud.DatabaseCreated]
	err = s.client.Request(http.MethodPost, EndpointDatabases(), body, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) GetDatabase(dbID string, opts ...RequestOpt) (squarecloud.Database, error) {
	var r squarecloud.APIResponse[squarecloud.Database]
	err := s.client.Request(http.MethodGet, EndpointDatabaseInformation(dbID), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) UpdateDatabase(dbID string, update squarecloud.DatabaseUpdateOptions, opts ...RequestOpt) error {
	body, err := jsonBody(update)
	if err != nil {
		return err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPatch, EndpointDatabaseInformation(dbID), body, &r, opts...)
}

func (s *databasesImpl) DeleteDatabase(dbID string, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodDelete, EndpointDatabaseInformation(dbID), nil, &r, opts...)
}

func (s *databasesImpl) GetDatabaseStatus(dbID string, opts ...RequestOpt) (squarecloud.DatabaseStatus, error) {
	var r squarecloud.APIResponse[squarecloud.DatabaseStatus]
	err := s.client.Request(http.MethodGet, EndpointDatabaseStatus(dbID), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) GetDatabaseStatusRaw(dbID string, opts ...RequestOpt) (squarecloud.DatabaseStatusRaw, error) {
	opts = append(opts, WithQueryParam("rawData", "true"))

	var r squarecloud.APIResponse[squarecloud.DatabaseStatusRaw]
	err := s.client.Request(http.MethodGet, EndpointDatabaseStatus(dbID), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) GetDatabaseMetrics(dbID string, opts ...RequestOpt) ([]squarecloud.MetricPoint, error) {
	var r squarecloud.APIResponse[[]squarecloud.MetricPoint]
	err := s.client.Request(http.MethodGet, EndpointDatabaseMetrics(dbID), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) StartDatabase(dbID string, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, EndpointDatabaseStart(dbID), nil, &r, opts...)
}

func (s *databasesImpl) StopDatabase(dbID string, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, EndpointDatabaseStop(dbID), nil, &r, opts...)
}

func (s *databasesImpl) GetDatabaseCertificate(dbID string, opts ...RequestOpt) (squarecloud.DatabaseCertificate, error) {
	var r squarecloud.APIResponse[squarecloud.DatabaseCertificate]
	err := s.client.Request(http.MethodGet, EndpointDatabaseCertificate(dbID), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) ResetDatabaseCredentials(dbID string, reset squarecloud.DatabaseResetType, opts ...RequestOpt) (squarecloud.DatabasePasswordReset, error) {
	body, err := jsonBody(map[string]squarecloud.DatabaseResetType{"reset": reset})
	if err != nil {
		return squarecloud.DatabasePasswordReset{}, err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[squarecloud.DatabasePasswordReset]
	err = s.client.Request(http.MethodPost, EndpointDatabaseCredentials(dbID), body, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) GetDatabaseSnapshots(dbID string, opts ...RequestOpt) ([]squarecloud.Snapshot, error) {
	var r squarecloud.APIResponse[[]squarecloud.Snapshot]
	err := s.client.Request(http.MethodGet, EndpointDatabaseSnapshots(dbID), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) CreateDatabaseSnapshot(dbID string, opts ...RequestOpt) (squarecloud.SnapshotCreated, error) {
	var r squarecloud.APIResponse[squarecloud.SnapshotCreated]
	err := s.client.Request(http.MethodPost, EndpointDatabaseSnapshots(dbID), nil, &r, opts...)

	return r.Response, err
}

func (s *databasesImpl) RestoreDatabaseSnapshot(dbID, snapshotID, versionID string, opts ...RequestOpt) error {
	body, err := jsonBody(map[string]string{"snapshotId": snapshotID, "versionId": versionID})
	if err != nil {
		return err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, EndpointDatabaseSnapshotRestore(dbID), body, &r, opts...)
}
