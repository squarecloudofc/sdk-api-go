package squarecloud

import "time"

// DNSRecord is one record the user must set at their DNS provider, returned
// by GET /apps/{id}/network/dns. Value for "cname" records is always
// "cname.squareweb.app". Status is passed through from the edge provider
// (e.g. "pending", "pending_validation", "active").
type DNSRecord struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

// AnalyticsTimeBucket is one point of the "visits" 15-minute time series.
type AnalyticsTimeBucket struct {
	Date     time.Time `json:"date"`
	Visits   int       `json:"visits"`
	Requests int       `json:"requests"`
	Bytes    int64     `json:"bytes"`
}

// AnalyticsBucket is a per-dimension 15-minute bucket — Type identifies the
// dimension value (country name, HTTP method, browser, ASN tag, ...).
type AnalyticsBucket struct {
	Type     string    `json:"type"`
	Date     time.Time `json:"date"`
	Visits   int       `json:"visits"`
	Requests int       `json:"requests"`
	Bytes    int64     `json:"bytes"`
}

// AnalyticsTotalBucket is a whole-window bucket (no time dimension) used by
// the ips, status_codes, bots and content_types breakdowns.
type AnalyticsTotalBucket struct {
	Type     string `json:"type"`
	Visits   int    `json:"visits"`
	Requests int    `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

// NetworkAnalytics is the aggregated edge analytics of
// GET /apps/{id}/network/analytics. The endpoint may return an empty object
// when the requested window precedes the application's creation date — all
// slices will be nil in that case.
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

// NetworkErrors is the edge error breakdown of GET /apps/{id}/network/errors.
type NetworkErrors struct {
	Summary    NetworkErrorsSummary      `json:"summary"`
	ByStatus   []NetworkErrorsByStatus   `json:"by_status"`
	Timeseries []NetworkErrorsTimeseries `json:"timeseries"`
	TopPaths   []NetworkErrorsTopPath    `json:"top_paths"`
	ByMethod   []NetworkErrorsByMethod   `json:"by_method"`
}

type NetworkErrorsSummary struct {
	Total   int                       `json:"total"`
	ByClass NetworkErrorsSummaryClass `json:"by_class"`
}

type NetworkErrorsSummaryClass struct {
	Class4xx int `json:"4xx"`
	Class5xx int `json:"5xx"`
}

type NetworkErrorsByStatus struct {
	Status   int `json:"status"`
	Requests int `json:"requests"`
}

type NetworkErrorsTimeseries struct {
	Date time.Time `json:"date"`
	// Buckets maps status code (as string) to request count.
	Buckets map[string]int `json:"buckets"`
	Total   int            `json:"total"`
}

type NetworkErrorsTopPath struct {
	Path string `json:"path"`
	// Method may be nil when the edge couldn't attribute one.
	Method   *string        `json:"method"`
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

type NetworkErrorsByMethod struct {
	Method   *string        `json:"method"`
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

// NetworkLog is a single edge log entry of GET /apps/{id}/network/logs.
// Available on Pro+ plans.
type NetworkLog struct {
	Timestamp time.Time          `json:"timestamp"`
	Client    NetworkLogClient   `json:"client"`
	Request   NetworkLogRequest  `json:"request"`
	Response  NetworkLogResponse `json:"response"`
}

type NetworkLogClient struct {
	IP       *string `json:"ip"`
	Country  *string `json:"country"`
	Location *string `json:"location"`
	ASN      string  `json:"asn"`
	Agent    *string `json:"agent"`
	Category *string `json:"category"`
}

type NetworkLogRequest struct {
	Mitigated bool    `json:"mitigated"`
	Method    string  `json:"method"`
	Host      string  `json:"host"`
	Path      string  `json:"path"`
	Query     *string `json:"query"`
	Protocol  string  `json:"protocol"`
	Referer   *string `json:"referer"`
}

type NetworkLogResponse struct {
	Status      int     `json:"status"`
	ContentType *string `json:"contentType"`
	Cache       *string `json:"cache"`
}

// NetworkLatency is the latency percentile triple used by the performance
// endpoint (milliseconds).
type NetworkLatency struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// NetworkPerformance is the latency report of
// GET /apps/{id}/network/performance. Available on Pro+ plans.
type NetworkPerformance struct {
	Summary      NetworkPerformanceSummary      `json:"summary"`
	Timeseries   []NetworkPerformanceTimeseries `json:"timeseries"`
	Countries    []NetworkPerformanceCountry    `json:"countries"`
	Colos        []NetworkPerformanceColo       `json:"colos"`
	SlowestPaths []NetworkPerformanceSlowPath   `json:"slowest_paths"`
}

type NetworkPerformanceSummary struct {
	Edge     NetworkLatency `json:"edge"`
	Origin   NetworkLatency `json:"origin"`
	Requests int            `json:"requests"`
}

type NetworkPerformanceTimeseries struct {
	Date     time.Time      `json:"date"`
	Requests int            `json:"requests"`
	Edge     NetworkLatency `json:"edge"`
	Origin   NetworkLatency `json:"origin"`
}

type NetworkPerformanceCountry struct {
	Type     string  `json:"type"`
	P50      float64 `json:"p50"`
	P95      float64 `json:"p95"`
	Requests int     `json:"requests"`
}

type NetworkPerformanceColo struct {
	Type     string  `json:"type"`
	City     string  `json:"city"`
	Country  string  `json:"country"`
	P50      float64 `json:"p50"`
	P95      float64 `json:"p95"`
	Requests int     `json:"requests"`
}

type NetworkPerformanceSlowPath struct {
	Path     string  `json:"path"`
	P95      float64 `json:"p95"`
	P99      float64 `json:"p99"`
	Requests int     `json:"requests"`
}

// AppDomainType of a hostname listed by GET /apps/domains.
type AppDomainType string

const (
	AppDomainTypeSubdomain AppDomainType = "subdomain"
	AppDomainTypeCustom    AppDomainType = "custom"
)

// AppDomain is one hostname configured on one of the account's applications,
// returned by GET /apps/domains. Custom domains are listed first.
type AppDomain struct {
	AppID    string        `json:"app_id"`
	Hostname string        `json:"hostname"`
	Type     AppDomainType `json:"type"`
}

// LoadBalancers is the response of GET /apps/load-balancers — the caller's
// applications grouped by attached custom domain. Limit is the per-domain
// app cap for the caller's plan (2 Standard / 5 Pro / 10 Enterprise).
type LoadBalancers struct {
	Limit     int            `json:"limit"`
	Balancers []LoadBalancer `json:"balancers"`
}

// LoadBalancer is one custom domain and the applications serving it. Two or
// more apps means an active load balancer with automatic failover.
type LoadBalancer struct {
	Hostname string            `json:"hostname"`
	Apps     []LoadBalancerApp `json:"apps"`
}

type LoadBalancerApp struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Cluster string `json:"cluster,omitempty"`
}
