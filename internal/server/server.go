package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kube-ops-view/assets"
	"kube-ops-view/internal/auth"
	"kube-ops-view/internal/cluster"
	"kube-ops-view/internal/config"
	"kube-ops-view/internal/errors"
	"kube-ops-view/internal/events"
	"kube-ops-view/internal/health"
	"kube-ops-view/internal/logging"
	"kube-ops-view/internal/metrics"
	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the HTTP server
type Server struct {
	config           *config.Config
	router           *gin.Engine
	httpServer       *http.Server
	store            store.Store
	clusterMgr       *cluster.Manager
	authMgr          *auth.Manager
	assetMgr         *assets.AssetManager
	sseHandler       *events.SSEHandler
	publisher        events.EventPublisher
	logger           *logging.Logger
	errorHandler     *errors.ErrorHandler
	metrics          *metrics.Metrics
	metricsCollector *metrics.Collector
	healthChecker    *health.Checker
	startTime        time.Time
	version          string
	commit           string
	buildDate        string
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	return NewWithVersion(cfg, "dev", "unknown", "unknown")
}

// NewWithVersion creates a new server instance with version information
func NewWithVersion(cfg *config.Config, version, commit, buildDate string) (*Server, error) {
	// Set Gin mode based on debug setting
	if !cfg.Server.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create logger
	loggerConfig := logging.Config{
		Level:      logging.LogLevel(cfg.Logging.Level),
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		AddSource:  cfg.Logging.AddSource,
		TimeFormat: cfg.Logging.TimeFormat,
	}

	logger, err := logging.New(loggerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create error handler with adapter
	errorHandler := errors.NewErrorHandler(&loggerAdapter{logger: logger})

	// Create store
	storeConfig := store.Config{
		Type:     cfg.Storage.Type,
		TokenTTL: 24 * time.Hour,
		Redis: store.RedisConfig{
			Addr:     cfg.Storage.RedisURL,
			Password: "", // TODO: Add password support
			DB:       cfg.Storage.RedisDB,
			TokenTTL: 24 * time.Hour,
		},
	}

	dataStore, err := store.NewStore(storeConfig)
	if err != nil {
		storeErr := errors.NewStoreConnectionError(err)
		logger.WithError(storeErr).Error("Failed to create data store")
		return nil, storeErr
	}

	// Create asset manager
	assetMgr, err := assets.NewAssetManager(cfg.Server.Debug)
	if err != nil {
		assetErr := errors.WrapError(errors.ErrCodeSystemUnavailable, "Failed to create asset manager", err)
		logger.WithError(assetErr).Error("Failed to create asset manager")
		return nil, assetErr
	}

	// Create event publisher
	publisher := events.NewPublisher(dataStore)

	// Create SSE handler
	sseConfig := events.SSEConfig{
		PingInterval:   30 * time.Second,
		ClientTimeout:  5 * time.Minute,
		MaxClients:     1000,
		BufferSize:     100,
		EnableCORS:     true,
		AllowedOrigins: []string{"*"},
	}
	sseHandler := events.NewSSEHandler(publisher, sseConfig)

	// Create auth manager if authentication is enabled
	var authMgr *auth.Manager
	if cfg.Auth.Enabled {
		baseURL := fmt.Sprintf("http://localhost:%d%s", cfg.Server.Port, cfg.Server.RoutePrefix)
		authMgr, err = auth.NewManager(&cfg.Auth, dataStore, baseURL)
		if err != nil {
			authErr := errors.WrapError(errors.ErrCodeAuthProviderError, "Failed to create auth manager", err)
			logger.WithError(authErr).Error("Failed to create authentication manager")
			return nil, authErr
		}
	}

	// Create cluster manager
	clusterConfig := cluster.Config{
		URLs:           cfg.Clusters.URLs,
		RegistryURL:    cfg.Clusters.RegistryURL,
		KubeconfigPath: cfg.Clusters.KubeconfigPath,
		Contexts:       cfg.Clusters.Contexts,
		QueryInterval:  cfg.Clusters.QueryInterval,
		ConnectTimeout: cfg.Clusters.ConnectTimeout,
		ReadTimeout:    cfg.Clusters.ReadTimeout,
		Mock:           cfg.Clusters.Mock,
	}

	// Create store adapter for cluster package
	storeAdapter := &storeAdapter{store: dataStore}

	// Create metrics system
	appMetrics := metrics.NewMetrics()
	metricsCollector := metrics.NewCollector(appMetrics)

	// Set application info in metrics
	appMetrics.SetApplicationInfo(version, commit, buildDate)

	// Create event publisher adapter for cluster package
	publisherAdapter := &eventPublisherAdapter{publisher: publisher}

	// Create metrics recorder adapter
	metricsRecorderAdapter := &metricsRecorderAdapter{metrics: appMetrics}

	clusterMgr, err := cluster.NewManager(clusterConfig, storeAdapter, publisherAdapter, metricsRecorderAdapter)
	if err != nil {
		clusterErr := errors.WrapError(errors.ErrCodeClusterConnection, "Failed to create cluster manager", err)
		logger.WithError(clusterErr).Error("Failed to create cluster manager")
		return nil, clusterErr
	}

	// Create health checker
	healthChecker := health.NewChecker()

	server := &Server{
		config:           cfg,
		store:            dataStore,
		clusterMgr:       clusterMgr,
		authMgr:          authMgr,
		assetMgr:         assetMgr,
		sseHandler:       sseHandler,
		publisher:        publisher,
		logger:           logger,
		errorHandler:     errorHandler,
		metrics:          appMetrics,
		metricsCollector: metricsCollector,
		healthChecker:    healthChecker,
		startTime:        time.Now(),
		version:          version,
		commit:           commit,
		buildDate:        buildDate,
	}

	// Setup health checks
	server.setupHealthChecks()

	// Setup router
	server.setupRouter()

	// Create HTTP server
	server.httpServer = &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      server.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server, nil
}

// setupRouter configures the Gin router with all routes and middleware
func (s *Server) setupRouter() {
	s.router = gin.New()

	// Add middleware
	s.addMiddleware()

	// Setup routes
	s.setupRoutes()
}

// addMiddleware adds all necessary middleware to the router
func (s *Server) addMiddleware() {
	// Create error middleware
	errorMiddleware := errors.NewErrorMiddleware(s.errorHandler)

	// Recovery middleware (panic recovery)
	s.router.Use(errorMiddleware.Recovery())

	// Metrics middleware (should be early in the chain)
	s.router.Use(metrics.HTTPMiddleware(s.metrics))

	// Logging middleware
	s.router.Use(errorMiddleware.LoggingMiddleware())

	// CORS middleware
	s.router.Use(errorMiddleware.CORSErrorMiddleware())

	// Security headers middleware
	s.router.Use(errorMiddleware.SecurityHeadersMiddleware())

	// Validation error middleware
	s.router.Use(errorMiddleware.ValidationErrorMiddleware())

	// General error handling middleware
	s.router.Use(errorMiddleware.Handle())

	// Rate limiting middleware (if not in debug mode)
	if !s.config.Server.Debug {
		s.router.Use(errorMiddleware.RateLimitMiddleware(100, time.Minute)) // 100 requests per minute
	}
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	prefix := s.config.Server.RoutePrefix
	if prefix == "/" {
		prefix = ""
	}

	// Health check endpoints (no auth required)
	s.router.GET(prefix+"/health", s.handleHealth)
	s.router.GET(prefix+"/healthz", s.handleHealth) // Kubernetes style
	s.router.GET(prefix+"/health/detailed", s.handleDetailedHealth)

	// Metrics endpoint (no auth required)
	s.router.GET(prefix+"/metrics", gin.WrapH(promhttp.HandlerFor(s.metrics.Registry, promhttp.HandlerOpts{})))

	// Static assets (no auth required)
	s.router.GET(prefix+"/static/*filepath", s.assetMgr.ServeStatic(prefix))

	// Authentication routes (always set up, but return not implemented if disabled)
	authGroup := s.router.Group(prefix + "/auth")
	{
		authGroup.GET("/login", s.handleAuthLogin)
		authGroup.GET("/callback", s.handleAuthCallback)
		authGroup.POST("/logout", s.handleAuthLogout)
	}

	// Screen token routes (if enabled)
	if s.authMgr != nil && s.config.Auth.ScreenTokens {
		tokenGroup := s.router.Group(prefix + "/screen-tokens")
		tokenGroup.Use(s.authMiddleware())
		{
			tokenGroup.GET("", s.handleScreenTokens)
			tokenGroup.POST("", s.handleCreateScreenToken)
		}
	}

	// Protected routes
	protected := s.router.Group(prefix)
	if s.authMgr != nil {
		protected.Use(s.authMiddleware())
	}
	{
		// Main application
		protected.GET("/", s.handleIndex)

		// SSE events endpoint
		protected.GET("/events", s.sseHandler.HandleSSE)

		// API endpoints
		api := protected.Group("/api")
		{
			api.GET("/clusters", s.handleClusters)
			api.GET("/clusters/:id", s.handleCluster)
			api.GET("/status", s.handleStatus)
		}
	}
}

// authMiddleware provides authentication middleware
func (s *Server) authMiddleware() gin.HandlerFunc {
	if s.authMgr == nil {
		// No auth configured, allow all requests
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Use the auth manager's middleware
	middleware := s.authMgr.GetMiddleware()
	return middleware.RequireAuth()
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Start metrics collector
	s.metricsCollector.Start()

	// Start cluster manager
	if err := s.clusterMgr.Start(); err != nil {
		clusterErr := errors.WrapError(errors.ErrCodeClusterConnection, "Failed to start cluster manager", err)
		s.logger.WithError(clusterErr).Error("Failed to start cluster manager")
		return clusterErr
	}

	// Log startup information
	s.logger.Info("Starting HTTP server",
		"address", s.httpServer.Addr,
		"route_prefix", s.config.Server.RoutePrefix,
		"debug_mode", s.config.Server.Debug,
		"authentication_enabled", s.config.Auth.Enabled,
		"storage_type", s.config.Storage.Type,
		"log_level", s.config.Logging.Level,
	)

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- errors.WrapError(errors.ErrCodeSystemUnavailable, "HTTP server failed", err)
		}
		close(serverErr)
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			s.logger.WithError(err).Error("Server failed to start")
			return err
		}
	case sig := <-quit:
		s.logger.Info("Received shutdown signal", "signal", sig.String())
	}

	return s.Shutdown()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	shutdownStart := time.Now()
	s.logger.Info("Starting graceful shutdown")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var shutdownErrors []error

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		shutdownErr := errors.WrapError(errors.ErrCodeSystemUnavailable, "HTTP server shutdown failed", err)
		s.logger.WithError(shutdownErr).Error("Failed to shutdown HTTP server gracefully")
		shutdownErrors = append(shutdownErrors, shutdownErr)
	} else {
		s.logger.Info("HTTP server shutdown completed")
	}

	// Stop cluster manager
	if err := s.clusterMgr.Stop(); err != nil {
		clusterErr := errors.WrapError(errors.ErrCodeClusterConnection, "Cluster manager shutdown failed", err)
		s.logger.WithError(clusterErr).Error("Failed to stop cluster manager")
		shutdownErrors = append(shutdownErrors, clusterErr)
	} else {
		s.logger.Info("Cluster manager stopped")
	}

	// Close SSE handler
	if err := s.sseHandler.Close(); err != nil {
		sseErr := errors.WrapError(errors.ErrCodeSystemUnavailable, "SSE handler close failed", err)
		s.logger.WithError(sseErr).Error("Failed to close SSE handler")
		shutdownErrors = append(shutdownErrors, sseErr)
	} else {
		s.logger.Info("SSE handler closed")
	}

	// Stop metrics collector
	s.metricsCollector.Stop()
	s.logger.Info("Metrics collector stopped")

	// Close store
	if closer, ok := s.store.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			storeErr := errors.WrapError(errors.ErrCodeStoreConnection, "Store close failed", err)
			s.logger.WithError(storeErr).Error("Failed to close data store")
			shutdownErrors = append(shutdownErrors, storeErr)
		} else {
			s.logger.Info("Data store closed")
		}
	}

	shutdownDuration := time.Since(shutdownStart)
	if len(shutdownErrors) > 0 {
		s.logger.Error("Server shutdown completed with errors",
			"duration", shutdownDuration,
			"error_count", len(shutdownErrors))
		// Return the first error
		return shutdownErrors[0]
	}

	s.logger.Info("Server shutdown completed successfully", "duration", shutdownDuration)
	return nil
}

// GetRouter returns the Gin router (useful for testing)
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// storeAdapter adapts the store.Store interface to the cluster.Store interface
type storeAdapter struct {
	store store.Store
}

func (s *storeAdapter) SetClusterData(clusterID string, data *models.ClusterData) error {
	return s.store.SetClusterData(clusterID, data)
}

func (s *storeAdapter) GetClusterData(clusterID string) (*models.ClusterData, error) {
	return s.store.GetClusterData(clusterID)
}

func (s *storeAdapter) SetClusterStatus(clusterID string, status *cluster.ClusterStatus) error {
	// Convert cluster.ClusterStatus to store.ClusterStatus
	storeStatus := &store.ClusterStatus{
		ID:           status.ID,
		Available:    status.Available,
		LastSeen:     status.LastSeen,
		ErrorMessage: status.Error,
		APIServerURL: "", // Not available in cluster.ClusterStatus
	}
	return s.store.SetClusterStatus(clusterID, storeStatus)
}

func (s *storeAdapter) GetClusterStatus(clusterID string) (*cluster.ClusterStatus, error) {
	storeStatus, err := s.store.GetClusterStatus(clusterID)
	if err != nil {
		return nil, err
	}

	// Convert store.ClusterStatus to cluster.ClusterStatus
	clusterStatus := &cluster.ClusterStatus{
		ID:        storeStatus.ID,
		Available: storeStatus.Available,
		LastSeen:  storeStatus.LastSeen,
		Error:     storeStatus.ErrorMessage,
	}
	return clusterStatus, nil
}

func (s *storeAdapter) DeleteCluster(clusterID string) error {
	return s.store.DeleteCluster(clusterID)
}

// loggerAdapter adapts the logging.Logger to the errors.Logger interface
type loggerAdapter struct {
	logger *logging.Logger
}

func (l *loggerAdapter) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, args...)
}

func (l *loggerAdapter) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, args...)
}

func (l *loggerAdapter) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, args...)
}

func (l *loggerAdapter) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, args...)
}

func (l *loggerAdapter) WithError(err error) errors.Logger {
	return &loggerAdapter{logger: l.logger.WithError(err)}
}

func (l *loggerAdapter) WithFields(fields map[string]interface{}) errors.Logger {
	return &loggerAdapter{logger: l.logger.WithFields(fields)}
}

// eventPublisherAdapter adapts the events.EventPublisher interface to the cluster.EventPublisher interface
type eventPublisherAdapter struct {
	publisher events.EventPublisher
}

func (e *eventPublisherAdapter) PublishClusterUpdate(clusterID string, data *models.ClusterData) error {
	return e.publisher.PublishClusterUpdate(clusterID, data)
}

func (e *eventPublisherAdapter) PublishClusterStatus(clusterID string, status *cluster.ClusterStatus) error {
	// Convert cluster.ClusterStatus to store.ClusterStatus
	storeStatus := &store.ClusterStatus{
		ID:           status.ID,
		Available:    status.Available,
		LastSeen:     status.LastSeen,
		ErrorMessage: status.Error,
		APIServerURL: "", // Not available in cluster.ClusterStatus
	}
	return e.publisher.PublishClusterStatus(clusterID, storeStatus)
}

// setupHealthChecks configures health checks for various components
func (s *Server) setupHealthChecks() {
	// Database/Store health check
	s.healthChecker.AddCheck("store", health.DatabaseCheck(func(ctx context.Context) error {
		// Try to get cluster IDs as a simple connectivity test
		clusterIDs := s.store.GetClusterIDs()
		_ = clusterIDs // Use the result to avoid unused variable warning
		return nil
	}))

	// Memory usage health check (warn at 512MB, error at 1GB)
	s.healthChecker.AddCheck("memory", health.MemoryCheck(1024))

	// Goroutine count health check (warn at 1000, error at 2000)
	s.healthChecker.AddCheck("goroutines", health.GoroutineCheck(2000))

	// Cluster connectivity health checks
	// These will be added dynamically as clusters are discovered
}

// addClusterHealthCheck adds a health check for a specific cluster
func (s *Server) addClusterHealthCheck(clusterID string) {
	checkName := fmt.Sprintf("cluster_%s", clusterID)
	s.healthChecker.AddCheck(checkName, health.ClusterCheck(clusterID, func(ctx context.Context) (bool, string, map[string]interface{}) {
		// Get cluster status
		status, err := s.store.GetClusterStatus(clusterID)
		if err != nil {
			return false, fmt.Sprintf("Failed to get cluster status: %v", err), nil
		}

		// Get cluster data for additional details
		data, err := s.store.GetClusterData(clusterID)
		details := make(map[string]interface{})
		if err == nil {
			details["nodes"] = len(data.Nodes)
			details["pods"] = s.countPods(data)
			details["last_update"] = data.LastUpdate
		}

		if status.Available {
			return true, "Cluster is healthy and reachable", details
		}

		message := "Cluster is not available"
		if status.ErrorMessage != "" {
			message = fmt.Sprintf("Cluster error: %s", status.ErrorMessage)
		}

		return false, message, details
	}))
}

// handleDetailedHealth provides detailed health information
func (s *Server) handleDetailedHealth(c *gin.Context) {
	ctx := c.Request.Context()
	response := s.healthChecker.CreateHealthResponse(ctx, s.startTime, s.version)

	// Add additional server information
	response.Version = fmt.Sprintf("%s (commit: %s, built: %s)", s.version, s.commit, s.buildDate)

	// Return appropriate HTTP status based on overall health
	statusCode := http.StatusOK
	switch response.Status {
	case health.StatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	case health.StatusDegraded:
		statusCode = http.StatusOK // Still return 200 for degraded
	}

	c.JSON(statusCode, response)
}

// metricsRecorderAdapter adapts the metrics.Metrics to the cluster.MetricsRecorder interface
type metricsRecorderAdapter struct {
	metrics *metrics.Metrics
}

func (m *metricsRecorderAdapter) RecordClusterQuery(clusterID, operation string, duration time.Duration, err error) {
	if m.metrics != nil {
		m.metrics.RecordClusterQuery(clusterID, operation, duration, err)
	}
}

func (m *metricsRecorderAdapter) UpdateClusterStats(clusterID string, nodes, totalPods, runningPods, pendingPods, errorPods int) {
	if m.metrics != nil {
		m.metrics.UpdateClusterStats(clusterID, nodes, totalPods, runningPods, pendingPods, errorPods)
	}
}
