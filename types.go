package squarecloud

import (
	"encoding/json"
	"time"
)

// The structs below are written by hand and checked against spec/openapi.json
// by the conformance tests. Unknown JSON fields are ignored, so new API fields
// never break decoding. Fields the API can send as null are pointers.

// ---- Account & service ----

// Account is what [AccountAPI.Me] returns: your user and the applications and
// databases you own. With an API key limited to some resources, the lists
// hold only those.
type Account struct {
	User         User              `json:"user"`
	Applications []AppSummary      `json:"applications"`
	Databases    []DatabaseSummary `json:"databases"`
}

// User is the owner of the API key.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	// Locale is the user's preferred language, such as "en-US".
	Locale    string    `json:"locale"`
	Plan      Plan      `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

// Plan is the account's plan.
type Plan struct {
	// Name is the plan, such as "standard-8". "free" means that the account
	// has no active plan.
	Name string `json:"name"`
	// Memory is the memory of the plan and how much of it is in use.
	Memory PlanMemory `json:"memory"`
	// Duration is when the plan expires, in Unix milliseconds. It is nil when
	// the account has no active plan.
	Duration *int64 `json:"duration"`
}

// PlanMemory is the memory of a [Plan] in MB. Limit is the total, Available
// is what is left, and Used is what your applications and databases take.
type PlanMemory struct {
	Limit     int `json:"limit"`
	Available int `json:"available"`
	Used      int `json:"used"`
}

// AppSummary is an application as [Account] lists it.
type AppSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Description is "" when the application has none.
	Description string `json:"desc,omitempty"`
	// RAM is the memory allocated to the application, in MB.
	RAM int `json:"ram"`
	// Lang is the application's language, such as "javascript".
	Lang string `json:"lang"`
	// Cluster identifies where the application is hosted.
	Cluster string `json:"cluster"`
	// Domain is the default hostname, or nil when the application has none.
	Domain *string `json:"domain"`
	// Custom is the custom domain, or nil when none is set.
	Custom    *string   `json:"custom"`
	CreatedAt time.Time `json:"created_at"`
}

// DatabaseSummary is a database as [Account] lists it.
type DatabaseSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// RAM is the memory allocated to the database, in MB.
	RAM  int          `json:"ram"`
	Type DatabaseType `json:"type"`
	// Cluster identifies where the database is hosted.
	Cluster   string    `json:"cluster"`
	CreatedAt time.Time `json:"created_at"`
}

// ServiceStatus is the platform status that [ServiceAPI.Status] returns.
type ServiceStatus struct {
	// Status is the overall state of Square Cloud: "online", "maintenance",
	// "degraded" or "unknown". "unknown" means that the check itself could not
	// run, not that the platform is down.
	Status string `json:"status"`
	// Message describes the overall state.
	Message string `json:"message"`
	// CheckedAt is when the status was checked.
	CheckedAt time.Time `json:"checked_at"`
	// Stale is true when the regular status report was unavailable and the
	// status was checked on the spot instead.
	Stale bool `json:"stale"`
	// Services holds the state of each Square Cloud service, keyed by a name
	// such as "blob_storage". The API can leave out services and fields.
	Services map[string]ServiceEntry `json:"services"`
	// Dependencies holds the state of each outside service that the platform
	// relies on, keyed by a name such as "github". An incident there does not
	// change Status.
	Dependencies map[string]ServiceEntry `json:"dependencies"`
	// Incidents lists the open incidents and maintenance windows, newest
	// first.
	Incidents []ServiceIncident `json:"incidents,omitempty"`
	// Components lists the parts of the platform that are not fully working.
	Components []ServiceComponent `json:"components,omitempty"`
}

// ServiceIncident is an open incident or maintenance window in
// [ServiceStatus].
type ServiceIncident struct {
	// Type is "maintenance" or "incident".
	Type  string `json:"type"`
	Title string `json:"title"`
	// Status is "maintenance", "degraded", "outage", "operational" or
	// "unknown".
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	// EndsAt is when the maintenance is expected to end, or nil when no end
	// was announced.
	EndsAt *time.Time `json:"ends_at"`
	// Affected names the affected components.
	Affected []string `json:"affected"`
	// Update is the latest update as written on the status page. It can use
	// Markdown.
	Update    string     `json:"update,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// ServiceComponent is a part of the platform that is not fully working.
// Status is "maintenance", "degraded" or "outage".
type ServiceComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ServiceEntry is the state of one service in [ServiceStatus].
type ServiceEntry struct {
	Name string `json:"name"`
	// Status is "operational", "degraded", "outage", "maintenance" or
	// "unknown". "unknown" means that the state could not be confirmed, not
	// that the service is down.
	Status string `json:"status"`
	// Summary says what is wrong and what it affects. It is "" while the
	// service is operational.
	Summary string `json:"summary,omitempty"`
	// UnresolvedIncidents is the number of open incidents.
	UnresolvedIncidents int `json:"unresolved_incidents,omitempty"`
	// DegradedComponents is the number of components that are not fully
	// working.
	DegradedComponents int `json:"degraded_components,omitempty"`
	// APILatency is the response time of the service's API, or nil when it is
	// not measured.
	APILatency *APILatency `json:"api_latency,omitempty"`
}

// APILatency is the response time of a service's API in milliseconds.
// RecentMs is the latest measurement and DayMeanMs the average over the last
// day. Elevated is true when the latency is higher than usual.
type APILatency struct {
	RecentMs  float64 `json:"recent_ms"`
	DayMeanMs float64 `json:"day_mean_ms"`
	Elevated  bool    `json:"elevated"`
}

// ---- Applications ----

// App is an application as [AppsAPI.Get] returns it.
type App struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Description is "" when the application has none.
	Description string `json:"desc,omitempty"`
	// Owner is the user id of the application's owner.
	Owner string `json:"owner"`
	// Cluster identifies where the application is hosted.
	Cluster string `json:"cluster"`
	// RAM is the memory allocated to the application, in MB.
	RAM int `json:"ram"`
	// Language is the application's language, such as "javascript",
	// "python" or "static".
	Language string `json:"language"`
	// Domain is the default hostname, such as "my-app.squareweb.app", or nil
	// when the application has none.
	Domain *string `json:"domain"`
	// Custom is the custom domain, or nil when none is set.
	Custom    *string   `json:"custom"`
	CreatedAt time.Time `json:"created_at"`
}

// AppCreated is the application that [AppsAPI.Create] deployed.
type AppCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Description is "" when the application has none.
	Description string `json:"description,omitempty"`
	// Domain is the full default hostname, such as "my-app.squareweb.app", or
	// "" when the application has no subdomain.
	Domain string `json:"domain,omitempty"`
	// Cluster identifies where the application is hosted.
	Cluster string `json:"cluster,omitempty"`
	// RAM is the memory allocated to the application, in MB.
	RAM int `json:"ram"`
	// CPU is the CPU allocated to the application. It can be a fraction, such
	// as 0.5.
	CPU float64 `json:"cpu"`
	// Language is the application's language and its version.
	Language AppLanguage `json:"language"`
}

// AppLanguage is the language of a new application and its version.
type AppLanguage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RuntimeStats is the current usage of an application or database as
// formatted strings. [AppsAPI.Status] and [DatabasesAPI.Status] return it.
type RuntimeStats struct {
	// CPU is the CPU usage, such as "0.5%".
	CPU string `json:"cpu"`
	// RAM is the memory in use, such as "120.4MB".
	RAM string `json:"ram"`
	// Status is the lifecycle state, such as "running" or "exited".
	Status string `json:"status"`
	// Running is true when Status is "running".
	Running bool `json:"running"`
	// Restoring is true while a snapshot restore is queued or running. Status
	// keeps its own value meanwhile, which is "exited" while the restore holds
	// the application or database stopped.
	Restoring bool `json:"restoring,omitempty"`
	// Storage is the disk space in use.
	Storage string `json:"storage"`
	// Network is the network traffic.
	Network StatsNetwork `json:"network"`
	// Uptime is when the application or database last started, in Unix
	// milliseconds. It is nil when it is not running.
	Uptime *int64 `json:"uptime"`
}

// StatsNetwork is the network traffic of [RuntimeStats]. Each field holds
// the incoming and outgoing traffic in one formatted string. Total is the
// overall traffic and Now the current traffic.
type StatsNetwork struct {
	Total string `json:"total"`
	Now   string `json:"now"`
}

// RuntimeStatsRaw is the current usage of an application or database as
// numbers. [AppsAPI.StatusRaw] and [DatabasesAPI.StatusRaw] return it. Fields
// without a comment mean the same as in [RuntimeStats].
type RuntimeStatsRaw struct {
	// CPU is the CPU usage in percent.
	CPU float64 `json:"cpu"`
	// RAM is the memory in use, in MB.
	RAM       float64 `json:"ram"`
	Status    string  `json:"status"`
	Running   bool    `json:"running"`
	Restoring bool    `json:"restoring,omitempty"`
	// Storage is the disk space in use, in bytes.
	Storage int64 `json:"storage"`
	// Network is the network traffic in bytes.
	Network StatsNetworkRaw `json:"network"`
	Uptime  *int64          `json:"uptime"`
}

// StatsNetworkRaw is the network traffic of [RuntimeStatsRaw]. Each pair
// holds the bytes received and the bytes sent. Total is the overall traffic
// and Now the current traffic.
type StatsNetworkRaw struct {
	Total [2]int64 `json:"total"`
	Now   [2]int64 `json:"now"`
}

// StatusListItem is the state of one application or database in
// [AppsAPI.StatusAll] or [DatabasesAPI.StatusAll]. CPU and RAM are formatted
// strings such as "0.00%" and "0.00MB". They are set only while it is
// running.
type StatusListItem struct {
	ID      string `json:"id"`
	Running bool   `json:"running"`
	CPU     string `json:"cpu,omitempty"`
	RAM     string `json:"ram,omitempty"`
}

// MetricPoint is one point of [AppsAPI.Metrics] or [DatabasesAPI.Metrics].
type MetricPoint struct {
	Date time.Time `json:"date"`
	// CPU is the CPU usage, in percent of the CPU your plan allocates. On "7d"
	// points it is the hour's average.
	CPU float64 `json:"cpu"`
	// RAM is the memory in use, in MB. On "7d" points it is the hour's
	// average.
	RAM float64 `json:"ram"`
	// Net holds the bytes received and sent since the application or database
	// last started. A value lower than on the previous point means that it
	// restarted.
	Net []float64 `json:"net"`
	// Disk holds the bytes read and written, counted the same way as Net. It
	// is nil on a point without a reading.
	Disk []int64 `json:"disk"`
	// Storage is the disk space in use, in bytes. It is nil on a point
	// without a reading.
	Storage *int64 `json:"storage"`
	// CPUMax is the hour's highest CPU usage. It is set only on "7d" points.
	CPUMax *float64 `json:"cpu_max,omitempty"`
	// RAMMax is the hour's highest memory use. It is set only on "7d" points.
	RAMMax *float64 `json:"ram_max,omitempty"`
}

// EnvVars maps environment variable names to their values.
type EnvVars map[string]string

// FileEntry is one entry of [AppFilesAPI.List].
type FileEntry struct {
	// Name is the name of the file or directory, without its path.
	Name string `json:"name"`
	// Type is "file" or "directory".
	Type string `json:"type"`
	// Size is the size in bytes. It is 0 for directories.
	Size int64 `json:"size"`
	// LastModified is when the entry last changed, in Unix milliseconds. It
	// can have a fractional part, such as 1790520641499.565, and is nil when
	// the API could not read it.
	LastModified *float64 `json:"lastModified"`
}

// Values of [DeployEvent.State], in the order a deploy goes through them.
// [DeployError] can replace any step after [DeployPending].
const (
	DeployPending    = "pending"    // The deploy is waiting to start.
	DeployClone      = "clone"      // The repository is being cloned.
	DeployCommit     = "commit"     // The changed files are being applied.
	DeployRestarting = "restarting" // The application is restarting.
	DeploySuccess    = "success"    // The deploy finished.
	DeployError      = "error"      // The deploy failed.
)

// DeployEvent is one step of a deploy. [AppDeploysAPI.List] returns one list
// of events per deploy.
type DeployEvent struct {
	// ID is the SHA of the commit being deployed.
	ID string `json:"id"`
	// State is the step, one of the Deploy constants such as [DeploySuccess].
	State string    `json:"state"`
	Date  time.Time `json:"date"`
	// Source is always "git", because only deploys from Git are listed.
	Source string `json:"source"`
	// Branch is the branch being deployed. It is set on "clone" events.
	Branch string `json:"branch,omitempty"`
	// Code says why the deploy failed, such as "CLONE_FAILED". It is set on
	// "error" events.
	Code string `json:"code,omitempty"`
	// Message gives the details of the failure on some "error" events.
	Message string `json:"message,omitempty"`
	// Files lists the changed files on "commit" events.
	Files *DeployFiles `json:"files,omitempty"`
}

// DeployFiles lists the files that a deploy added, removed and modified.
type DeployFiles struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

// DeployCurrent is the deploy setup that [AppDeploysAPI.Current] returns.
type DeployCurrent struct {
	// App is the repository linked through the GitHub App, or nil when none
	// is linked.
	App *DeployRepository `json:"app,omitempty"`
	// Webhook is the deploy webhook URL, or "" when no webhook is set. It is
	// masked unless your API key may configure the webhook, which needs the
	// apps:deploy scope.
	Webhook string `json:"webhook,omitempty"`
}

// LinkedRepository is the repository that [AppDeploysAPI.LinkGithubApp]
// linked.
type LinkedRepository struct {
	// ID is the GitHub id of the repository.
	ID int64 `json:"id"`
	// FullName is the repository written as "owner/name".
	FullName string `json:"full_name"`
	Branch   string `json:"branch"`
}

// DeployRepository is the repository linked through the GitHub App, as
// [DeployCurrent] reports it. A field is nil when the API does not send it.
type DeployRepository struct {
	// ID is the GitHub id of the repository.
	ID     *int64  `json:"id"`
	Name   *string `json:"name"`
	Branch *string `json:"branch"`
}

// Snapshot is one snapshot in a listing. Pass Name and VersionID to
// [SnapshotsAPI.Restore], and URL to [Client.DownloadSnapshot].
type Snapshot struct {
	// Name identifies the snapshot.
	Name string `json:"name"`
	// Size is the size of the archive, in bytes.
	Size int64 `json:"size"`
	// Modified is when the snapshot was taken.
	Modified time.Time `json:"modified"`
	// Key is the signed part of URL.
	Key string `json:"key"`
	// VersionID identifies this version of the snapshot.
	VersionID string `json:"version_id"`
	// URL is the download link. It is valid for 1 hour.
	URL string `json:"url"`
	// Runtime is what the archive holds: the application's language, such as
	// "python", or the database engine, such as "postgres". It is nil on old
	// snapshots.
	Runtime *string `json:"runtime"`
	// Origin is "automatic" for the daily snapshot or "manual" for one that
	// was requested. It is nil on old snapshots.
	Origin *string `json:"origin"`
}

// SnapshotCreated is the result of [SnapshotsAPI.Create].
type SnapshotCreated struct {
	// URL is the download link, valid for 1 hour. It is "" while Pending is
	// true, and also when the API key lacks the snapshots:read scope.
	URL string `json:"url"`
	// Key is the signed part of URL. It is "" in the same cases as URL.
	Key string `json:"key"`
	// Pending is true when the snapshot is still being taken, which is not an
	// error. It appears in [SnapshotsAPI.List] within about 2 minutes.
	Pending bool `json:"-"`
}

// SnapshotRestored is the result of [SnapshotsAPI.Restore].
type SnapshotRestored struct {
	// Pending is true when the restore is still running after the call
	// returned, which is not an error. Follow it with
	// [SnapshotsAPI.RestoreState].
	Pending bool `json:"-"`
}

// SnapshotRestoreState is the progress of the last restore, as
// [SnapshotsAPI.RestoreState] reports it.
type SnapshotRestoreState struct {
	// State is "queued" or "running" while the restore runs, "success" or
	// "error" for 1 hour after it ends, and "none" otherwise.
	State string `json:"state"`
	// Step is the current step while the restore runs, and "done" once it
	// ends.
	Step string `json:"step,omitempty"`
	// Code says why the restore failed when State is "error".
	Code string `json:"code,omitempty"`
	// StartedAt is when the restore started, in Unix milliseconds, or nil when
	// the API does not report it.
	StartedAt *int64 `json:"started_at,omitempty"`
	// FinishedAt is when the restore ended, in Unix milliseconds, or nil when
	// the API does not report it.
	FinishedAt *int64 `json:"finished_at,omitempty"`
}

// SnapshotScope selects whether [AccountAPI.Snapshots] lists the snapshots
// of applications or of databases.
type SnapshotScope string

// SnapshotScope values.
const (
	SnapshotScopeApplications SnapshotScope = "applications"
	SnapshotScopeDatabases    SnapshotScope = "databases"
)

// ---- Network ----

// AppDomain is one hostname in [AppsAPI.Domains].
type AppDomain struct {
	// AppID is the application that the hostname points to.
	AppID    string `json:"app_id"`
	Hostname string `json:"hostname"`
	// Type is "subdomain" for a default hostname or "custom" for a custom
	// domain.
	Type string `json:"type"`
}

// LoadBalancers is the result of [AppsAPI.LoadBalancers].
type LoadBalancers struct {
	// Limit is how many applications can share one custom domain on your
	// plan.
	Limit int `json:"limit"`
	// Balancers holds one entry for each custom domain.
	Balancers []LoadBalancer `json:"balancers"`
}

// LoadBalancer is a custom domain and the applications that serve it. When
// there are two or more, the traffic is spread across them.
type LoadBalancer struct {
	Hostname string            `json:"hostname"`
	Apps     []LoadBalancerApp `json:"apps"`
}

// LoadBalancerApp is one application behind a [LoadBalancer].
type LoadBalancerApp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Cluster identifies where the application is hosted.
	Cluster string `json:"cluster,omitempty"`
}

// DNSRecord is one record to create at your DNS provider for a custom domain.
type DNSRecord struct {
	// Type is "txt" or "cname".
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
	// Status is the validation state, such as "pending" or "active".
	Status string `json:"status"`
	// Errors holds up to 3 messages that say why a record is not active yet.
	Errors []string `json:"errors,omitempty"`
	// Fallback is true on the one _acme-challenge CNAME record that can
	// replace every SSL TXT record. Create either that CNAME or the TXT
	// records, never both.
	Fallback bool `json:"fallback,omitempty"`
}

// AnalyticsFilters narrows the results of [AppNetworkAPI.Analytics]. Leave a
// field "" to skip that filter. A value the API does not accept returns 400
// INVALID_FILTER.
type AnalyticsFilters struct {
	// Country is a two-letter country code, such as "BR".
	Country string
	IP      string
	// Path keeps the requests whose path starts with this value.
	Path string
	// Status is an HTTP status code, such as "404".
	Status   string
	OS       string
	Browser  string
	Protocol string
	Referer  string
	// Provider is a network provider written as in
	// [NetworkAnalytics.Providers], such as "GOOGLE (15169)".
	Provider    string
	ContentType string
	Bot         string
}

// NetworkAnalytics is the traffic summary that [AppNetworkAPI.Analytics]
// returns. Visits is the traffic over time, and every other field breaks the
// traffic down by one dimension.
type NetworkAnalytics struct {
	Visits    []AnalyticsTimeBucket `json:"visits"`
	Countries []AnalyticsBucket     `json:"countries"`
	Devices   []AnalyticsBucket     `json:"devices"`
	OS        []AnalyticsBucket     `json:"os"`
	Browsers  []AnalyticsBucket     `json:"browsers"`
	Protocols []AnalyticsBucket     `json:"protocols"`
	Methods   []AnalyticsBucket     `json:"methods"`
	Paths     []AnalyticsBucket     `json:"paths"`
	// Referers uses "Direct" for requests without a referer.
	Referers []AnalyticsBucket `json:"referers"`
	// Providers breaks the traffic down by network provider, written as
	// "NAME (ASN)".
	Providers []AnalyticsBucket `json:"providers"`
	// IPs lists the client IP addresses with the most requests.
	IPs []AnalyticsTotalBucket `json:"ips"`
	// StatusCodes counts every response status, not only errors.
	StatusCodes []AnalyticsTotalBucket `json:"status_codes"`
	// Bots uses "Unverified" for traffic that is not from a verified bot.
	Bots         []AnalyticsTotalBucket `json:"bots"`
	ContentTypes []AnalyticsTotalBucket `json:"content_types"`
}

// AnalyticsTimeBucket is the traffic of one 15-minute slot in
// [NetworkAnalytics.Visits]. Date is the start of the slot, Visits counts
// unique visitors, Bytes is what was served to them, and BytesIn is what was
// received from them, such as uploads.
type AnalyticsTimeBucket struct {
	Date     time.Time `json:"date"`
	Visits   int64     `json:"visits"`
	Requests int64     `json:"requests"`
	Bytes    int64     `json:"bytes"`
	BytesIn  int64     `json:"bytes_in,omitempty"`
}

// AnalyticsBucket is the traffic of one value of a breakdown, such as one
// country, in the 15-minute slot that starts at Date. Type is the value.
// Visits counts unique visitors and Bytes is what was served to them.
type AnalyticsBucket struct {
	Type     string    `json:"type"`
	Date     time.Time `json:"date"`
	Visits   int64     `json:"visits"`
	Requests int64     `json:"requests"`
	Bytes    int64     `json:"bytes"`
}

// AnalyticsTotalBucket is the traffic of one value of a breakdown over the
// whole period. Type is the value. Visits counts unique visitors and Bytes is
// what was served to them.
type AnalyticsTotalBucket struct {
	Type     string `json:"type"`
	Visits   int64  `json:"visits"`
	Requests int64  `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

// NetworkErrors is the error summary that [AppNetworkAPI.Errors] returns.
type NetworkErrors struct {
	Summary NetworkErrorsSummary `json:"summary"`
	// ByStatus counts the requests of each status code.
	ByStatus []NetworkErrorsStatus `json:"by_status"`
	// Timeseries is the errors over time.
	Timeseries []NetworkErrorsBucket `json:"timeseries"`
	// TopPaths lists the paths with the most errors.
	TopPaths []NetworkErrorsPath `json:"top_paths"`
	// ByMethod counts the errors of each HTTP method.
	ByMethod []NetworkErrorsMethod `json:"by_method"`
}

// NetworkErrorsSummary is the total number of errors. ByClass splits it into
// the keys "4xx" and "5xx".
type NetworkErrorsSummary struct {
	Total   int64            `json:"total"`
	ByClass map[string]int64 `json:"by_class"`
}

// NetworkErrorsStatus is the number of requests that got one status code.
type NetworkErrorsStatus struct {
	Status   int   `json:"status"`
	Requests int64 `json:"requests"`
}

// NetworkErrorsBucket is the errors of one time slot in
// [NetworkErrors.Timeseries]. Buckets maps each status code, written as a
// string, to its number of requests.
type NetworkErrorsBucket struct {
	Date    time.Time        `json:"date"`
	Buckets map[string]int64 `json:"buckets"`
	Total   int64            `json:"total"`
}

// NetworkErrorsPath is one of the paths with the most errors. ByStatus maps
// each status code, written as a string, to its number of requests. Method is
// nil when it is unknown.
type NetworkErrorsPath struct {
	Path     string           `json:"path"`
	Method   *string          `json:"method"`
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"by_status"`
}

// NetworkErrorsMethod is the errors of one HTTP method. ByStatus maps each
// status code, written as a string, to its number of requests. Method is nil
// when it is unknown.
type NetworkErrorsMethod struct {
	Method   *string          `json:"method"`
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"by_status"`
}

// NetworkLog is one request served from the application's domains, as
// [AppNetworkAPI.Logs] returns it.
type NetworkLog struct {
	Timestamp time.Time          `json:"timestamp"`
	Client    NetworkLogClient   `json:"client"`
	Request   NetworkLogRequest  `json:"request"`
	Response  NetworkLogResponse `json:"response"`
}

// NetworkLogClient is who sent the request of a [NetworkLog]. A pointer
// field is nil when it is unknown.
type NetworkLogClient struct {
	IP       *string `json:"ip"`
	Country  *string `json:"country"`
	Location *string `json:"location"`
	// ASN is the client's network, written as its autonomous system number.
	ASN string `json:"asn"`
	// Agent is the client's User-Agent header.
	Agent    *string `json:"agent"`
	Category *string `json:"category"`
}

// NetworkLogRequest is the request of a [NetworkLog]. A pointer field is nil
// when the request did not have it.
type NetworkLogRequest struct {
	// Mitigated is true when the edge's protection acted on the request.
	Mitigated bool    `json:"mitigated"`
	Method    string  `json:"method"`
	Host      string  `json:"host"`
	Path      string  `json:"path"`
	Query     *string `json:"query"`
	Protocol  string  `json:"protocol"`
	Referer   *string `json:"referer"`
}

// NetworkLogResponse is the response of a [NetworkLog]. A pointer field is
// nil when it is unknown.
type NetworkLogResponse struct {
	Status      int     `json:"status"`
	ContentType *string `json:"contentType"`
	// Cache is the edge cache status of the response.
	Cache *string `json:"cache"`
}

// Percentiles holds the 50th, 95th and 99th percentile latencies, in
// milliseconds. A field is nil when there were no requests.
type Percentiles struct {
	P50 *float64 `json:"p50"`
	P95 *float64 `json:"p95"`
	P99 *float64 `json:"p99"`
}

// NetworkPerformance is the latency summary that [AppNetworkAPI.Performance]
// returns. Edge latency is the time until the edge sends the first byte, and
// origin latency is how long your application took to respond.
type NetworkPerformance struct {
	Summary    PerformanceSummary  `json:"summary"`
	Timeseries []PerformanceBucket `json:"timeseries"`
	// Countries breaks the latency down by the visitor's country.
	Countries []PerformanceRegion `json:"countries"`
	// Colos breaks the latency down by edge location.
	Colos        []PerformanceRegion `json:"colos"`
	SlowestPaths []PerformancePath   `json:"slowest_paths"`
}

// PerformanceSummary is the latency over the whole period.
type PerformanceSummary struct {
	Edge     Percentiles `json:"edge"`
	Origin   Percentiles `json:"origin"`
	Requests int64       `json:"requests"`
}

// PerformanceBucket is the latency in one time slot of
// [NetworkPerformance.Timeseries].
type PerformanceBucket struct {
	Date     time.Time   `json:"date"`
	Requests int64       `json:"requests"`
	Edge     Percentiles `json:"edge"`
	Origin   Percentiles `json:"origin"`
}

// PerformanceRegion is the latency in one country or edge location, in
// milliseconds. For a country, Type is the country code. For an edge
// location, Type is the location code, and City and Country say where it is.
// A pointer field is nil when there is no data.
type PerformanceRegion struct {
	Type     string   `json:"type"`
	City     *string  `json:"city,omitempty"`
	Country  *string  `json:"country,omitempty"`
	P50      *float64 `json:"p50"`
	P95      *float64 `json:"p95"`
	Requests int64    `json:"requests"`
}

// PerformancePath is one of the slowest paths, with latencies in
// milliseconds. A latency is nil when there is no data.
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
	DatabaseMongo    DatabaseType = "mongo"    // MongoDB.
	DatabaseMySQL    DatabaseType = "mysql"    // MySQL.
	DatabaseRedis    DatabaseType = "redis"    // Redis.
	DatabasePostgres DatabaseType = "postgres" // PostgreSQL.
)

// Database is a database as [DatabasesAPI.Get] returns it.
type Database struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Owner is the user id of the database's owner.
	Owner string `json:"owner"`
	// Cluster identifies where the database is hosted.
	Cluster string `json:"cluster"`
	// RAM is the memory allocated to the database, in MB.
	RAM  int          `json:"ram"`
	Type DatabaseType `json:"type"`
	// Port is the TCP port that the database listens on.
	Port      int       `json:"port"`
	CreatedAt time.Time `json:"created_at"`
}

// DatabaseCreate describes the database that [DatabasesAPI.Create] creates.
type DatabaseCreate struct {
	Name string `json:"name"`
	// Memory is the memory to allocate, in MB. Redis needs at least 512 and
	// the other engines at least 1024.
	Memory int          `json:"memory"`
	Type   DatabaseType `json:"type"`
	// Version is the engine version. It can be a full version or a major or
	// minor version such as "8".
	Version string `json:"version"`
}

// DatabaseUpdate holds the changes that [DatabasesAPI.Update] makes. A field
// left at its zero value does not change, but at least one must be set.
type DatabaseUpdate struct {
	Name string `json:"name,omitempty"`
	// RAM is the new memory, in MB.
	RAM int `json:"ram,omitempty"`
}

// DatabaseCreated is the database that [DatabasesAPI.Create] created. Save
// Password, because the API returns it only once. ConnectionURL includes it.
// [DatabasesAPI.Certificate] returns the certificate again later.
type DatabaseCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Memory is the memory allocated to the database, in MB.
	Memory int `json:"memory"`
	// CPU is the CPU allocated to the database. It can be a fraction, such as
	// 0.5.
	CPU      float64      `json:"cpu"`
	Type     DatabaseType `json:"type"`
	Password string       `json:"password"`
	// Certificate is the TLS file, encoded in base64. It is a PEM file with
	// the private key followed by the certificate, or nil when the API could
	// not create it.
	Certificate *string `json:"certificate,omitempty"`
	// ConnectionURL is a ready-to-use connection string that includes the
	// password.
	ConnectionURL string `json:"connection_url"`
	// Cluster identifies where the database is hosted.
	Cluster string `json:"cluster"`
}

// DatabaseReset selects what [DatabasesAPI.ResetCredentials] replaces.
type DatabaseReset string

// DatabaseReset values.
const (
	ResetPassword    DatabaseReset = "password"
	ResetCertificate DatabaseReset = "certificate"
)

// ---- Workspaces ----

// Workspace is a workspace with its members and shared applications.
type Workspace struct {
	// ID is 32 hexadecimal characters, or 40 for older workspaces.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Owner is the user id of the workspace's owner.
	Owner        string            `json:"owner"`
	Members      []WorkspaceMember `json:"members"`
	Applications []WorkspaceApp    `json:"applications"`
	CreatedAt    time.Time         `json:"createdAt"`
}

// WorkspaceMember is a member of a [Workspace].
type WorkspaceMember struct {
	// ID is the member's user id.
	ID string `json:"id"`
	// Name is the member's name, or nil when it cannot be found, for example
	// for a deleted account.
	Name *string `json:"name"`
	// Group is "owner" or one of the [WorkspaceGroup] values.
	Group    string    `json:"group"`
	JoinedAt time.Time `json:"joinedAt"`
}

// WorkspaceApp is an application shared in a [Workspace].
type WorkspaceApp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Description is nil when the application has none.
	Description *string `json:"desc"`
	// RAM is the memory allocated to the application, in MB.
	RAM int `json:"ram"`
	// Lang is the application's language, such as "javascript".
	Lang string `json:"lang"`
	// Domain is the default hostname, or nil when the application has none.
	Domain *string `json:"domain"`
	// Custom is the custom domain, or nil when none is set.
	Custom *string `json:"custom"`
}

// WorkspaceCreated is the workspace that [WorkspacesAPI.Create] created.
type WorkspaceCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkspaceGroup is the permission level of a workspace member, which you
// pass to [WorkspaceMembersAPI.Add] and [WorkspaceMembersAPI.Update]. The
// owner's group is "owner", which cannot be assigned.
type WorkspaceGroup string

// WorkspaceGroup values.
const (
	GroupAdmin    WorkspaceGroup = "admin"    // An admin has full control, like the owner.
	GroupMaintain WorkspaceGroup = "maintain" // A maintainer has manager access, plus commits, files, DNS and the cache.
	GroupManager  WorkspaceGroup = "manager"  // A manager can monitor, start, stop and restart applications.
	GroupView     WorkspaceGroup = "view"     // A viewer can only read.
)

// ---- AI ----

// ChatRequest is the request of [AIAPI.Chat], in the OpenAI chat completions
// format. Streaming is not supported.
type ChatRequest struct {
	// Model is ignored, because every request uses the "cubic" model.
	Model string `json:"model,omitempty"`
	// Messages is the conversation so far.
	Messages []ChatMessage `json:"messages"`
	// Tools lists the tools that the model can call, as OpenAI tool
	// definitions in JSON.
	Tools []json.RawMessage `json:"tools,omitempty"`
	// ToolChoice controls whether and which tool the model calls, as in the
	// OpenAI format.
	ToolChoice any `json:"tool_choice,omitempty"`
	// MaxTokens is the older name of MaxCompletionTokens. When both are set,
	// MaxCompletionTokens wins.
	MaxTokens int `json:"max_tokens,omitempty"`
	// MaxCompletionTokens limits the length of the reply. A value above your
	// plan's limit is lowered to it.
	MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`
	// Temperature controls how varied the reply is. Leave it nil for the
	// default.
	Temperature *float64 `json:"temperature,omitempty"`
}

// ChatMessage is one message of a conversation.
type ChatMessage struct {
	// Role is "system", "user", "assistant" or "tool".
	Role string `json:"role"`
	// Content is the text of the message. It is always sent, even when it is
	// "", because the API requires it.
	Content string `json:"content"`
	// ToolCallID is the id of the tool call that a "tool" message answers.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolCalls holds the tool calls of an "assistant" message, as OpenAI
	// JSON objects.
	ToolCalls []json.RawMessage `json:"tool_calls,omitempty"`
}

// ChatCompletion is the reply of [AIAPI.Chat], in the OpenAI chat completions
// format.
type ChatCompletion struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	// Created is when the reply was made, in Unix seconds.
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage"`
}

// ChatChoice is one reply in a [ChatCompletion]. FinishReason says why the
// model stopped, such as "stop".
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatUsage counts the tokens of a [ChatCompletion].
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
