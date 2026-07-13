package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

var _ Applications = (*applicationsImpl)(nil)

func NewApplications(client Client) Applications {
	return &applicationsImpl{client: client}
}

type Applications interface {
	GetApplications(options ...RequestOpt) ([]squarecloud.UserApplication, error)
	GetApplicationListStatus(options ...RequestOpt) ([]squarecloud.ApplicationStatusListItem, error)
	GetApplicationDomains(options ...RequestOpt) ([]squarecloud.AppDomain, error)
	GetLoadBalancers(options ...RequestOpt) (squarecloud.LoadBalancers, error)

	PostApplications(reader io.Reader, options ...RequestOpt) (*squarecloud.ApplicationUploaded, error)

	GetApplication(appID string, options ...RequestOpt) (squarecloud.Application, error)
	GetApplicationStatus(appID string, options ...RequestOpt) (squarecloud.ApplicationStatus, error)
	GetApplicationStatusRaw(appID string, options ...RequestOpt) (squarecloud.ApplicationStatusRaw, error)
	GetApplicationLogs(appID string, options ...RequestOpt) (squarecloud.ApplicationLogs, error)
	GetApplicationMetrics(appID string, options ...RequestOpt) ([]squarecloud.MetricPoint, error)
	PostApplicationSignal(appID string, signal squarecloud.ApplicationSignal, options ...RequestOpt) error
	PostApplicationCommit(appID string, reader io.Reader, options ...RequestOpt) error
	DeleteApplication(appID string, options ...RequestOpt) error

	// ApplicationRealtime opens the SSE stream of /apps/{id}/realtime. The
	// caller must Close the returned stream.
	ApplicationRealtime(appID string, options ...RequestOpt) (*RealtimeStream, error)

	GetApplicationSnapshots(appID string, options ...RequestOpt) ([]squarecloud.Snapshot, error)
	CreateApplicationSnapshot(appID string, options ...RequestOpt) (squarecloud.SnapshotCreated, error)
	RestoreApplicationSnapshot(appID, snapshotID, versionID string, options ...RequestOpt) error

	GetApplicationEnvs(appID string, options ...RequestOpt) (squarecloud.EnvVars, error)
	SetApplicationEnvs(appID string, envs squarecloud.EnvVars, options ...RequestOpt) (squarecloud.EnvVars, error)
	ReplaceApplicationEnvs(appID string, envs squarecloud.EnvVars, options ...RequestOpt) (squarecloud.EnvVars, error)
	DeleteApplicationEnvs(appID string, keys []string, options ...RequestOpt) error

	GetApplicationFiles(appID, path string, options ...RequestOpt) ([]squarecloud.FileInfo, error)
	ReadApplicationFile(appID, path string, options ...RequestOpt) (squarecloud.FileContent, error)
	PutApplicationFile(appID, path string, content []byte, options ...RequestOpt) (squarecloud.FileWritten, error)
	MoveApplicationFile(appID, path, to string, options ...RequestOpt) error
	DeleteApplicationFile(appID, path string, options ...RequestOpt) error

	GetApplicationDeployments(appID string, options ...RequestOpt) ([][]squarecloud.Deployment, error)
	GetApplicationCurrentDeployment(appID string, options ...RequestOpt) (squarecloud.DeploymentCurrent, error)
	PostApplicationDeployWebhook(appID, accessToken string, options ...RequestOpt) (squarecloud.GithubWebhook, error)
	LinkApplicationGithubApp(appID, repositoryName, repositoryBranch string, options ...RequestOpt) (squarecloud.GithubAppLink, error)
	UnlinkApplicationGithubApp(appID string, options ...RequestOpt) error

	GetApplicationDNS(appID string, options ...RequestOpt) ([]squarecloud.DNSRecord, error)
	SetApplicationCustomDomain(appID, domain string, options ...RequestOpt) error
	GetApplicationAnalytics(appID string, start, end time.Time, options ...RequestOpt) (squarecloud.NetworkAnalytics, error)
	GetApplicationNetworkErrors(appID string, start, end time.Time, options ...RequestOpt) (squarecloud.NetworkErrors, error)
	GetApplicationNetworkLogs(appID string, start, end time.Time, options ...RequestOpt) ([]squarecloud.NetworkLog, error)
	GetApplicationNetworkPerformance(appID string, start, end time.Time, options ...RequestOpt) (squarecloud.NetworkPerformance, error)
	PurgeApplicationCache(appID string, options ...RequestOpt) error
}

type applicationsImpl struct {
	client Client
}

// multipartZip wraps reader as a multipart form-data body with a single
// "file" field named filename.
func multipartZip(reader io.Reader, filename string) ([]byte, string, error) {
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}

	if _, err := io.Copy(part, reader); err != nil {
		return nil, "", err
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return bodyBuffer.Bytes(), writer.FormDataContentType(), nil
}

func jsonBody(v any) ([]byte, error) {
	return json.Marshal(v)
}

func withRange(start, end time.Time, opts []RequestOpt) []RequestOpt {
	return append(opts,
		WithQueryParam("start", start.UTC().Format(time.RFC3339)),
		WithQueryParam("end", end.UTC().Format(time.RFC3339)),
	)
}

func (s *applicationsImpl) GetApplications(opts ...RequestOpt) ([]squarecloud.UserApplication, error) {
	var r squarecloud.APIResponse[responseUser]
	err := s.client.Request(http.MethodGet, EndpointUser(), nil, &r, opts...)

	return r.Response.Applications, err
}

func (s *applicationsImpl) GetApplicationListStatus(opts ...RequestOpt) ([]squarecloud.ApplicationStatusListItem, error) {
	var r squarecloud.APIResponse[[]squarecloud.ApplicationStatusListItem]
	err := s.client.Request(http.MethodGet, EndpointApplicationListStatus(), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationDomains(opts ...RequestOpt) ([]squarecloud.AppDomain, error) {
	var r squarecloud.APIResponse[[]squarecloud.AppDomain]
	err := s.client.Request(http.MethodGet, EndpointApplicationDomains(), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetLoadBalancers(opts ...RequestOpt) (squarecloud.LoadBalancers, error) {
	var r squarecloud.APIResponse[squarecloud.LoadBalancers]
	err := s.client.Request(http.MethodGet, EndpointLoadBalancers(), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) PostApplications(reader io.Reader, opts ...RequestOpt) (*squarecloud.ApplicationUploaded, error) {
	body, contentType, err := multipartZip(reader, "upload.zip")
	if err != nil {
		return nil, err
	}

	opts = append(opts, WithHeader("Content-Type", contentType))

	var r squarecloud.APIResponse[squarecloud.ApplicationUploaded]
	if err := s.client.Request(http.MethodPost, EndpointApplication(), body, &r, opts...); err != nil {
		return nil, err
	}

	return &r.Response, nil
}

func (s *applicationsImpl) GetApplication(appID string, opts ...RequestOpt) (squarecloud.Application, error) {
	var r squarecloud.APIResponse[squarecloud.Application]
	err := s.client.Request(http.MethodGet, EndpointApplicationInformation(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationStatus(appID string, opts ...RequestOpt) (squarecloud.ApplicationStatus, error) {
	var r squarecloud.APIResponse[squarecloud.ApplicationStatus]
	err := s.client.Request(http.MethodGet, EndpointApplicationStatus(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationStatusRaw(appID string, opts ...RequestOpt) (squarecloud.ApplicationStatusRaw, error) {
	opts = append(opts, WithQueryParam("rawData", "true"))

	var r squarecloud.APIResponse[squarecloud.ApplicationStatusRaw]
	err := s.client.Request(http.MethodGet, EndpointApplicationStatus(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationLogs(appID string, opts ...RequestOpt) (squarecloud.ApplicationLogs, error) {
	var r squarecloud.APIResponse[squarecloud.ApplicationLogs]
	err := s.client.Request(http.MethodGet, EndpointApplicationLogs(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationMetrics(appID string, opts ...RequestOpt) ([]squarecloud.MetricPoint, error) {
	var r squarecloud.APIResponse[[]squarecloud.MetricPoint]
	err := s.client.Request(http.MethodGet, EndpointApplicationMetrics(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) PostApplicationSignal(appID string, signal squarecloud.ApplicationSignal, opts ...RequestOpt) error {
	var endpoint string

	switch signal {
	case squarecloud.ApplicationSignalStart:
		endpoint = EndpointApplicationStart(appID)
	case squarecloud.ApplicationSignalRestart:
		endpoint = EndpointApplicationRestart(appID)
	case squarecloud.ApplicationSignalStop:
		endpoint = EndpointApplicationStop(appID)
	}

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, endpoint, nil, &r, opts...)
}

// PostApplicationCommit uploads a zip to /apps/{id}/commit. Use
// WithQueryParam("path", dir) to commit into a subdirectory.
func (s *applicationsImpl) PostApplicationCommit(appID string, reader io.Reader, opts ...RequestOpt) error {
	body, contentType, err := multipartZip(reader, "commit.zip")
	if err != nil {
		return err
	}

	opts = append(opts, WithHeader("Content-Type", contentType))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, EndpointApplicationCommit(appID), body, &r, opts...)
}

func (s *applicationsImpl) DeleteApplication(appID string, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodDelete, EndpointApplicationInformation(appID), nil, &r, opts...)
}

func (s *applicationsImpl) ApplicationRealtime(appID string, opts ...RequestOpt) (*RealtimeStream, error) {
	body, err := s.client.Stream(http.MethodGet, EndpointApplicationRealtime(appID), opts...)
	if err != nil {
		return nil, err
	}

	return newRealtimeStream(body), nil
}

func (s *applicationsImpl) GetApplicationSnapshots(appID string, opts ...RequestOpt) ([]squarecloud.Snapshot, error) {
	var r squarecloud.APIResponse[[]squarecloud.Snapshot]
	err := s.client.Request(http.MethodGet, EndpointApplicationSnapshots(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) CreateApplicationSnapshot(appID string, opts ...RequestOpt) (squarecloud.SnapshotCreated, error) {
	var r squarecloud.APIResponse[squarecloud.SnapshotCreated]
	err := s.client.Request(http.MethodPost, EndpointApplicationSnapshots(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) RestoreApplicationSnapshot(appID, snapshotID, versionID string, opts ...RequestOpt) error {
	body, err := jsonBody(map[string]string{"snapshotId": snapshotID, "versionId": versionID})
	if err != nil {
		return err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, EndpointApplicationSnapshotRestore(appID), body, &r, opts...)
}

func (s *applicationsImpl) GetApplicationEnvs(appID string, opts ...RequestOpt) (squarecloud.EnvVars, error) {
	var r squarecloud.APIResponse[squarecloud.EnvVars]
	err := s.client.Request(http.MethodGet, EndpointApplicationEnvs(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) SetApplicationEnvs(appID string, envs squarecloud.EnvVars, opts ...RequestOpt) (squarecloud.EnvVars, error) {
	return s.sendEnvs(http.MethodPost, appID, envs, opts)
}

func (s *applicationsImpl) ReplaceApplicationEnvs(appID string, envs squarecloud.EnvVars, opts ...RequestOpt) (squarecloud.EnvVars, error) {
	return s.sendEnvs(http.MethodPut, appID, envs, opts)
}

func (s *applicationsImpl) sendEnvs(method, appID string, envs squarecloud.EnvVars, opts []RequestOpt) (squarecloud.EnvVars, error) {
	body, err := jsonBody(map[string]squarecloud.EnvVars{"envs": envs})
	if err != nil {
		return nil, err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[squarecloud.EnvVars]
	err = s.client.Request(method, EndpointApplicationEnvs(appID), body, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) DeleteApplicationEnvs(appID string, keys []string, opts ...RequestOpt) error {
	body, err := jsonBody(map[string][]string{"envs": keys})
	if err != nil {
		return err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodDelete, EndpointApplicationEnvs(appID), body, &r, opts...)
}

func (s *applicationsImpl) GetApplicationFiles(appID, path string, opts ...RequestOpt) ([]squarecloud.FileInfo, error) {
	if path != "" {
		opts = append(opts, WithQueryParam("path", path))
	}

	var r squarecloud.APIResponse[[]squarecloud.FileInfo]
	err := s.client.Request(http.MethodGet, EndpointApplicationFiles(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) ReadApplicationFile(appID, path string, opts ...RequestOpt) (squarecloud.FileContent, error) {
	opts = append(opts, WithQueryParam("path", path))

	var r squarecloud.APIResponse[squarecloud.FileContent]
	err := s.client.Request(http.MethodGet, EndpointApplicationFileRead(appID), nil, &r, opts...)

	return r.Response, err
}

// PutApplicationFile creates or overwrites a file. Content is sent as a
// UTF-8 string.
// ponytail: binary uploads (Node Buffer object form) not supported yet — add
// when someone needs to push non-text files through the file manager.
func (s *applicationsImpl) PutApplicationFile(appID, path string, content []byte, opts ...RequestOpt) (squarecloud.FileWritten, error) {
	body, err := jsonBody(map[string]string{"path": path, "content": string(content)})
	if err != nil {
		return squarecloud.FileWritten{}, err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[squarecloud.FileWritten]
	err = s.client.Request(http.MethodPut, EndpointApplicationFiles(appID), body, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) MoveApplicationFile(appID, path, to string, opts ...RequestOpt) error {
	body, err := jsonBody(map[string]string{"path": path, "to": to})
	if err != nil {
		return err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPatch, EndpointApplicationFiles(appID), body, &r, opts...)
}

func (s *applicationsImpl) DeleteApplicationFile(appID, path string, opts ...RequestOpt) error {
	opts = append(opts, WithQueryParam("path", path))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodDelete, EndpointApplicationFiles(appID), nil, &r, opts...)
}

func (s *applicationsImpl) GetApplicationDeployments(appID string, opts ...RequestOpt) ([][]squarecloud.Deployment, error) {
	var r squarecloud.APIResponse[[][]squarecloud.Deployment]
	err := s.client.Request(http.MethodGet, EndpointApplicationDeployments(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationCurrentDeployment(appID string, opts ...RequestOpt) (squarecloud.DeploymentCurrent, error) {
	var r squarecloud.APIResponse[squarecloud.DeploymentCurrent]
	err := s.client.Request(http.MethodGet, EndpointApplicationCurrentDeployment(appID), nil, &r, opts...)

	return r.Response, err
}

// PostApplicationDeployWebhook enrolls a GitHub personal access token
// (ghp_* / github_pat_*) for webhook deploys, or removes the existing
// webhook when accessToken is "@".
func (s *applicationsImpl) PostApplicationDeployWebhook(appID, accessToken string, opts ...RequestOpt) (squarecloud.GithubWebhook, error) {
	body, err := jsonBody(map[string]string{"access_token": accessToken})
	if err != nil {
		return squarecloud.GithubWebhook{}, err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[squarecloud.GithubWebhook]
	err = s.client.Request(http.MethodPost, EndpointApplicationDeployWebhook(appID), body, &r, opts...)

	return r.Response, err
}

// LinkApplicationGithubApp links a repository via the Square Cloud GitHub
// App. Requires a session token (not an API key).
func (s *applicationsImpl) LinkApplicationGithubApp(appID, repositoryName, repositoryBranch string, opts ...RequestOpt) (squarecloud.GithubAppLink, error) {
	body, err := jsonBody(map[string]string{
		"repositoryName":   repositoryName,
		"repositoryBranch": repositoryBranch,
	})
	if err != nil {
		return squarecloud.GithubAppLink{}, err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[squarecloud.GithubAppLink]
	err = s.client.Request(http.MethodPost, EndpointApplicationGithubApp(appID), body, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) UnlinkApplicationGithubApp(appID string, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodDelete, EndpointApplicationGithubApp(appID), nil, &r, opts...)
}

func (s *applicationsImpl) GetApplicationDNS(appID string, opts ...RequestOpt) ([]squarecloud.DNSRecord, error) {
	var r squarecloud.APIResponse[[]squarecloud.DNSRecord]
	err := s.client.Request(http.MethodGet, EndpointApplicationDNS(appID), nil, &r, opts...)

	return r.Response, err
}

func (s *applicationsImpl) SetApplicationCustomDomain(appID, domain string, opts ...RequestOpt) error {
	body, err := jsonBody(map[string]string{"custom": domain})
	if err != nil {
		return err
	}

	opts = append(opts, WithHeader("Content-Type", "application/json"))

	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, EndpointApplicationCustomDomain(appID), body, &r, opts...)
}

// GetApplicationAnalytics requires a start/end window (max 7 days
// retention). Optional drill-down filters (country, ip, path, status, os,
// browser, protocol, referer, provider, content_type, bot) go through
// WithQueryParam.
func (s *applicationsImpl) GetApplicationAnalytics(appID string, start, end time.Time, opts ...RequestOpt) (squarecloud.NetworkAnalytics, error) {
	var r squarecloud.APIResponse[squarecloud.NetworkAnalytics]
	err := s.client.Request(http.MethodGet, EndpointApplicationAnalytics(appID), nil, &r, withRange(start, end, opts)...)

	return r.Response, err
}

// GetApplicationNetworkErrors defaults to 5xx only; pass
// WithQueryParam("include_4xx", "true") to include 4xx.
func (s *applicationsImpl) GetApplicationNetworkErrors(appID string, start, end time.Time, opts ...RequestOpt) (squarecloud.NetworkErrors, error) {
	var r squarecloud.APIResponse[squarecloud.NetworkErrors]
	err := s.client.Request(http.MethodGet, EndpointApplicationNetErrors(appID), nil, &r, withRange(start, end, opts)...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationNetworkLogs(appID string, start, end time.Time, opts ...RequestOpt) ([]squarecloud.NetworkLog, error) {
	var r squarecloud.APIResponse[[]squarecloud.NetworkLog]
	err := s.client.Request(http.MethodGet, EndpointApplicationNetLogs(appID), nil, &r, withRange(start, end, opts)...)

	return r.Response, err
}

func (s *applicationsImpl) GetApplicationNetworkPerformance(appID string, start, end time.Time, opts ...RequestOpt) (squarecloud.NetworkPerformance, error) {
	var r squarecloud.APIResponse[squarecloud.NetworkPerformance]
	err := s.client.Request(http.MethodGet, EndpointApplicationNetPerf(appID), nil, &r, withRange(start, end, opts)...)

	return r.Response, err
}

// PurgeApplicationCache purges the entire edge cache of the application.
func (s *applicationsImpl) PurgeApplicationCache(appID string, opts ...RequestOpt) error {
	var r squarecloud.APIResponse[any]
	return s.client.Request(http.MethodPost, EndpointApplicationPurgeCache(appID), nil, &r, opts...)
}
