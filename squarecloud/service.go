package squarecloud

// ServiceStatus is the response of GET /service/status. Note this endpoint
// returns {status, message} directly, without the usual response envelope.
type ServiceStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
