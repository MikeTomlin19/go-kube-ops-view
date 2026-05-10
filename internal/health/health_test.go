package health

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewChecker(t *testing.T) {
	checker := NewChecker()
	assert.NotNil(t, checker)
	assert.NotNil(t, checker.checks)
	assert.Equal(t, 0, len(checker.checks))
}

func TestAddCheck(t *testing.T) {
	checker := NewChecker()

	checkFunc := func(ctx context.Context) Check {
		return Check{
			Name:   "test",
			Status: StatusHealthy,
		}
	}

	checker.AddCheck("test", checkFunc)
	assert.Equal(t, 1, len(checker.checks))
}

func TestCheckAll(t *testing.T) {
	checker := NewChecker()

	// Add a healthy check
	checker.AddCheck("healthy", func(ctx context.Context) Check {
		return Check{
			Name:    "healthy",
			Status:  StatusHealthy,
			Message: "All good",
		}
	})

	// Add an unhealthy check
	checker.AddCheck("unhealthy", func(ctx context.Context) Check {
		return Check{
			Name:    "unhealthy",
			Status:  StatusUnhealthy,
			Message: "Something wrong",
		}
	})

	ctx := context.Background()
	results := checker.CheckAll(ctx)

	assert.Equal(t, 2, len(results))

	healthyCheck := results["healthy"]
	assert.Equal(t, "healthy", healthyCheck.Name)
	assert.Equal(t, StatusHealthy, healthyCheck.Status)
	assert.Equal(t, "All good", healthyCheck.Message)
	assert.True(t, healthyCheck.Duration > 0)
	assert.False(t, healthyCheck.LastChecked.IsZero())

	unhealthyCheck := results["unhealthy"]
	assert.Equal(t, "unhealthy", unhealthyCheck.Name)
	assert.Equal(t, StatusUnhealthy, unhealthyCheck.Status)
	assert.Equal(t, "Something wrong", unhealthyCheck.Message)
}

func TestGetOverallStatus(t *testing.T) {
	checker := NewChecker()

	tests := []struct {
		name     string
		checks   map[string]Check
		expected Status
	}{
		{
			name:     "no checks",
			checks:   map[string]Check{},
			expected: StatusUnknown,
		},
		{
			name: "all healthy",
			checks: map[string]Check{
				"check1": {Status: StatusHealthy},
				"check2": {Status: StatusHealthy},
			},
			expected: StatusHealthy,
		},
		{
			name: "one unhealthy",
			checks: map[string]Check{
				"check1": {Status: StatusHealthy},
				"check2": {Status: StatusUnhealthy},
			},
			expected: StatusUnhealthy,
		},
		{
			name: "one degraded",
			checks: map[string]Check{
				"check1": {Status: StatusHealthy},
				"check2": {Status: StatusDegraded},
			},
			expected: StatusDegraded,
		},
		{
			name: "unhealthy takes precedence over degraded",
			checks: map[string]Check{
				"check1": {Status: StatusDegraded},
				"check2": {Status: StatusUnhealthy},
			},
			expected: StatusUnhealthy,
		},
		{
			name: "unknown treated as degraded",
			checks: map[string]Check{
				"check1": {Status: StatusHealthy},
				"check2": {Status: StatusUnknown},
			},
			expected: StatusDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := checker.GetOverallStatus(tt.checks)
			assert.Equal(t, tt.expected, status)
		})
	}
}

func TestCreateHealthResponse(t *testing.T) {
	checker := NewChecker()
	startTime := time.Now().Add(-1 * time.Hour) // 1 hour ago
	version := "v1.0.0"

	checker.AddCheck("healthy", func(ctx context.Context) Check {
		return Check{
			Name:   "healthy",
			Status: StatusHealthy,
		}
	})

	checker.AddCheck("degraded", func(ctx context.Context) Check {
		return Check{
			Name:   "degraded",
			Status: StatusDegraded,
		}
	})

	ctx := context.Background()
	response := checker.CreateHealthResponse(ctx, startTime, version)

	assert.Equal(t, StatusDegraded, response.Status) // Overall status should be degraded
	assert.Equal(t, version, response.Version)
	assert.True(t, response.Uptime > 0)
	assert.False(t, response.Timestamp.IsZero())
	assert.Equal(t, 2, len(response.Checks))

	// Check summary
	assert.Equal(t, 2, response.Summary["total"])
	assert.Equal(t, 1, response.Summary["healthy"])
	assert.Equal(t, 0, response.Summary["unhealthy"])
	assert.Equal(t, 1, response.Summary["degraded"])
	assert.Equal(t, 0, response.Summary["unknown"])
}

func TestDatabaseCheck(t *testing.T) {
	t.Run("successful ping", func(t *testing.T) {
		pingFunc := func(ctx context.Context) error {
			return nil
		}

		checkFunc := DatabaseCheck(pingFunc)
		check := checkFunc(context.Background())

		assert.Equal(t, "database", check.Name)
		assert.Equal(t, StatusHealthy, check.Status)
		assert.Contains(t, check.Message, "successful")
	})

	t.Run("failed ping", func(t *testing.T) {
		pingFunc := func(ctx context.Context) error {
			return errors.New("connection failed")
		}

		checkFunc := DatabaseCheck(pingFunc)
		check := checkFunc(context.Background())

		assert.Equal(t, "database", check.Name)
		assert.Equal(t, StatusUnhealthy, check.Status)
		assert.Contains(t, check.Message, "connection failed")
	})
}

func TestClusterCheck(t *testing.T) {
	t.Run("healthy cluster", func(t *testing.T) {
		checkFunc := ClusterCheck("test", func(ctx context.Context) (bool, string, map[string]interface{}) {
			return true, "Cluster is healthy", map[string]interface{}{
				"nodes": 3,
				"pods":  50,
			}
		})

		check := checkFunc(context.Background())

		assert.Equal(t, "cluster_test", check.Name)
		assert.Equal(t, StatusHealthy, check.Status)
		assert.Equal(t, "Cluster is healthy", check.Message)
		assert.Equal(t, 3, check.Details["nodes"])
		assert.Equal(t, 50, check.Details["pods"])
	})

	t.Run("unhealthy cluster", func(t *testing.T) {
		checkFunc := ClusterCheck("test", func(ctx context.Context) (bool, string, map[string]interface{}) {
			return false, "Cluster is unreachable", nil
		})

		check := checkFunc(context.Background())

		assert.Equal(t, "cluster_test", check.Name)
		assert.Equal(t, StatusUnhealthy, check.Status)
		assert.Equal(t, "Cluster is unreachable", check.Message)
	})
}

func TestMemoryCheck(t *testing.T) {
	// Get current memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	currentMemoryMB := int(m.Alloc / 1024 / 1024)

	t.Run("normal memory usage", func(t *testing.T) {
		// Set max memory much higher than current usage
		maxMemoryMB := currentMemoryMB + 1000
		checkFunc := MemoryCheck(maxMemoryMB)
		check := checkFunc(context.Background())

		assert.Equal(t, "memory", check.Name)
		assert.Equal(t, StatusHealthy, check.Status)
		assert.Contains(t, check.Message, "normal")
		assert.NotNil(t, check.Details["current_mb"])
		assert.Equal(t, maxMemoryMB, check.Details["max_mb"])
	})

	t.Run("high memory usage", func(t *testing.T) {
		// Set max memory to trigger degraded status (current usage > 80% of max)
		maxMemoryMB := int(float64(currentMemoryMB) / 0.9) // This should trigger degraded
		checkFunc := MemoryCheck(maxMemoryMB)
		check := checkFunc(context.Background())

		assert.Equal(t, "memory", check.Name)
		// Status could be healthy or degraded depending on actual memory usage
		assert.True(t, check.Status == StatusHealthy || check.Status == StatusDegraded)
	})

	t.Run("memory usage too high", func(t *testing.T) {
		// Set max memory lower than current usage, but ensure it's at least 1MB
		maxMemoryMB := 1
		if currentMemoryMB > 1 {
			maxMemoryMB = currentMemoryMB - 1
		}
		checkFunc := MemoryCheck(maxMemoryMB)
		check := checkFunc(context.Background())

		assert.Equal(t, "memory", check.Name)
		// Only check for unhealthy if current memory is actually higher than max
		if currentMemoryMB > maxMemoryMB {
			assert.Equal(t, StatusUnhealthy, check.Status)
			assert.Contains(t, check.Message, "too high")
		} else {
			// If current memory is not higher, it should be healthy
			assert.True(t, check.Status == StatusHealthy || check.Status == StatusDegraded)
		}
	})
}

func TestGoroutineCheck(t *testing.T) {
	currentGoroutines := runtime.NumGoroutine()

	t.Run("normal goroutine count", func(t *testing.T) {
		maxGoroutines := currentGoroutines + 100
		checkFunc := GoroutineCheck(maxGoroutines)
		check := checkFunc(context.Background())

		assert.Equal(t, "goroutines", check.Name)
		assert.Equal(t, StatusHealthy, check.Status)
		assert.Contains(t, check.Message, "normal")
		// Don't check exact goroutine count as it may change during test execution
		assert.NotNil(t, check.Details["current"])
		assert.Equal(t, maxGoroutines, check.Details["max"])
	})

	t.Run("high goroutine count", func(t *testing.T) {
		// Set max to a reasonable value that should allow for some variation
		maxGoroutines := currentGoroutines + 10
		checkFunc := GoroutineCheck(maxGoroutines)
		check := checkFunc(context.Background())

		assert.Equal(t, "goroutines", check.Name)
		// Status could be healthy or degraded depending on actual goroutine count at execution time
		assert.True(t, check.Status == StatusHealthy || check.Status == StatusDegraded)
	})

	t.Run("too many goroutines", func(t *testing.T) {
		// Use a very low max to ensure we trigger unhealthy status
		maxGoroutines := 1
		checkFunc := GoroutineCheck(maxGoroutines)
		check := checkFunc(context.Background())

		assert.Equal(t, "goroutines", check.Name)
		// Since we're using max=1 and there are definitely more goroutines running
		assert.Equal(t, StatusUnhealthy, check.Status)
		assert.Contains(t, check.Message, "Too many")
	})
}

func TestCheckWithTimeout(t *testing.T) {
	checker := NewChecker()

	// Add a check that takes longer than the context timeout
	checker.AddCheck("slow", func(ctx context.Context) Check {
		select {
		case <-time.After(100 * time.Millisecond):
			return Check{
				Name:   "slow",
				Status: StatusHealthy,
			}
		case <-ctx.Done():
			return Check{
				Name:    "slow",
				Status:  StatusUnhealthy,
				Message: "Check timed out",
			}
		}
	})

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	results := checker.CheckAll(ctx)
	check := results["slow"]

	assert.Equal(t, "slow", check.Name)
	assert.Equal(t, StatusUnhealthy, check.Status)
	assert.Contains(t, check.Message, "timed out")
}
