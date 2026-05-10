package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HTTPMiddleware creates a Gin middleware for collecting HTTP metrics
func HTTPMiddleware(metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		statusCode := strconv.Itoa(c.Writer.Status())

		// Normalize path to avoid high cardinality
		path := normalizePath(c.FullPath())

		metrics.RecordHTTPRequest(c.Request.Method, path, statusCode, duration)
	}
}

// normalizePath normalizes URL paths to reduce metric cardinality
func normalizePath(path string) string {
	// If path is empty (e.g., for 404s), use a generic label
	if path == "" {
		return "unknown"
	}

	// For paths with parameters, the Gin framework should already
	// provide the route pattern (e.g., "/api/clusters/:id")
	// which is what we want for metrics
	return path
}
