package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kube-ops-view/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_MonitoringEndpoints(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			RoutePrefix: "/",
			Debug:       true,
		},
		Storage: config.StorageConfig{
			Type: "memory",
		},
		Clusters: config.ClusterConfig{
			Mock: true, // Use mock clusters for testing
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// Create server with version info
	server, err := NewWithVersion(cfg, "test", "abc123", "2024-01-01")
	require.NoError(t, err)
	require.NotNil(t, server)

	// Test that metrics endpoint is available
	t.Run("metrics endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")

		// Check that some basic Prometheus metrics are present
		body := w.Body.String()
		assert.Contains(t, body, "# HELP")
		assert.Contains(t, body, "# TYPE")
	})

	// Test basic health endpoint
	t.Run("health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

		// Basic check that it returns JSON
		body := w.Body.String()
		assert.Contains(t, body, "status")
		assert.Contains(t, body, "timestamp")
	})

	// Test detailed health endpoint
	t.Run("detailed health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health/detailed", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// Should return 200 or 503 depending on health status
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusServiceUnavailable)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

		// Check that detailed health response structure is present
		body := w.Body.String()
		assert.Contains(t, body, "status")
		assert.Contains(t, body, "timestamp")
		assert.Contains(t, body, "uptime")
		assert.Contains(t, body, "version")
		assert.Contains(t, body, "checks")
		assert.Contains(t, body, "summary")
	})
}

func TestServer_MetricsCollection(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			RoutePrefix: "/",
			Debug:       true,
		},
		Storage: config.StorageConfig{
			Type: "memory",
		},
		Clusters: config.ClusterConfig{
			Mock: true,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// Create server
	server, err := NewWithVersion(cfg, "test", "abc123", "2024-01-01")
	require.NoError(t, err)
	require.NotNil(t, server)

	// Verify that metrics components are initialized
	assert.NotNil(t, server.metrics, "Metrics should be initialized")
	assert.NotNil(t, server.metricsCollector, "Metrics collector should be initialized")
	assert.NotNil(t, server.healthChecker, "Health checker should be initialized")

	// Test that application info metrics are set
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "kube_ops_view_application_info")
	assert.Contains(t, body, "version=\"test\"")
	assert.Contains(t, body, "commit=\"abc123\"")
	assert.Contains(t, body, "build_date=\"2024-01-01\"")
}

func TestServer_HealthChecks(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			RoutePrefix: "/",
			Debug:       true,
		},
		Storage: config.StorageConfig{
			Type: "memory",
		},
		Clusters: config.ClusterConfig{
			Mock: true,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// Create server
	server, err := NewWithVersion(cfg, "test", "abc123", "2024-01-01")
	require.NoError(t, err)

	// Test detailed health endpoint to verify health checks are working
	req := httptest.NewRequest("GET", "/health/detailed", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Should return a valid response
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusServiceUnavailable)

	body := w.Body.String()

	// Verify that basic health checks are present
	assert.Contains(t, body, "store")      // Database/store health check
	assert.Contains(t, body, "memory")     // Memory usage health check
	assert.Contains(t, body, "goroutines") // Goroutine count health check
}

func TestServer_StartupShutdown(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        0, // Use random port for testing
			RoutePrefix: "/",
			Debug:       true,
		},
		Storage: config.StorageConfig{
			Type: "memory",
		},
		Clusters: config.ClusterConfig{
			Mock: true,
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
		Logging: config.LoggingConfig{
			Level:  "error", // Reduce log noise in tests
			Format: "json",
			Output: "stdout",
		},
	}

	// Create server
	server, err := NewWithVersion(cfg, "test", "abc123", "2024-01-01")
	require.NoError(t, err)

	// Test that server can start and stop gracefully
	// We'll use a context with timeout to avoid hanging tests
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown server
	shutdownErr := server.Shutdown()
	assert.NoError(t, shutdownErr, "Server shutdown should not return error")

	// Wait for server to finish or timeout
	select {
	case err := <-serverErr:
		// Server should shutdown cleanly
		assert.NoError(t, err, "Server should shutdown without error")
	case <-ctx.Done():
		t.Fatal("Server did not shutdown within timeout")
	}
}
