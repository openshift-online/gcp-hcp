package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	FormatJSON = "json"
	FormatText = "text"
)

// Logger is the interface for structured logging.
// Context is passed as a parameter to each method (aligned with sentinel pattern).
type Logger interface {
	// Debug logs at debug level
	Debug(ctx context.Context, message string)
	// Debugf logs at debug level with formatting
	Debugf(ctx context.Context, format string, args ...interface{})
	// Info logs at info level
	Info(ctx context.Context, message string)
	// Infof logs at info level with formatting
	Infof(ctx context.Context, format string, args ...interface{})
	// Warn logs at warn level
	Warn(ctx context.Context, message string)
	// Warnf logs at warn level with formatting
	Warnf(ctx context.Context, format string, args ...interface{})
	// Error logs at error level
	Error(ctx context.Context, message string)
	// Errorf logs at error level with formatting
	Errorf(ctx context.Context, format string, args ...interface{})
	// Fatal logs at error level and exits
	Fatal(ctx context.Context, message string)

	// With returns a new logger with additional fields
	With(key string, value interface{}) Logger
	// WithFields returns a new logger with multiple additional fields
	WithFields(fields map[string]interface{}) Logger
	// Without returns a new logger with the specified field removed
	Without(key string) Logger
}

var _ Logger = &logger{}

// logger is the concrete implementation using log/slog
type logger struct {
	slog      *slog.Logger
	fields    map[string]interface{}
	component string
	version   string
	hostname  string
}

// Config holds logger configuration
type Config struct {
	// Level is the minimum log level: "debug", "info", "warn", "error"
	Level string
	// Format is the output format: FormatText ("text") or FormatJSON ("json")
	Format string
	// Output is the output destination: "stdout", "stderr", or empty (defaults to stdout)
	// Ignored if Writer is set.
	Output string
	// Writer is an optional custom io.Writer for log output.
	// If set, Output is ignored. Useful for testing (e.g., bytes.Buffer).
	Writer io.Writer
	// Component is the component name (e.g., "adapter", "sentinel")
	Component string
	// Version is the component version
	Version string
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Level:     "info",
		Format:    FormatJSON,
		Output:    "stdout",
		Component: "adapter",
		Version:   "unknown",
	}
}

// ConfigFromEnv creates a Config from environment variables with defaults
func ConfigFromEnv() Config {
	cfg := DefaultConfig()

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Level = strings.ToLower(level)
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Format = strings.ToLower(format)
	}
	if output := os.Getenv("LOG_OUTPUT"); output != "" {
		cfg.Output = output
	}

	return cfg
}

// NewLogger creates a new Logger with the given configuration
// Returns error if output is invalid (must be "stdout", "stderr", or empty)
func NewLogger(cfg Config) (Logger, error) {
	// Determine output writer
	var writer io.Writer
	if cfg.Writer != nil {
		// Use custom writer (e.g., for testing with bytes.Buffer)
		writer = cfg.Writer
	} else {
		// Use Output string config
		switch cfg.Output {
		case "stdout", "":
			writer = os.Stdout
		case "stderr":
			writer = os.Stderr
		default:
			return nil, fmt.Errorf("invalid log output %q: must be 'stdout', 'stderr', or empty", cfg.Output)
		}
	}

	// Parse log level
	level := parseLevel(cfg.Level)

	// Create handler options
	opts := &slog.HandlerOptions{
		Level: level,
		// Add source location for error level only
		AddSource: false,
	}

	// Create handler based on format
	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case FormatJSON:
		handler = slog.NewJSONHandler(writer, opts)
	case FormatText:
		handler = slog.NewTextHandler(writer, opts)
	default:
		return nil, fmt.Errorf("invalid log format %q: must be %q or %q", cfg.Format, FormatJSON, FormatText)
	}

	// Get hostname
	hostname, _ := os.Hostname() //nolint:errcheck // fallback to alternatives below
	if hostname == "" {
		hostname = os.Getenv("POD_NAME")
	}
	if hostname == "" {
		hostname = "unknown"
	}

	// Create base logger with required fields (per logging spec)
	slogLogger := slog.New(handler).With(
		ComponentKey, cfg.Component,
		VersionKey, cfg.Version,
		HostnameKey, hostname,
	)

	return &logger{
		slog:      slogLogger,
		fields:    make(map[string]interface{}),
		component: cfg.Component,
		version:   cfg.Version,
		hostname:  hostname,
	}, nil
}

// parseLevel converts string level to slog.Level
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// buildArgs builds the slog args from fields and context
func (l *logger) buildArgs(ctx context.Context) []any {
	args := make([]any, 0, len(l.fields)*2+10)

	// Add fields from the logger
	for k, v := range l.fields {
		args = append(args, k, v)
	}

	// Extract all log fields from context (flat structure)
	if ctx != nil {
		if logFields, ok := ctx.Value(LogFieldsKey).(LogFields); ok {
			for k, v := range logFields {
				args = append(args, k, v)
			}
		}
	}

	return args
}

// Debug logs at debug level
func (l *logger) Debug(ctx context.Context, message string) {
	l.slog.DebugContext(ctx, message, l.buildArgs(ctx)...)
}

// Debugf logs at debug level with formatting
func (l *logger) Debugf(ctx context.Context, format string, args ...interface{}) {
	l.slog.DebugContext(ctx, fmt.Sprintf(format, args...), l.buildArgs(ctx)...)
}

// Info logs at info level
func (l *logger) Info(ctx context.Context, message string) {
	l.slog.InfoContext(ctx, message, l.buildArgs(ctx)...)
}

// Infof logs at info level with formatting
func (l *logger) Infof(ctx context.Context, format string, args ...interface{}) {
	l.slog.InfoContext(ctx, fmt.Sprintf(format, args...), l.buildArgs(ctx)...)
}

// Warn logs at warn level
func (l *logger) Warn(ctx context.Context, message string) {
	l.slog.WarnContext(ctx, message, l.buildArgs(ctx)...)
}

// Warnf logs at warn level with formatting
func (l *logger) Warnf(ctx context.Context, format string, args ...interface{}) {
	l.slog.WarnContext(ctx, fmt.Sprintf(format, args...), l.buildArgs(ctx)...)
}

// Error logs at error level
func (l *logger) Error(ctx context.Context, message string) {
	l.slog.ErrorContext(ctx, message, l.buildArgs(ctx)...)
}

// Errorf logs at error level with formatting
func (l *logger) Errorf(ctx context.Context, format string, args ...interface{}) {
	l.slog.ErrorContext(ctx, fmt.Sprintf(format, args...), l.buildArgs(ctx)...)
}

// Fatal logs at error level and exits
func (l *logger) Fatal(ctx context.Context, message string) {
	l.slog.ErrorContext(ctx, message, l.buildArgs(ctx)...)
	os.Exit(1)
}

// copyFields creates a shallow copy of the fields map
func copyFields(f map[string]interface{}) map[string]interface{} {
	if f == nil {
		return make(map[string]interface{})
	}
	newFields := make(map[string]interface{}, len(f))
	for k, v := range f {
		newFields[k] = v
	}
	return newFields
}

// With returns a new logger with an additional field
func (l *logger) With(key string, value interface{}) Logger {
	newFields := copyFields(l.fields)
	newFields[key] = value
	return &logger{
		slog:      l.slog,
		fields:    newFields,
		component: l.component,
		version:   l.version,
		hostname:  l.hostname,
	}
}

// WithFields returns a new logger with multiple additional fields
func (l *logger) WithFields(fields map[string]interface{}) Logger {
	newFields := copyFields(l.fields)
	for k, v := range fields {
		newFields[k] = v
	}
	return &logger{
		slog:      l.slog,
		fields:    newFields,
		component: l.component,
		version:   l.version,
		hostname:  l.hostname,
	}
}

// Without returns a new logger with the specified field removed.
// If the field doesn't exist, returns a new logger with the same fields.
func (l *logger) Without(key string) Logger {
	newFields := copyFields(l.fields)
	delete(newFields, key)
	return &logger{
		slog:      l.slog,
		fields:    newFields,
		component: l.component,
		version:   l.version,
		hostname:  l.hostname,
	}
}
