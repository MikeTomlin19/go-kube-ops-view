package errors

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorMiddleware provides error handling middleware for Gin
type ErrorMiddleware struct {
	handler *ErrorHandler
}

// NewErrorMiddleware creates a new error middleware
func NewErrorMiddleware(handler *ErrorHandler) *ErrorMiddleware {
	return &ErrorMiddleware{
		handler: handler,
	}
}

// Handle returns a Gin middleware function for error handling
func (m *ErrorMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process the request
		c.Next()

		// Check if there are any errors to handle
		if len(c.Errors) > 0 {
			// Handle the last error (most recent)
			err := c.Errors.Last().Err
			m.handler.HandleError(c, err)
		}
	}
}

// Recovery returns a Gin middleware function for panic recovery
func (m *ErrorMiddleware) Recovery() gin.HandlerFunc {
	return m.handler.RecoverFromPanic()
}

// TimeoutMiddleware creates middleware that enforces request timeouts
func (m *ErrorMiddleware) TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a context with timeout
		ctx, cancel := context.WithCancel(c.Request.Context())
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(c.Request.Context(), timeout)
		}
		defer cancel()

		// Replace request context
		c.Request = c.Request.WithContext(ctx)

		// Channel to signal completion
		done := make(chan struct{})

		// Run the request in a goroutine
		go func() {
			defer close(done)
			c.Next()
		}()

		// Wait for completion or timeout
		select {
		case <-done:
			// Request completed normally
			return
		case <-ctx.Done():
			// Request timed out
			if ctx.Err() != nil {
				err := NewAppErrorWithCause(ErrCodeClusterTimeout, "Request timeout", ctx.Err())
				m.handler.HandleError(c, err)
				c.Abort()
			}
		}
	}
}

// RateLimitMiddleware creates middleware for rate limiting
func (m *ErrorMiddleware) RateLimitMiddleware(maxRequests int, window time.Duration) gin.HandlerFunc {
	// Simple in-memory rate limiter (for production, use Redis-based solution)
	clients := make(map[string][]time.Time)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		// Clean old entries
		if requests, exists := clients[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range requests {
				if now.Sub(reqTime) < window {
					validRequests = append(validRequests, reqTime)
				}
			}
			clients[clientIP] = validRequests
		}

		// Check rate limit
		if len(clients[clientIP]) >= maxRequests {
			err := NewAppError(ErrCodeSystemOverloaded, "Rate limit exceeded")
			err = err.WithContext("client_ip", clientIP).
				WithContext("max_requests", maxRequests).
				WithContext("window", window.String())
			m.handler.HandleError(c, err)
			c.Abort()
			return
		}

		// Add current request
		clients[clientIP] = append(clients[clientIP], now)
		c.Next()
	}
}

// HealthCheckMiddleware creates middleware for health check endpoints
func (m *ErrorMiddleware) HealthCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip error handling for health check endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/healthz" {
			c.Next()
			return
		}

		// Apply normal error handling for other endpoints
		c.Next()

		// Handle any errors that occurred
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			m.handler.HandleError(c, err)
		}
	}
}

// CORSErrorMiddleware handles CORS-related errors
func (m *ErrorMiddleware) CORSErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set CORS headers
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// ValidationErrorMiddleware handles request validation errors
func (m *ErrorMiddleware) ValidationErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check for validation errors
		if len(c.Errors) > 0 {
			for _, ginErr := range c.Errors {
				if ginErr.Type == gin.ErrorTypeBind {
					// Convert binding errors to validation errors
					err := NewAppErrorWithCause(ErrCodeHTTPBadRequest, "Request validation failed", ginErr.Err)
					m.handler.HandleError(c, err)
					return
				}
			}
		}
	}
}

// SecurityHeadersMiddleware adds security headers and handles security-related errors
func (m *ErrorMiddleware) SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Add HSTS header for HTTPS
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

// LoggingMiddleware provides structured logging for requests
func (m *ErrorMiddleware) LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Get status code
		status := c.Writer.Status()

		// Build full path
		if raw != "" {
			path = path + "?" + raw
		}

		// Log request with appropriate level
		fields := map[string]interface{}{
			"method":     c.Request.Method,
			"path":       path,
			"status":     status,
			"duration":   duration,
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
			"size":       c.Writer.Size(),
		}

		// Add error information if present
		if len(c.Errors) > 0 {
			fields["error"] = c.Errors.String()
		}

		logger := m.handler.logger.WithFields(fields)

		switch {
		case status >= 500:
			logger.Error("HTTP request completed with server error")
		case status >= 400:
			logger.Warn("HTTP request completed with client error")
		case status >= 300:
			logger.Info("HTTP request completed with redirect")
		default:
			logger.Info("HTTP request completed successfully")
		}
	}
}

// NotFoundHandler handles 404 errors
func (m *ErrorMiddleware) NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := NewHTTPError(http.StatusNotFound, "Endpoint not found")
		err = err.WithContext("method", c.Request.Method).
			WithContext("path", c.Request.URL.Path)
		m.handler.HandleError(c, err)
	}
}

// MethodNotAllowedHandler handles 405 errors
func (m *ErrorMiddleware) MethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := NewHTTPError(http.StatusMethodNotAllowed, "Method not allowed")
		err = err.WithContext("method", c.Request.Method).
			WithContext("path", c.Request.URL.Path)
		m.handler.HandleError(c, err)
	}
}
