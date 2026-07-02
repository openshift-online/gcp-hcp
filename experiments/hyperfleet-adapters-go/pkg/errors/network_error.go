package errors

import (
	"errors"
	"net"
	"syscall"

	utilnet "k8s.io/apimachinery/pkg/util/net"
)

// IsNetworkError checks if the error is a network-level error (connection issues, DNS, etc.)
// Uses syscall error codes and k8s.io/apimachinery/pkg/util/net for stable, universal detection.
//
// Detected errors include:
//   - Connection refused (ECONNREFUSED)
//   - Connection reset (ECONNRESET)
//   - Connection timed out (ETIMEDOUT)
//   - Network unreachable (ENETUNREACH)
//   - No route to host (EHOSTUNREACH)
//   - Connection aborted (ECONNABORTED)
//   - Broken pipe (EPIPE)
//   - EOF and connection closure errors
//   - Timeout errors (net.Error.Timeout())
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Use k8s.io/apimachinery/pkg/util/net utilities for common network errors
	// These use syscall.Errno under the hood for stable detection
	if utilnet.IsConnectionRefused(err) {
		return true
	}
	if utilnet.IsConnectionReset(err) {
		return true
	}
	if utilnet.IsTimeout(err) {
		return true
	}
	if utilnet.IsProbableEOF(err) {
		return true
	}

	// Check for additional syscall errors not covered by utilnet
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno { //nolint:exhaustive // only matching specific network-related errors
		case syscall.ETIMEDOUT: // Connection timed out
			return true
		case syscall.ENETUNREACH: // Network is unreachable
			return true
		case syscall.EHOSTUNREACH: // No route to host
			return true
		case syscall.ECONNABORTED: // Connection aborted
			return true
		case syscall.EPIPE: // Broken pipe
			return true
		}
	}

	// Check for net.OpError which wraps network operation failures
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Recursively check the underlying error
		return IsNetworkError(opErr.Err)
	}

	// Check for net.Error interface (includes custom network errors)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	return false
}
