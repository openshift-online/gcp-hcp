package logger

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// Context keys for storing values in context.Context
const (
	LogFieldsKey contextKey = "log_fields"
)

// Log field name constants - use these directly in WithFields maps
const (
	// Required fields (per logging spec)
	ComponentKey = "component"
	VersionKey   = "version"
	HostnameKey  = "hostname"

	// Error fields (per logging spec)
	ErrorKey      = "error"
	StackTraceKey = "stack_trace"

	// Correlation fields (distributed tracing)
	TraceIDKey = "trace_id"
	SpanIDKey  = "span_id"
	EventIDKey = "event_id"

	// Resource fields (from event data)
	ResourceTypeKey = "resource_type"

	// K8s manifest fields
	K8sKindKey      = "k8s_kind"
	K8sNameKey      = "k8s_name"
	K8sNamespaceKey = "k8s_namespace"
	K8sResultKey    = "k8s_result"

	// Adapter-specific fields
	AdapterKey            = "adapter"
	ObservedGenerationKey = "observed_generation"
	SubscriptionKey       = "subscription"

	// Maestro-specific fields
	MaestroConsumerKey = "maestro_consumer"
)

// LogFields holds dynamic key-value pairs for logging
type LogFields map[string]interface{}

// -----------------------------------------------------------------------------
// Context Setters
// -----------------------------------------------------------------------------

// WithLogField adds a single dynamic log field to the context
// These fields will be extracted and included in all log entries
func WithLogField(ctx context.Context, key string, value interface{}) context.Context {
	fields := GetLogFields(ctx)
	if fields == nil {
		fields = make(LogFields)
	}
	fields[key] = value
	return context.WithValue(ctx, LogFieldsKey, fields)
}

// WithLogFields adds multiple dynamic log fields to the context
// These fields will be extracted and included in all log entries
func WithLogFields(ctx context.Context, newFields LogFields) context.Context {
	fields := GetLogFields(ctx)
	if fields == nil {
		fields = make(LogFields)
	}
	for k, v := range newFields {
		fields[k] = v
	}
	return context.WithValue(ctx, LogFieldsKey, fields)
}

// WithDynamicResourceID adds a resource ID as a dynamic log field
// The field name is derived from the resource type (e.g., "Cluster" -> "cluster_id", "NodePool" -> "nodepool_id")
func WithDynamicResourceID(ctx context.Context, resourceType string, resourceID string) context.Context {
	fieldName := strings.ToLower(resourceType) + "_id"
	return WithLogField(ctx, fieldName, resourceID)
}

// WithTraceID returns a context with the trace ID set
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return WithLogField(ctx, TraceIDKey, traceID)
}

// WithSpanID returns a context with the span ID set
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return WithLogField(ctx, SpanIDKey, spanID)
}

// WithEventID returns a context with the event ID set
func WithEventID(ctx context.Context, eventID string) context.Context {
	return WithLogField(ctx, EventIDKey, eventID)
}

// WithResourceType returns a context with the event resource type set (e.g., "cluster", "nodepool")
func WithResourceType(ctx context.Context, resourceType string) context.Context {
	return WithLogField(ctx, ResourceTypeKey, resourceType)
}

// WithK8sKind returns a context with the K8s resource kind set (e.g., "Deployment", "Job")
func WithK8sKind(ctx context.Context, kind string) context.Context {
	return WithLogField(ctx, K8sKindKey, kind)
}

// WithK8sName returns a context with the K8s resource name set
func WithK8sName(ctx context.Context, name string) context.Context {
	return WithLogField(ctx, K8sNameKey, name)
}

// WithK8sNamespace returns a context with the K8s resource namespace set
func WithK8sNamespace(ctx context.Context, namespace string) context.Context {
	return WithLogField(ctx, K8sNamespaceKey, namespace)
}

// WithK8sResult returns a context with the K8s resource operation result set (SUCCESS/FAILED)
func WithK8sResult(ctx context.Context, result string) context.Context {
	return WithLogField(ctx, K8sResultKey, result)
}

// WithAdapter returns a context with the adapter name set
func WithAdapter(ctx context.Context, adapter string) context.Context {
	return WithLogField(ctx, AdapterKey, adapter)
}

// WithObservedGeneration returns a context with the observed generation set
func WithObservedGeneration(ctx context.Context, generation int64) context.Context {
	return WithLogField(ctx, ObservedGenerationKey, generation)
}

// WithSubscription returns a context with the subscription name set
func WithSubscription(ctx context.Context, subscription string) context.Context {
	return WithLogField(ctx, SubscriptionKey, subscription)
}

// WithMaestroConsumer returns a context with the Maestro consumer name set
func WithMaestroConsumer(ctx context.Context, consumer string) context.Context {
	return WithLogField(ctx, MaestroConsumerKey, consumer)
}

// WithErrorField returns a context with the error message set.
// Stack traces are captured only for unexpected/internal errors to avoid
// performance overhead under high event load. Expected operational errors
// (network issues, not found, auth failures) skip stack trace capture.
// If err is nil, returns the context unchanged.
func WithErrorField(ctx context.Context, err error) context.Context {
	if err == nil {
		return ctx
	}
	ctx = WithLogField(ctx, ErrorKey, err.Error())

	// Only capture stack trace for unexpected/internal errors
	if shouldCaptureStackTrace(err) {
		ctx = withStackTraceField(ctx, CaptureStackTrace(1))
	}

	return ctx
}

// WithOTelTraceContext extracts OpenTelemetry trace context (trace_id, span_id)
// from the context and adds them as log fields for distributed tracing correlation.
// If no active span exists, returns the context unchanged.
//
// This function is safe to call multiple times (e.g., once per span creation).
// Since Go contexts are immutable, each call returns a new context with the
// current span's IDs. The parent function's context remains unchanged, so logs
// after a child span completes will correctly use the parent's span_id.
//
// Example flow:
//
//	func Parent(ctx context.Context) {
//	    ctx, span := tracer.Start(ctx, "Parent")
//	    ctx = logger.WithOTelTraceContext(ctx)  // span_id=A
//	    Child(ctx)                               // Child logs use span_id=B
//	    log.Info(ctx, "Back in parent")          // Still uses span_id=A
//	}
//
// This will produce logs with trace_id and span_id fields:
//
//	{"message":"...","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7",...}
func WithOTelTraceContext(ctx context.Context) context.Context {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return ctx
	}

	// Add trace_id if valid
	if spanCtx.HasTraceID() {
		ctx = WithLogField(ctx, TraceIDKey, spanCtx.TraceID().String())
	}

	// Add span_id if valid
	if spanCtx.HasSpanID() {
		ctx = WithLogField(ctx, SpanIDKey, spanCtx.SpanID().String())
	}

	return ctx
}

// -----------------------------------------------------------------------------
// Context Getters
// -----------------------------------------------------------------------------

// GetLogFields returns the dynamic log fields from the context, or nil if not set
func GetLogFields(ctx context.Context) LogFields {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(LogFieldsKey).(LogFields); ok {
		// Return a copy to avoid mutation
		fields := make(LogFields, len(v))
		for k, val := range v {
			fields[k] = val
		}
		return fields
	}
	return nil
}
