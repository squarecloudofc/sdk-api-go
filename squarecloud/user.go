package squarecloud

import "time"

// User is the authenticated account returned by GET /users/me.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Locale    string    `json:"locale"`
	Plan      UserPlan  `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

type UserPlan struct {
	Name     string         `json:"name"`
	Memory   UserPlanMemory `json:"memory"`
	Duration *int64         `json:"duration"`
}

type UserPlanMemory struct {
	Limit     int `json:"limit"`
	Available int `json:"available"`
	Used      int `json:"used"`
}

// UserApplication is the compact application descriptor returned under
// response.applications of GET /users/me.
type UserApplication struct {
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

// UserDatabase is the compact database descriptor returned under
// response.databases of GET /users/me.
type UserDatabase struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RAM       int       `json:"ram"`
	Type      string    `json:"type"`
	Cluster   string    `json:"cluster"`
	CreatedAt time.Time `json:"created_at"`
}
