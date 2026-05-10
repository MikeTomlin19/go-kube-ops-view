package logging

import (
	"log/slog"
)

// ErrorHandlerAdapter adapts the logging.Logger to work with the error handler
type ErrorHandlerAdapter struct {
	logger *Logger
}

// NewErrorHandlerAdapter creates a new adapter for the error handler
func NewErrorHandlerAdapter(logger *Logger) *ErrorHandlerAdapter {
	return &ErrorHandlerAdapter{
		logger: logger,
	}
}

// Error logs an error message with variadic arguments
func (a *ErrorHandlerAdapter) Error(msg string, args ...interface{}) {
	if len(args) == 0 {
		a.logger.Error(msg)
	} else {
		// Convert args to slog.Attr pairs
		attrs := make([]interface{}, 0, len(args))
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				attrs = append(attrs, slog.Any(toString(args[i]), args[i+1]))
			}
		}
		a.logger.Error(msg, attrs...)
	}
}

// Warn logs a warning message with variadic arguments
func (a *ErrorHandlerAdapter) Warn(msg string, args ...interface{}) {
	if len(args) == 0 {
		a.logger.Warn(msg)
	} else {
		// Convert args to slog.Attr pairs
		attrs := make([]interface{}, 0, len(args))
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				attrs = append(attrs, slog.Any(toString(args[i]), args[i+1]))
			}
		}
		a.logger.Warn(msg, attrs...)
	}
}

// Info logs an info message with variadic arguments
func (a *ErrorHandlerAdapter) Info(msg string, args ...interface{}) {
	if len(args) == 0 {
		a.logger.Info(msg)
	} else {
		// Convert args to slog.Attr pairs
		attrs := make([]interface{}, 0, len(args))
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				attrs = append(attrs, slog.Any(toString(args[i]), args[i+1]))
			}
		}
		a.logger.Info(msg, attrs...)
	}
}

// Debug logs a debug message with variadic arguments
func (a *ErrorHandlerAdapter) Debug(msg string, args ...interface{}) {
	if len(args) == 0 {
		a.logger.Debug(msg)
	} else {
		// Convert args to slog.Attr pairs
		attrs := make([]interface{}, 0, len(args))
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				attrs = append(attrs, slog.Any(toString(args[i]), args[i+1]))
			}
		}
		a.logger.Debug(msg, attrs...)
	}
}

// WithError returns an adapter with error context
func (a *ErrorHandlerAdapter) WithError(err error) interface{} {
	return &ErrorHandlerAdapter{
		logger: a.logger.WithError(err),
	}
}

// WithFields returns an adapter with additional fields
func (a *ErrorHandlerAdapter) WithFields(fields map[string]interface{}) interface{} {
	return &ErrorHandlerAdapter{
		logger: a.logger.WithFields(fields),
	}
}

// toString converts an interface{} to string for use as a key
func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "unknown"
}
