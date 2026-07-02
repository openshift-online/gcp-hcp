package maestroclient

import (
	"context"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/pkg/logger"
	"github.com/openshift-online/ocm-sdk-go/logging"
)

// Ensure ocmLoggerAdapter implements the OCM SDK logging.Logger interface
var _ logging.Logger = &ocmLoggerAdapter{}

// ocmLoggerAdapter adapts our logger.Logger interface to the OCM SDK logging.Logger interface.
// This allows using our logger with Maestro's grpcsource client.
type ocmLoggerAdapter struct {
	log logger.Logger
}

// newOCMLoggerAdapter creates a new OCM SDK compatible logger adapter
func newOCMLoggerAdapter(log logger.Logger) *ocmLoggerAdapter {
	return &ocmLoggerAdapter{log: log}
}

// DebugEnabled returns true if the debug level is enabled.
// Always returns true - let the underlying logger filter.
func (a *ocmLoggerAdapter) DebugEnabled() bool {
	return true
}

// InfoEnabled returns true if the information level is enabled.
func (a *ocmLoggerAdapter) InfoEnabled() bool {
	return true
}

// WarnEnabled returns true if the warning level is enabled.
func (a *ocmLoggerAdapter) WarnEnabled() bool {
	return true
}

// ErrorEnabled returns true if the error level is enabled.
func (a *ocmLoggerAdapter) ErrorEnabled() bool {
	return true
}

// Debug logs at debug level with formatting.
func (a *ocmLoggerAdapter) Debug(ctx context.Context, format string, args ...interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.log.Debugf(ctx, format, args...)
}

// Info logs at info level with formatting.
func (a *ocmLoggerAdapter) Info(ctx context.Context, format string, args ...interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.log.Infof(ctx, format, args...)
}

// Warn logs at warn level with formatting.
func (a *ocmLoggerAdapter) Warn(ctx context.Context, format string, args ...interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.log.Warnf(ctx, format, args...)
}

// Error logs at error level with formatting.
func (a *ocmLoggerAdapter) Error(ctx context.Context, format string, args ...interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.log.Errorf(ctx, format, args...)
}

// Fatal logs at error level with formatting.
// Note: Does not exit - the underlying logger handles that behavior.
func (a *ocmLoggerAdapter) Fatal(ctx context.Context, format string, args ...interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.log.Errorf(ctx, "FATAL: "+format, args...)
}
