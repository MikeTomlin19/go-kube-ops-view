package errors

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents different types of application errors
type ErrorCode string

const (
	// Configuration errors
	ErrCodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
	ErrCodeConfigMissing    ErrorCode = "CONFIG_MISSING"
	ErrCodeConfigValidation ErrorCode = "CONFIG_VALIDATION"

	// Cluster connection errors
	ErrCodeClusterConnection  ErrorCode = "CLUSTER_CONNECTION"
	ErrCodeClusterAuth        ErrorCode = "CLUSTER_AUTH"
	ErrCodeClusterTimeout     ErrorCode = "CLUSTER_TIMEOUT"
	ErrCodeClusterUnavailable ErrorCode = "CLUSTER_UNAVAILABLE"
	ErrCodeClusterNotFound    ErrorCode = "CLUSTER_NOT_FOUND"
	ErrCodeClusterAPIError    ErrorCode = "CLUSTER_API_ERROR"

	// Authentication errors
	ErrCodeAuthInvalid       ErrorCode = "AUTH_INVALID"
	ErrCodeAuthExpired       ErrorCode = "AUTH_EXPIRED"
	ErrCodeAuthMissing       ErrorCode = "AUTH_MISSING"
	ErrCodeAuthForbidden     ErrorCode = "AUTH_FORBIDDEN"
	ErrCodeAuthProviderError ErrorCode = "AUTH_PROVIDER_ERROR"

	// Data store errors
	ErrCodeStoreConnection  ErrorCode = "STORE_CONNECTION"
	ErrCodeStoreTimeout     ErrorCode = "STORE_TIMEOUT"
	ErrCodeStoreNotFound    ErrorCode = "STORE_NOT_FOUND"
	ErrCodeStoreCorrupted   ErrorCode = "STORE_CORRUPTED"
	ErrCodeStoreUnavailable ErrorCode = "STORE_UNAVAILABLE"

	// HTTP/API errors
	ErrCodeHTTPBadRequest         ErrorCode = "HTTP_BAD_REQUEST"
	ErrCodeHTTPNotFound           ErrorCode = "HTTP_NOT_FOUND"
	ErrCodeHTTPMethodNotAllowed   ErrorCode = "HTTP_METHOD_NOT_ALLOWED"
	ErrCodeHTTPInternalError      ErrorCode = "HTTP_INTERNAL_ERROR"
	ErrCodeHTTPServiceUnavailable ErrorCode = "HTTP_SERVICE_UNAVAILABLE"

	// Resource errors
	ErrCodeResourceNotFound ErrorCode = "RESOURCE_NOT_FOUND"
	ErrCodeResourceInvalid  ErrorCode = "RESOURCE_INVALID"
	ErrCodeResourceConflict ErrorCode = "RESOURCE_CONFLICT"

	// System errors
	ErrCodeSystemUnavailable ErrorCode = "SYSTEM_UNAVAILABLE"
	ErrCodeSystemOverloaded  ErrorCode = "SYSTEM_OVERLOADED"
	ErrCodeSystemMaintenance ErrorCode = "SYSTEM_MAINTENANCE"

	// Unknown/Generic errors
	ErrCodeUnknown ErrorCode = "UNKNOWN"
)

// AppError represents an application-specific error with additional context
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Cause      error                  `json:"-"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Retryable  bool                   `json:"retryable"`
	HTTPStatus int                    `json:"http_status,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error
func (e *AppError) Is(target error) bool {
	if target == nil {
		return false
	}

	if appErr, ok := target.(*AppError); ok {
		return e.Code == appErr.Code
	}

	return errors.Is(e.Cause, target)
}

// WithContext adds context information to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithCluster adds cluster context to the error
func (e *AppError) WithCluster(clusterID string) *AppError {
	return e.WithContext("cluster_id", clusterID)
}

// WithHTTPStatus sets the HTTP status code for the error
func (e *AppError) WithHTTPStatus(status int) *AppError {
	e.HTTPStatus = status
	return e
}

// GetHTTPStatus returns the appropriate HTTP status code for the error
func (e *AppError) GetHTTPStatus() int {
	if e.HTTPStatus != 0 {
		return e.HTTPStatus
	}

	// Default HTTP status codes based on error code
	switch e.Code {
	case ErrCodeHTTPBadRequest, ErrCodeConfigInvalid, ErrCodeResourceInvalid:
		return http.StatusBadRequest
	case ErrCodeAuthMissing, ErrCodeAuthInvalid:
		return http.StatusUnauthorized
	case ErrCodeAuthForbidden:
		return http.StatusForbidden
	case ErrCodeHTTPNotFound, ErrCodeClusterNotFound, ErrCodeStoreNotFound, ErrCodeResourceNotFound:
		return http.StatusNotFound
	case ErrCodeHTTPMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case ErrCodeResourceConflict:
		return http.StatusConflict
	case ErrCodeClusterTimeout, ErrCodeStoreTimeout:
		return http.StatusRequestTimeout
	case ErrCodeSystemOverloaded:
		return http.StatusTooManyRequests
	case ErrCodeHTTPServiceUnavailable, ErrCodeClusterUnavailable, ErrCodeStoreUnavailable, ErrCodeSystemUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// NewAppErrorWithCause creates a new application error with a cause
func NewAppErrorWithCause(code ErrorCode, message string, cause error) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// NewAppErrorWithDetails creates a new application error with details
func NewAppErrorWithDetails(code ErrorCode, message, details string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// WrapError wraps an existing error as an AppError
func WrapError(code ErrorCode, message string, err error) *AppError {
	if err == nil {
		return nil
	}

	// If it's already an AppError, preserve the original
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	return &AppError{
		Code:      code,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Retryable: isRetryable(code),
	}
}

// isRetryable determines if an error type is retryable
func isRetryable(code ErrorCode) bool {
	switch code {
	case ErrCodeClusterTimeout, ErrCodeClusterConnection, ErrCodeClusterUnavailable,
		ErrCodeStoreTimeout, ErrCodeStoreConnection, ErrCodeStoreUnavailable,
		ErrCodeSystemOverloaded, ErrCodeSystemUnavailable:
		return true
	default:
		return false
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Retryable
	}
	return false
}

// IsClusterError checks if an error is cluster-related
func IsClusterError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrCodeClusterConnection, ErrCodeClusterAuth, ErrCodeClusterTimeout,
			ErrCodeClusterUnavailable, ErrCodeClusterNotFound, ErrCodeClusterAPIError:
			return true
		}
	}
	return false
}

// IsAuthError checks if an error is authentication-related
func IsAuthError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrCodeAuthInvalid, ErrCodeAuthExpired, ErrCodeAuthMissing,
			ErrCodeAuthForbidden, ErrCodeAuthProviderError:
			return true
		}
	}
	return false
}

// IsStoreError checks if an error is store-related
func IsStoreError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrCodeStoreConnection, ErrCodeStoreTimeout, ErrCodeStoreNotFound,
			ErrCodeStoreCorrupted, ErrCodeStoreUnavailable:
			return true
		}
	}
	return false
}

// Common error constructors for frequently used errors

// NewClusterConnectionError creates a cluster connection error
func NewClusterConnectionError(clusterID string, cause error) *AppError {
	return NewAppErrorWithCause(ErrCodeClusterConnection, "Failed to connect to cluster", cause).
		WithCluster(clusterID)
}

// NewClusterTimeoutError creates a cluster timeout error
func NewClusterTimeoutError(clusterID string, operation string) *AppError {
	return NewAppErrorWithDetails(ErrCodeClusterTimeout, "Cluster operation timed out", operation).
		WithCluster(clusterID)
}

// NewClusterUnavailableError creates a cluster unavailable error
func NewClusterUnavailableError(clusterID string) *AppError {
	return NewAppError(ErrCodeClusterUnavailable, "Cluster is unavailable").
		WithCluster(clusterID)
}

// NewAuthInvalidError creates an authentication invalid error
func NewAuthInvalidError(details string) *AppError {
	return NewAppErrorWithDetails(ErrCodeAuthInvalid, "Authentication failed", details)
}

// NewAuthExpiredError creates an authentication expired error
func NewAuthExpiredError() *AppError {
	return NewAppError(ErrCodeAuthExpired, "Authentication token has expired")
}

// NewStoreConnectionError creates a store connection error
func NewStoreConnectionError(cause error) *AppError {
	return NewAppErrorWithCause(ErrCodeStoreConnection, "Failed to connect to data store", cause)
}

// NewConfigValidationError creates a configuration validation error
func NewConfigValidationError(details string) *AppError {
	return NewAppErrorWithDetails(ErrCodeConfigValidation, "Configuration validation failed", details)
}

// NewHTTPError creates an HTTP error with appropriate status code
func NewHTTPError(status int, message string) *AppError {
	var code ErrorCode
	switch status {
	case http.StatusBadRequest:
		code = ErrCodeHTTPBadRequest
	case http.StatusNotFound:
		code = ErrCodeHTTPNotFound
	case http.StatusMethodNotAllowed:
		code = ErrCodeHTTPMethodNotAllowed
	case http.StatusServiceUnavailable:
		code = ErrCodeHTTPServiceUnavailable
	default:
		code = ErrCodeHTTPInternalError
	}

	return NewAppError(code, message).WithHTTPStatus(status)
}
