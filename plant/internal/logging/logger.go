// Package logging provides a structured logger interface for the plant simulator.
// It wraps the standard log/slog package with a consistent interface that supports
// leveled output, field enrichment, and test-friendly quiet mode.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Level represents the severity of a log message.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger is the structured logging interface used throughout the plant module.
// All plant components accept a Logger, enabling consistent log formatting and
// level control without coupling to a specific implementation.
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Fatal(msg string, fields ...any)
	WithFields(fields ...any) Logger
	WithLevel(level Level) Logger
}

// slogLogger is the standard Logger implementation backed by log/slog.
type slogLogger struct {
	handler slog.Handler
	level   slog.Level
	fields  []any
}

// NewLogger constructs a Logger that writes JSON-formatted records to stdout.
// Use this constructor for all production plant components.
func NewLogger() Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &slogLogger{
		handler: handler,
		level:   slog.LevelDebug,
	}
}

// NewTestLogger constructs a Logger that discards all output.
// Use this constructor in unit tests to suppress log noise.
func NewTestLogger() Logger {
	handler := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	return &slogLogger{
		handler: handler,
		level:   slog.LevelError,
	}
}

// Debug logs a message at debug severity with optional key-value fields.
func (l *slogLogger) Debug(msg string, fields ...any) {
	l.log(slog.LevelDebug, msg, fields...)
}

// Info logs a message at info severity with optional key-value fields.
func (l *slogLogger) Info(msg string, fields ...any) {
	l.log(slog.LevelInfo, msg, fields...)
}

// Warn logs a message at warn severity with optional key-value fields.
func (l *slogLogger) Warn(msg string, fields ...any) {
	l.log(slog.LevelWarn, msg, fields...)
}

// Error logs a message at error severity with optional key-value fields.
func (l *slogLogger) Error(msg string, fields ...any) {
	l.log(slog.LevelError, msg, fields...)
}

// Fatal logs a message at error severity then terminates the process.
// Use sparingly -- prefer returning errors to callers when possible.
func (l *slogLogger) Fatal(msg string, fields ...any) {
	l.log(slog.LevelError, msg, fields...)
	os.Exit(1)
}

// WithFields returns a new Logger that includes the given key-value fields
// on every subsequent log record. Fields accumulate across chained calls.
func (l *slogLogger) WithFields(fields ...any) Logger {
	merged := make([]any, len(l.fields)+len(fields))
	copy(merged, l.fields)
	copy(merged[len(l.fields):], fields)
	return &slogLogger{
		handler: l.handler,
		level:   l.level,
		fields:  merged,
	}
}

// WithLevel returns a new Logger with the minimum severity set to level.
// Records below this level are discarded without formatting overhead.
func (l *slogLogger) WithLevel(level Level) Logger {
	slogLevel := toSlogLevel(level)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})
	return &slogLogger{
		handler: handler,
		level:   slogLevel,
		fields:  l.fields,
	}
}

// log is the internal dispatch method that builds and emits the slog record.
func (l *slogLogger) log(level slog.Level, msg string, fields ...any) {
	if !l.handler.Enabled(context.Background(), level) {
		return
	}
	allFields := make([]any, len(l.fields)+len(fields))
	copy(allFields, l.fields)
	copy(allFields[len(l.fields):], fields)
	logger := slog.New(l.handler)
	logger.Log(context.Background(), level, msg, allFields...)
}

// toSlogLevel maps the package Level type to the slog equivalent.
func toSlogLevel(l Level) slog.Level {
	switch l {
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
