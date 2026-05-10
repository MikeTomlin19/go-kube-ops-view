package config

import (
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	config := New()

	// Test default values
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}

	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got '%s'", config.Server.Host)
	}

	if config.Server.RoutePrefix != "/" {
		t.Errorf("Expected default route prefix '/', got '%s'", config.Server.RoutePrefix)
	}

	if config.Server.Debug != false {
		t.Errorf("Expected default debug false, got %t", config.Server.Debug)
	}

	if config.Server.SecretKey != "development" {
		t.Errorf("Expected default secret key 'development', got '%s'", config.Server.SecretKey)
	}

	if config.Clusters.QueryInterval != 5*time.Second {
		t.Errorf("Expected default query interval 5s, got %s", config.Clusters.QueryInterval)
	}

	if config.Clusters.ConnectTimeout != 10*time.Second {
		t.Errorf("Expected default connect timeout 10s, got %s", config.Clusters.ConnectTimeout)
	}

	if config.Clusters.ReadTimeout != 10*time.Second {
		t.Errorf("Expected default read timeout 10s, got %s", config.Clusters.ReadTimeout)
	}

	if config.Auth.Enabled != false {
		t.Errorf("Expected default auth enabled false, got %t", config.Auth.Enabled)
	}

	if config.Auth.Scope != "openid" {
		t.Errorf("Expected default auth scope 'openid', got '%s'", config.Auth.Scope)
	}

	if config.Storage.Type != "memory" {
		t.Errorf("Expected default storage type 'memory', got '%s'", config.Storage.Type)
	}

	if config.UI.Theme != "default" {
		t.Errorf("Expected default theme 'default', got '%s'", config.UI.Theme)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"SERVER_PORT", "HOST", "ROUTE_PREFIX", "DEBUG", "SECRET_KEY",
		"CLUSTERS", "CLUSTER_REGISTRY_URL", "KUBECONFIG_PATH", "KUBECONFIG_CONTEXTS",
		"QUERY_INTERVAL", "CONNECT_TIMEOUT", "READ_TIMEOUT", "MOCK",
		"OAUTH2_ENABLED", "OAUTH2_AUTHORIZE_URL", "OAUTH2_TOKEN_URL",
		"OAUTH2_CLIENT_ID", "OAUTH2_CLIENT_SECRET", "OAUTH2_SCOPE",
		"CREDENTIALS_DIR", "SCREEN_TOKENS",
		"STORAGE_TYPE", "REDIS_URL", "REDIS_DB",
		"URL_TEMPLATE", "NODE_LINK_URL_TEMPLATE", "POD_LINK_URL_TEMPLATE",
		"THEME", "SHOW_CAPACITY", "SHOW_REQUESTS", "SHOW_LIMITS", "SHOW_USAGE",
	}

	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
	}

	// Clean environment
	defer func() {
		for _, env := range envVars {
			if originalEnv[env] == "" {
				os.Unsetenv(env)
			} else {
				os.Setenv(env, originalEnv[env])
			}
		}
	}()

	// Set test environment variables
	testEnv := map[string]string{
		"SERVER_PORT":            "9090",
		"HOST":                   "127.0.0.1",
		"ROUTE_PREFIX":           "/kube-ops-view",
		"DEBUG":                  "true",
		"SECRET_KEY":             "test-secret",
		"CLUSTERS":               "https://k8s1.example.com,https://k8s2.example.com",
		"CLUSTER_REGISTRY_URL":   "https://registry.example.com",
		"KUBECONFIG_PATH":        "/home/user/.kube/config",
		"KUBECONFIG_CONTEXTS":    "context1,context2",
		"QUERY_INTERVAL":         "10s",
		"CONNECT_TIMEOUT":        "15s",
		"READ_TIMEOUT":           "20s",
		"MOCK":                   "true",
		"OAUTH2_ENABLED":         "true",
		"OAUTH2_AUTHORIZE_URL":   "https://auth.example.com/authorize",
		"OAUTH2_TOKEN_URL":       "https://auth.example.com/token",
		"OAUTH2_CLIENT_ID":       "test-client-id",
		"OAUTH2_CLIENT_SECRET":   "test-client-secret",
		"OAUTH2_SCOPE":           "read write",
		"CREDENTIALS_DIR":        "/etc/credentials",
		"SCREEN_TOKENS":          "true",
		"STORAGE_TYPE":           "redis",
		"REDIS_URL":              "redis://localhost:6379",
		"REDIS_DB":               "1",
		"URL_TEMPLATE":           "https://dashboard.example.com/{cluster}",
		"NODE_LINK_URL_TEMPLATE": "https://dashboard.example.com/node/{node}",
		"POD_LINK_URL_TEMPLATE":  "https://dashboard.example.com/pod/{pod}",
		"THEME":                  "dark",
		"SHOW_CAPACITY":          "false",
		"SHOW_REQUESTS":          "false",
		"SHOW_LIMITS":            "false",
		"SHOW_USAGE":             "false",
	}

	for key, value := range testEnv {
		os.Setenv(key, value)
	}

	config := New()
	err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	// Test server configuration
	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}

	if config.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", config.Server.Host)
	}

	if config.Server.RoutePrefix != "/kube-ops-view" {
		t.Errorf("Expected route prefix '/kube-ops-view', got '%s'", config.Server.RoutePrefix)
	}

	if config.Server.Debug != true {
		t.Errorf("Expected debug true, got %t", config.Server.Debug)
	}

	if config.Server.SecretKey != "test-secret" {
		t.Errorf("Expected secret key 'test-secret', got '%s'", config.Server.SecretKey)
	}

	// Test cluster configuration
	expectedURLs := []string{"https://k8s1.example.com", "https://k8s2.example.com"}
	if len(config.Clusters.URLs) != len(expectedURLs) {
		t.Errorf("Expected %d cluster URLs, got %d", len(expectedURLs), len(config.Clusters.URLs))
	}
	for i, url := range expectedURLs {
		if i < len(config.Clusters.URLs) && config.Clusters.URLs[i] != url {
			t.Errorf("Expected cluster URL '%s', got '%s'", url, config.Clusters.URLs[i])
		}
	}

	if config.Clusters.RegistryURL != "https://registry.example.com" {
		t.Errorf("Expected registry URL 'https://registry.example.com', got '%s'", config.Clusters.RegistryURL)
	}

	if config.Clusters.KubeconfigPath != "/home/user/.kube/config" {
		t.Errorf("Expected kubeconfig path '/home/user/.kube/config', got '%s'", config.Clusters.KubeconfigPath)
	}

	expectedContexts := []string{"context1", "context2"}
	if len(config.Clusters.Contexts) != len(expectedContexts) {
		t.Errorf("Expected %d contexts, got %d", len(expectedContexts), len(config.Clusters.Contexts))
	}

	if config.Clusters.QueryInterval != 10*time.Second {
		t.Errorf("Expected query interval 10s, got %s", config.Clusters.QueryInterval)
	}

	if config.Clusters.ConnectTimeout != 15*time.Second {
		t.Errorf("Expected connect timeout 15s, got %s", config.Clusters.ConnectTimeout)
	}

	if config.Clusters.ReadTimeout != 20*time.Second {
		t.Errorf("Expected read timeout 20s, got %s", config.Clusters.ReadTimeout)
	}

	if config.Clusters.Mock != true {
		t.Errorf("Expected mock true, got %t", config.Clusters.Mock)
	}

	// Test auth configuration
	if config.Auth.Enabled != true {
		t.Errorf("Expected auth enabled true, got %t", config.Auth.Enabled)
	}

	if config.Auth.AuthorizeURL != "https://auth.example.com/authorize" {
		t.Errorf("Expected authorize URL 'https://auth.example.com/authorize', got '%s'", config.Auth.AuthorizeURL)
	}

	if config.Auth.TokenURL != "https://auth.example.com/token" {
		t.Errorf("Expected token URL 'https://auth.example.com/token', got '%s'", config.Auth.TokenURL)
	}

	if config.Auth.ClientID != "test-client-id" {
		t.Errorf("Expected client ID 'test-client-id', got '%s'", config.Auth.ClientID)
	}

	if config.Auth.ClientSecret != "test-client-secret" {
		t.Errorf("Expected client secret 'test-client-secret', got '%s'", config.Auth.ClientSecret)
	}

	if config.Auth.Scope != "read write" {
		t.Errorf("Expected scope 'read write', got '%s'", config.Auth.Scope)
	}

	if config.Auth.CredentialsDir != "/etc/credentials" {
		t.Errorf("Expected credentials dir '/etc/credentials', got '%s'", config.Auth.CredentialsDir)
	}

	if config.Auth.ScreenTokens != true {
		t.Errorf("Expected screen tokens true, got %t", config.Auth.ScreenTokens)
	}

	// Test storage configuration
	if config.Storage.Type != "redis" {
		t.Errorf("Expected storage type 'redis', got '%s'", config.Storage.Type)
	}

	if config.Storage.RedisURL != "redis://localhost:6379" {
		t.Errorf("Expected Redis URL 'redis://localhost:6379', got '%s'", config.Storage.RedisURL)
	}

	if config.Storage.RedisDB != 1 {
		t.Errorf("Expected Redis DB 1, got %d", config.Storage.RedisDB)
	}

	// Test UI configuration
	if config.UI.URLTemplate != "https://dashboard.example.com/{cluster}" {
		t.Errorf("Expected URL template 'https://dashboard.example.com/{cluster}', got '%s'", config.UI.URLTemplate)
	}

	if config.UI.NodeLinkURL != "https://dashboard.example.com/node/{node}" {
		t.Errorf("Expected node link URL 'https://dashboard.example.com/node/{node}', got '%s'", config.UI.NodeLinkURL)
	}

	if config.UI.PodLinkURL != "https://dashboard.example.com/pod/{pod}" {
		t.Errorf("Expected pod link URL 'https://dashboard.example.com/pod/{pod}', got '%s'", config.UI.PodLinkURL)
	}

	if config.UI.Theme != "dark" {
		t.Errorf("Expected theme 'dark', got '%s'", config.UI.Theme)
	}

	if config.UI.ShowCapacity != false {
		t.Errorf("Expected show capacity false, got %t", config.UI.ShowCapacity)
	}

	if config.UI.ShowRequests != false {
		t.Errorf("Expected show requests false, got %t", config.UI.ShowRequests)
	}

	if config.UI.ShowLimits != false {
		t.Errorf("Expected show limits false, got %t", config.UI.ShowLimits)
	}

	if config.UI.ShowUsage != false {
		t.Errorf("Expected show usage false, got %t", config.UI.ShowUsage)
	}
}

func TestLoadFromEnvInvalidValues(t *testing.T) {
	// Save original environment
	originalPort := os.Getenv("SERVER_PORT")
	originalInterval := os.Getenv("QUERY_INTERVAL")

	defer func() {
		if originalPort == "" {
			os.Unsetenv("SERVER_PORT")
		} else {
			os.Setenv("SERVER_PORT", originalPort)
		}
		if originalInterval == "" {
			os.Unsetenv("QUERY_INTERVAL")
		} else {
			os.Setenv("QUERY_INTERVAL", originalInterval)
		}
	}()

	// Test invalid port
	os.Setenv("SERVER_PORT", "invalid")
	config := New()
	err := config.LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid SERVER_PORT, got nil")
	}

	// Test invalid query interval
	os.Unsetenv("SERVER_PORT")
	os.Setenv("QUERY_INTERVAL", "invalid")
	config = New()
	err = config.LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid QUERY_INTERVAL, got nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			config:      New(),
			expectError: false,
		},
		{
			name: "invalid port - too low",
			config: &Config{
				Server: ServerConfig{Port: 0, SecretKey: "test"},
			},
			expectError: true,
			errorMsg:    "server port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Server: ServerConfig{Port: 70000, SecretKey: "test"},
			},
			expectError: true,
			errorMsg:    "server port must be between 1 and 65535",
		},
		{
			name: "empty secret key",
			config: &Config{
				Server: ServerConfig{Port: 8080, SecretKey: ""},
			},
			expectError: true,
			errorMsg:    "server secret key cannot be empty",
		},
		{
			name: "invalid route prefix",
			config: &Config{
				Server: ServerConfig{Port: 8080, SecretKey: "test", RoutePrefix: "invalid"},
			},
			expectError: true,
			errorMsg:    "route prefix must start with '/'",
		},
		{
			name: "no cluster sources",
			config: &Config{
				Server:   ServerConfig{Port: 8080, SecretKey: "test"},
				Clusters: ClusterConfig{QueryInterval: 5 * time.Second, ConnectTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second},
			},
			expectError: true,
			errorMsg:    "at least one cluster source must be configured",
		},
		{
			name: "invalid query interval",
			config: &Config{
				Server:   ServerConfig{Port: 8080, SecretKey: "test"},
				Clusters: ClusterConfig{URLs: []string{"https://k8s.example.com"}, QueryInterval: 500 * time.Millisecond, ConnectTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second},
			},
			expectError: true,
			errorMsg:    "query interval must be at least 1 second",
		},
		{
			name: "auth enabled without authorize URL",
			config: &Config{
				Server:   ServerConfig{Port: 8080, SecretKey: "test"},
				Clusters: ClusterConfig{URLs: []string{"https://k8s.example.com"}, QueryInterval: 5 * time.Second, ConnectTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second},
				Auth:     AuthConfig{Enabled: true},
			},
			expectError: true,
			errorMsg:    "OAuth2 authorize URL is required when authentication is enabled",
		},
		{
			name: "invalid storage type",
			config: &Config{
				Server:   ServerConfig{Port: 8080, SecretKey: "test"},
				Clusters: ClusterConfig{URLs: []string{"https://k8s.example.com"}, QueryInterval: 5 * time.Second, ConnectTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second},
				Storage:  StorageConfig{Type: "invalid"},
			},
			expectError: true,
			errorMsg:    "storage type must be 'memory' or 'redis'",
		},
		{
			name: "redis storage without URL",
			config: &Config{
				Server:   ServerConfig{Port: 8080, SecretKey: "test"},
				Clusters: ClusterConfig{URLs: []string{"https://k8s.example.com"}, QueryInterval: 5 * time.Second, ConnectTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second},
				Storage:  StorageConfig{Type: "redis"},
			},
			expectError: true,
			errorMsg:    "Redis URL is required when using Redis storage",
		},
		{
			name: "invalid theme",
			config: &Config{
				Server:   ServerConfig{Port: 8080, SecretKey: "test"},
				Clusters: ClusterConfig{URLs: []string{"https://k8s.example.com"}, QueryInterval: 5 * time.Second, ConnectTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second},
				UI:       UIConfig{Theme: "invalid"},
			},
			expectError: true,
			errorMsg:    "theme must be one of: default, dark, light, colorblind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestGetClusterSources(t *testing.T) {
	config := &Config{
		Clusters: ClusterConfig{
			URLs:           []string{"https://k8s1.example.com", "https://k8s2.example.com"},
			RegistryURL:    "https://registry.example.com",
			KubeconfigPath: "/home/user/.kube/config",
			Contexts:       []string{"context1", "context2"},
			Mock:           true,
		},
	}

	sources := config.GetClusterSources()

	expectedSources := []string{
		"Static URLs: https://k8s1.example.com, https://k8s2.example.com",
		"Registry: https://registry.example.com",
		"Kubeconfig: /home/user/.kube/config (context1, context2)",
		"Mock clusters",
	}

	if len(sources) != len(expectedSources) {
		t.Errorf("Expected %d sources, got %d", len(expectedSources), len(sources))
	}

	for i, expected := range expectedSources {
		if i < len(sources) && sources[i] != expected {
			t.Errorf("Expected source '%s', got '%s'", expected, sources[i])
		}
	}
}

func TestIsAuthEnabled(t *testing.T) {
	config := New()
	if config.IsAuthEnabled() {
		t.Error("Expected auth disabled by default")
	}

	config.Auth.Enabled = true
	if !config.IsAuthEnabled() {
		t.Error("Expected auth enabled")
	}
}

func TestIsRedisEnabled(t *testing.T) {
	config := New()
	if config.IsRedisEnabled() {
		t.Error("Expected Redis disabled by default")
	}

	config.Storage.Type = "redis"
	if !config.IsRedisEnabled() {
		t.Error("Expected Redis enabled")
	}
}

func TestGetServerAddress(t *testing.T) {
	config := New()
	expected := "0.0.0.0:8080"
	if addr := config.GetServerAddress(); addr != expected {
		t.Errorf("Expected server address '%s', got '%s'", expected, addr)
	}

	config.Server.Host = "127.0.0.1"
	config.Server.Port = 9090
	expected = "127.0.0.1:9090"
	if addr := config.GetServerAddress(); addr != expected {
		t.Errorf("Expected server address '%s', got '%s'", expected, addr)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
