package squarecloud

// Codes that the SDK sets itself. They come with Status 0, except
// CodeUnknown, which comes with the HTTP status of the response.
const (
	// CodeNetworkError means that no complete response arrived, for example
	// because the connection dropped. [APIError.Unwrap] returns the cause.
	CodeNetworkError = "NETWORK_ERROR"
	// CodeTimeout means that the context deadline or the client timeout
	// passed before the call finished. See [WithTimeout].
	CodeTimeout = "TIMEOUT"
	// CodeUnknown means that the response had no error code, such as an error
	// page from a proxy, or that a successful response was not valid JSON.
	CodeUnknown = "UNKNOWN_ERROR"
	// CodeInvalidAPIKey means that [New] got an empty API key, so the call
	// was not sent.
	CodeInvalidAPIKey = "INVALID_API_KEY"

	// CodeRateLimit is the former code of the account rate limit.
	//
	// Deprecated: The API sends [CodeRateLimited] instead.
	CodeRateLimit = "RATE_LIMIT"
	// CodeRateLimitExceeded is the former code of the rate limit on the
	// network calls and on [AccountAPI.Snapshots].
	//
	// Deprecated: The API sends [CodeRateLimited] instead.
	CodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	// CodeMetricsNotSupported was the answer to Metrics for applications with
	// less than 512 MB.
	//
	// Deprecated: The API no longer sends it. Applications with less than
	// 512 MB now get a metrics point every 10 minutes.
	CodeMetricsNotSupported = "METRICS_NOT_SUPPORTED"
)

// Codes that the API sends, one for every error code its calls document.
// Compare [APIError.Code] with them. The API can add codes at any time, so
// handle a code you do not know by its HTTP status. Errors from [AIAPI.Chat]
// use lowercase OpenAI codes instead, such as "access_denied",
// "rate_limit_exceeded" and "server_overloaded", which Code holds as they
// are.
const (
	// CodeAccessDenied comes with 401 when the API key is missing, unknown,
	// revoked or expired.
	CodeAccessDenied = "ACCESS_DENIED"
	// CodeActionFailed comes with 409 when Start, Stop or Restart could not
	// complete the action.
	CodeActionFailed           = "ACTION_FAILED"
	CodeAIDailyLimitReached    = "AI_DAILY_LIMIT_REACHED"
	CodeAIMaxConcurrentStreams = "AI_MAX_CONCURRENT_STREAMS"
	CodeAINoPlanLimitReached   = "AI_NO_PLAN_LIMIT_REACHED"
	CodeAIUnavailable          = "AI_UNAVAILABLE"
	CodeAIWeeklyLimitReached   = "AI_WEEKLY_LIMIT_REACHED"
	// CodeAnalyticsBusy comes with 503 when the API refused a network call
	// before doing any work. The SDK retries it.
	CodeAnalyticsBusy                  = "ANALYTICS_BUSY"
	CodeApplicationsLimitReached       = "APPLICATIONS_LIMIT_REACHED"
	CodeAppAlreadyInWorkspace          = "APP_ALREADY_IN_WORKSPACE"
	CodeAppNotFound                    = "APP_NOT_FOUND"
	CodeBlockedPath                    = "BLOCKED_PATH"
	CodeBranchNotFound                 = "BRANCH_NOT_FOUND"
	CodeCannotEditOwner                = "CANNOT_EDIT_OWNER"
	CodeCannotInviteOwner              = "CANNOT_INVITE_OWNER"
	CodeCannotLeaveOwner               = "CANNOT_LEAVE_OWNER"
	CodeCannotSetSubdomain             = "CANNOT_SET_SUBDOMAIN"
	CodeClusterMaintenanceTryLater     = "CLUSTER_MAINTENANCE_TRY_LATER"
	CodeClusterSelectionFailed         = "CLUSTER_SELECTION_FAILED"
	CodeClusterTimeout                 = "CLUSTER_TIMEOUT"
	CodeClusterUnavailable             = "CLUSTER_UNAVAILABLE"
	CodeCommitFailed                   = "COMMIT_FAILED"
	CodeConflictingResources           = "CONFLICTING_RESOURCES"
	CodeContainerAlreadyStarted        = "CONTAINER_ALREADY_STARTED"
	CodeContainerAlreadyStopped        = "CONTAINER_ALREADY_STOPPED"
	CodeContainerInsufficientDiskSpace = "CONTAINER_INSUFFICIENT_DISK_SPACE"
	CodeContainerNetworkConflict       = "CONTAINER_NETWORK_CONFLICT"
	CodeContainerNotFound              = "CONTAINER_NOT_FOUND"
	CodeContainerTemporarilySuspended  = "CONTAINER_TEMPORARILY_SUSPENDED"
	CodeDailySnapshotsLimitReached     = "DAILY_SNAPSHOTS_LIMIT_REACHED"
	CodeDatabaseCreationFailed         = "DATABASE_CREATION_FAILED"
	CodeDatabaseNotFound               = "DATABASE_NOT_FOUND"
	CodeDatabaseNotRunning             = "DATABASE_NOT_RUNNING"
	// CodeDatabaseUnavailable comes with 503 when the API cannot serve the
	// request for a moment. The SDK retries it on GET requests only. See
	// [APIError].
	CodeDatabaseUnavailable = "DATABASE_UNAVAILABLE"
	CodeDeleteFailed        = "DELETE_FAILED"
	CodeDNSFailed           = "DNS_FAILED"
	CodeDomainAlreadyExists = "DOMAIN_ALREADY_EXISTS"
	CodeEmptyResponse       = "EMPTY_RESPONSE"
	// CodeEnvBusy comes with 409 when another write to the same environment
	// variables was in progress. The SDK does not retry it.
	CodeEnvBusy           = "ENV_BUSY"
	CodeEnvContentTooLong = "ENV_CONTENT_TOO_LONG"
	CodeEnvNameTooLong    = "ENV_NAME_TOO_LONG"
	// CodeFailedToFetch comes with 502 when GitHub did not confirm the branch
	// in [AppDeploysAPI.LinkGithubApp]. It is safe to try again.
	CodeFailedToFetch = "FAILED_TO_FETCH"
	CodeFileNotFound  = "FILE_NOT_FOUND"
	// CodeFileTooLarge comes with 413 from the API, or with Status 0 from the
	// SDK's own check, when an upload is over 100 MB or a file is over 10 MB.
	CodeFileTooLarge         = "FILE_TOO_LARGE"
	CodeGitAlreadyConfigured = "GIT_ALREADY_CONFIGURED"
	// CodeGitBranchDeleted is sent only in answers to GitHub push webhooks,
	// never to the calls of this SDK.
	CodeGitBranchDeleted = "GIT_BRANCH_DELETED"
	CodeGitNotConfigured = "GIT_NOT_CONFIGURED"
	// CodeGitRefNotSupported is sent only in answers to GitHub push webhooks,
	// never to the calls of this SDK.
	CodeGitRefNotSupported     = "GIT_REF_NOT_SUPPORTED"
	CodeGithubNotConnected     = "GITHUB_NOT_CONNECTED"
	CodeInsufficientMemory     = "INSUFFICIENT_MEMORY"
	CodeInternalServerError    = "INTERNAL_SERVER_ERROR"
	CodeInvalidAccessToken     = "INVALID_ACCESS_TOKEN"
	CodeInvalidAutorestart     = "INVALID_AUTORESTART"
	CodeInvalidBranchLength    = "INVALID_BRANCH_LENGTH"
	CodeInvalidCode            = "INVALID_CODE"
	CodeInvalidContent         = "INVALID_CONTENT"
	CodeInvalidContentType     = "INVALID_CONTENT_TYPE"
	CodeInvalidDatabaseType    = "INVALID_DATABASE_TYPE"
	CodeInvalidDatabaseVersion = "INVALID_DATABASE_VERSION"
	CodeInvalidDescription     = "INVALID_DESCRIPTION"
	CodeInvalidDisplayName     = "INVALID_DISPLAY_NAME"
	CodeInvalidDomain          = "INVALID_DOMAIN"
	CodeInvalidEncoding        = "INVALID_ENCODING"
	CodeInvalidEnvContent      = "INVALID_ENV_CONTENT"
	CodeInvalidFile            = "INVALID_FILE"
	CodeInvalidFilename        = "INVALID_FILENAME"
	CodeInvalidFilter          = "INVALID_FILTER"
	CodeInvalidGroup           = "INVALID_GROUP"
	// CodeInvalidID comes with 400 from the API for a malformed id. The SDK
	// also returns it with Status 0, without sending anything, when an id is
	// empty, "." or "..".
	CodeInvalidID         = "INVALID_ID"
	CodeInvalidInput      = "INVALID_INPUT"
	CodeInvalidJSONBody   = "INVALID_JSON_BODY"
	CodeInvalidMemory     = "INVALID_MEMORY"
	CodeInvalidName       = "INVALID_NAME"
	CodeInvalidParameters = "INVALID_PARAMETERS"
	CodeInvalidPath       = "INVALID_PATH"
	CodeInvalidResetType  = "INVALID_RESET_TYPE"
	CodeInvalidScope      = "INVALID_SCOPE"
	CodeInvalidSnapshotID = "INVALID_SNAPSHOT_ID"
	CodeInvalidSubdomain  = "INVALID_SUBDOMAIN"
	CodeInvalidTimeRange  = "INVALID_TIME_RANGE"
	CodeInvalidVersionID  = "INVALID_VERSION_ID"
	// CodeKeepCalm comes with 429 when one call is made too often. The SDK
	// never retries it.
	CodeKeepCalm                 = "KEEP_CALM"
	CodeLoadBalancerLimitReached = "LOAD_BALANCER_LIMIT_REACHED"
	CodeLogsUnavailable          = "LOGS_UNAVAILABLE"
	CodeMembersLimitReached      = "MEMBERS_LIMIT_REACHED"
	CodeMemberAlreadyAdded       = "MEMBER_ALREADY_ADDED"
	CodeMemberNotFound           = "MEMBER_NOT_FOUND"
	CodeMissingParameters        = "MISSING_PARAMETERS"
	CodeMissingRequiredFields    = "MISSING_REQUIRED_FIELDS"
	CodeMissingScope             = "MISSING_SCOPE"
	CodeNoCustomDomain           = "NO_CUSTOM_DOMAIN"
	CodeNoUpdateData             = "NO_UPDATE_DATA"
	CodePayloadTooLarge          = "PAYLOAD_TOO_LARGE"
	CodePermissionDenied         = "PERMISSION_DENIED"
	CodePurgeCacheFailed         = "PURGE_CACHE_FAILED"
	// CodeRateLimited comes with 429 when the account, API key or IP address
	// went over its rate limit, which can block it for about 30 minutes. It
	// also limits the network calls and [AccountAPI.Snapshots]. The SDK never
	// retries it.
	CodeRateLimited                       = "RATE_LIMITED"
	CodeReadFailed                        = "READ_FAILED"
	CodeRealtimeMaxConnections            = "REALTIME_MAX_CONNECTIONS"
	CodeRealtimeMaxConnectionsApp         = "REALTIME_MAX_CONNECTIONS_APP"
	CodeRenameFailed                      = "RENAME_FAILED"
	CodeRepositoryBranchAlreadyConfigured = "REPOSITORY_BRANCH_ALREADY_CONFIGURED"
	CodeRepositoryNotAvailable            = "REPOSITORY_NOT_AVAILABLE"
	CodeRepositoryNotFound                = "REPOSITORY_NOT_FOUND"
	// CodeRepositoryPermissionRequired comes with 403 when your GitHub
	// account cannot write to the repository.
	CodeRepositoryPermissionRequired = "REPOSITORY_PERMISSION_REQUIRED"
	CodeRequestAborted               = "REQUEST_ABORTED"
	CodeReservedDomain               = "RESERVED_DOMAIN"
	CodeResetFailed                  = "RESET_FAILED"
	CodeResourceNotAllowed           = "RESOURCE_NOT_ALLOWED"
	// CodeRestoreInProgress comes with 409 while a snapshot is being restored
	// into the application or database. The SDK does not retry it.
	CodeRestoreInProgress  = "RESTORE_IN_PROGRESS"
	CodeRestoreInterrupted = "RESTORE_INTERRUPTED"
	CodeRouteNotFound      = "ROUTE_NOT_FOUND"
	CodeSaveFailed         = "SAVE_FAILED"
	CodeScopeNotGrantable  = "SCOPE_NOT_GRANTABLE"
	// CodeSnapshotsUnavailable comes with 503 when snapshot storage cannot
	// answer a listing. The SDK does not retry it.
	CodeSnapshotsUnavailable     = "SNAPSHOTS_UNAVAILABLE"
	CodeSnapshotCorrupt          = "SNAPSHOT_CORRUPT"
	CodeSnapshotDatabaseMismatch = "SNAPSHOT_DATABASE_MISMATCH"
	CodeSnapshotFailed           = "SNAPSHOT_FAILED"
	// CodeSnapshotInProgress comes with 409 from Restore while a snapshot of
	// the same application or database is being taken.
	CodeSnapshotInProgress = "SNAPSHOT_IN_PROGRESS"
	CodeSnapshotNotFound   = "SNAPSHOT_NOT_FOUND"
	// CodeSnapshotProcessing is never returned as an error. The SDK reports it
	// as [SnapshotCreated.Pending].
	CodeSnapshotProcessing    = "SNAPSHOT_PROCESSING"
	CodeSnapshotRestoreFailed = "SNAPSHOT_RESTORE_FAILED"
	// CodeSnapshotRestoreProcessing is never returned as an error. The SDK
	// reports it as [SnapshotRestored.Pending].
	CodeSnapshotRestoreProcessing = "SNAPSHOT_RESTORE_PROCESSING"
	CodeStaticAppEnvNotSupported  = "STATIC_APP_ENV_NOT_SUPPORTED"
	CodeStorageUploadFailed       = "STORAGE_UPLOAD_FAILED"
	CodeTooManyEnvVars            = "TOO_MANY_ENV_VARS"
	CodeUnableToFetchAnalytics    = "UNABLE_TO_FETCH_ANALYTICS"
	CodeUnableToFetchErrors       = "UNABLE_TO_FETCH_ERRORS"
	CodeUnableToFetchPerformance  = "UNABLE_TO_FETCH_PERFORMANCE"
	CodeUpgradeRequired           = "UPGRADE_REQUIRED"
	CodeUploadAborted             = "UPLOAD_ABORTED"
	// CodeUploadBusy comes with 503 when the API refused an upload before
	// doing any work. The SDK retries it.
	CodeUploadBusy              = "UPLOAD_BUSY"
	CodeUploadFailed            = "UPLOAD_FAILED"
	CodeValidationFailed        = "VALIDATION_FAILED"
	CodeValidationTimeout       = "VALIDATION_TIMEOUT"
	CodeWorkspaceCreationFailed = "WORKSPACE_CREATION_FAILED"
	CodeWorkspaceLimitReached   = "WORKSPACE_LIMIT_REACHED"
	CodeWorkspaceNotFound       = "WORKSPACE_NOT_FOUND"
	CodeWriteFailed             = "WRITE_FAILED"
	// CodeWrongHost comes with 421, but never in answer to the calls of this
	// SDK. It is listed so that every documented code has a constant.
	CodeWrongHost = "WRONG_HOST"
)
