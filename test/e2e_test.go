package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kube-ops-view/internal/config"
	serverpkg "kube-ops-view/internal/server"
)

// E2ETestSuite represents the end-to-end test suite
type E2ETestSuite struct {
	server *httptest.Server
	app    *serverpkg.Server
	config *config.Config
}

// TestE2EBasicFunctionality tests basic end-to-end functionality
func TestE2EBasicFunctionality(t *testing.T) {
	suite := setupE2ETestSuite(t)
	defer suite.cleanup()

	// Test main page loads
	resp, err := http.Get(suite.server.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify HTML contains expected elements
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "Kubernetes Operational View")
	assert.Contains(t, bodyStr, "static/build/")
	assert.Contains(t, bodyStr, "new App")
}

// TestE2EClusterDataAPI tests the cluster data API endpoints
func TestE2EClusterDataAPI(t *testing.T) {
	suite := setupE2ETestSuite(t)
	defer suite.cleanup()

	resp, err := http.Get(suite.server.URL + "/api/clusters")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	assert.Contains(t, payload, "clusters")
}

// TestE2EEventsEndpoint tests the event stream endpoint.
func TestE2EEventsEndpoint(t *testing.T) {
	suite := setupE2ETestSuite(t)
	defer suite.cleanup()

	// Test events endpoint (SSE)
	resp, err := http.Get(suite.server.URL + "/events")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

// TestE2EHealthEndpoint tests the health check endpoint
func TestE2EHealthEndpoint(t *testing.T) {
	suite := setupE2ETestSuite(t)
	defer suite.cleanup()

	resp, err := http.Get(suite.server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.Contains(t, health, "timestamp")
}

// setupE2ETestSuite creates a test suite with all components
func setupE2ETestSuite(t *testing.T) *E2ETestSuite {
	gin.SetMode(gin.TestMode)

	// Create test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			RoutePrefix: "/",
			Debug:       true,
			SecretKey:   "test-secret",
		},
		Clusters: config.ClusterConfig{
			QueryInterval:  5 * time.Second,
			ConnectTimeout: 5 * time.Second,
			ReadTimeout:    5 * time.Second,
			Mock:           true,
		},
		Storage: config.StorageConfig{
			Type: "memory",
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
		Logging: config.LoggingConfig{
			Level:  "error",
			Format: "json",
			Output: "stdout",
		},
	}

	// Create test server
	app, err := serverpkg.NewWithVersion(cfg, "test", "abc123", "2024-01-01")
	require.NoError(t, err)
	testServer := httptest.NewServer(app.GetRouter())

	return &E2ETestSuite{
		server: testServer,
		app:    app,
		config: cfg,
	}
}

// cleanup closes the test suite resources
func (s *E2ETestSuite) cleanup() {
	if s.server != nil {
		s.server.Close()
	}
	if s.app != nil {
		requireNoPanic(func() {
			_ = s.app.Shutdown()
		})
	}
}

func requireNoPanic(fn func()) {
	defer func() {
		_ = recover()
	}()
	fn()
}
