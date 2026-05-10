package health

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
	StatusUnknown   Status = "unknown"
)

// Check represents a health check for a specific component
type Check struct {
	Name        string                 `json:"name"`
	Status      Status                 `json:"status"`
	Message     string                 `json:"message,omitempty"`
	LastChecked time.Time              `json:"last_checked"`
	Duration    time.Duration          `json:"duration"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// CheckFunc is a function that performs a health check
type CheckFunc func(ctx context.Context) Check

// Checker manages health checks for various components
type Checker struct {
	checks map[string]CheckFunc
}

// NewChecker creates a new health checker
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFunc),
	}
}

// AddCheck adds a health check for a component
func (c *Checker) AddCheck(name string, checkFunc CheckFunc) {
	c.checks[name] = checkFunc
}

// CheckAll runs all registered health checks
func (c *Checker) CheckAll(ctx context.Context) map[string]Check {
	results := make(map[string]Check)

	for name, checkFunc := range c.checks {
		start := time.Now()
		check := checkFunc(ctx)
		check.Duration = time.Since(start)
		check.LastChecked = time.Now()
		results[name] = check
	}

	return results
}

// GetOverallStatus determines the overall health status based on individual checks
func (c *Checker) GetOverallStatus(checks map[string]Check) Status {
	if len(checks) == 0 {
		return StatusUnknown
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, check := range checks {
		switch check.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		case StatusUnknown:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}

	return StatusHealthy
}

// HealthResponse represents the complete health check response
type HealthResponse struct {
	Status    Status           `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Uptime    time.Duration    `json:"uptime"`
	Version   string           `json:"version,omitempty"`
	Checks    map[string]Check `json:"checks"`
	Summary   map[string]int   `json:"summary"`
}

// CreateHealthResponse creates a complete health response
func (c *Checker) CreateHealthResponse(ctx context.Context, startTime time.Time, version string) HealthResponse {
	checks := c.CheckAll(ctx)
	overallStatus := c.GetOverallStatus(checks)

	// Create summary
	summary := map[string]int{
		"total":     len(checks),
		"healthy":   0,
		"unhealthy": 0,
		"degraded":  0,
		"unknown":   0,
	}

	for _, check := range checks {
		switch check.Status {
		case StatusHealthy:
			summary["healthy"]++
		case StatusUnhealthy:
			summary["unhealthy"]++
		case StatusDegraded:
			summary["degraded"]++
		case StatusUnknown:
			summary["unknown"]++
		}
	}

	return HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Uptime:    time.Since(startTime),
		Version:   version,
		Checks:    checks,
		Summary:   summary,
	}
}

// Common health check functions

// DatabaseCheck creates a health check for database connectivity
func DatabaseCheck(pingFunc func(ctx context.Context) error) CheckFunc {
	return func(ctx context.Context) Check {
		check := Check{
			Name: "database",
		}

		if err := pingFunc(ctx); err != nil {
			check.Status = StatusUnhealthy
			check.Message = fmt.Sprintf("Database connection failed: %v", err)
		} else {
			check.Status = StatusHealthy
			check.Message = "Database connection successful"
		}

		return check
	}
}

// ClusterCheck creates a health check for cluster connectivity
func ClusterCheck(name string, checkFunc func(ctx context.Context) (bool, string, map[string]interface{})) CheckFunc {
	return func(ctx context.Context) Check {
		check := Check{
			Name: fmt.Sprintf("cluster_%s", name),
		}

		healthy, message, details := checkFunc(ctx)
		if healthy {
			check.Status = StatusHealthy
		} else {
			check.Status = StatusUnhealthy
		}
		check.Message = message
		check.Details = details

		return check
	}
}

// MemoryCheck creates a health check for memory usage
func MemoryCheck(maxMemoryMB int) CheckFunc {
	return func(ctx context.Context) Check {
		check := Check{
			Name: "memory",
		}

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		currentMemoryMB := int(m.Alloc / 1024 / 1024)

		check.Details = map[string]interface{}{
			"current_mb": currentMemoryMB,
			"max_mb":     maxMemoryMB,
			"usage_pct":  float64(currentMemoryMB) / float64(maxMemoryMB) * 100,
		}

		if currentMemoryMB > maxMemoryMB {
			check.Status = StatusUnhealthy
			check.Message = fmt.Sprintf("Memory usage too high: %dMB > %dMB", currentMemoryMB, maxMemoryMB)
		} else if currentMemoryMB > int(float64(maxMemoryMB)*0.8) {
			check.Status = StatusDegraded
			check.Message = fmt.Sprintf("Memory usage high: %dMB (%.1f%%)", currentMemoryMB, float64(currentMemoryMB)/float64(maxMemoryMB)*100)
		} else {
			check.Status = StatusHealthy
			check.Message = fmt.Sprintf("Memory usage normal: %dMB (%.1f%%)", currentMemoryMB, float64(currentMemoryMB)/float64(maxMemoryMB)*100)
		}

		return check
	}
}

// GoroutineCheck creates a health check for goroutine count
func GoroutineCheck(maxGoroutines int) CheckFunc {
	return func(ctx context.Context) Check {
		check := Check{
			Name: "goroutines",
		}

		currentGoroutines := runtime.NumGoroutine()

		check.Details = map[string]interface{}{
			"current": currentGoroutines,
			"max":     maxGoroutines,
		}

		if currentGoroutines > maxGoroutines {
			check.Status = StatusUnhealthy
			check.Message = fmt.Sprintf("Too many goroutines: %d > %d", currentGoroutines, maxGoroutines)
		} else if currentGoroutines > int(float64(maxGoroutines)*0.8) {
			check.Status = StatusDegraded
			check.Message = fmt.Sprintf("High goroutine count: %d", currentGoroutines)
		} else {
			check.Status = StatusHealthy
			check.Message = fmt.Sprintf("Goroutine count normal: %d", currentGoroutines)
		}

		return check
	}
}
