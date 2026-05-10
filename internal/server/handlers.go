package server

import (
	"encoding/json"
	"fmt"
	"kube-ops-view/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleIndex serves the main application page
func (s *Server) handleIndex(c *gin.Context) {
	// Prepare template data
	data := map[string]interface{}{
		"Version":       "dev", // TODO: Get from build info
		"RoutePrefix":   s.config.Server.RoutePrefix,
		"AppJS":         s.assetMgr.GetAppJSFilename(),
		"AppConfigJSON": s.getAppConfigJSON(),
	}

	// Render template
	if err := s.assetMgr.RenderTemplate(c, "index", data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to render template",
			"code":  "TEMPLATE_ERROR",
		})
		return
	}
}

// getAppConfigJSON returns the frontend configuration as JSON
// This matches the Python version's configuration format exactly
func (s *Server) getAppConfigJSON() string {
	config := map[string]interface{}{
		"route_prefix":           s.config.Server.RoutePrefix,
		"node_link_url_template": s.config.UI.NodeLinkURL,
		"pod_link_url_template":  s.config.UI.PodLinkURL,
		"theme":                  s.config.UI.Theme,
		"show_capacity":          s.config.UI.ShowCapacity,
		"show_requests":          s.config.UI.ShowRequests,
		"show_limits":            s.config.UI.ShowLimits,
		"show_usage":             s.config.UI.ShowUsage,
		"auth_enabled":           s.config.Auth.Enabled,
		"clusters":               s.getClusterList(),
	}

	jsonData, err := json.Marshal(config)
	if err != nil {
		return "{}"
	}

	return string(jsonData)
}

// getClusterList returns a list of cluster IDs for the frontend
func (s *Server) getClusterList() []string {
	clusterIDs := s.store.GetClusterIDs()
	if len(clusterIDs) == 0 {
		// Return empty array instead of null for frontend compatibility
		return []string{}
	}
	return clusterIDs
}

// handleHealth provides basic health check endpoint (Kubernetes-style)
func (s *Server) handleHealth(c *gin.Context) {
	// Get cluster manager status
	clusterStatus := s.clusterMgr.GetStatus()

	// Get SSE handler status
	sseStatus := s.sseHandler.HealthCheck()

	// Get store status
	storeStatus := map[string]interface{}{
		"type":     s.config.Storage.Type,
		"clusters": len(s.store.GetClusterIDs()),
	}

	// Determine overall health
	healthy := true
	status := "healthy"

	// Check if any clusters are configured and accessible
	if len(clusterStatus.Clusters) == 0 && !s.config.Clusters.Mock {
		healthy = false
		status = "no_clusters"
	}

	// Check for cluster connection issues
	for _, cluster := range clusterStatus.Clusters {
		if !cluster.Available || cluster.Error != "" {
			healthy = false
			status = "cluster_errors"
			break
		}
	}

	response := map[string]interface{}{
		"status":    status,
		"healthy":   healthy,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   fmt.Sprintf("%s (commit: %s, built: %s)", s.version, s.commit, s.buildDate),
		"uptime":    time.Since(s.startTime).String(),
		"clusters":  clusterStatus,
		"sse":       sseStatus,
		"store":     storeStatus,
	}

	// Update metrics
	if s.metrics != nil {
		totalClusters := len(clusterStatus.Clusters)
		availableClusters := 0
		for _, cluster := range clusterStatus.Clusters {
			if cluster.Available {
				availableClusters++
			}
		}
		s.metricsCollector.UpdateClusterMetrics(totalClusters, availableClusters)
	}

	// Return appropriate HTTP status
	if healthy {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusServiceUnavailable, response)
	}
}

// handleClusters returns information about all clusters
func (s *Server) handleClusters(c *gin.Context) {
	clusterIDs := s.store.GetClusterIDs()
	clusters := make([]map[string]interface{}, 0, len(clusterIDs))

	for _, clusterID := range clusterIDs {
		// Get cluster data
		data, err := s.store.GetClusterData(clusterID)
		if err != nil {
			continue
		}

		// Get cluster status
		status, err := s.store.GetClusterStatus(clusterID)
		if err != nil {
			continue
		}

		clusterInfo := map[string]interface{}{
			"id":             clusterID,
			"api_server_url": data.APIServerURL,
			"last_update":    data.LastUpdate,
			"node_count":     len(data.Nodes),
			"pod_count":      s.countPods(data),
			"status":         getStatusString(status.Available, status.ErrorMessage),
		}

		clusters = append(clusters, clusterInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"clusters": clusters,
	})
}

// handleCluster returns detailed information about a specific cluster
func (s *Server) handleCluster(c *gin.Context) {
	clusterID := c.Param("id")
	if clusterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cluster ID is required",
			"code":  "MISSING_CLUSTER_ID",
		})
		return
	}

	// Get cluster data
	data, err := s.store.GetClusterData(clusterID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Cluster not found",
			"code":  "CLUSTER_NOT_FOUND",
		})
		return
	}

	// Get cluster status
	status, err := s.store.GetClusterStatus(clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get cluster status",
			"code":  "STATUS_ERROR",
		})
		return
	}

	response := map[string]interface{}{
		"cluster": data,
		"status":  status,
	}

	c.JSON(http.StatusOK, response)
}

// handleStatus returns overall application status
func (s *Server) handleStatus(c *gin.Context) {
	clusterStatus := s.clusterMgr.GetStatus()
	sseStatus := s.sseHandler.HealthCheck()

	response := map[string]interface{}{
		"clusters":      clusterStatus,
		"sse":           sseStatus,
		"configuration": s.getConfigurationStatus(),
	}

	c.JSON(http.StatusOK, response)
}

// getConfigurationStatus returns configuration information (without sensitive data)
func (s *Server) getConfigurationStatus() map[string]interface{} {
	return map[string]interface{}{
		"server": map[string]interface{}{
			"port":         s.config.Server.Port,
			"route_prefix": s.config.Server.RoutePrefix,
			"debug":        s.config.Server.Debug,
		},
		"clusters": map[string]interface{}{
			"query_interval":  s.config.Clusters.QueryInterval.String(),
			"connect_timeout": s.config.Clusters.ConnectTimeout.String(),
			"read_timeout":    s.config.Clusters.ReadTimeout.String(),
			"mock":            s.config.Clusters.Mock,
			"sources":         s.config.GetClusterSources(),
		},
		"auth": map[string]interface{}{
			"enabled":       s.config.Auth.Enabled,
			"screen_tokens": s.config.Auth.ScreenTokens,
		},
		"storage": map[string]interface{}{
			"type": s.config.Storage.Type,
		},
		"ui": map[string]interface{}{
			"theme":         s.config.UI.Theme,
			"show_capacity": s.config.UI.ShowCapacity,
			"show_requests": s.config.UI.ShowRequests,
			"show_limits":   s.config.UI.ShowLimits,
			"show_usage":    s.config.UI.ShowUsage,
		},
	}
}

// countPods counts the total number of pods in cluster data
func (s *Server) countPods(data *models.ClusterData) int {
	count := 0

	// Count pods in nodes
	for _, node := range data.Nodes {
		count += len(node.Pods)
	}

	// Count unassigned pods
	count += len(data.UnassignedPods)

	return count
}

// getStatusString converts availability and error to a status string
func getStatusString(available bool, errorMsg string) string {
	if available {
		return "healthy"
	}
	if errorMsg != "" {
		return "error"
	}
	return "unknown"
}

// Authentication handlers

// handleAuthLogin initiates the OAuth login flow
func (s *Server) handleAuthLogin(c *gin.Context) {
	if s.authMgr == nil {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Authentication not configured",
			"code":  "AUTH_NOT_CONFIGURED",
		})
		return
	}

	// For now, redirect to a placeholder login page
	// TODO: Implement proper OAuth flow using the auth manager
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "OAuth login not yet implemented",
		"code":  "NOT_IMPLEMENTED",
	})
}

// handleAuthCallback handles the OAuth callback
func (s *Server) handleAuthCallback(c *gin.Context) {
	if s.authMgr == nil {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Authentication not configured",
			"code":  "AUTH_NOT_CONFIGURED",
		})
		return
	}

	// For now, just return not implemented
	// TODO: Implement proper OAuth callback handling using the auth manager
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "OAuth callback not yet implemented",
		"code":  "NOT_IMPLEMENTED",
	})
}

// handleAuthLogout handles user logout
func (s *Server) handleAuthLogout(c *gin.Context) {
	if s.authMgr == nil {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Authentication not configured",
			"code":  "AUTH_NOT_CONFIGURED",
		})
		return
	}

	// For now, just return success
	// TODO: Implement proper logout using the auth manager
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// Screen token handlers

// handleScreenTokens displays the screen tokens page
func (s *Server) handleScreenTokens(c *gin.Context) {
	data := map[string]interface{}{
		"RoutePrefix": s.config.Server.RoutePrefix,
	}

	// Check if a new token was just created
	if newToken := c.Query("token"); newToken != "" {
		data["NewToken"] = newToken
	}

	if err := s.assetMgr.RenderTemplate(c, "screen-tokens", data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to render template",
			"code":  "TEMPLATE_ERROR",
		})
		return
	}
}

// handleCreateScreenToken creates a new screen token
func (s *Server) handleCreateScreenToken(c *gin.Context) {
	if !s.config.Auth.ScreenTokens {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Screen tokens not enabled",
			"code":  "SCREEN_TOKENS_DISABLED",
		})
		return
	}

	// Create new token
	token, err := s.store.CreateScreenToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create screen token",
			"code":  "TOKEN_CREATION_ERROR",
		})
		return
	}

	// Redirect back to screen tokens page with the new token
	redirectURL := s.config.Server.RoutePrefix + "screen-tokens?token=" + token
	c.Redirect(http.StatusFound, redirectURL)
}
