package errors

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		expected string
	}{
		{
			name: "error with details",
			appError: &AppError{
				Code:    ErrCodeClusterConnection,
				Message: "Connection failed",
				Details: "timeout after 30s",
			},
			expected: "CLUSTER_CONNECTION: Connection failed (timeout after 30s)",
		},
		{
			name: "error without details",
			appError: &AppError{
				Code:    ErrCodeAuthInvalid,
				Message: "Authentication failed",
			},
			expected: "AUTH_INVALID: Authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.appError.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := NewAppErrorWithCause(ErrCodeClusterConnection, "Connection failed", originalErr)

	assert.Equal(t, originalErr, appErr.Unwrap())
}

func TestAppError_Is(t *testing.T) {
	originalErr := errors.New("original error")
	appErr1 := NewAppErrorWithCause(ErrCodeClusterConnection, "Connection failed", originalErr)
	appErr2 := NewAppError(ErrCodeClusterConnection, "Another connection error")
	appErr3 := NewAppError(ErrCodeAuthInvalid, "Auth error")

	// Test matching AppError codes
	assert.True(t, appErr1.Is(appErr2))
	assert.False(t, appErr1.Is(appErr3))

	// Test matching underlying errors
	assert.True(t, errors.Is(appErr1, originalErr))
}

func TestAppError_WithContext(t *testing.T) {
	appErr := NewAppError(ErrCodeClusterConnection, "Connection failed")

	result := appErr.WithContext("cluster_id", "test-cluster")

	assert.Equal(t, "test-cluster", result.Context["cluster_id"])
	assert.Same(t, appErr, result) // Should modify in place
}

func TestAppError_WithCluster(t *testing.T) {
	appErr := NewAppError(ErrCodeClusterConnection, "Connection failed")

	result := appErr.WithCluster("test-cluster")

	assert.Equal(t, "test-cluster", result.Context["cluster_id"])
}

func TestAppError_GetHTTPStatus(t *testing.T) {
	tests := []struct {
		name         string
		code         ErrorCode
		customStatus int
		expected     int
	}{
		{
			name:     "bad request error",
			code:     ErrCodeHTTPBadRequest,
			expected: http.StatusBadRequest,
		},
		{
			name:     "unauthorized error",
			code:     ErrCodeAuthInvalid,
			expected: http.StatusUnauthorized,
		},
		{
			name:     "forbidden error",
			code:     ErrCodeAuthForbidden,
			expected: http.StatusForbidden,
		},
		{
			name:     "not found error",
			code:     ErrCodeResourceNotFound,
			expected: http.StatusNotFound,
		},
		{
			name:     "timeout error",
			code:     ErrCodeClusterTimeout,
			expected: http.StatusRequestTimeout,
		},
		{
			name:     "service unavailable error",
			code:     ErrCodeClusterUnavailable,
			expected: http.StatusServiceUnavailable,
		},
		{
			name:     "unknown error defaults to internal server error",
			code:     ErrCodeUnknown,
			expected: http.StatusInternalServerError,
		},
		{
			name:         "custom status takes precedence",
			code:         ErrCodeUnknown,
			customStatus: http.StatusTeapot,
			expected:     http.StatusTeapot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := NewAppError(tt.code, "Test error")
			if tt.customStatus != 0 {
				appErr = appErr.WithHTTPStatus(tt.customStatus)
			}

			assert.Equal(t, tt.expected, appErr.GetHTTPStatus())
		})
	}
}

func TestNewAppError(t *testing.T) {
	appErr := NewAppError(ErrCodeClusterConnection, "Connection failed")

	assert.Equal(t, ErrCodeClusterConnection, appErr.Code)
	assert.Equal(t, "Connection failed", appErr.Message)
	assert.True(t, appErr.Retryable)
	assert.WithinDuration(t, time.Now(), appErr.Timestamp, time.Second)
}

func TestNewAppErrorWithCause(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := NewAppErrorWithCause(ErrCodeClusterConnection, "Connection failed", originalErr)

	assert.Equal(t, ErrCodeClusterConnection, appErr.Code)
	assert.Equal(t, "Connection failed", appErr.Message)
	assert.Equal(t, originalErr, appErr.Cause)
	assert.True(t, appErr.Retryable)
}

func TestNewAppErrorWithDetails(t *testing.T) {
	appErr := NewAppErrorWithDetails(ErrCodeClusterTimeout, "Operation timed out", "after 30 seconds")

	assert.Equal(t, ErrCodeClusterTimeout, appErr.Code)
	assert.Equal(t, "Operation timed out", appErr.Message)
	assert.Equal(t, "after 30 seconds", appErr.Details)
	assert.True(t, appErr.Retryable)
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected *AppError
	}{
		{
			name:     "nil error returns nil",
			err:      nil,
			expected: nil,
		},
		{
			name: "existing AppError is preserved",
			err:  NewAppError(ErrCodeClusterConnection, "Original error"),
			expected: &AppError{
				Code:    ErrCodeClusterConnection,
				Message: "Original error",
			},
		},
		{
			name: "regular error is wrapped",
			err:  errors.New("regular error"),
			expected: &AppError{
				Code:    ErrCodeUnknown,
				Message: "Operation failed",
				Cause:   errors.New("regular error"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapError(ErrCodeUnknown, "Operation failed", tt.err)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.expected.Code, result.Code)
			assert.Equal(t, tt.expected.Message, result.Message)

			if tt.expected.Cause != nil {
				assert.Equal(t, tt.expected.Cause.Error(), result.Cause.Error())
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable AppError",
			err:      NewAppError(ErrCodeClusterTimeout, "Timeout"),
			expected: true,
		},
		{
			name:     "non-retryable AppError",
			err:      NewAppError(ErrCodeAuthInvalid, "Invalid auth"),
			expected: false,
		},
		{
			name:     "regular error is not retryable",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRetryable(tt.err))
		})
	}
}

func TestIsClusterError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "cluster connection error",
			err:      NewAppError(ErrCodeClusterConnection, "Connection failed"),
			expected: true,
		},
		{
			name:     "cluster timeout error",
			err:      NewAppError(ErrCodeClusterTimeout, "Timeout"),
			expected: true,
		},
		{
			name:     "auth error is not cluster error",
			err:      NewAppError(ErrCodeAuthInvalid, "Invalid auth"),
			expected: false,
		},
		{
			name:     "regular error is not cluster error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsClusterError(tt.err))
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "auth invalid error",
			err:      NewAppError(ErrCodeAuthInvalid, "Invalid auth"),
			expected: true,
		},
		{
			name:     "auth expired error",
			err:      NewAppError(ErrCodeAuthExpired, "Expired"),
			expected: true,
		},
		{
			name:     "cluster error is not auth error",
			err:      NewAppError(ErrCodeClusterConnection, "Connection failed"),
			expected: false,
		},
		{
			name:     "regular error is not auth error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAuthError(tt.err))
		})
	}
}

func TestIsStoreError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "store connection error",
			err:      NewAppError(ErrCodeStoreConnection, "Connection failed"),
			expected: true,
		},
		{
			name:     "store timeout error",
			err:      NewAppError(ErrCodeStoreTimeout, "Timeout"),
			expected: true,
		},
		{
			name:     "auth error is not store error",
			err:      NewAppError(ErrCodeAuthInvalid, "Invalid auth"),
			expected: false,
		},
		{
			name:     "regular error is not store error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsStoreError(tt.err))
		})
	}
}

func TestCommonErrorConstructors(t *testing.T) {
	t.Run("NewClusterConnectionError", func(t *testing.T) {
		originalErr := errors.New("connection refused")
		appErr := NewClusterConnectionError("test-cluster", originalErr)

		assert.Equal(t, ErrCodeClusterConnection, appErr.Code)
		assert.Equal(t, "Failed to connect to cluster", appErr.Message)
		assert.Equal(t, originalErr, appErr.Cause)
		assert.Equal(t, "test-cluster", appErr.Context["cluster_id"])
		assert.True(t, appErr.Retryable)
	})

	t.Run("NewClusterTimeoutError", func(t *testing.T) {
		appErr := NewClusterTimeoutError("test-cluster", "list pods")

		assert.Equal(t, ErrCodeClusterTimeout, appErr.Code)
		assert.Equal(t, "Cluster operation timed out", appErr.Message)
		assert.Equal(t, "list pods", appErr.Details)
		assert.Equal(t, "test-cluster", appErr.Context["cluster_id"])
		assert.True(t, appErr.Retryable)
	})

	t.Run("NewAuthInvalidError", func(t *testing.T) {
		appErr := NewAuthInvalidError("invalid token")

		assert.Equal(t, ErrCodeAuthInvalid, appErr.Code)
		assert.Equal(t, "Authentication failed", appErr.Message)
		assert.Equal(t, "invalid token", appErr.Details)
		assert.False(t, appErr.Retryable)
	})

	t.Run("NewStoreConnectionError", func(t *testing.T) {
		originalErr := errors.New("redis connection failed")
		appErr := NewStoreConnectionError(originalErr)

		assert.Equal(t, ErrCodeStoreConnection, appErr.Code)
		assert.Equal(t, "Failed to connect to data store", appErr.Message)
		assert.Equal(t, originalErr, appErr.Cause)
		assert.True(t, appErr.Retryable)
	})

	t.Run("NewHTTPError", func(t *testing.T) {
		appErr := NewHTTPError(http.StatusBadRequest, "Invalid request")

		assert.Equal(t, ErrCodeHTTPBadRequest, appErr.Code)
		assert.Equal(t, "Invalid request", appErr.Message)
		assert.Equal(t, http.StatusBadRequest, appErr.HTTPStatus)
		assert.False(t, appErr.Retryable)
	})
}

func TestIsRetryableFunction(t *testing.T) {
	retryableCodes := []ErrorCode{
		ErrCodeClusterTimeout,
		ErrCodeClusterConnection,
		ErrCodeClusterUnavailable,
		ErrCodeStoreTimeout,
		ErrCodeStoreConnection,
		ErrCodeStoreUnavailable,
		ErrCodeSystemOverloaded,
		ErrCodeSystemUnavailable,
	}

	nonRetryableCodes := []ErrorCode{
		ErrCodeAuthInvalid,
		ErrCodeAuthForbidden,
		ErrCodeConfigInvalid,
		ErrCodeResourceNotFound,
		ErrCodeHTTPBadRequest,
	}

	for _, code := range retryableCodes {
		t.Run(string(code)+" should be retryable", func(t *testing.T) {
			assert.True(t, isRetryable(code))
		})
	}

	for _, code := range nonRetryableCodes {
		t.Run(string(code)+" should not be retryable", func(t *testing.T) {
			assert.False(t, isRetryable(code))
		})
	}
}
