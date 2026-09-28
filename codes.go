package squarecloud

// Error codes. APIError.Code is a plain string: compare it with these
// constants. The API may add codes; treat an unknown one by its HTTP status.

// Codes the SDK sets itself.
const (
	CodeNetworkError  = "NETWORK_ERROR"   // no complete HTTP response (Status 0)
	CodeTimeout       = "TIMEOUT"         // deadline exceeded (Status 0)
	CodeUnknown       = "UNKNOWN_ERROR"   // no code in the body (e.g. a proxy page), or a 2xx that is not JSON
	CodeInvalidAPIKey = "INVALID_API_KEY" // local: New was given an empty key (Status 0)

	// Deprecated: use CodeRateLimited.
	CodeRateLimit = "RATE_LIMIT"
	// Deprecated: use CodeRateLimited.
	CodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
)

// Codes the API sends: every value of the spec's ErrorCode enum. On the AI
// route the API sends lowercase OpenAI-style codes instead (access_denied,
// rate_limit_exceeded, server_overloaded, ...), which Code carries verbatim.
const (
	CodeAccessDenied                      = "ACCESS_DENIED" // 401: a missing, unknown, revoked or expired API key
	CodeActionFailed                      = "ACTION_FAILED" // 409 from Start/Stop/Restart: the cluster refused
	CodeAIDailyLimitReached               = "AI_DAILY_LIMIT_REACHED"
	CodeAIMaxConcurrentStreams            = "AI_MAX_CONCURRENT_STREAMS"
	CodeAINoPlanLimitReached              = "AI_NO_PLAN_LIMIT_REACHED"
	CodeAIUnavailable                     = "AI_UNAVAILABLE"
	CodeAnalyticsBusy                     = "ANALYTICS_BUSY" // 503, rejected before any work: retried
	CodeApplicationsLimitReached          = "APPLICATIONS_LIMIT_REACHED"
	CodeAppAlreadyInWorkspace             = "APP_ALREADY_IN_WORKSPACE"
	CodeAppNotFound                       = "APP_NOT_FOUND"
	CodeBlockedPath                       = "BLOCKED_PATH"
	CodeBranchNotFound                    = "BRANCH_NOT_FOUND"
	CodeCannotEditOwner                   = "CANNOT_EDIT_OWNER"
	CodeCannotInviteOwner                 = "CANNOT_INVITE_OWNER"
	CodeCannotLeaveOwner                  = "CANNOT_LEAVE_OWNER"
	CodeCannotSetSubdomain                = "CANNOT_SET_SUBDOMAIN"
	CodeClusterMaintenanceTryLater        = "CLUSTER_MAINTENANCE_TRY_LATER"
	CodeClusterSelectionFailed            = "CLUSTER_SELECTION_FAILED"
	CodeClusterTimeout                    = "CLUSTER_TIMEOUT"
	CodeClusterUnavailable                = "CLUSTER_UNAVAILABLE"
	CodeCommitFailed                      = "COMMIT_FAILED"
	CodeConflictingResources              = "CONFLICTING_RESOURCES"
	CodeContainerAlreadyStarted           = "CONTAINER_ALREADY_STARTED"
	CodeContainerAlreadyStopped           = "CONTAINER_ALREADY_STOPPED"
	CodeContainerInsufficientDiskSpace    = "CONTAINER_INSUFFICIENT_DISK_SPACE"
	CodeContainerNetworkConflict          = "CONTAINER_NETWORK_CONFLICT"
	CodeContainerNotFound                 = "CONTAINER_NOT_FOUND"
	CodeContainerTemporarilySuspended     = "CONTAINER_TEMPORARILY_SUSPENDED"
	CodeDailySnapshotsLimitReached        = "DAILY_SNAPSHOTS_LIMIT_REACHED"
	CodeDatabaseCreationFailed            = "DATABASE_CREATION_FAILED"
	CodeDatabaseNotFound                  = "DATABASE_NOT_FOUND"
	CodeDatabaseNotRunning                = "DATABASE_NOT_RUNNING"
	CodeDatabaseUnavailable               = "DATABASE_UNAVAILABLE" // 503: retried on GET only (see APIError)
	CodeDeleteFailed                      = "DELETE_FAILED"
	CodeDNSFailed                         = "DNS_FAILED"
	CodeDomainAlreadyExists               = "DOMAIN_ALREADY_EXISTS"
	CodeEmptyResponse                     = "EMPTY_RESPONSE"
	CodeEnvContentTooLong                 = "ENV_CONTENT_TOO_LONG"
	CodeEnvNameTooLong                    = "ENV_NAME_TOO_LONG"
	CodeFailedToFetch                     = "FAILED_TO_FETCH" // 502 on LinkGithubApp: safe for the caller to retry
	CodeFileNotFound                      = "FILE_NOT_FOUND"
	CodeFileTooLarge                      = "FILE_TOO_LARGE" // 413, or local (Status 0): zip over 100 MB or file over 10 MB
	CodeGitAlreadyConfigured              = "GIT_ALREADY_CONFIGURED"
	CodeGitNotConfigured                  = "GIT_NOT_CONFIGURED"
	CodeGithubNotConnected                = "GITHUB_NOT_CONNECTED"
	CodeInsufficientMemory                = "INSUFFICIENT_MEMORY"
	CodeInternalServerError               = "INTERNAL_SERVER_ERROR"
	CodeInvalidAccessToken                = "INVALID_ACCESS_TOKEN"
	CodeInvalidAutorestart                = "INVALID_AUTORESTART"
	CodeInvalidBranchLength               = "INVALID_BRANCH_LENGTH"
	CodeInvalidCode                       = "INVALID_CODE"
	CodeInvalidContent                    = "INVALID_CONTENT"
	CodeInvalidContentType                = "INVALID_CONTENT_TYPE"
	CodeInvalidDatabaseType               = "INVALID_DATABASE_TYPE"
	CodeInvalidDatabaseVersion            = "INVALID_DATABASE_VERSION"
	CodeInvalidDescription                = "INVALID_DESCRIPTION"
	CodeInvalidDisplayName                = "INVALID_DISPLAY_NAME"
	CodeInvalidDomain                     = "INVALID_DOMAIN"
	CodeInvalidEncoding                   = "INVALID_ENCODING"
	CodeInvalidEnvContent                 = "INVALID_ENV_CONTENT"
	CodeInvalidFile                       = "INVALID_FILE"
	CodeInvalidFilename                   = "INVALID_FILENAME"
	CodeInvalidFilter                     = "INVALID_FILTER"
	CodeInvalidGroup                      = "INVALID_GROUP"
	CodeInvalidID                         = "INVALID_ID" // 400, or local (Status 0): an ID is empty, "." or ".."
	CodeInvalidInput                      = "INVALID_INPUT"
	CodeInvalidJSONBody                   = "INVALID_JSON_BODY"
	CodeInvalidMemory                     = "INVALID_MEMORY"
	CodeInvalidName                       = "INVALID_NAME"
	CodeInvalidParameters                 = "INVALID_PARAMETERS"
	CodeInvalidPath                       = "INVALID_PATH"
	CodeInvalidResetType                  = "INVALID_RESET_TYPE"
	CodeInvalidScope                      = "INVALID_SCOPE"
	CodeInvalidSnapshotID                 = "INVALID_SNAPSHOT_ID"
	CodeInvalidSubdomain                  = "INVALID_SUBDOMAIN"
	CodeInvalidTimeRange                  = "INVALID_TIME_RANGE"
	CodeInvalidVersionID                  = "INVALID_VERSION_ID"
	CodeKeepCalm                          = "KEEP_CALM" // 429 per-endpoint cooldown: never retried
	CodeLoadBalancerLimitReached          = "LOAD_BALANCER_LIMIT_REACHED"
	CodeLogsUnavailable                   = "LOGS_UNAVAILABLE"
	CodeMembersLimitReached               = "MEMBERS_LIMIT_REACHED"
	CodeMemberAlreadyAdded                = "MEMBER_ALREADY_ADDED"
	CodeMemberNotFound                    = "MEMBER_NOT_FOUND"
	CodeMetricsNotSupported               = "METRICS_NOT_SUPPORTED"
	CodeMissingParameters                 = "MISSING_PARAMETERS"
	CodeMissingRequiredFields             = "MISSING_REQUIRED_FIELDS"
	CodeMissingScope                      = "MISSING_SCOPE"
	CodeNoCustomDomain                    = "NO_CUSTOM_DOMAIN"
	CodeNoUpdateData                      = "NO_UPDATE_DATA"
	CodePayloadTooLarge                   = "PAYLOAD_TOO_LARGE"
	CodePermissionDenied                  = "PERMISSION_DENIED"
	CodePurgeCacheFailed                  = "PURGE_CACHE_FAILED"
	CodeRateLimited                       = "RATE_LIMITED" // 429: account, key or IP block (up to ~30 min), network and snapshot-listing limit; never retried
	CodeReadFailed                        = "READ_FAILED"
	CodeRealtimeMaxConnections            = "REALTIME_MAX_CONNECTIONS"
	CodeRealtimeMaxConnectionsApp         = "REALTIME_MAX_CONNECTIONS_APP"
	CodeRenameFailed                      = "RENAME_FAILED"
	CodeRepositoryBranchAlreadyConfigured = "REPOSITORY_BRANCH_ALREADY_CONFIGURED"
	CodeRepositoryNotAvailable            = "REPOSITORY_NOT_AVAILABLE"
	CodeRepositoryNotFound                = "REPOSITORY_NOT_FOUND"
	CodeRepositoryPermissionRequired      = "REPOSITORY_PERMISSION_REQUIRED" // 403: no write access to the repository
	CodeRequestAborted                    = "REQUEST_ABORTED"
	CodeReservedDomain                    = "RESERVED_DOMAIN"
	CodeResetFailed                       = "RESET_FAILED"
	CodeResourceNotAllowed                = "RESOURCE_NOT_ALLOWED"
	CodeRestoreInProgress                 = "RESTORE_IN_PROGRESS"
	CodeRouteNotFound                     = "ROUTE_NOT_FOUND"
	CodeSaveFailed                        = "SAVE_FAILED"
	CodeScopeNotGrantable                 = "SCOPE_NOT_GRANTABLE"
	CodeSnapshotDatabaseMismatch          = "SNAPSHOT_DATABASE_MISMATCH"
	CodeSnapshotFailed                    = "SNAPSHOT_FAILED"
	CodeSnapshotNotFound                  = "SNAPSHOT_NOT_FOUND"
	CodeSnapshotProcessing                = "SNAPSHOT_PROCESSING" // surfaced as SnapshotCreated.Pending, not as an error
	CodeSnapshotRestoreFailed             = "SNAPSHOT_RESTORE_FAILED"
	CodeStaticAppEnvNotSupported          = "STATIC_APP_ENV_NOT_SUPPORTED"
	CodeStorageUploadFailed               = "STORAGE_UPLOAD_FAILED"
	CodeTooManyEnvVars                    = "TOO_MANY_ENV_VARS"
	CodeUnableToFetchAnalytics            = "UNABLE_TO_FETCH_ANALYTICS"
	CodeUnableToFetchErrors               = "UNABLE_TO_FETCH_ERRORS"
	CodeUnableToFetchPerformance          = "UNABLE_TO_FETCH_PERFORMANCE"
	CodeUpgradeRequired                   = "UPGRADE_REQUIRED"
	CodeUploadAborted                     = "UPLOAD_ABORTED"
	CodeUploadBusy                        = "UPLOAD_BUSY" // 503, rejected before any work: retried
	CodeUploadFailed                      = "UPLOAD_FAILED"
	CodeValidationFailed                  = "VALIDATION_FAILED"
	CodeValidationTimeout                 = "VALIDATION_TIMEOUT"
	CodeWorkspaceCreationFailed           = "WORKSPACE_CREATION_FAILED"
	CodeWorkspaceLimitReached             = "WORKSPACE_LIMIT_REACHED"
	CodeWorkspaceNotFound                 = "WORKSPACE_NOT_FOUND"
)
