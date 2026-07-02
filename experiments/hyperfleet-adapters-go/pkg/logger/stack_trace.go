package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"

	apperrors "github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// -----------------------------------------------------------------------------
// Stack Trace Capture
// -----------------------------------------------------------------------------

// skipStackTraceCheckers is a list of functions that check if an error should skip stack trace capture.
// Each checker returns true if the error is an expected operational error.
// Add new error types here to extend the blocklist.
var skipStackTraceCheckers = []func(error) bool{
	// Context errors (expected in graceful shutdown)
	func(err error) bool { return errors.Is(err, context.Canceled) },
	func(err error) bool { return errors.Is(err, context.DeadlineExceeded) },
	func(err error) bool { return errors.Is(err, io.EOF) },

	// Network/transient errors (expected in distributed systems)
	apperrors.IsNetworkError,

	// HyperFleet API errors (HTTP 4xx/5xx responses)
	isExpectedAPIError,

	// K8s resource data errors
	isK8sResourceDataError,

	// K8s API errors
	apierrors.IsNotFound,
	apierrors.IsConflict,
	apierrors.IsAlreadyExists,
	apierrors.IsForbidden,
	apierrors.IsUnauthorized,
	apierrors.IsInvalid,
	apierrors.IsBadRequest,
	apierrors.IsGone,
	apierrors.IsResourceExpired,
	apierrors.IsServiceUnavailable,
	apierrors.IsTimeout,
	apierrors.IsTooManyRequests,
}

// isExpectedAPIError checks if the error is an expected HyperFleet API error
func isExpectedAPIError(err error) bool {
	apiErr, ok := apperrors.IsAPIError(err)
	if !ok {
		return false
	}
	return apiErr.IsNotFound() ||
		apiErr.IsUnauthorized() ||
		apiErr.IsForbidden() ||
		apiErr.IsBadRequest() ||
		apiErr.IsConflict() ||
		apiErr.IsRateLimited() ||
		apiErr.IsTimeout() ||
		apiErr.IsServerError()
}

// isK8sResourceDataError checks if the error is an expected K8s resource data error
func isK8sResourceDataError(err error) bool {
	var k8sKeyNotFound *apperrors.K8sResourceKeyNotFoundError
	if errors.As(err, &k8sKeyNotFound) {
		return true
	}
	var k8sInvalidPath *apperrors.K8sInvalidPathError
	if errors.As(err, &k8sInvalidPath) {
		return true
	}
	var k8sDataErr *apperrors.K8sResourceDataError
	return errors.As(err, &k8sDataErr)
}

// shouldCaptureStackTrace determines if a stack trace should be captured for the given error.
// Returns false for expected operational errors (high frequency, known causes) to avoid
// performance overhead during error storms. Returns true for unexpected errors that
// indicate bugs or require investigation.
func shouldCaptureStackTrace(err error) bool {
	if err == nil {
		return false
	}

	// Check all blocklist conditions
	for _, check := range skipStackTraceCheckers {
		if check(err) {
			return false
		}
	}

	// Capture stack trace for unexpected/internal errors
	return true
}

// withStackTraceField returns a context with the stack trace set.
// If frames is nil or empty, returns the context unchanged.
func withStackTraceField(ctx context.Context, frames []string) context.Context {
	if len(frames) == 0 {
		return ctx
	}
	return WithLogField(ctx, StackTraceKey, frames)
}

// CaptureStackTrace captures the current call stack and returns it as a slice of strings.
// Each string contains the file path, line number, and function name.
// The skip parameter specifies how many stack frames to skip:
//   - skip=0 starts from the caller of CaptureStackTrace
//   - skip=1 skips one additional level, etc.
func CaptureStackTrace(skip int) []string {
	const maxFrames = 32
	pcs := make([]uintptr, maxFrames)
	// +2 to skip runtime.Callers and CaptureStackTrace itself
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pcs[:n])
	var stack []string
	for {
		frame, more := frames.Next()
		stack = append(stack, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}
	return stack
}
