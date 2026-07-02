package errors

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// -----------------------------------------------------------------------------
// K8s Operation Error
// -----------------------------------------------------------------------------

// K8sOperationError represents a structured Kubernetes operation error with detailed context.
// This allows callers to handle K8s errors with full information about what failed.
type K8sOperationError struct {
	// Err is the underlying error
	Err error
	// Operation is the operation that failed: "create", "update", "delete", "get", "patch", "list"
	Operation string
	// Resource is the resource name
	Resource string
	// Kind is the Kubernetes resource kind
	Kind string
	// Namespace is the resource namespace
	Namespace string
	// Message is the error message
	Message string
}

// Error implements the error interface
func (e *K8sOperationError) Error() string {
	if e.Namespace != "" {
		return fmt.Sprintf("K8s %s operation failed: %s/%s (namespace: %s): %s",
			e.Operation, e.Kind, e.Resource, e.Namespace, e.Message)
	}
	return fmt.Sprintf("K8s %s operation failed: %s/%s: %s",
		e.Operation, e.Kind, e.Resource, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *K8sOperationError) Unwrap() error {
	return e.Err
}

// IsK8sOperationError checks if an error is a K8sOperationError and returns it.
// This function supports wrapped errors.
func IsK8sOperationError(err error) (*K8sOperationError, bool) {
	var k8sErr *K8sOperationError
	if errors.As(err, &k8sErr) {
		return k8sErr, true
	}
	return nil, false
}

// -----------------------------------------------------------------------------
// K8s Resource Data Extraction Errors
// -----------------------------------------------------------------------------

// K8sResourceKeyNotFoundError represents an error when a key is not found in a K8s resource
type K8sResourceKeyNotFoundError struct {
	ResourceType string // "Secret" or "ConfigMap"
	Namespace    string
	ResourceName string
	Key          string
}

// Error implements the error interface
func (e *K8sResourceKeyNotFoundError) Error() string {
	return fmt.Sprintf("key '%s' not found in %s %s/%s", e.Key, e.ResourceType, e.Namespace, e.ResourceName)
}

// NewK8sResourceKeyNotFoundError creates a new K8sResourceKeyNotFoundError
func NewK8sResourceKeyNotFoundError(resourceType, namespace, resourceName, key string) *K8sResourceKeyNotFoundError {
	return &K8sResourceKeyNotFoundError{
		ResourceType: resourceType,
		Namespace:    namespace,
		ResourceName: resourceName,
		Key:          key,
	}
}

// K8sInvalidPathError represents an error when a resource path format is invalid
type K8sInvalidPathError struct {
	ResourceType   string
	Path           string
	ExpectedFormat string
}

// Error implements the error interface
func (e *K8sInvalidPathError) Error() string {
	return fmt.Sprintf("invalid %s path format: %s (expected: %s)", e.ResourceType, e.Path, e.ExpectedFormat)
}

// NewK8sInvalidPathError creates a new K8sInvalidPathError
func NewK8sInvalidPathError(resourceType, path, expectedFormat string) *K8sInvalidPathError {
	return &K8sInvalidPathError{
		ResourceType:   resourceType,
		Path:           path,
		ExpectedFormat: expectedFormat,
	}
}

// K8sResourceDataError represents an error when accessing or parsing resource data
type K8sResourceDataError struct {
	// Err is the underlying error
	Err error
	// ResourceType is "Secret" or "ConfigMap"
	ResourceType string
	// ResourceName is the resource name
	ResourceName string
	// Namespace is the resource namespace
	Namespace string
	// Field is the field name (e.g., "data", or specific key name)
	Field string
	// Reason explains what went wrong
	Reason string
}

// Error implements the error interface
func (e *K8sResourceDataError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s %s/%s: %s: %v", e.ResourceType, e.Namespace, e.ResourceName, e.Reason, e.Err)
	}
	return fmt.Sprintf("%s %s/%s: %s", e.ResourceType, e.Namespace, e.ResourceName, e.Reason)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *K8sResourceDataError) Unwrap() error {
	return e.Err
}

// NewK8sResourceDataError creates a new K8sResourceDataError
func NewK8sResourceDataError(resourceType, namespace, resourceName, reason string, err error) *K8sResourceDataError {
	return &K8sResourceDataError{
		ResourceType: resourceType,
		Namespace:    namespace,
		ResourceName: resourceName,
		Reason:       reason,
		Err:          err,
	}
}

// -----------------------------------------------------------------------------
// K8s Retryable Error Detection
// -----------------------------------------------------------------------------

// IsRetryableDiscoveryError determines if a discovery error is transient/retryable
// (and thus safe to ignore and proceed with create) or fatal (and should fail fast).
//
// Retryable errors (returns true):
//   - Timeouts (request/server timeouts)
//   - Server errors (5xx status codes)
//   - Network/connection errors (connection refused, reset, etc.)
//   - Service unavailable
//   - Too many requests (rate limiting)
//
// Non-retryable/fatal errors (returns false):
//   - Forbidden (403) - permission denied
//   - Unauthorized (401) - authentication failure
//   - Bad request (400) - invalid request
//   - Invalid/validation errors
//   - Gone (410) - resource no longer exists
//   - Method not supported
//   - Not acceptable
func IsRetryableDiscoveryError(err error) bool {
	if err == nil {
		return false
	}

	// Check for transient Kubernetes API errors (retryable)
	if apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) ||
		apierrors.IsTooManyRequests(err) {
		return true
	}

	// Check for fatal Kubernetes API errors (non-retryable)
	if apierrors.IsForbidden(err) ||
		apierrors.IsUnauthorized(err) ||
		apierrors.IsBadRequest(err) ||
		apierrors.IsInvalid(err) ||
		apierrors.IsGone(err) ||
		apierrors.IsMethodNotSupported(err) ||
		apierrors.IsNotAcceptable(err) {
		return false
	}

	// Check for network-level errors (retryable)
	if IsNetworkError(err) {
		return true
	}

	// Default: treat unknown errors as non-retryable to surface issues early
	return false
}
