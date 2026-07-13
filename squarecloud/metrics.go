package squarecloud

import "time"

// MetricPoint is a single point of the 24h metrics window returned by
// GET /apps/{id}/metrics and GET /databases/{id}/metrics — up to 288 points
// (24h sampled every 5 minutes).
type MetricPoint struct {
	Date time.Time `json:"date"`
	CPU  float64   `json:"cpu"`
	RAM  float64   `json:"ram"`
	Net  []float64 `json:"net"`
}
