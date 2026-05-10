package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid json config",
			config: Config{
				Level:      LevelInfo,
				Format:     "json",
				Output:     "stdout",
				AddSource:  false,
				TimeFormat: "rfc3339",
			},
			expectError: false,
		},
		{
			name: "valid text config",
			config: Config{
				Level:      LevelDebug,
				Format:     "text",
				Output:     "stderr",
				AddSource:  true,
				TimeFormat: "unix",
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			config: Config{
				Level:  "invalid",
				Format: "json",
				Output: "stdout",
			},
			expectError: true,
		},
		{
			name: "invalid format",
			config: Config{
				Level:  LevelInfo,
				Format: "invalid",
				Output: "stdout",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
			}
		})
	}
}

func TestNewDefault(t *testing.T) {
	logger := NewDefault()

	assert.NotNil(t, logger)
	assert.Equal(t, slog.LevelInfo, logger.level)
}

func TestLogger_WithMethods(t *testing.T) {
	config := Config{
		Level:      LevelDebug,
		Format:     "json",
		Output:     "stdout",
		AddSource:  false,
		TimeFormat: "rfc3339",
	}

	// Create logger with custom writer for testing
	logger, err := New(config)
	require.NoError(t, err)

	// Test WithComponent
	componentLogger := logger.WithComponent("test-component")
	assert.NotNil(t, componentLogger)
	assert.NotSame(t, logger, componentLogger)

	// Test WithFields
	fieldsLogger := logger.WithFields(map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	})
	assert.NotNil(t, fieldsLogger)
	assert.NotSame(t, logger, fieldsLogger)

	// Test WithError
	testErr := assert.AnError
	errorLogger := logger.WithError(testErr)
	assert.NotNil(t, errorLogger)
	assert.NotSame(t, logger, errorLogger)

	// Test WithError with nil
	nilErrorLogger := logger.WithError(nil)
	assert.Same(t, logger, nilErrorLogger)

	// Test WithCluster
	clusterLogger := logger.WithCluster("test-cluster")
	assert.NotNil(t, clusterLogger)
	assert.NotSame(t, logger, clusterLogger)

	// Test WithRequest
	requestLogger := logger.WithRequest("GET", "/api/test", "test-agent", "127.0.0.1")
	assert.NotNil(t, requestLogger)
	assert.NotSame(t, logger, requestLogger)
}

func TestLogger_LevelChecks(t *testing.T) {
	tests := []struct {
		name         string
		level        LogLevel
		debugEnabled bool
		infoEnabled  bool
		warnEnabled  bool
		errorEnabled bool
	}{
		{
			name:         "debug level",
			level:        LevelDebug,
			debugEnabled: true,
			infoEnabled:  true,
			warnEnabled:  true,
			errorEnabled: true,
		},
		{
			name:         "info level",
			level:        LevelInfo,
			debugEnabled: false,
			infoEnabled:  true,
			warnEnabled:  true,
			errorEnabled: true,
		},
		{
			name:         "warn level",
			level:        LevelWarn,
			debugEnabled: false,
			infoEnabled:  false,
			warnEnabled:  true,
			errorEnabled: true,
		},
		{
			name:         "error level",
			level:        LevelError,
			debugEnabled: false,
			infoEnabled:  false,
			warnEnabled:  false,
			errorEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Level:      tt.level,
				Format:     "json",
				Output:     "stdout",
				AddSource:  false,
				TimeFormat: "rfc3339",
			}

			logger, err := New(config)
			require.NoError(t, err)

			assert.Equal(t, tt.debugEnabled, logger.IsDebugEnabled())
			assert.Equal(t, tt.infoEnabled, logger.IsInfoEnabled())
			assert.Equal(t, tt.warnEnabled, logger.IsWarnEnabled())
			assert.Equal(t, tt.errorEnabled, logger.IsErrorEnabled())
		})
	}
}

func TestLogger_LogClusterOperation(t *testing.T) {
	var buf bytes.Buffer

	// Create a logger that writes to our buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := &Logger{
		Logger: slog.New(handler),
		level:  slog.LevelDebug,
	}

	// Test successful operation
	logger.LogClusterOperation("test-cluster", "list-pods", 100*time.Millisecond, nil)

	output := buf.String()
	assert.Contains(t, output, "test-cluster")
	assert.Contains(t, output, "list-pods")
	assert.Contains(t, output, "Cluster operation completed")

	// Reset buffer
	buf.Reset()

	// Test failed operation
	testErr := assert.AnError
	logger.LogClusterOperation("test-cluster", "list-pods", 100*time.Millisecond, testErr)

	output = buf.String()
	assert.Contains(t, output, "test-cluster")
	assert.Contains(t, output, "list-pods")
	assert.Contains(t, output, "Cluster operation failed")
}

func TestLogger_LogHTTPRequest(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := &Logger{
		Logger: slog.New(handler),
		level:  slog.LevelDebug,
	}

	tests := []struct {
		name        string
		statusCode  int
		expectLevel string
	}{
		{
			name:        "success status",
			statusCode:  200,
			expectLevel: "INFO",
		},
		{
			name:        "client error status",
			statusCode:  404,
			expectLevel: "WARN",
		},
		{
			name:        "server error status",
			statusCode:  500,
			expectLevel: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			logger.LogHTTPRequest("GET", "/api/test", "test-agent", "127.0.0.1", tt.statusCode, 50*time.Millisecond)

			output := buf.String()
			assert.Contains(t, output, "GET")
			assert.Contains(t, output, "/api/test")
			assert.Contains(t, output, "127.0.0.1")
			assert.Contains(t, output, "HTTP request processed")

			// Parse JSON to check level
			var logEntry map[string]interface{}
			err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
			require.NoError(t, err)

			assert.Equal(t, tt.expectLevel, logEntry["level"])
		})
	}
}

func TestLogger_LogStartup(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := &Logger{
		Logger: slog.New(handler),
		level:  slog.LevelDebug,
	}

	config := map[string]interface{}{
		"port":  8080,
		"debug": true,
	}

	logger.LogStartup("v1.0.0", "abc123", "2023-01-01", config)

	output := buf.String()
	assert.Contains(t, output, "Application starting")
	assert.Contains(t, output, "v1.0.0")
	assert.Contains(t, output, "abc123")
	assert.Contains(t, output, "2023-01-01")
}

func TestLogger_LogShutdown(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := &Logger{
		Logger: slog.New(handler),
		level:  slog.LevelDebug,
	}

	logger.LogShutdown("SIGTERM", 5*time.Second)

	output := buf.String()
	assert.Contains(t, output, "Application shutting down")
	assert.Contains(t, output, "SIGTERM")
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name        string
		level       LogLevel
		expected    slog.Level
		expectError bool
	}{
		{
			name:     "debug level",
			level:    LevelDebug,
			expected: slog.LevelDebug,
		},
		{
			name:     "info level",
			level:    LevelInfo,
			expected: slog.LevelInfo,
		},
		{
			name:     "warn level",
			level:    LevelWarn,
			expected: slog.LevelWarn,
		},
		{
			name:     "error level",
			level:    LevelError,
			expected: slog.LevelError,
		},
		{
			name:     "warning alias",
			level:    "warning",
			expected: slog.LevelWarn,
		},
		{
			name:     "empty defaults to info",
			level:    "",
			expected: slog.LevelInfo,
		},
		{
			name:        "invalid level",
			level:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLevel(tt.level)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetCaller(t *testing.T) {
	file, line := GetCaller(0)

	assert.Contains(t, file, "logger_test.go")
	assert.Greater(t, line, 0)
}

func TestLogger_WithContext(t *testing.T) {
	logger := NewDefault()
	ctx := context.WithValue(context.Background(), "test", "value")

	contextLogger := logger.WithContext(ctx)

	assert.NotNil(t, contextLogger)
	assert.NotSame(t, logger, contextLogger)
}
