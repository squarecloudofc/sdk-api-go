package squarecloud

import "time"

// DatabaseType is the database engine slug.
type DatabaseType string

const (
	DatabaseTypeMongo    DatabaseType = "mongo"
	DatabaseTypeMySQL    DatabaseType = "mysql"
	DatabaseTypeRedis    DatabaseType = "redis"
	DatabaseTypePostgres DatabaseType = "postgres"
)

// DatabaseResetType selects which credential to rotate via
// POST /databases/{id}/credentials/reset.
type DatabaseResetType string

const (
	DatabaseResetPassword    DatabaseResetType = "password"
	DatabaseResetCertificate DatabaseResetType = "certificate"
)

// Database is the full descriptor returned by GET /databases/{id}.
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

// DatabaseCreateOptions is the body of POST /databases.
type DatabaseCreateOptions struct {
	// Name is the display name, 1-32 chars.
	Name string `json:"name"`
	// Memory in MB.
	Memory int          `json:"memory"`
	Type   DatabaseType `json:"type"`
	// Version key supported for the chosen Type (see the Square Cloud docs).
	Version string `json:"version"`
}

// DatabaseUpdateOptions is the body of PATCH /databases/{id}. At least one
// field must be set.
type DatabaseUpdateOptions struct {
	Name string `json:"name,omitempty"`
	RAM  int    `json:"ram,omitempty"`
}

// DatabaseCreated is the response of POST /databases. Password (and
// Certificate when applicable) are returned only at creation and cannot be
// recovered later.
type DatabaseCreated struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Memory int          `json:"memory"`
	CPU    int          `json:"cpu"`
	Type   DatabaseType `json:"type"`
	// Password is the one-time database password. Shown only at creation.
	Password string `json:"password"`
	// Certificate is a base64-encoded PEM TLS certificate, when applicable.
	Certificate string `json:"certificate,omitempty"`
	// ConnectionURL is a ready-to-use connection string with the password embedded.
	ConnectionURL string `json:"connection_url"`
	Cluster       string `json:"cluster"`
}

// DatabaseStatus shares the runtime stats shape with ApplicationStatus.
type DatabaseStatus = ApplicationStatus

// DatabaseStatusRaw shares the ?rawData=true shape with ApplicationStatusRaw.
type DatabaseStatusRaw = ApplicationStatusRaw

// DatabaseStatusListItem is one entry of GET /databases/status.
type DatabaseStatusListItem = ApplicationStatusListItem

// DatabaseCertificate is the response of
// GET /databases/{id}/credentials/certificate.
type DatabaseCertificate struct {
	// Certificate is a base64-encoded PEM certificate.
	Certificate string `json:"certificate"`
}

// DatabasePasswordReset is the response body of a password reset. Certificate
// resets return a status-only payload.
type DatabasePasswordReset struct {
	// Password is shown only once.
	Password string `json:"password"`
}
