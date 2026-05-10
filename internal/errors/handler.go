package errors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorHandler provides centralized error handling functionality
type ErrorHandler struct {
	logger Logger
}

// Logger interface for error handler
type Logger interface {
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	WithError(err error) Logger
	WithFields(fields map[string]interface{}) Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// HandleError processes an error and returns appropriate response
func (h *ErrorHandler) HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	// Convert to AppError if not already
	appErr := h.convertToAppError(err)

	// Log the error with context
	h.logError(c, appErr)

	// Send HTTP response
	h.sendErrorResponse(c, appErr)
}

// HandleClusterError handles cluster-specific errors with graceful degradation
func (h *ErrorHandler) HandleClusterError(clusterID string, err error) error {
	if err == nil {
		return nil
	}

	appErr := h.convertToAppError(err)

	// Add cluster context if not already present
	if appErr.Context == nil || appErr.Context["cluster_id"] == nil {
		appErr = appErr.WithCluster(clusterID)
	}

	// Log cluster error
	h.logger.WithError(appErr).WithFields(map[string]interface{}{
		"cluster_id": clusterID,
		"error_code": appErr.Code,
		"retryable":  appErr.Retryable,
	}).Error("Cluster operation failed")

	// Apply graceful degradation strategies
	return h.applyClusterErrorStrategy(appErr)
}

// HandleStoreError handles store-specific errors with fallback strategies
func (h *ErrorHandler) HandleStoreError(operation string, err error) error {
	if err == nil {
		return nil
	}

	appErr := h.convertToAppError(err)
	appErr = appErr.WithContext("operation", operation)

	// Log store error
	h.logger.WithError(appErr).WithFields(map[string]interface{}{
		"operation":  operation,
		"error_code": appErr.Code,
		"retryable":  appErr.Retryable,
	}).Error("Store operation failed")

	// Apply store error strategies
	return h.applyStoreErrorStrategy(appErr)
}

// RecoverFromPanic recovers from panics and converts them to errors
func (h *ErrorHandler) RecoverFromPanic() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch x := r.(type) {
				case string:
					err = fmt.Errorf("panic: %s", x)
				case error:
					err = fmt.Errorf("panic: %w", x)
				default:
					err = fmt.Errorf("panic: %v", x)
				}

				appErr := NewAppErrorWithCause(ErrCodeHTTPInternalError, "Internal server error", err)
				h.HandleError(c, appErr)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// convertToAppError converts any error to AppError
func (h *ErrorHandler) convertToAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	// Try to infer error type from error message/type
	code := h.inferErrorCode(err)
	return NewAppErrorWithCause(code, "Operation failed", err)
}

// inferErrorCode attempts to infer error code from error content
func (h *ErrorHandler) inferErrorCode(err error) ErrorCode {
	errStr := err.Error()

	// Check for common error patterns
	switch {
	case contains(errStr, "connection refused", "connection timeout", "no route to host"):
		return ErrCodeClusterConnection
	case contains(errStr, "context deadline exceeded", "timeout"):
		return ErrCodeClusterTimeout
	case contains(errStr, "unauthorized", "authentication", "invalid credentials"):
		return ErrCodeAuthInvalid
	case contains(errStr, "forbidden", "access denied"):
		return ErrCodeAuthForbidden
	case contains(errStr, "not found"):
		return ErrCodeResourceNotFound
	case contains(errStr, "redis", "store", "database"):
		return ErrCodeStoreConnection
	default:
		return ErrCodeUnknown
	}
}

// logError logs the error with appropriate level and context
func (h *ErrorHandler) logError(c *gin.Context, appErr *AppError) {
	logger := h.logger.WithError(appErr).WithFields(map[string]interface{}{
		"error_code":  appErr.Code,
		"retryable":   appErr.Retryable,
		"http_status": appErr.GetHTTPStatus(),
		"method":      c.Request.Method,
		"path":        c.Request.URL.Path,
		"client_ip":   c.ClientIP(),
		"user_agent":  c.Request.UserAgent(),
	})

	// Add error context if available
	if appErr.Context != nil {
		logger = logger.WithFields(appErr.Context)
	}

	// Log at appropriate level based on error severity
	switch appErr.GetHTTPStatus() {
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		logger.Error("Request failed with server error")
	case http.StatusUnauthorized, http.StatusForbidden:
		logger.Warn("Request failed with authentication/authorization error")
	case http.StatusBadRequest, http.StatusNotFound:
		logger.Info("Request failed with client error")
	default:
		logger.Error("Request failed")
	}
}

// sendErrorResponse sends appropriate HTTP error response
func (h *ErrorHandler) sendErrorResponse(c *gin.Context, appErr *AppError) {
	status := appErr.GetHTTPStatus()

	// Create response payload
	response := gin.H{
		"error": gin.H{
			"code":      appErr.Code,
			"message":   appErr.Message,
			"timestamp": appErr.Timestamp.Format(time.RFC3339),
		},
	}

	// Add details if available (but not for production to avoid information leakage)
	if appErr.Details != "" && h.shouldIncludeDetails(status) {
		response["error"].(gin.H)["details"] = appErr.Details
	}

	// Add retry information for retryable errors
	if appErr.Retryable {
		response["error"].(gin.H)["retryable"] = true
		response["error"].(gin.H)["retry_after"] = h.getRetryAfter(appErr.Code)
	}

	c.JSON(status, response)
}

// applyClusterErrorStrategy applies graceful degradation for cluster errors
func (h *ErrorHandler) applyClusterErrorStrategy(appErr *AppError) error {
	switch appErr.Code {
	case ErrCodeClusterConnection, ErrCodeClusterTimeout:
		// For connection/timeout errors, mark cluster as unavailable but continue
		h.logger.Warn("Cluster marked as unavailable due to connection issues",
			"cluster_id", appErr.Context["cluster_id"])
		return appErr // Return error to mark cluster as unavailable

	case ErrCodeClusterAuth:
		// For auth errors, disable cluster temporarily
		h.logger.Error("Cluster authentication failed, disabling cluster",
			"cluster_id", appErr.Context["cluster_id"])
		return appErr

	case ErrCodeClusterAPIError:
		// For API errors, continue with degraded functionality
		h.logger.Warn("Cluster API error, continuing with limited data",
			"cluster_id", appErr.Context["cluster_id"])
		return nil // Don't propagate API errors, use cached data

	default:
		return appErr
	}
}

// applyStoreErrorStrategy applies fallback strategies for store errors
func (h *ErrorHandler) applyStoreErrorStrategy(appErr *AppError) error {
	switch appErr.Code {
	case ErrCodeStoreConnection, ErrCodeStoreTimeout:
		// For store connection issues, fall back to memory store
		h.logger.Warn("Store connection failed, falling back to memory store")
		return appErr // Let caller handle fallback

	case ErrCodeStoreUnavailable:
		// For unavailable store, continue with read-only mode
		h.logger.Warn("Store unavailable, operating in read-only mode")
		return appErr

	default:
		return appErr
	}
}

// shouldIncludeDetails determines if error details should be included in response
func (h *ErrorHandler) shouldIncludeDetails(status int) bool {
	// Only include details for client errors, not server errors
	return status >= 400 && status < 500
}

// getRetryAfter returns appropriate retry delay for different error types
func (h *ErrorHandler) getRetryAfter(code ErrorCode) int {
	switch code {
	case ErrCodeClusterTimeout, ErrCodeStoreTimeout:
		return 30 // 30 seconds for timeout errors
	case ErrCodeClusterConnection, ErrCodeStoreConnection:
		return 60 // 1 minute for connection errors
	case ErrCodeSystemOverloaded:
		return 120 // 2 minutes for overload errors
	default:
		return 60 // Default 1 minute
	}
}

// contains checks if any of the substrings are present in the main string
func contains(s string, substrings ...string) bool {
	for _, substr := range substrings {
		if len(substr) > 0 && len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}
}

// RetryWithBackoff executes a function with exponential backoff retry
func (h *ErrorHandler) RetryWithBackoff(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			h.logger.Debug("Error is not retryable, stopping retry attempts",
				"error", err.Error(),
				"attempt", attempt)
			return err
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		exponentialFactor := 1 << uint(attempt-1)
		delay := time.Duration(float64(config.BaseDelay) * float64(exponentialFactor) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		h.logger.Debug("Retrying operation after delay",
			"attempt", attempt,
			"max_attempts", config.MaxAttempts,
			"delay", delay,
			"error", err.Error())

		// Wait for delay or context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	h.logger.Error("Operation failed after all retry attempts",
		"max_attempts", config.MaxAttempts,
		"final_error", lastErr.Error())

	return lastErr
}
