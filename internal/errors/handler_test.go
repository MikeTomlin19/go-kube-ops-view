package errors

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger implements the Logger interface for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	callArgs := []interface{}{msg}
	callArgs = append(callArgs, args...)
	m.Called(callArgs...)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	callArgs := []interface{}{msg}
	callArgs = append(callArgs, args...)
	m.Called(callArgs...)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	callArgs := []interface{}{msg}
	callArgs = append(callArgs, args...)
	m.Called(callArgs...)
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	callArgs := []interface{}{msg}
	callArgs = append(callArgs, args...)
	m.Called(callArgs...)
}

func (m *MockLogger) WithError(err error) Logger {
	args := m.Called(err)
	return args.Get(0).(Logger)
}

func (m *MockLogger) WithFields(fields map[string]interface{}) Logger {
	args := m.Called(fields)
	return args.Get(0).(Logger)
}

func TestNewErrorHandler(t *testing.T) {
	logger := &MockLogger{}
	handler := NewErrorHandler(logger)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
}

func TestErrorHandler_HandleError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   ErrorCode
	}{
		{
			name:           "nil error does nothing",
			err:            nil,
			expectedStatus: 200, // Gin default status
		},
		{
			name:           "AppError is handled correctly",
			err:            NewAppError(ErrCodeClusterConnection, "Connection failed"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   ErrCodeClusterConnection,
		},
		{
			name:           "regular error is converted to AppError",
			err:            errors.New("regular error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   ErrCodeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			logger := &MockLogger{}
			handler := NewErrorHandler(logger)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/test", nil)

			// Setup logger expectations
			if tt.err != nil {
				logger.On("WithError", mock.AnythingOfType("*errors.AppError")).Return(logger)
				logger.On("WithFields", mock.AnythingOfType("map[string]interface {}")).Return(logger)

				if tt.expectedStatus >= 500 {
					logger.On("Error", mock.Anything, mock.Anything).Return()
				} else if tt.expectedStatus >= 400 {
					logger.On("Warn", mock.Anything, mock.Anything).Return()
				} else {
					logger.On("Info", mock.Anything, mock.Anything).Return()
				}
			}

			// Execute
			handler.HandleError(c, tt.err)

			// Verify
			if tt.err == nil {
				assert.Equal(t, 200, w.Code) // Gin default status
			} else {
				assert.Equal(t, tt.expectedStatus, w.Code)

				// Verify response contains error code
				if tt.expectedCode != "" {
					assert.Contains(t, w.Body.String(), string(tt.expectedCode))
				}
			}

			logger.AssertExpectations(t)
		})
	}
}

func TestErrorHandler_HandleClusterError(t *testing.T) {
	tests := []struct {
		name        string
		clusterID   string
		err         error
		expectNil   bool
		expectError bool
	}{
		{
			name:      "nil error returns nil",
			clusterID: "test-cluster",
			err:       nil,
			expectNil: true,
		},
		{
			name:        "connection error is returned for marking unavailable",
			clusterID:   "test-cluster",
			err:         NewAppError(ErrCodeClusterConnection, "Connection failed"),
			expectError: true,
		},
		{
			name:        "auth error is returned for disabling cluster",
			clusterID:   "test-cluster",
			err:         NewAppError(ErrCodeClusterAuth, "Auth failed"),
			expectError: true,
		},
		{
			name:      "API error returns nil for graceful degradation",
			clusterID: "test-cluster",
			err:       NewAppError(ErrCodeClusterAPIError, "API error"),
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}
			handler := NewErrorHandler(logger)

			// Setup logger expectations
			if tt.err != nil {
				logger.On("WithError", mock.AnythingOfType("*errors.AppError")).Return(logger)
				logger.On("WithFields", mock.AnythingOfType("map[string]interface {}")).Return(logger)
				logger.On("Error", mock.Anything).Return()

				switch {
				case IsClusterError(tt.err) && tt.err.(*AppError).Code == ErrCodeClusterAPIError:
					logger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Return()
				case IsClusterError(tt.err) && tt.err.(*AppError).Code == ErrCodeClusterAuth:
					logger.On("Error", mock.Anything, mock.Anything, mock.Anything).Return()
				case IsClusterError(tt.err):
					logger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Return()
				}
			}

			result := handler.HandleClusterError(tt.clusterID, tt.err)

			if tt.expectNil {
				assert.Nil(t, result)
			} else if tt.expectError {
				assert.NotNil(t, result)
				assert.True(t, IsClusterError(result))
			}

			logger.AssertExpectations(t)
		})
	}
}

func TestErrorHandler_HandleStoreError(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		err         error
		expectNil   bool
		expectError bool
	}{
		{
			name:      "nil error returns nil",
			operation: "get",
			err:       nil,
			expectNil: true,
		},
		{
			name:        "connection error is returned for fallback",
			operation:   "set",
			err:         NewAppError(ErrCodeStoreConnection, "Connection failed"),
			expectError: true,
		},
		{
			name:        "unavailable error is returned for read-only mode",
			operation:   "delete",
			err:         NewAppError(ErrCodeStoreUnavailable, "Store unavailable"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}
			handler := NewErrorHandler(logger)

			// Setup logger expectations
			if tt.err != nil {
				logger.On("WithError", mock.AnythingOfType("*errors.AppError")).Return(logger)
				logger.On("WithFields", mock.AnythingOfType("map[string]interface {}")).Return(logger)
				logger.On("Error", mock.Anything).Return()

				switch {
				case IsStoreError(tt.err) && (tt.err.(*AppError).Code == ErrCodeStoreConnection || tt.err.(*AppError).Code == ErrCodeStoreTimeout):
					logger.On("Warn", mock.Anything).Return()
				case IsStoreError(tt.err) && tt.err.(*AppError).Code == ErrCodeStoreUnavailable:
					logger.On("Warn", mock.Anything).Return()
				}
			}

			result := handler.HandleStoreError(tt.operation, tt.err)

			if tt.expectNil {
				assert.Nil(t, result)
			} else if tt.expectError {
				assert.NotNil(t, result)
				assert.True(t, IsStoreError(result))
			}

			logger.AssertExpectations(t)
		})
	}
}

func TestErrorHandler_InferErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: ErrCodeClusterConnection,
		},
		{
			name:     "timeout error",
			err:      errors.New("context deadline exceeded"),
			expected: ErrCodeClusterTimeout,
		},
		{
			name:     "unauthorized error",
			err:      errors.New("unauthorized access"),
			expected: ErrCodeAuthInvalid,
		},
		{
			name:     "forbidden error",
			err:      errors.New("access denied"),
			expected: ErrCodeAuthForbidden,
		},
		{
			name:     "not found error",
			err:      errors.New("resource not found"),
			expected: ErrCodeResourceNotFound,
		},
		{
			name:     "redis error",
			err:      errors.New("redis connection failed"),
			expected: ErrCodeStoreConnection,
		},
		{
			name:     "unknown error",
			err:      errors.New("some unknown error"),
			expected: ErrCodeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &MockLogger{}
			handler := NewErrorHandler(logger)

			result := handler.inferErrorCode(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorHandler_RetryWithBackoff(t *testing.T) {
	t.Run("successful operation on first attempt", func(t *testing.T) {
		logger := &MockLogger{}
		handler := NewErrorHandler(logger)

		attempts := 0
		operation := func() error {
			attempts++
			return nil
		}

		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
			Multiplier:  2.0,
		}

		ctx := context.Background()
		err := handler.RetryWithBackoff(ctx, config, operation)

		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("successful operation on second attempt", func(t *testing.T) {
		logger := &MockLogger{}
		handler := NewErrorHandler(logger)

		// Setup logger expectations for retry
		logger.On("Debug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		attempts := 0
		operation := func() error {
			attempts++
			if attempts == 1 {
				return NewAppError(ErrCodeClusterTimeout, "temporary error")
			}
			return nil
		}

		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
			Multiplier:  2.0,
		}

		ctx := context.Background()
		err := handler.RetryWithBackoff(ctx, config, operation)

		assert.NoError(t, err)
		assert.Equal(t, 2, attempts)
		logger.AssertExpectations(t)
	})

	t.Run("non-retryable error stops immediately", func(t *testing.T) {
		logger := &MockLogger{}
		handler := NewErrorHandler(logger)

		// Setup logger expectations
		logger.On("Debug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		attempts := 0
		operation := func() error {
			attempts++
			return NewAppError(ErrCodeAuthInvalid, "Authentication failed")
		}

		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
			Multiplier:  2.0,
		}

		ctx := context.Background()
		err := handler.RetryWithBackoff(ctx, config, operation)

		assert.Error(t, err)
		assert.Equal(t, 1, attempts)
		assert.True(t, IsAuthError(err))
		logger.AssertExpectations(t)
	})

	t.Run("context cancellation stops retry", func(t *testing.T) {
		logger := &MockLogger{}
		handler := NewErrorHandler(logger)

		// Allow any debug calls
		logger.On("Debug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

		attempts := 0
		operation := func() error {
			attempts++
			return NewAppError(ErrCodeClusterTimeout, "timeout error")
		}

		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			Multiplier:  2.0,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := handler.RetryWithBackoff(ctx, config, operation)

		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Equal(t, 1, attempts)

		logger.AssertExpectations(t)
	})

	t.Run("all attempts exhausted", func(t *testing.T) {
		logger := &MockLogger{}
		handler := NewErrorHandler(logger)

		// Setup logger expectations for retries
		logger.On("Debug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Times(2)
		logger.On("Error", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		attempts := 0
		operation := func() error {
			attempts++
			return NewAppError(ErrCodeClusterTimeout, "Persistent error")
		}

		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Millisecond,
			MaxDelay:    10 * time.Millisecond,
			Multiplier:  2.0,
		}

		ctx := context.Background()
		err := handler.RetryWithBackoff(ctx, config, operation)

		assert.Error(t, err)
		assert.Equal(t, 3, attempts)
		assert.True(t, IsClusterError(err))
		logger.AssertExpectations(t)
	})
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.BaseDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
}

func TestContains(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		substrings []string
		expected   bool
	}{
		{
			name:       "single match",
			s:          "connection refused",
			substrings: []string{"connection"},
			expected:   true,
		},
		{
			name:       "multiple substrings, one matches",
			s:          "timeout occurred",
			substrings: []string{"connection", "timeout"},
			expected:   true,
		},
		{
			name:       "no match",
			s:          "some error",
			substrings: []string{"connection", "timeout"},
			expected:   false,
		},
		{
			name:       "empty string",
			s:          "",
			substrings: []string{"test"},
			expected:   false,
		},
		{
			name:       "empty substring",
			s:          "test string",
			substrings: []string{""},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substrings...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
