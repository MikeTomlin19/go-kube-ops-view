package errors

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationErrorHandling tests the complete error handling flow
func TestIntegrationErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a simple logger that captures output
	var logOutput bytes.Buffer
	logger := &testLogger{output: &logOutput}

	// Create error handler
	handler := NewErrorHandler(logger)

	// Create error middleware
	middleware := NewErrorMiddleware(handler)

	// Create Gin router with middleware
	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.Handle())

	// Add test routes
	router.GET("/success", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	router.GET("/app-error", func(c *gin.Context) {
		err := NewClusterConnectionError("test-cluster", assert.AnError)
		c.Error(err)
	})

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	router.GET("/regular-error", func(c *gin.Context) {
		c.Error(assert.AnError)
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedCode   string
		expectLog      bool
	}{
		{
			name:           "successful request",
			path:           "/success",
			expectedStatus: http.StatusOK,
			expectLog:      false,
		},
		{
			name:           "app error handling",
			path:           "/app-error",
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   string(ErrCodeClusterConnection),
			expectLog:      true,
		},
		{
			name:           "panic recovery",
			path:           "/panic",
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   string(ErrCodeHTTPInternalError),
			expectLog:      true,
		},
		{
			name:           "regular error conversion",
			path:           "/regular-error",
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   string(ErrCodeUnknown),
			expectLog:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset log output
			logOutput.Reset()

			// Make request
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", tt.path, nil)
			router.ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Check response for errors
			if tt.expectedCode != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				errorData, ok := response["error"].(map[string]interface{})
				require.True(t, ok, "Response should contain error object")

				assert.Equal(t, tt.expectedCode, errorData["code"])
				assert.NotEmpty(t, errorData["message"])
				assert.NotEmpty(t, errorData["timestamp"])
			}

			// Check logging
			if tt.expectLog {
				assert.NotEmpty(t, logOutput.String(), "Should have logged the error")
			}
		})
	}
}

// TestRetryWithBackoffIntegration tests the retry functionality
func TestRetryWithBackoffIntegration(t *testing.T) {
	var logOutput bytes.Buffer
	logger := &testLogger{output: &logOutput}
	handler := NewErrorHandler(logger)

	t.Run("successful retry", func(t *testing.T) {
		logOutput.Reset()

		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 3 {
				return NewAppError(ErrCodeClusterTimeout, "temporary failure")
			}
			return nil
		}

		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Millisecond,
			MaxDelay:    10 * time.Millisecond,
			Multiplier:  2.0,
		}

		ctx := context.Background()
		err := handler.RetryWithBackoff(ctx, config, operation)

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
		assert.Contains(t, logOutput.String(), "Retrying operation")
	})

	t.Run("non-retryable error", func(t *testing.T) {
		logOutput.Reset()

		attempts := 0
		operation := func() error {
			attempts++
			return NewAppError(ErrCodeAuthInvalid, "auth failed")
		}

		config := DefaultRetryConfig()
		ctx := context.Background()
		err := handler.RetryWithBackoff(ctx, config, operation)

		assert.Error(t, err)
		assert.True(t, IsAuthError(err))
		assert.Equal(t, 1, attempts)
		assert.Contains(t, logOutput.String(), "not retryable")
	})
}

// testLogger implements the Logger interface for testing
type testLogger struct {
	output *bytes.Buffer
}

func (l *testLogger) Error(msg string, args ...interface{}) {
	l.output.WriteString("ERROR: " + msg + "\n")
}

func (l *testLogger) Warn(msg string, args ...interface{}) {
	l.output.WriteString("WARN: " + msg + "\n")
}

func (l *testLogger) Info(msg string, args ...interface{}) {
	l.output.WriteString("INFO: " + msg + "\n")
}

func (l *testLogger) Debug(msg string, args ...interface{}) {
	l.output.WriteString("DEBUG: " + msg + "\n")
}

func (l *testLogger) WithError(err error) Logger {
	return l // Simplified for testing
}

func (l *testLogger) WithFields(fields map[string]interface{}) Logger {
	return l // Simplified for testing
}
