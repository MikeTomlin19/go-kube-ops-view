package config

import (
	"github.com/spf13/cobra"
)

// AddFlags adds configuration flags to the cobra command
func (c *Config) AddFlags(cmd *cobra.Command) {
	// Server flags
	cmd.Flags().IntVar(&c.Server.Port, "port", c.Server.Port, "Server port")
	cmd.Flags().StringVar(&c.Server.Host, "host", c.Server.Host, "Server host")
	cmd.Flags().StringVar(&c.Server.RoutePrefix, "route-prefix", c.Server.RoutePrefix, "Route prefix for reverse proxy setups")
	cmd.Flags().BoolVar(&c.Server.Debug, "debug", c.Server.Debug, "Enable debug mode")
	cmd.Flags().StringVar(&c.Server.SecretKey, "secret-key", c.Server.SecretKey, "Secret key for session management")

	// Cluster flags
	cmd.Flags().StringSliceVar(&c.Clusters.URLs, "clusters", c.Clusters.URLs, "Comma-separated list of Kubernetes API server URLs")
	cmd.Flags().StringVar(&c.Clusters.RegistryURL, "cluster-registry-url", c.Clusters.RegistryURL, "URL of cluster registry service")
	cmd.Flags().StringVar(&c.Clusters.KubeconfigPath, "kubeconfig-path", c.Clusters.KubeconfigPath, "Path to kubeconfig file")
	cmd.Flags().StringSliceVar(&c.Clusters.Contexts, "kubeconfig-contexts", c.Clusters.Contexts, "Comma-separated list of kubeconfig contexts to use")
	cmd.Flags().DurationVar(&c.Clusters.QueryInterval, "query-interval", c.Clusters.QueryInterval, "Interval for querying cluster data")
	cmd.Flags().DurationVar(&c.Clusters.ConnectTimeout, "connect-timeout", c.Clusters.ConnectTimeout, "Timeout for connecting to clusters")
	cmd.Flags().DurationVar(&c.Clusters.ReadTimeout, "read-timeout", c.Clusters.ReadTimeout, "Timeout for reading from clusters")
	cmd.Flags().BoolVar(&c.Clusters.Mock, "mock", c.Clusters.Mock, "Use mock cluster data for testing")

	// Auth flags
	cmd.Flags().BoolVar(&c.Auth.Enabled, "oauth2-enabled", c.Auth.Enabled, "Enable OAuth2 authentication")
	cmd.Flags().StringVar(&c.Auth.AuthorizeURL, "oauth2-authorize-url", c.Auth.AuthorizeURL, "OAuth2 authorization URL")
	cmd.Flags().StringVar(&c.Auth.TokenURL, "oauth2-token-url", c.Auth.TokenURL, "OAuth2 token URL")
	cmd.Flags().StringVar(&c.Auth.ClientID, "oauth2-client-id", c.Auth.ClientID, "OAuth2 client ID")
	cmd.Flags().StringVar(&c.Auth.ClientSecret, "oauth2-client-secret", c.Auth.ClientSecret, "OAuth2 client secret")
	cmd.Flags().StringVar(&c.Auth.Scope, "oauth2-scope", c.Auth.Scope, "OAuth2 scope")
	cmd.Flags().StringVar(&c.Auth.CredentialsDir, "credentials-dir", c.Auth.CredentialsDir, "Directory containing OAuth2 credentials")
	cmd.Flags().BoolVar(&c.Auth.ScreenTokens, "screen-tokens", c.Auth.ScreenTokens, "Enable screen token authentication")

	// Storage flags
	cmd.Flags().StringVar(&c.Storage.Type, "storage-type", c.Storage.Type, "Storage type (memory or redis)")
	cmd.Flags().StringVar(&c.Storage.RedisURL, "redis-url", c.Storage.RedisURL, "Redis connection URL")
	cmd.Flags().IntVar(&c.Storage.RedisDB, "redis-db", c.Storage.RedisDB, "Redis database number")

	// UI flags
	cmd.Flags().StringVar(&c.UI.URLTemplate, "url-template", c.UI.URLTemplate, "URL template for external links")
	cmd.Flags().StringVar(&c.UI.NodeLinkURL, "node-link-url", c.UI.NodeLinkURL, "URL template for node links")
	cmd.Flags().StringVar(&c.UI.PodLinkURL, "pod-link-url", c.UI.PodLinkURL, "URL template for pod links")
	cmd.Flags().StringVar(&c.UI.Theme, "theme", c.UI.Theme, "UI theme (default, dark, light, colorblind)")
	cmd.Flags().BoolVar(&c.UI.ShowCapacity, "show-capacity", c.UI.ShowCapacity, "Show node capacity information")
	cmd.Flags().BoolVar(&c.UI.ShowRequests, "show-requests", c.UI.ShowRequests, "Show resource requests")
	cmd.Flags().BoolVar(&c.UI.ShowLimits, "show-limits", c.UI.ShowLimits, "Show resource limits")
	cmd.Flags().BoolVar(&c.UI.ShowUsage, "show-usage", c.UI.ShowUsage, "Show resource usage")
}

// LoadFromFlags loads configuration from command line flags
// This is automatically handled by cobra when flags are bound to variables
func (c *Config) LoadFromFlags() error {
	// Cobra automatically populates the bound variables when flags are parsed
	// No additional work needed here
	return nil
}
