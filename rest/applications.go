package rest

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/squarecloudofc/sdk-api-go/squarecloud"
)

var _ Applications = (*applicationsImpl)(nil)

func NewApplications(client Client) Applications {
	return &applicationsImpl{client: client}
}

type Applications interface {
	GetApplications(options ...RequestOpt) ([]squarecloud.UserApplication, error)
	GetApplicationListStatus(options ...RequestOpt) ([]squarecloud.ApplicationListStatus, error)

	PostApplications(reader io.Reader, options ...RequestOpt) (*squarecloud.ApplicationUploaded, error)

	GetApplication(appID string, options ...RequestOpt) (squarecloud.Application, error)
	GetApplicationStatus(appID string, options ...RequestOpt) (squarecloud.ApplicationStatus, error)

	GetApplicationLogs(appID string, options ...RequestOpt) (squarecloud.ApplicationLogs, error)
	PostApplicationSignal(appID string, signal squarecloud.ApplicationSignal, options ...RequestOpt) error
	PostApplicationCommit(appID string, reader io.Reader, options ...RequestOpt) error

	GetApplicationBackups(appID string, options ...RequestOpt) ([]squarecloud.ApplicationBackup, error)
	CreateApplicationBackup(appID string, options ...RequestOpt) (squarecloud.ApplicationBackupCreated, error)

	// GetApplicationFileContent(appID string, path string, options ...RequestOption) error
	// GetApplicationFiles(appID string, path string, options ...RequestOption) error
	// CreateApplicationFile(appID string, path string, options ...RequestOption) error
	// PatchApplicationFile(appID string, path string, to string, options ...RequestOption) error
	// DeleteApplicationFile(appID string, path string, options ...RequestOption) error

	// GetApplicationDeployments(appID string, options ...RequestOption) error
	// GetApplicationCurrentDeployments(appID string, options ...RequestOption) error
	// PostApplicationDeployWebhook(appID string, options ...RequestOption) error

	// GetApplicationDNSRecords(appID string, options ...RequestOption) error
	// GetApplicationAnalytics(appID string, options ...RequestOption) error
	// PostApplicationCustomDomain(appID string, options ...RequestOption) error
	// PostApplicationPurgeCache(appID string, options ...RequestOption) error

	DeleteApplication(appID string, options ...RequestOpt) error
}

type applicationsImpl struct {
	client Client
}

func (s *applicationsImpl) GetApplications(opts ...RequestOpt) ([]squarecloud.UserApplication, error) {
	var r squarecloud.APIResponse[responseUser]
	err := s.client.Request(http.MethodGet, EndpointUser(), nil, &r, opts...)

	return r.Response.Applications, err
}

func (s *applicationsImpl) GetApplicationListStatus(opts ...RequestOpt) ([]squarecloud.ApplicationListStatus, error) {
	var r squarecloud.APIResponse[[]squarecloud.ApplicationListStatus]
	err := s.client.Request(http.MethodGet, EndpointApplicationListStatus(), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) PostApplications(reader io.Reader, opts ...RequestOpt) (*squarecloud.ApplicationUploaded, error) {
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	part, err := writer.CreateFormFile("file", "upload.zip")
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, reader); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	opts = append(opts, WithHeader("Content-Type", writer.FormDataContentType()))

	var r squarecloud.APIResponse[squarecloud.ApplicationUploaded]

	if err = s.client.Request(http.MethodPost, EndpointApplication(), bodyBuffer.Bytes(), &r, opts...); err != nil {
		return nil, err
	}

	return &r.Response, err
}

func (s *applicationsImpl) GetApplication(appID string, opts ...RequestOpt) (squarecloud.Application, error) {
	var r squarecloud.APIResponse[squarecloud.Application]
	err := s.client.Request(http.MethodGet, EndpointApplicationInformation(appID), nil, &r, opts...)

	return r.Response, err
}

func (c *applicationsImpl) PostApplicationCommit(appID string, reader io.Reader, options ...RequestOpt) error {
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	part, err := writer.CreateFormFile("file", "commit.zip")
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, reader); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	options = append(options, WithHeader("Content-Type", writer.FormDataContentType()))

	var r squarecloud.APIResponse[any]
	return c.client.Request(http.MethodPost, EndpointApplicationCommit(appID), bodyBuffer.Bytes(), &r, options...)
}

func (s *applicationsImpl) GetApplicationStatus(appID string, opts ...RequestOpt) (squarecloud.ApplicationStatus, error) {
	var r squarecloud.APIResponse[squarecloud.ApplicationStatus]
	err := s.client.Request(http.MethodGet, EndpointApplicationStatus(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationLogs(appID string, opts ...RequestOpt) (squarecloud.ApplicationLogs, error) {
	var r squarecloud.APIResponse[squarecloud.ApplicationLogs]
	err := s.client.Request(http.MethodGet, EndpointApplicationLogs(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) PostApplicationSignal(appID string, signal squarecloud.ApplicationSignal, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	var endpoint string

	switch signal {
	case squarecloud.ApplicationSignalStart:
		endpoint = EndpointApplicationStart(appID)
	case squarecloud.ApplicationSignalRestart:
		endpoint = EndpointApplicationRestart(appID)
	case squarecloud.ApplicationSignalStop:
		endpoint = EndpointApplicationStop(appID)
	}

	return s.client.Request(http.MethodPost, endpoint, nil, &r, opts...)
}

func (s *applicationsImpl) GetApplicationBackups(appID string, opts ...RequestOpt) ([]squarecloud.ApplicationBackup, error) {
	var r squarecloud.APIResponse[[]squarecloud.ApplicationBackup]
	err := s.client.Request(http.MethodGet, EndpointApplicationSnapshots(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) CreateApplicationBackup(appID string, opts ...RequestOpt) (squarecloud.ApplicationBackupCreated, error) {
	var r squarecloud.APIResponse[squarecloud.ApplicationBackupCreated]
	err := s.client.Request(http.MethodPost, EndpointApplicationSnapshots(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) DeleteApplication(appID string, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodDelete, EndpointApplicationInformation(appID), nil, &r, opts...)
}
