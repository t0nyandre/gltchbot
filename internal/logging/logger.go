package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Logger is a structured logger instance.
var (
	defaultLogger *slog.Logger
	once          sync.Once
)

// Level represents the log level.
type Level string

const (
	// LevelDebug logs debug, info, warn, and error messages.
	LevelDebug Level = "debug"
	// LevelInfo logs info, warn, and error messages (default).
	LevelInfo Level = "info"
	// LevelWarn logs warn and error messages.
	LevelWarn Level = "warn"
	// LevelError logs only error messages.
	LevelError Level = "error"
)

// Config holds logging configuration.
type Config struct {
	// Level is the minimum log level to output.
	Level Level
	// Format is the output format: "text" or "json".
	Format string
	// AddSource includes source file and line information.
	AddSource bool
	// Output is the writer to write logs to (default: os.Stderr).
	Output io.Writer
}

// DefaultConfig returns the default logging configuration.
// Uses INFO level, text format for development, JSON for production.
func DefaultConfig() Config {
	// Use text format in development (when DISCORD_DEV_GUILD_ID is set)
	// and JSON format in production for better log aggregation.
	format := "json"
	if os.Getenv("DISCORD_DEV_GUILD_ID") != "" {
		format = "text"
	}

	return Config{
		Level:     LevelInfo,
		Format:    format,
		AddSource: false,
		Output:    os.Stderr,
	}
}

// Init initializes the default logger with the given configuration.
// If called multiple times, only the first call applies.
func Init(cfg Config) {
	once.Do(func() {
		defaultLogger = newLogger(cfg)
	})
}

// newLogger creates a new slog.Logger with the given configuration.
func newLogger(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(cfg.Output, opts)
	case "json":
		handler = slog.NewJSONHandler(cfg.Output, opts)
	default:
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	return slog.New(handler)
}

// parseLevel converts a Level string to slog.Level.
func parseLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// L returns the default logger.
// If the logger hasn't been initialized, it's initialized with DefaultConfig.
func L() *slog.Logger {
	if defaultLogger == nil {
		Init(DefaultConfig())
	}
	return defaultLogger
}

// With returns a new logger with the given attributes.
func With(args ...any) *slog.Logger {
	return L().With(args...)
}

// Debug logs at Debug level.
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs at Info level.
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs at Warn level.
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs at Error level.
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// Errorf logs a formatted error at Error level.
func Errorf(format string, args ...any) {
	L().Error(fmt.Sprintf(format, args...))
}

// Fatal logs at Error level and exits with status 1.
// Use for CLI tools where immediate exit is required.
func Fatal(msg string, args ...any) {
	L().Error(msg, args...)
	os.Exit(1)
}

// Fatalf logs a formatted error at Error level and exits with status 1.
func Fatalf(format string, args ...any) {
	L().Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

// ContextKey is the key used to store the logger in a context.
type ContextKey struct{}

// WithContext returns a context with the logger attached.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ContextKey{}, logger)
}

// FromContext returns the logger from the context, or the default logger if not found.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ContextKey{}).(*slog.Logger); ok {
		return logger
	}
	return L()
}

// ModuleLogger returns a logger with the module name attached.
func ModuleLogger(module string) *slog.Logger {
	return L().With("module", module)
}

// RequestLogger returns a logger for HTTP requests with request ID attached.
// If requestID is empty, it generates a new one.
func RequestLogger(requestID string) *slog.Logger {
	if requestID == "" {
		requestID = GenerateRequestID()
	}
	return L().With("request_id", requestID)
}

// GenerateRequestID generates a simple request ID.
// In production, you might want to use a proper UUID or similar.
func GenerateRequestID() string {
	return fmt.Sprintf("req_%d", os.Getpid())
}