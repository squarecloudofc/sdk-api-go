package rest

import (
	"fmt"
)

var (
	APIVersion = 2
	APIURL     = fmt.Sprintf("https://api.squarecloud.app/v%d", APIVersion)
)

var (
	// Square Cloud Service
	EndpointServiceStatistics = func() string { return "/service/statistics" }

	// User
	EndpointUser          = func() string { return "/users/me" }
	EndpointUserSnapshots = func(snapshotType string) string {
		return fmt.Sprintf("/users/snapshots?scope=%s", snapshotType)
	}

	// Application
	EndpointApplication            = func() string { return "/apps" }
	EndpointApplicationListStatus  = func() string { return "/apps/status" }
	EndpointApplicationInformation = func(appID string) string { return fmt.Sprintf("/apps/%s", appID) }
	EndpointApplicationStatus      = func(appID string) string { return fmt.Sprintf("/apps/%s/status", appID) }
	EndpointApplicationLogs        = func(appID string) string { return fmt.Sprintf("/apps/%s/logs", appID) }
	EndpointApplicationStart       = func(appID string) string { return fmt.Sprintf("/apps/%s/start", appID) }
	EndpointApplicationRestart     = func(appID string) string { return fmt.Sprintf("/apps/%s/restart", appID) }
	EndpointApplicationStop        = func(appID string) string { return fmt.Sprintf("/apps/%s/stop", appID) }
	EndpointApplicationSnapshots   = func(appID string) string { return fmt.Sprintf("/apps/%s/snapshots", appID) }
	EndpointApplicationCommit      = func(appID string) string { return fmt.Sprintf("/apps/%s/commit", appID) }

	// Application File Manager
	EndpointApplicationFiles    = func(appID, path string) string { return fmt.Sprintf("/apps/%s/files?path=%s", appID, path) }
	EndpointApplicationFileRead = func(appID, path string) string { return fmt.Sprintf("/apps/%s/files/content?path=%s", appID, path) }

	// Application Deploy
	EndpointApplicationDeploys           = func(appID string) string { return fmt.Sprintf("/apps/%s/deploy/list", appID) }
	EndpointApplicationGithubIntegration = func(appID string) string { return fmt.Sprintf("/apps/%s/deploy/git-webhook", appID) }

	// Application Network
	EndpointApplicationNetwork      = func(appID string) string { return fmt.Sprintf("/apps/%s/network/analytics", appID) }
	EndpointApplicationCustomDomain = func(appID, domain string) string { return fmt.Sprintf("/apps/%s/network/custom/%s", appID, domain) }
)
