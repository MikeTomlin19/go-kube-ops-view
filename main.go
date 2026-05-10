package main

import (
	"fmt"
	"os"

	"kube-ops-view/internal/config"
	"kube-ops-view/internal/server"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "kube-ops-view",
	Short: "Kubernetes Operational View - Visual dashboard for multiple Kubernetes clusters",
	Long: `Kube Ops View provides a visual dashboard for monitoring multiple Kubernetes clusters.
It displays nodes, pods, resource usage, and cluster status information in real-time.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	RunE:    runServer,
}

func init() {
	// Server configuration flags
	rootCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	rootCmd.Flags().String("route-prefix", "/", "Route prefix for the application")
	rootCmd.Flags().Bool("debug", false, "Enable debug mode")
	rootCmd.Flags().String("secret-key", "development", "Secret key for session management")

	// Cluster configuration flags
	rootCmd.Flags().StringSlice("clusters", []string{}, "Comma-separated list of cluster API server URLs")
	rootCmd.Flags().String("cluster-registry-url", "", "URL of cluster registry endpoint")
	rootCmd.Flags().String("kubeconfig-path", "", "Path to kubeconfig file")
	rootCmd.Flags().StringSlice("kubeconfig-contexts", []string{}, "Kubeconfig contexts to use")
	rootCmd.Flags().Duration("query-interval", 0, "Interval for querying cluster data (e.g., 5s, 1m)")
	rootCmd.Flags().Bool("mock", false, "Use mock data instead of real clusters")

	// Storage configuration flags
	rootCmd.Flags().String("redis-url", "", "Redis URL for pub/sub and job locking")
	rootCmd.Flags().String("redis-password", "", "Redis password")
	rootCmd.Flags().Int("redis-db", 0, "Redis database number")

	// Authentication configuration flags
	rootCmd.Flags().String("oauth-authorize-url", "", "OAuth2 authorization URL")
	rootCmd.Flags().String("oauth-token-url", "", "OAuth2 token URL")
	rootCmd.Flags().String("oauth-client-id", "", "OAuth2 client ID")
	rootCmd.Flags().String("oauth-client-secret", "", "OAuth2 client secret")
	rootCmd.Flags().String("oauth-scope", "", "OAuth2 scope")

	// UI configuration flags
	rootCmd.Flags().String("node-link-url-template", "", "URL template for node links")
	rootCmd.Flags().String("pod-link-url-template", "", "URL template for pod links")

	// Logging configuration flags
	rootCmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.Flags().String("log-format", "json", "Log format (json, text)")
	rootCmd.Flags().String("log-output", "stdout", "Log output (stdout, stderr, or file path)")
	rootCmd.Flags().Bool("log-add-source", false, "Add source file and line to log entries")
	rootCmd.Flags().String("log-time-format", "rfc3339", "Time format for log entries")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration from flags and environment
	cfg, err := loadConfigFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Create and start server with version information
	srv, err := server.NewWithVersion(cfg, version, commit, date)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server (this blocks until shutdown)
	return srv.Start()
}

func loadConfigFromFlags(cmd *cobra.Command) (*config.Config, error) {
	cfg := config.New()

	// Server configuration
	if port, err := cmd.Flags().GetInt("port"); err == nil {
		cfg.Server.Port = port
	}
	if routePrefix, err := cmd.Flags().GetString("route-prefix"); err == nil {
		cfg.Server.RoutePrefix = routePrefix
	}
	if debug, err := cmd.Flags().GetBool("debug"); err == nil {
		cfg.Server.Debug = debug
	}
	if secretKey, err := cmd.Flags().GetString("secret-key"); err == nil {
		cfg.Server.SecretKey = secretKey
	}

	// Cluster configuration
	if clusters, err := cmd.Flags().GetStringSlice("clusters"); err == nil && len(clusters) > 0 {
		cfg.Clusters.URLs = clusters
	}
	if registryURL, err := cmd.Flags().GetString("cluster-registry-url"); err == nil && registryURL != "" {
		cfg.Clusters.RegistryURL = registryURL
	}
	if kubeconfigPath, err := cmd.Flags().GetString("kubeconfig-path"); err == nil && kubeconfigPath != "" {
		cfg.Clusters.KubeconfigPath = kubeconfigPath
	}
	if contexts, err := cmd.Flags().GetStringSlice("kubeconfig-contexts"); err == nil && len(contexts) > 0 {
		cfg.Clusters.Contexts = contexts
	}
	if queryInterval, err := cmd.Flags().GetDuration("query-interval"); err == nil && queryInterval > 0 {
		cfg.Clusters.QueryInterval = queryInterval
	}
	if mock, err := cmd.Flags().GetBool("mock"); err == nil {
		cfg.Clusters.Mock = mock
	}

	// Storage configuration
	if redisURL, err := cmd.Flags().GetString("redis-url"); err == nil && redisURL != "" {
		cfg.Storage.RedisURL = redisURL
		cfg.Storage.Type = "redis"
	}

	// Auth configuration
	if authURL, err := cmd.Flags().GetString("oauth-authorize-url"); err == nil && authURL != "" {
		cfg.Auth.Enabled = true
		cfg.Auth.AuthorizeURL = authURL
	}
	if tokenURL, err := cmd.Flags().GetString("oauth-token-url"); err == nil && tokenURL != "" {
		cfg.Auth.TokenURL = tokenURL
	}
	if clientID, err := cmd.Flags().GetString("oauth-client-id"); err == nil && clientID != "" {
		cfg.Auth.ClientID = clientID
	}
	if clientSecret, err := cmd.Flags().GetString("oauth-client-secret"); err == nil && clientSecret != "" {
		cfg.Auth.ClientSecret = clientSecret
	}
	if scope, err := cmd.Flags().GetString("oauth-scope"); err == nil && scope != "" {
		cfg.Auth.Scope = scope
	}

	// Logging configuration
	if logLevel, err := cmd.Flags().GetString("log-level"); err == nil && logLevel != "" {
		cfg.Logging.Level = logLevel
	}
	if logFormat, err := cmd.Flags().GetString("log-format"); err == nil && logFormat != "" {
		cfg.Logging.Format = logFormat
	}
	if logOutput, err := cmd.Flags().GetString("log-output"); err == nil && logOutput != "" {
		cfg.Logging.Output = logOutput
	}
	if logAddSource, err := cmd.Flags().GetBool("log-add-source"); err == nil {
		cfg.Logging.AddSource = logAddSource
	}
	if logTimeFormat, err := cmd.Flags().GetString("log-time-format"); err == nil && logTimeFormat != "" {
		cfg.Logging.TimeFormat = logTimeFormat
	}

	// Load from environment variables as well
	if err := cfg.LoadFromEnv(); err != nil {
		return nil, err
	}

	return cfg, nil
}
