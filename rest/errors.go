package rest

import (
	"errors"
	"fmt"

	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

// APIError is a non-2xx response from the Square Cloud API. Code carries the
// machine-readable error code (e.g. "APP_NOT_FOUND", "ACCESS_DENIED",
// "RATE_LIMIT_EXCEEDED") — the canonical list lives in
// @squarecloud/api-types' APIErrorCode.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("squarecloud: %s (%s, HTTP %d)", e.Message, e.Code, e.StatusCode)
	}

	return fmt.Sprintf("squarecloud: request failed with code %s (HTTP %d)", e.Code, e.StatusCode)
}

// ErrorCode extracts the Square Cloud error code from err, or "" if err is
// not an APIError.
func ErrorCode(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}

	return ""
}

// IsRateLimit reports whether err is a rate-limit rejection (including the
// short KEEP_CALM retry and legacy DELAY_NOW/RATELIMIT codes).
func IsRateLimit(err error) bool {
	switch ErrorCode(err) {
	case "KEEP_CALM", "RATE_LIMIT", "RATE_LIMIT_EXCEEDED", "RATELIMIT", "DELAY_NOW":
		return true
	}

	return false
}

func parseError(statusCode int, r *squarecloud.APIResponse[any]) error {
	return &APIError{StatusCode: statusCode, Code: r.Code, Message: r.Message}
}
