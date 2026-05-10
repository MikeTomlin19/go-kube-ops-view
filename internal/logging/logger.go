package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	level slog.Level
}

// Config holds logger configuration
type Config struct {
	Level      LogLevel `yaml:"level" env:"LOG_LEVEL" default:"info"`
	Format     string   `yaml:"format" env:"LOG_FORMAT" default:"json"`   // json or text
	Output     string   `yaml:"output" env:"LOG_OUTPUT" default:"stdout"` // stdout, stderr, or file path
	AddSource  bool     `yaml:"add_source" env:"LOG_ADD_SOURCE" default:"false"`
	TimeFormat string   `yaml:"time_format" env:"LOG_TIME_FORMAT" default:"rfc3339"`
}

// New creates a new logger with the given configuration
func New(config Config) (*Logger, error) {
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Determine output writer
	var writer io.Writer
	switch config.Output {
	case "stdout", "":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		// Assume it's a file path
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", config.Output, err)
		}
		writer = file
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format
			if a.Key == slog.TimeKey {
				switch config.TimeFormat {
				case "unix":
					return slog.Int64("time", a.Value.Time().Unix())
				case "rfc3339", "":
					return slog.String("time", a.Value.Time().Format(time.RFC3339))
				case "rfc3339nano":
					return slog.String("time", a.Value.Time().Format(time.RFC3339Nano))
				default:
					return slog.String("time", a.Value.Time().Format(config.TimeFormat))
				}
			}
			return a
		},
	}

	// Create handler based on format
	var handler slog.Handler
	switch config.Format {
	case "json", "":
		handler = slog.NewJSONHandler(writer, opts)
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		return nil, fmt.Errorf("unsupported log format: %s", config.Format)
	}

	logger := slog.New(handler)
	return &Logger{
		Logger: logger,
		level:  level,
	}, nil
}

// NewDefault creates a logger with default configuration
func NewDefault() *Logger {
	logger, _ := New(Config{
		Level:      LevelInfo,
		Format:     "json",
		Output:     "stdout",
		AddSource:  false,
		TimeFormat: "rfc3339",
	})
	return logger
}

// WithContext returns a logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.Any("context", ctx)),
		level:  l.level,
	}
}

// WithComponent returns a logger with a component field
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("component", component)),
		level:  l.level,
	}
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &Logger{
		Logger: l.Logger.With(args...),
		level:  l.level,
	}
}

// WithError returns a logger with an error field
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return &Logger{
		Logger: l.Logger.With(slog.String("error", err.Error())),
		level:  l.level,
	}
}

// WithCluster returns a logger with cluster information
func (l *Logger) WithCluster(clusterID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("cluster_id", clusterID)),
		level:  l.level,
	}
}

// WithRequest returns a logger with HTTP request information
func (l *Logger) WithRequest(method, path, userAgent, clientIP string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String("method", method),
			slog.String("path", path),
			slog.String("user_agent", userAgent),
			slog.String("client_ip", clientIP),
		),
		level: l.level,
	}
}

// IsDebugEnabled returns true if debug logging is enabled
func (l *Logger) IsDebugEnabled() bool {
	return l.level <= slog.LevelDebug
}

// IsInfoEnabled returns true if info logging is enabled
func (l *Logger) IsInfoEnabled() bool {
	return l.level <= slog.LevelInfo
}

// IsWarnEnabled returns true if warn logging is enabled
func (l *Logger) IsWarnEnabled() bool {
	return l.level <= slog.LevelWarn
}

// IsErrorEnabled returns true if error logging is enabled
func (l *Logger) IsErrorEnabled() bool {
	return l.level <= slog.LevelError
}

// LogClusterOperation logs cluster-related operations
func (l *Logger) LogClusterOperation(clusterID, operation string, duration time.Duration, err error) {
	logger := l.WithCluster(clusterID).With(
		slog.String("operation", operation),
		slog.Duration("duration", duration),
	)

	if err != nil {
		logger.Error("Cluster operation failed", slog.String("error", err.Error()))
	} else {
		logger.Info("Cluster operation completed")
	}
}

// LogHTTPRequest logs HTTP request information
func (l *Logger) LogHTTPRequest(method, path, userAgent, clientIP string, statusCode int, duration time.Duration) {
	logger := l.WithRequest(method, path, userAgent, clientIP).With(
		slog.Int("status_code", statusCode),
		slog.Duration("duration", duration),
	)

	level := slog.LevelInfo
	if statusCode >= 400 && statusCode < 500 {
		level = slog.LevelWarn
	} else if statusCode >= 500 {
		level = slog.LevelError
	}

	logger.Log(context.Background(), level, "HTTP request processed")
}

// LogStartup logs application startup information
func (l *Logger) LogStartup(version, commit, buildDate string, config interface{}) {
	l.Info("Application starting",
		slog.String("version", version),
		slog.String("commit", commit),
		slog.String("build_date", buildDate),
		slog.Any("config", config),
	)
}

// LogShutdown logs application shutdown information
func (l *Logger) LogShutdown(reason string, duration time.Duration) {
	l.Info("Application shutting down",
		slog.String("reason", reason),
		slog.Duration("shutdown_duration", duration),
	)
}

// parseLevel converts string level to slog.Level
func parseLevel(level LogLevel) (slog.Level, error) {
	switch strings.ToLower(string(level)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %s", level)
	}
}

// GetCaller returns the caller information
func GetCaller(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown", 0
	}

	// Get just the filename, not the full path
	parts := strings.Split(file, "/")
	if len(parts) > 0 {
		file = parts[len(parts)-1]
	}

	return file, line
}
