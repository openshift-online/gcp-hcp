package errors

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// -----------------------------------------------------------------------------
// API Error Type
// -----------------------------------------------------------------------------

// APIError represents an error from an HTTP API call with detailed context.
// This allows the adapter runtime to capture and handle request errors properly.
type APIError struct {
	// Err is the underlying error
	Err error
	// Method is the HTTP method used (GET, POST, PUT, PATCH, DELETE)
	Method string
	// URL is the request URL
	URL string
	// Status is the HTTP status string (e.g., "503 Service Unavailable")
	Status string
	// ResponseBody is the response body (may contain error details from the API)
	ResponseBody []byte
	// Duration is the total duration including retries
	Duration time.Duration
	// StatusCode is the HTTP status code (0 if request failed before getting response)
	StatusCode int
	// Attempts is how many attempts were made (including retries)
	Attempts int
}

// Error implements the error interface.
// Note: Err should always be non-nil when APIError is created in production code.
// The client.Do() method always sets lastErr before returning an APIError.
func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("API request failed: %s %s returned %d %s after %d attempt(s): %v",
			e.Method, e.URL, e.StatusCode, e.Status, e.Attempts, e.Err)
	}
	return fmt.Sprintf("API request failed: %s %s after %d attempt(s): %v",
		e.Method, e.URL, e.Attempts, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *APIError) Unwrap() error {
	return e.Err
}

// -----------------------------------------------------------------------------
// Status Code Helpers
// -----------------------------------------------------------------------------

// IsTimeout returns true if the error was caused by a timeout
func (e *APIError) IsTimeout() bool {
	return e.StatusCode == 408 || errors.Is(e.Err, context.DeadlineExceeded)
}

// IsServerError returns true if the error was a server error (5xx)
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// IsClientError returns true if the error was a client error (4xx)
func (e *APIError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsNotFound returns true if the error was a 404 Not Found
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsUnauthorized returns true if the error was a 401 Unauthorized
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == 401
}

// IsForbidden returns true if the error was a 403 Forbidden
func (e *APIError) IsForbidden() bool {
	return e.StatusCode == 403
}

// IsRateLimited returns true if the error was a 429 Too Many Requests
func (e *APIError) IsRateLimited() bool {
	return e.StatusCode == 429
}

// IsBadRequest returns true if the error was a 400 Bad Request
func (e *APIError) IsBadRequest() bool {
	return e.StatusCode == 400
}

// IsConflict returns true if the error was a 409 Conflict
func (e *APIError) IsConflict() bool {
	return e.StatusCode == 409
}

// -----------------------------------------------------------------------------
// Response Body Helpers
// -----------------------------------------------------------------------------

// ResponseBodyString returns the response body as a string
func (e *APIError) ResponseBodyString() string {
	if e.ResponseBody == nil {
		return ""
	}
	return string(e.ResponseBody)
}

// HasResponseBody returns true if there is a response body
func (e *APIError) HasResponseBody() bool {
	return len(e.ResponseBody) > 0
}

// -----------------------------------------------------------------------------
// Constructor and Helper Functions
// -----------------------------------------------------------------------------

// NewAPIError creates a new APIError with all fields
func NewAPIError(
	method, url string,
	statusCode int,
	status string,
	body []byte,
	attempts int,
	duration time.Duration,
	err error,
) *APIError {
	return &APIError{
		Method:       method,
		URL:          url,
		StatusCode:   statusCode,
		Status:       status,
		ResponseBody: body,
		Attempts:     attempts,
		Duration:     duration,
		Err:          err,
	}
}

// IsAPIError checks if an error is an APIError and returns it.
// This function supports wrapped errors via errors.As.
//
// Example usage:
//
//	if apiErr, ok := errors.IsAPIError(err); ok {
//	    log.Printf("API call failed: status=%d body=%s", apiErr.StatusCode, apiErr.ResponseBodyString())
//	}
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
