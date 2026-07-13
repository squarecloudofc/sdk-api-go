package squarecloud

import "time"

// SnapshotScope selects the domain of GET /users/snapshots.
type SnapshotScope string

const (
	SnapshotScopeApplications SnapshotScope = "applications"
	SnapshotScopeDatabases    SnapshotScope = "databases"
)

// Snapshot is one entry of the snapshot listings (user-, app- and
// database-scoped).
type Snapshot struct {
	Name     string    `json:"name"`
	Size     int       `json:"size"`
	Modified time.Time `json:"modified"`
	Key      string    `json:"key"`
}

// SnapshotCreated is the response of POST /apps/{id}/snapshots and
// POST /databases/{id}/snapshots.
type SnapshotCreated struct {
	URL string `json:"url"`
	Key string `json:"key"`
}
