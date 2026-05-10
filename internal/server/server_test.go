package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kube-ops-view/internal/config"
	"kube-ops-view/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true // Use mock mode for testing

	server, err := New(cfg)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.router)
	assert.NotNil(t, server.store)
	assert.NotNil(t, server.clusterMgr)
	assert.NotNil(t, server.assetMgr)
	assert.NotNil(t, server.sseHandler)
	assert.NotNil(t, server.publisher)
}

func TestHealthEndpoint(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Start cluster manager for realistic health check
	err = server.clusterMgr.Start()
	require.NoError(t, err)
	defer server.clusterMgr.Stop()

	// Give cluster manager time to initialize
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name           string
		endpoint       string
		expectedStatus int
	}{
		{
			name:           "health endpoint",
			endpoint:       "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "healthz endpoint",
			endpoint:       "/healthz",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.endpoint, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			server.router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)

			// Check required fields
			assert.Contains(t, response, "status")
			assert.Contains(t, response, "healthy")
			assert.Contains(t, response, "timestamp")
			assert.Contains(t, response, "version")
			assert.Contains(t, response, "clusters")
			assert.Contains(t, response, "sse")
			assert.Contains(t, response, "store")
		})
	}
}

func TestIndexEndpoint(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")

	// Check that the response contains expected HTML elements
	body := rr.Body.String()
	assert.Contains(t, body, "<!doctype html>")
	assert.Contains(t, body, "Kubernetes Operational View")
	assert.Contains(t, body, "static/build/")
	assert.Contains(t, body, "const app = new App(")
}

func TestClustersEndpoint(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Start cluster manager to populate data
	err = server.clusterMgr.Start()
	require.NoError(t, err)
	defer server.clusterMgr.Stop()

	// Give cluster manager time to populate mock data
	time.Sleep(200 * time.Millisecond)

	req, err := http.NewRequest("GET", "/api/clusters", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check response structure
	assert.Contains(t, response, "clusters")
	clusters, ok := response["clusters"].([]interface{})
	assert.True(t, ok)

	// In mock mode, we should have at least one cluster
	if len(clusters) > 0 {
		cluster := clusters[0].(map[string]interface{})
		assert.Contains(t, cluster, "id")
		assert.Contains(t, cluster, "api_server_url")
		assert.Contains(t, cluster, "last_update")
		assert.Contains(t, cluster, "node_count")
		assert.Contains(t, cluster, "pod_count")
		assert.Contains(t, cluster, "status")
	}
}

func TestClusterEndpoint(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Start cluster manager to populate data
	err = server.clusterMgr.Start()
	require.NoError(t, err)
	defer server.clusterMgr.Stop()

	// Give cluster manager time to populate mock data
	time.Sleep(200 * time.Millisecond)

	// Get cluster IDs
	clusterIDs := server.store.GetClusterIDs()

	if len(clusterIDs) > 0 {
		// Test valid cluster ID
		req, err := http.NewRequest("GET", "/api/clusters/"+clusterIDs[0], nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		server.router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Parse response
		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check response structure
		assert.Contains(t, response, "cluster")
		assert.Contains(t, response, "status")
	}

	// Test invalid cluster ID
	req, err := http.NewRequest("GET", "/api/clusters/nonexistent", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestStatusEndpoint(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/api/status", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check response structure
	assert.Contains(t, response, "clusters")
	assert.Contains(t, response, "sse")
	assert.Contains(t, response, "configuration")

	// Check configuration structure
	config, ok := response["configuration"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, config, "server")
	assert.Contains(t, config, "clusters")
	assert.Contains(t, config, "auth")
	assert.Contains(t, config, "storage")
	assert.Contains(t, config, "ui")
}

func TestCORSMiddleware(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Test CORS headers
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:3000")

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Content-Type")

	// Test OPTIONS preflight request
	req, err = http.NewRequest("OPTIONS", "/api/clusters", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:3000")

	rr = httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestSecurityHeaders(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", rr.Header().Get("X-XSS-Protection"))
}

func TestRoutePrefix(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true
	cfg.Server.RoutePrefix = "/kube-ops-view/"

	server, err := New(cfg)
	require.NoError(t, err)

	// Test health endpoint with prefix
	req, err := http.NewRequest("GET", "/kube-ops-view/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Test that endpoint without prefix returns 404
	req, err = http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestAuthenticationDisabled(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true
	cfg.Auth.Enabled = false

	server, err := New(cfg)
	require.NoError(t, err)

	// Test that protected endpoints are accessible without auth
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Test that auth endpoints return not implemented
	req, err = http.NewRequest("GET", "/auth/login", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotImplemented, rr.Code)
}

func TestSSEEndpoint(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/events", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	rr := httptest.NewRecorder()

	// Use a channel to signal when headers are written
	headersCh := make(chan struct{})
	go func() {
		server.router.ServeHTTP(rr, req)
		close(headersCh)
	}()

	// Wait a bit for headers to be set
	select {
	case <-headersCh:
		// Headers written, check them
	case <-time.After(100 * time.Millisecond):
		// Timeout, but that's expected for SSE
	}

	// Check SSE headers
	assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rr.Header().Get("Connection"))
}

func TestAppConfigJSON(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true
	cfg.Server.RoutePrefix = "/test/"
	cfg.UI.Theme = "dark"
	cfg.UI.ShowCapacity = false

	server, err := New(cfg)
	require.NoError(t, err)

	configJSON := server.getAppConfigJSON()
	assert.NotEmpty(t, configJSON)

	// Parse the JSON
	var config map[string]interface{}
	err = json.Unmarshal([]byte(configJSON), &config)
	require.NoError(t, err)

	// Check expected values
	assert.Equal(t, "/test/", config["route_prefix"])
	assert.Equal(t, "dark", config["theme"])
	assert.Equal(t, false, config["show_capacity"])
	assert.Equal(t, false, config["auth_enabled"])
	assert.Contains(t, config, "clusters")
}

func TestCustomLogger(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true
	cfg.Server.Debug = false // Use custom logger

	server, err := New(cfg)
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Custom logger should not affect the response
}

func TestMissingClusterID(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Test cluster endpoint without ID (should not match route)
	req, err := http.NewRequest("GET", "/api/clusters/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	// Should return 301 redirect because Gin removes trailing slash
	assert.Equal(t, http.StatusMovedPermanently, rr.Code)
}

func TestStaticAssets(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Test favicon
	req, err := http.NewRequest("GET", "/static/favicon.ico", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Cache-Control"), "public")
}

func TestGetClusterList(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Test with empty store
	clusterList := server.getClusterList()
	assert.NotNil(t, clusterList)
	assert.IsType(t, []string{}, clusterList)
}

func TestCountPods(t *testing.T) {
	cfg := config.New()
	cfg.Clusters.Mock = true

	server, err := New(cfg)
	require.NoError(t, err)

	// Create test cluster data
	data := &models.ClusterData{
		Nodes: map[string]*models.Node{
			"node1": {
				Pods: map[string]*models.Pod{
					"pod1": {},
					"pod2": {},
				},
			},
			"node2": {
				Pods: map[string]*models.Pod{
					"pod3": {},
				},
			},
		},
		UnassignedPods: map[string]*models.Pod{
			"pod4": {},
		},
	}

	count := server.countPods(data)
	assert.Equal(t, 4, count) // 2 + 1 + 1 = 4 pods total
}
