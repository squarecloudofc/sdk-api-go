package squarecloud

type APIResponse[T any] struct {
	Response T      `json:"response,omitempty"`
	Message  string `json:"message,omitempty"`
	Status   string `json:"status,omitempty"`
	Code     string `json:"code"`
}
