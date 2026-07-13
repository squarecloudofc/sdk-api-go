package squarecloud

import "time"

type ApplicationSignal string

const (
	ApplicationSignalStart   ApplicationSignal = "START"
	ApplicationSignalStop    ApplicationSignal = "STOP"
	ApplicationSignalRestart ApplicationSignal = "RESTART"
)

// Application is the full descriptor returned by GET /apps/{id}.
type Application struct {
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

// ApplicationStatus is the runtime stats of GET /apps/{id}/status (and
// GET /databases/{id}/status). Usage fields come formatted as strings
// ("0.50%", "120/512MB"); see ApplicationStatusRaw for ?rawData=true.
type ApplicationStatus struct {
	CPU     string                   `json:"cpu"`
	RAM     string                   `json:"ram"`
	Status  string                   `json:"status"`
	Running bool                     `json:"running"`
	Storage string                   `json:"storage"`
	Network ApplicationStatusNetwork `json:"network"`
	// Uptime is the Unix timestamp (ms) at which the resource last started.
	// nil when not running.
	Uptime *int64 `json:"uptime"`
}

type ApplicationStatusNetwork struct {
	Total string `json:"total"`
	Now   string `json:"now"`
}

// ApplicationStatusRaw is the ?rawData=true variant of ApplicationStatus:
// numbers instead of formatted strings, [in, out] byte pairs for network.
type ApplicationStatusRaw struct {
	CPU     float64                     `json:"cpu"`
	RAM     float64                     `json:"ram"`
	Status  string                      `json:"status"`
	Running bool                        `json:"running"`
	Storage int64                       `json:"storage"`
	Network ApplicationStatusNetworkRaw `json:"network"`
	Uptime  *int64                      `json:"uptime"`
}

type ApplicationStatusNetworkRaw struct {
	Total [2]int64 `json:"total"`
	Now   [2]int64 `json:"now"`
}

// ApplicationStatusListItem is one entry of GET /apps/status. CPU and RAM are
// present only when the application is running.
type ApplicationStatusListItem struct {
	ID      string `json:"id"`
	Running bool   `json:"running"`
	CPU     string `json:"cpu,omitempty"`
	RAM     string `json:"ram,omitempty"`
}

type ApplicationLogs struct {
	Logs string `json:"logs"`
}

// ApplicationUploaded is the response of POST /apps. Description and
// Subdomain are present only when set in squarecloud.config.
type ApplicationUploaded struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Subdomain   string              `json:"subdomain,omitempty"`
	RAM         int                 `json:"ram"`
	CPU         int                 `json:"cpu"`
	Language    ApplicationLanguage `json:"language"`
}

// ApplicationLanguage is the runtime detected for an uploaded application.
type ApplicationLanguage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
