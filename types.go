package squarecloud

import (
	"encoding/json"
	"time"
)

// Structs are hand-written against spec/openapi.json. Unknown JSON
// fields are ignored, so new API fields never break decoding. Nullable fields
// are pointers.

// ---- Account & service ----

// Account is GET /users/me: the user plus their applications and databases
// (lists are filtered to the key's resources for resource-restricted keys).
type Account struct {
	User         User              `json:"user"`
	Applications []AppSummary      `json:"applications"`
	Databases    []DatabaseSummary `json:"databases"`
}

// User is the account owner in Account.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Locale    string    `json:"locale"`
	Plan      Plan      `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

// Plan is the account's plan.
type Plan struct {
	Name   string     `json:"name"`
	Memory PlanMemory `json:"memory"`
	// Duration is the expiry as Unix ms; nil for plans that never expire.
	Duration *int64 `json:"duration"`
}

// PlanMemory values are in MB.
type PlanMemory struct {
	Limit     int `json:"limit"`
	Available int `json:"available"`
	Used      int `json:"used"`
}

// AppSummary is an application as listed in Account.
type AppSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc,omitempty"`
	RAM         int       `json:"ram"`
	Lang        string    `json:"lang"`
	Cluster     string    `json:"cluster"`
	Domain      *string   `json:"domain"`
	Custom      *string   `json:"custom"`
	CreatedAt   time.Time `json:"created_at"`
}

// DatabaseSummary is a database as listed in Account.
type DatabaseSummary struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	RAM       int          `json:"ram"`
	Type      DatabaseType `json:"type"`
	Cluster   string       `json:"cluster"`
	CreatedAt time.Time    `json:"created_at"`
}

// ServiceStatus is GET /service/status (public, not enveloped). Status is
// "online" or a degraded state. Services and Dependencies are keyed by slug
// (blob_storage, cubic, ai_gateway / cloudflare, github, discord, ...); the
// API omits entries and fields at runtime, hence maps.
type ServiceStatus struct {
	Status       string                  `json:"status"`
	Message      string                  `json:"message"`
	CheckedAt    time.Time               `json:"checked_at"`
	Stale        bool                    `json:"stale"`
	Services     map[string]ServiceEntry `json:"services"`
	Dependencies map[string]ServiceEntry `json:"dependencies"`
}

// ServiceEntry status is operational, degraded, outage, maintenance or unknown.
type ServiceEntry struct {
	Name                string      `json:"name"`
	Status              string      `json:"status"`
	Summary             string      `json:"summary,omitempty"`
	UnresolvedIncidents int         `json:"unresolved_incidents,omitempty"`
	DegradedComponents  int         `json:"degraded_components,omitempty"`
	APILatency          *APILatency `json:"api_latency,omitempty"`
}

// APILatency is a dependency's API latency in ServiceEntry.
type APILatency struct {
	RecentMs  float64 `json:"recent_ms"`
	DayMeanMs float64 `json:"day_mean_ms"`
	Elevated  bool    `json:"elevated"`
}

// ---- Applications ----

// App is GET /apps/{id}.
type App struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc,omitempty"`
	Owner       string    `json:"owner"`
	Cluster     string    `json:"cluster"`
	RAM         int       `json:"ram"`
	Language    string    `json:"language"`
	Domain      *string   `json:"domain"`
	Custom      *string   `json:"custom"`
	CreatedAt   time.Time `json:"created_at"`
}

// AppCreated is POST /apps. CPU is fractional-safe. Domain is the full host
// (e.g. "my-app.squareweb.app"), "" for an app without a subdomain.
type AppCreated struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Domain      string      `json:"domain,omitempty"`
	Cluster     string      `json:"cluster,omitempty"`
	RAM         int         `json:"ram"`
	CPU         float64     `json:"cpu"`
	Language    AppLanguage `json:"language"`
}

// AppLanguage is the runtime of AppCreated.
type AppLanguage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RuntimeStats is GET /apps/{id}/status and GET /databases/{id}/status with
// formatted strings ("0.5%", "120.4MB").
type RuntimeStats struct {
	CPU     string       `json:"cpu"`
	RAM     string       `json:"ram"`
	Status  string       `json:"status"`
	Running bool         `json:"running"`
	Storage string       `json:"storage"`
	Network StatsNetwork `json:"network"`
	// Uptime is the start time in Unix ms; nil when not running.
	Uptime *int64 `json:"uptime"`
}

// StatsNetwork is the formatted network traffic of RuntimeStats.
type StatsNetwork struct {
	Total string `json:"total"`
	Now   string `json:"now"`
}

// RuntimeStatsRaw is the ?rawData=true variant: CPU and RAM are numbers,
// Storage is bytes and Network holds [in, out] byte pairs.
type RuntimeStatsRaw struct {
	CPU     float64         `json:"cpu"`
	RAM     float64         `json:"ram"`
	Status  string          `json:"status"`
	Running bool            `json:"running"`
	Storage int64           `json:"storage"`
	Network StatsNetworkRaw `json:"network"`
	Uptime  *int64          `json:"uptime"`
}

// StatsNetworkRaw is the network traffic of RuntimeStatsRaw as [in, out] bytes.
type StatsNetworkRaw struct {
	Total [2]int64 `json:"total"`
	Now   [2]int64 `json:"now"`
}

// StatusListItem is one entry of GET /apps/status or /databases/status.
// CPU and RAM are set only while running.
type StatusListItem struct {
	ID      string `json:"id"`
	Running bool   `json:"running"`
	CPU     string `json:"cpu,omitempty"`
	RAM     string `json:"ram,omitempty"`
}

// MetricPoint is one 5-minute sample of the last 24h. The API lists them
// newest first.
type MetricPoint struct {
	Date time.Time `json:"date"`
	CPU  float64   `json:"cpu"`
	RAM  float64   `json:"ram"`
	Net  []float64 `json:"net"`
}

// EnvVars maps variable name to value.
type EnvVars map[string]string

// FileEntry is one entry of GET /apps/{id}/files.
type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "directory"
	// Size is 0 on directory entries the API sends without it (e.g. a
	// directory a commit just unpacked).
	Size int64 `json:"size"`
	// LastModified is Unix ms, fractional for some files (e.g.
	// 1790520641499.565); nil when the API sends null or omits it.
	LastModified *float64 `json:"lastModified"`
}

// Values of DeployEvent.State.
const (
	DeployPending    = "pending"
	DeployClone      = "clone"
	DeployCommit     = "commit"
	DeployRestarting = "restarting"
	DeploySuccess    = "success"
	DeployError      = "error"
)

// DeployEvent is one step of a deploy timeline (GET /apps/{id}/deployments
// returns one []DeployEvent per deploy). Branch is set on "clone" events, Files
// on "commit" events. An "error" event carries Code (why the deploy failed,
// e.g. CLONE_FAILED) and sometimes Message. Source is "git".
type DeployEvent struct {
	ID      string       `json:"id"`
	State   string       `json:"state"`
	Date    time.Time    `json:"date"`
	Source  string       `json:"source"`
	Branch  string       `json:"branch,omitempty"`
	Code    string       `json:"code,omitempty"`
	Message string       `json:"message,omitempty"`
	Files   *DeployFiles `json:"files,omitempty"`
}

// DeployFiles lists the files a "commit" DeployEvent changed.
type DeployFiles struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

// DeployCurrent is GET /apps/{id}/deployments/current. App is nil when no
// GitHub App repository is linked.
type DeployCurrent struct {
	App     *DeployRepository `json:"app,omitempty"`
	Webhook string            `json:"webhook,omitempty"`
}

// LinkedRepository is the repository Deploys.LinkGithubApp linked.
type LinkedRepository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Branch   string `json:"branch"`
}

// DeployRepository is the GitHub App repository of DeployCurrent.
type DeployRepository struct {
	ID     *int64  `json:"id"`
	Name   *string `json:"name"`
	Branch *string `json:"branch"`
}

// Snapshot is one entry of the snapshot listings. Restore takes Name and
// VersionID.
type Snapshot struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	// Key is the signed query string of this snapshot version.
	Key string `json:"key"`
	// VersionID is the storage version of this snapshot.
	VersionID string `json:"version_id"`
	// URL is a signed download link, valid for 30 days (see
	// Client.DownloadSnapshot).
	URL string `json:"url"`
	// Runtime is the app language or database engine; nil on old snapshots.
	Runtime *string `json:"runtime"`
	// Origin is "automatic" or "manual"; nil on old snapshots.
	Origin *string `json:"origin"`
}

// SnapshotCreated is POST .../snapshots. When the API answers 202
// SNAPSHOT_PROCESSING the snapshot is still being generated: Pending is true,
// URL and Key are empty, and it is not an error.
type SnapshotCreated struct {
	URL     string `json:"url"`
	Key     string `json:"key"`
	Pending bool   `json:"-"`
}

// SnapshotScope selects Account.Snapshots' domain.
type SnapshotScope string

// SnapshotScope values.
const (
	SnapshotScopeApplications SnapshotScope = "applications"
	SnapshotScopeDatabases    SnapshotScope = "databases"
)

// ---- Network ----

// AppDomain is one hostname of Apps.Domains.
type AppDomain struct {
	AppID    string `json:"app_id"`
	Hostname string `json:"hostname"`
	Type     string `json:"type"` // "subdomain" or "custom"
}

// LoadBalancers is GET /apps/load-balancers.
type LoadBalancers struct {
	Limit     int            `json:"limit"`
	Balancers []LoadBalancer `json:"balancers"`
}

// LoadBalancer is one hostname balanced across Apps.
type LoadBalancer struct {
	Hostname string            `json:"hostname"`
	Apps     []LoadBalancerApp `json:"apps"`
}

// LoadBalancerApp is one application behind a LoadBalancer.
type LoadBalancerApp struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Cluster string `json:"cluster,omitempty"`
}

// DNSRecord is one record to configure for a custom domain.
type DNSRecord struct {
	Type   string `json:"type"` // "txt" or "cname"
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

// AnalyticsFilters are the optional drill-down filters of Network.Analytics
// ("" for none). A value the API rejects is 400 INVALID_FILTER.
type AnalyticsFilters struct {
	Country     string // 2-letter code
	IP          string
	Path        string // prefix
	Status      string // e.g. "404"
	OS          string
	Browser     string
	Protocol    string
	Referer     string
	Provider    string // "NAME (ASN)" as in NetworkAnalytics.Providers, e.g. "GOOGLE (15169)"
	ContentType string
	Bot         string
}

// NetworkAnalytics is GET /apps/{id}/network/analytics.
type NetworkAnalytics struct {
	Visits       []AnalyticsTimeBucket  `json:"visits"`
	Countries    []AnalyticsBucket      `json:"countries"`
	Devices      []AnalyticsBucket      `json:"devices"`
	OS           []AnalyticsBucket      `json:"os"`
	Browsers     []AnalyticsBucket      `json:"browsers"`
	Protocols    []AnalyticsBucket      `json:"protocols"`
	Methods      []AnalyticsBucket      `json:"methods"`
	Paths        []AnalyticsBucket      `json:"paths"`
	Referers     []AnalyticsBucket      `json:"referers"`
	Providers    []AnalyticsBucket      `json:"providers"`
	IPs          []AnalyticsTotalBucket `json:"ips"`
	StatusCodes  []AnalyticsTotalBucket `json:"status_codes"`
	Bots         []AnalyticsTotalBucket `json:"bots"`
	ContentTypes []AnalyticsTotalBucket `json:"content_types"`
}

// AnalyticsTimeBucket is one time slot of NetworkAnalytics.Visits.
type AnalyticsTimeBucket struct {
	Date     time.Time `json:"date"`
	Visits   int64     `json:"visits"`
	Requests int64     `json:"requests"`
	Bytes    int64     `json:"bytes"`
}

// AnalyticsBucket is one value (Type) of a NetworkAnalytics breakdown at Date.
// A provider's Type reads "NAME (ASN)".
type AnalyticsBucket struct {
	Type     string    `json:"type"`
	Date     time.Time `json:"date"`
	Visits   int64     `json:"visits"`
	Requests int64     `json:"requests"`
	Bytes    int64     `json:"bytes"`
}

// AnalyticsTotalBucket is one value (Type) of a NetworkAnalytics breakdown
// over the whole window.
type AnalyticsTotalBucket struct {
	Type     string `json:"type"`
	Visits   int64  `json:"visits"`
	Requests int64  `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

// NetworkErrors is GET /apps/{id}/network/errors.
type NetworkErrors struct {
	Summary    NetworkErrorsSummary  `json:"summary"`
	ByStatus   []NetworkErrorsStatus `json:"by_status"`
	Timeseries []NetworkErrorsBucket `json:"timeseries"`
	TopPaths   []NetworkErrorsPath   `json:"top_paths"`
	ByMethod   []NetworkErrorsMethod `json:"by_method"`
}

// NetworkErrorsSummary is the error total, also by class.
type NetworkErrorsSummary struct {
	Total   int64            `json:"total"`
	ByClass map[string]int64 `json:"by_class"` // "4xx", "5xx"
}

// NetworkErrorsStatus is the request count of one status code.
type NetworkErrorsStatus struct {
	Status   int   `json:"status"`
	Requests int64 `json:"requests"`
}

// NetworkErrorsBucket is one time slot of NetworkErrors.Timeseries.
type NetworkErrorsBucket struct {
	Date    time.Time        `json:"date"`
	Buckets map[string]int64 `json:"buckets"` // status code -> requests
	Total   int64            `json:"total"`
}

// NetworkErrorsPath is one of the paths with the most errors.
type NetworkErrorsPath struct {
	Path     string           `json:"path"`
	Method   *string          `json:"method"`
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"by_status"`
}

// NetworkErrorsMethod is the errors of one HTTP method.
type NetworkErrorsMethod struct {
	Method   *string          `json:"method"`
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"by_status"`
}

// NetworkLog is one edge request of Network.Logs.
type NetworkLog struct {
	Timestamp time.Time          `json:"timestamp"`
	Client    NetworkLogClient   `json:"client"`
	Request   NetworkLogRequest  `json:"request"`
	Response  NetworkLogResponse `json:"response"`
}

// NetworkLogClient is the client of a NetworkLog.
type NetworkLogClient struct {
	IP       *string `json:"ip"`
	Country  *string `json:"country"`
	Location *string `json:"location"`
	ASN      string  `json:"asn"`
	Agent    *string `json:"agent"`
	Category *string `json:"category"`
}

// NetworkLogRequest is the request of a NetworkLog.
type NetworkLogRequest struct {
	Mitigated bool    `json:"mitigated"`
	Method    string  `json:"method"`
	Host      string  `json:"host"`
	Path      string  `json:"path"`
	Query     *string `json:"query"`
	Protocol  string  `json:"protocol"`
	Referer   *string `json:"referer"`
}

// NetworkLogResponse is the response of a NetworkLog.
type NetworkLogResponse struct {
	Status      int     `json:"status"`
	ContentType *string `json:"contentType"`
	Cache       *string `json:"cache"`
}

// Percentiles are latencies in milliseconds; nil when the window had no
// requests (the API sends null).
type Percentiles struct {
	P50 *float64 `json:"p50"`
	P95 *float64 `json:"p95"`
	P99 *float64 `json:"p99"`
}

// NetworkPerformance is GET /apps/{id}/network/performance.
type NetworkPerformance struct {
	Summary      PerformanceSummary  `json:"summary"`
	Timeseries   []PerformanceBucket `json:"timeseries"`
	Countries    []PerformanceRegion `json:"countries"`
	Colos        []PerformanceRegion `json:"colos"`
	SlowestPaths []PerformancePath   `json:"slowest_paths"`
}

// PerformanceSummary is the latency over the whole window.
type PerformanceSummary struct {
	Edge     Percentiles `json:"edge"`
	Origin   Percentiles `json:"origin"`
	Requests int64       `json:"requests"`
}

// PerformanceBucket is one time slot of NetworkPerformance.Timeseries.
type PerformanceBucket struct {
	Date     time.Time   `json:"date"`
	Requests int64       `json:"requests"`
	Edge     Percentiles `json:"edge"`
	Origin   Percentiles `json:"origin"`
}

// PerformanceRegion is a country (Type = country code) or a colo (Type =
// colo code, with City and Country). Latencies are milliseconds; nil fields
// are null in the API.
type PerformanceRegion struct {
	Type     string   `json:"type"`
	City     *string  `json:"city,omitempty"`
	Country  *string  `json:"country,omitempty"`
	P50      *float64 `json:"p50"`
	P95      *float64 `json:"p95"`
	Requests int64    `json:"requests"`
}

// PerformancePath is one of the slowest paths; latencies as in
// PerformanceRegion.
type PerformancePath struct {
	Path     string   `json:"path"`
	P95      *float64 `json:"p95"`
	P99      *float64 `json:"p99"`
	Requests int64    `json:"requests"`
}

// ---- Databases ----

// DatabaseType is a database engine.
type DatabaseType string

// DatabaseType values.
const (
	DatabaseMongo    DatabaseType = "mongo"
	DatabaseMySQL    DatabaseType = "mysql"
	DatabaseRedis    DatabaseType = "redis"
	DatabasePostgres DatabaseType = "postgres"
)

// Database is GET /databases/{id}.
type Database struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Owner     string       `json:"owner"`
	Cluster   string       `json:"cluster"`
	RAM       int          `json:"ram"`
	Type      DatabaseType `json:"type"`
	Port      int          `json:"port"`
	CreatedAt time.Time    `json:"created_at"`
}

// DatabaseCreate is the POST /databases body. Memory is an integer number of
// MB (a fraction or a string is 400 INVALID_MEMORY; redis needs at least 512,
// the other engines 1024). Version accepts a full version key or a
// major/minor prefix such as "8".
type DatabaseCreate struct {
	Name    string       `json:"name"`
	Memory  int          `json:"memory"`
	Type    DatabaseType `json:"type"`
	Version string       `json:"version"`
}

// DatabaseUpdate is the PATCH /databases/{id} body; zero fields are omitted.
type DatabaseUpdate struct {
	Name string `json:"name,omitempty"`
	RAM  int    `json:"ram,omitempty"`
}

// DatabaseCreated is POST /databases. Password, Certificate and ConnectionURL
// are shown only once. CPU is fractional (e.g. 0.5). Certificate is base64
// PEM, nil when the API sends none.
type DatabaseCreated struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Memory        int          `json:"memory"`
	CPU           float64      `json:"cpu"`
	Type          DatabaseType `json:"type"`
	Password      string       `json:"password"`
	Certificate   *string      `json:"certificate,omitempty"`
	ConnectionURL string       `json:"connection_url"`
	Cluster       string       `json:"cluster"`
}

// DatabaseReset selects the credential ResetCredentials rotates.
type DatabaseReset string

// DatabaseReset values.
const (
	ResetPassword    DatabaseReset = "password"
	ResetCertificate DatabaseReset = "certificate"
)

// ---- Workspaces ----

// Workspace is a workspace with its members and shared applications. ID is
// 32 or 40 lowercase hex characters.
type Workspace struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Owner        string            `json:"owner"`
	Members      []WorkspaceMember `json:"members"`
	Applications []WorkspaceApp    `json:"applications"`
	CreatedAt    time.Time         `json:"createdAt"`
}

// WorkspaceMember is a member of a Workspace. Group is "owner" or one of
// the WorkspaceGroup values. Name is nil when the API sends null.
type WorkspaceMember struct {
	ID       string    `json:"id"`
	Name     *string   `json:"name"`
	Group    string    `json:"group"`
	JoinedAt time.Time `json:"joinedAt"`
}

// WorkspaceApp is an application shared in a Workspace.
type WorkspaceApp struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"desc"`
	RAM         int     `json:"ram"`
	Lang        string  `json:"lang"`
	Domain      *string `json:"domain"`
	Custom      *string `json:"custom"`
}

// WorkspaceCreated is POST /workspaces.
type WorkspaceCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkspaceGroup is a member tier accepted by Members.Add/Update ("owner" is
// output-only).
type WorkspaceGroup string

// WorkspaceGroup values.
const (
	GroupAdmin    WorkspaceGroup = "admin"
	GroupMaintain WorkspaceGroup = "maintain"
	GroupManager  WorkspaceGroup = "manager"
	GroupView     WorkspaceGroup = "view"
)

// ---- AI ----

// ChatRequest is the OpenAI-compatible POST /ai/chat/completions body.
// Streaming is not supported by the API. Model is ignored (always "cubic").
type ChatRequest struct {
	Model       string            `json:"model,omitempty"`
	Messages    []ChatMessage     `json:"messages"`
	Tools       []json.RawMessage `json:"tools,omitempty"`
	ToolChoice  any               `json:"tool_choice,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
}

// ChatMessage role is system, user, assistant or tool.
type ChatMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"` // always sent: the API requires a string, "" included
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []json.RawMessage `json:"tool_calls,omitempty"`
}

// ChatCompletion is the OpenAI-compatible AI.Chat response.
type ChatCompletion struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// ChatChoice is one completion of ChatCompletion.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatUsage is the token count of a ChatCompletion.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
