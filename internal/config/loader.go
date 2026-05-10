package config

import (
	"fmt"
)

// LoadOptions contains options for loading configuration
type LoadOptions struct {
	ConfigFile      string
	SkipEnv         bool
	SkipDefaultFile bool
	Validate        bool
}

// Load loads configuration from multiple sources in order of precedence:
// 1. Default values
// 2. Configuration file (if specified or found in default locations)
// 3. Environment variables
// 4. Command line flags (handled by cobra)
func Load(opts LoadOptions) (*Config, error) {
	// Start with default configuration
	config := New()

	// Load from configuration file
	if !opts.SkipDefaultFile {
		if opts.ConfigFile != "" {
			// Load from specified file
			if err := config.LoadFromFile(opts.ConfigFile); err != nil {
				return nil, fmt.Errorf("failed to load configuration file: %v", err)
			}
		} else {
			// Try to load from default locations
			if err := config.LoadFromDefaultLocations(); err != nil {
				return nil, fmt.Errorf("failed to load configuration from default locations: %v", err)
			}
		}
	}

	// Load from environment variables (overrides file configuration)
	if !opts.SkipEnv {
		if err := config.LoadFromEnv(); err != nil {
			return nil, fmt.Errorf("failed to load configuration from environment: %v", err)
		}
	}

	// Validate configuration if requested
	if opts.Validate {
		if err := config.Validate(); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// LoadDefault loads configuration with default options
func LoadDefault() (*Config, error) {
	return Load(LoadOptions{
		Validate: true,
	})
}

// LoadWithFile loads configuration from a specific file
func LoadWithFile(filename string) (*Config, error) {
	return Load(LoadOptions{
		ConfigFile: filename,
		Validate:   true,
	})
}

// LoadForTesting loads configuration suitable for testing
func LoadForTesting() (*Config, error) {
	config := New()

	// Set test-friendly defaults
	config.Server.Port = 0 // Use random port
	config.Server.SecretKey = "test-secret-key"
	config.Clusters.Mock = true
	config.Auth.Enabled = false
	config.Storage.Type = "memory"

	return config, nil
}

// PrintConfiguration prints the current configuration (excluding sensitive data)
func (c *Config) PrintConfiguration() {
	fmt.Println("Current Configuration:")
	fmt.Printf("  Server:\n")
	fmt.Printf("    Host: %s\n", c.Server.Host)
	fmt.Printf("    Port: %d\n", c.Server.Port)
	fmt.Printf("    Route Prefix: %s\n", c.Server.RoutePrefix)
	fmt.Printf("    Debug: %t\n", c.Server.Debug)
	fmt.Printf("    Secret Key: %s\n", maskSecret(c.Server.SecretKey))

	fmt.Printf("  Clusters:\n")
	if len(c.Clusters.URLs) > 0 {
		fmt.Printf("    URLs: %v\n", c.Clusters.URLs)
	}
	if c.Clusters.RegistryURL != "" {
		fmt.Printf("    Registry URL: %s\n", c.Clusters.RegistryURL)
	}
	if c.Clusters.KubeconfigPath != "" {
		fmt.Printf("    Kubeconfig Path: %s\n", c.Clusters.KubeconfigPath)
	}
	if len(c.Clusters.Contexts) > 0 {
		fmt.Printf("    Contexts: %v\n", c.Clusters.Contexts)
	}
	fmt.Printf("    Query Interval: %s\n", c.Clusters.QueryInterval)
	fmt.Printf("    Connect Timeout: %s\n", c.Clusters.ConnectTimeout)
	fmt.Printf("    Read Timeout: %s\n", c.Clusters.ReadTimeout)
	fmt.Printf("    Mock: %t\n", c.Clusters.Mock)

	fmt.Printf("  Authentication:\n")
	fmt.Printf("    Enabled: %t\n", c.Auth.Enabled)
	if c.Auth.Enabled {
		fmt.Printf("    Authorize URL: %s\n", c.Auth.AuthorizeURL)
		fmt.Printf("    Token URL: %s\n", c.Auth.TokenURL)
		fmt.Printf("    Client ID: %s\n", maskSecret(c.Auth.ClientID))
		fmt.Printf("    Client Secret: %s\n", maskSecret(c.Auth.ClientSecret))
		fmt.Printf("    Scope: %s\n", c.Auth.Scope)
		if c.Auth.CredentialsDir != "" {
			fmt.Printf("    Credentials Dir: %s\n", c.Auth.CredentialsDir)
		}
	}
	fmt.Printf("    Screen Tokens: %t\n", c.Auth.ScreenTokens)

	fmt.Printf("  Storage:\n")
	fmt.Printf("    Type: %s\n", c.Storage.Type)
	if c.Storage.Type == "redis" {
		fmt.Printf("    Redis URL: %s\n", maskSecret(c.Storage.RedisURL))
		fmt.Printf("    Redis DB: %d\n", c.Storage.RedisDB)
	}

	fmt.Printf("  UI:\n")
	if c.UI.URLTemplate != "" {
		fmt.Printf("    URL Template: %s\n", c.UI.URLTemplate)
	}
	if c.UI.NodeLinkURL != "" {
		fmt.Printf("    Node Link URL: %s\n", c.UI.NodeLinkURL)
	}
	if c.UI.PodLinkURL != "" {
		fmt.Printf("    Pod Link URL: %s\n", c.UI.PodLinkURL)
	}
	fmt.Printf("    Theme: %s\n", c.UI.Theme)
	fmt.Printf("    Show Capacity: %t\n", c.UI.ShowCapacity)
	fmt.Printf("    Show Requests: %t\n", c.UI.ShowRequests)
	fmt.Printf("    Show Limits: %t\n", c.UI.ShowLimits)
	fmt.Printf("    Show Usage: %t\n", c.UI.ShowUsage)
}

// maskSecret masks sensitive information for display
func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}
