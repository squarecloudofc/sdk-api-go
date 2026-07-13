package rest

import (
	"fmt"
)

var (
	APIVersion = 2
	APIURL     = fmt.Sprintf("https://api.squarecloud.app/v%d", APIVersion)
)

// Query parameters (scope, path, rawData, start/end, ...) are appended via
// WithQueryParam so they are always URL-encoded.
var (
	// Square Cloud Service
	EndpointServiceStatus = func() string { return "/service/status" }

	// User
	EndpointUser          = func() string { return "/users/me" }
	EndpointUserSnapshots = func() string { return "/users/snapshots" }

	// Application
	EndpointApplication            = func() string { return "/apps" }
	EndpointApplicationListStatus  = func() string { return "/apps/status" }
	EndpointApplicationDomains     = func() string { return "/apps/domains" }
	EndpointLoadBalancers          = func() string { return "/apps/load-balancers" }
	EndpointApplicationInformation = func(appID string) string { return fmt.Sprintf("/apps/%s", appID) }
	EndpointApplicationStatus      = func(appID string) string { return fmt.Sprintf("/apps/%s/status", appID) }
	EndpointApplicationLogs        = func(appID string) string { return fmt.Sprintf("/apps/%s/logs", appID) }
	EndpointApplicationMetrics     = func(appID string) string { return fmt.Sprintf("/apps/%s/metrics", appID) }
	EndpointApplicationRealtime    = func(appID string) string { return fmt.Sprintf("/apps/%s/realtime", appID) }
	EndpointApplicationStart       = func(appID string) string { return fmt.Sprintf("/apps/%s/start", appID) }
	EndpointApplicationRestart     = func(appID string) string { return fmt.Sprintf("/apps/%s/restart", appID) }
	EndpointApplicationStop        = func(appID string) string { return fmt.Sprintf("/apps/%s/stop", appID) }
	EndpointApplicationCommit      = func(appID string) string { return fmt.Sprintf("/apps/%s/commit", appID) }

	// Application Snapshots
	EndpointApplicationSnapshots       = func(appID string) string { return fmt.Sprintf("/apps/%s/snapshots", appID) }
	EndpointApplicationSnapshotRestore = func(appID string) string { return fmt.Sprintf("/apps/%s/snapshots/restore", appID) }

	// Application Environment Variables
	EndpointApplicationEnvs = func(appID string) string { return fmt.Sprintf("/apps/%s/envs", appID) }

	// Application File Manager
	EndpointApplicationFiles    = func(appID string) string { return fmt.Sprintf("/apps/%s/files", appID) }
	EndpointApplicationFileRead = func(appID string) string { return fmt.Sprintf("/apps/%s/files/content", appID) }

	// Application Deployments
	EndpointApplicationDeployments       = func(appID string) string { return fmt.Sprintf("/apps/%s/deployments", appID) }
	EndpointApplicationCurrentDeployment = func(appID string) string { return fmt.Sprintf("/apps/%s/deployments/current", appID) }
	EndpointApplicationDeployWebhook     = func(appID string) string { return fmt.Sprintf("/apps/%s/deploy/webhook", appID) }
	EndpointApplicationGithubApp         = func(appID string) string { return fmt.Sprintf("/apps/%s/deploy/github-app", appID) }

	// Application Network
	EndpointApplicationDNS          = func(appID string) string { return fmt.Sprintf("/apps/%s/network/dns", appID) }
	EndpointApplicationCustomDomain = func(appID string) string { return fmt.Sprintf("/apps/%s/network/custom", appID) }
	EndpointApplicationAnalytics    = func(appID string) string { return fmt.Sprintf("/apps/%s/network/analytics", appID) }
	EndpointApplicationNetErrors    = func(appID string) string { return fmt.Sprintf("/apps/%s/network/errors", appID) }
	EndpointApplicationNetLogs      = func(appID string) string { return fmt.Sprintf("/apps/%s/network/logs", appID) }
	EndpointApplicationNetPerf      = func(appID string) string { return fmt.Sprintf("/apps/%s/network/performance", appID) }
	EndpointApplicationPurgeCache   = func(appID string) string { return fmt.Sprintf("/apps/%s/network/purge_cache", appID) }

	// Databases
	EndpointDatabases               = func() string { return "/databases" }
	EndpointDatabaseListStatus      = func() string { return "/databases/status" }
	EndpointDatabaseInformation     = func(dbID string) string { return fmt.Sprintf("/databases/%s", dbID) }
	EndpointDatabaseStatus          = func(dbID string) string { return fmt.Sprintf("/databases/%s/status", dbID) }
	EndpointDatabaseMetrics         = func(dbID string) string { return fmt.Sprintf("/databases/%s/metrics", dbID) }
	EndpointDatabaseStart           = func(dbID string) string { return fmt.Sprintf("/databases/%s/start", dbID) }
	EndpointDatabaseStop            = func(dbID string) string { return fmt.Sprintf("/databases/%s/stop", dbID) }
	EndpointDatabaseCertificate     = func(dbID string) string { return fmt.Sprintf("/databases/%s/credentials/certificate", dbID) }
	EndpointDatabaseCredentials     = func(dbID string) string { return fmt.Sprintf("/databases/%s/credentials/reset", dbID) }
	EndpointDatabaseSnapshots       = func(dbID string) string { return fmt.Sprintf("/databases/%s/snapshots", dbID) }
	EndpointDatabaseSnapshotRestore = func(dbID string) string { return fmt.Sprintf("/databases/%s/snapshots/restore", dbID) }

	// Workspaces
	EndpointWorkspaces            = func() string { return "/workspaces" }
	EndpointWorkspaceInformation  = func(workspaceID string) string { return fmt.Sprintf("/workspaces/%s", workspaceID) }
	EndpointWorkspaceLeave        = func() string { return "/workspaces/leave" }
	EndpointWorkspaceApplications = func() string { return "/workspaces/applications" }
	EndpointWorkspaceMembers      = func() string { return "/workspaces/members" }
	EndpointWorkspaceInviteCode   = func() string { return "/workspaces/members/code" }
)
