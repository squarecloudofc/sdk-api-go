package squarecloud

import "time"

// DeploymentState is the lifecycle step of a single deploy event.
type DeploymentState string

const (
	DeploymentStatePending    DeploymentState = "pending"
	DeploymentStateClone      DeploymentState = "clone"
	DeploymentStateCommit     DeploymentState = "commit"
	DeploymentStateRestarting DeploymentState = "restarting"
	DeploymentStateSuccess    DeploymentState = "success"
	DeploymentStateError      DeploymentState = "error"
)

// Deployment is one event in the timeline of a deploy returned by
// GET /apps/{id}/deployments. ID is the commit SHA-1 (40 hex chars), shared
// by every event of the same deploy. Branch is set on "clone" events and
// Files on "commit" events.
type Deployment struct {
	ID     string           `json:"id"`
	Date   time.Time        `json:"date"`
	State  DeploymentState  `json:"state"`
	Branch string           `json:"branch,omitempty"`
	Files  *DeploymentFiles `json:"files,omitempty"`
}

// DeploymentFiles is the changed-file list emitted alongside the "commit"
// step of a deploy timeline.
type DeploymentFiles struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

// DeploymentCurrent is the current Git deploy configuration returned by
// GET /apps/{id}/deployments/current. A nil App means no GitHub App linkage.
type DeploymentCurrent struct {
	App     *DeploymentGithubApp `json:"app,omitempty"`
	Webhook string               `json:"webhook,omitempty"`
}

type DeploymentGithubApp struct {
	ID     *int64  `json:"id"`
	Name   *string `json:"name"`
	Branch *string `json:"branch"`
}

// GithubWebhook is the response of POST /apps/{id}/deploy/webhook when
// enrolling a token. Absent when removing (access_token "@").
type GithubWebhook struct {
	Webhook string `json:"webhook"`
}

// GithubAppLink is the response of POST /apps/{id}/deploy/github-app.
type GithubAppLink struct {
	Repository GithubAppRepository `json:"repository"`
}

type GithubAppRepository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Branch   string `json:"branch"`
}
