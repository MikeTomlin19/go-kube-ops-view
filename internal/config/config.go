package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the complete application configuration
type Config struct {
	Server   ServerConfig  `yaml:"server"`
	Clusters ClusterConfig `yaml:"clusters"`
	Auth     AuthConfig    `yaml:"auth"`
	Storage  StorageConfig `yaml:"storage"`
	UI       UIConfig      `yaml:"ui"`
	Logging  LoggingConfig `yaml:"logging"`
}

// ServerConfig contains web server configuration
type ServerConfig struct {
	Port        int    `yaml:"port" env:"SERVER_PORT" default:"8080"`
	Host        string `yaml:"host" env:"HOST" default:"0.0.0.0"`
	RoutePrefix string `yaml:"route_prefix" env:"ROUTE_PREFIX" default:"/"`
	Debug       bool   `yaml:"debug" env:"DEBUG" default:"false"`
	SecretKey   string `yaml:"secret_key" env:"SECRET_KEY" default:"development"`
}

// ClusterConfig contains Kubernetes cluster configuration
type ClusterConfig struct {
	URLs           []string      `yaml:"urls" env:"CLUSTERS"`
	RegistryURL    string        `yaml:"registry_url" env:"CLUSTER_REGISTRY_URL"`
	KubeconfigPath string        `yaml:"kubeconfig_path" env:"KUBECONFIG_PATH"`
	Contexts       []string      `yaml:"contexts" env:"KUBECONFIG_CONTEXTS"`
	QueryInterval  time.Duration `yaml:"query_interval" env:"QUERY_INTERVAL" default:"5s"`
	ConnectTimeout time.Duration `yaml:"connect_timeout" env:"CONNECT_TIMEOUT" default:"10s"`
	ReadTimeout    time.Duration `yaml:"read_timeout" env:"READ_TIMEOUT" default:"10s"`
	Mock           bool          `yaml:"mock" env:"MOCK" default:"false"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Enabled        bool   `yaml:"enabled" env:"OAUTH2_ENABLED" default:"false"`
	AuthorizeURL   string `yaml:"authorize_url" env:"OAUTH2_AUTHORIZE_URL"`
	TokenURL       string `yaml:"token_url" env:"OAUTH2_TOKEN_URL"`
	ClientID       string `yaml:"client_id" env:"OAUTH2_CLIENT_ID"`
	ClientSecret   string `yaml:"client_secret" env:"OAUTH2_CLIENT_SECRET"`
	Scope          string `yaml:"scope" env:"OAUTH2_SCOPE" default:"openid"`
	CredentialsDir string `yaml:"credentials_dir" env:"CREDENTIALS_DIR"`
	ScreenTokens   bool   `yaml:"screen_tokens" env:"SCREEN_TOKENS" default:"false"`
}

// StorageConfig contains data storage configuration
type StorageConfig struct {
	Type     string `yaml:"type" env:"STORAGE_TYPE" default:"memory"`
	RedisURL string `yaml:"redis_url" env:"REDIS_URL"`
	RedisDB  int    `yaml:"redis_db" env:"REDIS_DB" default:"0"`
}

// UIConfig contains user interface configuration
type UIConfig struct {
	URLTemplate  string `yaml:"url_template" env:"URL_TEMPLATE"`
	NodeLinkURL  string `yaml:"node_link_url" env:"NODE_LINK_URL_TEMPLATE"`
	PodLinkURL   string `yaml:"pod_link_url" env:"POD_LINK_URL_TEMPLATE"`
	Theme        string `yaml:"theme" env:"THEME" default:"default"`
	ShowCapacity bool   `yaml:"show_capacity" env:"SHOW_CAPACITY" default:"true"`
	ShowRequests bool   `yaml:"show_requests" env:"SHOW_REQUESTS" default:"true"`
	ShowLimits   bool   `yaml:"show_limits" env:"SHOW_LIMITS" default:"true"`
	ShowUsage    bool   `yaml:"show_usage" env:"SHOW_USAGE" default:"true"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level" env:"LOG_LEVEL" default:"info"`
	Format     string `yaml:"format" env:"LOG_FORMAT" default:"json"`
	Output     string `yaml:"output" env:"LOG_OUTPUT" default:"stdout"`
	AddSource  bool   `yaml:"add_source" env:"LOG_ADD_SOURCE" default:"false"`
	TimeFormat string `yaml:"time_format" env:"LOG_TIME_FORMAT" default:"rfc3339"`
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		Server: ServerConfig{
			Port:        8080,
			Host:        "0.0.0.0",
			RoutePrefix: "/",
			Debug:       false,
			SecretKey:   "development",
		},
		Clusters: ClusterConfig{
			QueryInterval:  5 * time.Second,
			ConnectTimeout: 10 * time.Second,
			ReadTimeout:    10 * time.Second,
			Mock:           true, // Default to mock mode for easier setup
		},
		Auth: AuthConfig{
			Enabled:      false,
			Scope:        "openid",
			ScreenTokens: false,
		},
		Storage: StorageConfig{
			Type:    "memory",
			RedisDB: 0,
		},
		UI: UIConfig{
			Theme:        "default",
			ShowCapacity: true,
			ShowRequests: true,
			ShowLimits:   true,
			ShowUsage:    true,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			AddSource:  false,
			TimeFormat: "rfc3339",
		},
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() error {
	// Server configuration
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Server.Port = p
		} else {
			return fmt.Errorf("invalid SERVER_PORT: %v", err)
		}
	}

	if host := os.Getenv("HOST"); host != "" {
		c.Server.Host = host
	}

	if prefix := os.Getenv("ROUTE_PREFIX"); prefix != "" {
		c.Server.RoutePrefix = prefix
	}

	if debug := os.Getenv("DEBUG"); debug != "" {
		c.Server.Debug = strings.ToLower(debug) == "true"
	}

	if secret := os.Getenv("SECRET_KEY"); secret != "" {
		c.Server.SecretKey = secret
	}

	// Cluster configuration
	if clusters := os.Getenv("CLUSTERS"); clusters != "" {
		c.Clusters.URLs = strings.Split(clusters, ",")
		for i := range c.Clusters.URLs {
			c.Clusters.URLs[i] = strings.TrimSpace(c.Clusters.URLs[i])
		}
	}

	if registryURL := os.Getenv("CLUSTER_REGISTRY_URL"); registryURL != "" {
		c.Clusters.RegistryURL = registryURL
	}

	if kubeconfigPath := os.Getenv("KUBECONFIG_PATH"); kubeconfigPath != "" {
		c.Clusters.KubeconfigPath = kubeconfigPath
	}

	if contexts := os.Getenv("KUBECONFIG_CONTEXTS"); contexts != "" {
		c.Clusters.Contexts = strings.Split(contexts, ",")
		for i := range c.Clusters.Contexts {
			c.Clusters.Contexts[i] = strings.TrimSpace(c.Clusters.Contexts[i])
		}
	}

	if interval := os.Getenv("QUERY_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			c.Clusters.QueryInterval = d
		} else {
			return fmt.Errorf("invalid QUERY_INTERVAL: %v", err)
		}
	}

	if timeout := os.Getenv("CONNECT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.Clusters.ConnectTimeout = d
		} else {
			return fmt.Errorf("invalid CONNECT_TIMEOUT: %v", err)
		}
	}

	if timeout := os.Getenv("READ_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.Clusters.ReadTimeout = d
		} else {
			return fmt.Errorf("invalid READ_TIMEOUT: %v", err)
		}
	}

	if mock := os.Getenv("MOCK"); mock != "" {
		c.Clusters.Mock = strings.ToLower(mock) == "true"
	}

	// Auth configuration
	if enabled := os.Getenv("OAUTH2_ENABLED"); enabled != "" {
		c.Auth.Enabled = strings.ToLower(enabled) == "true"
	}

	if authURL := os.Getenv("OAUTH2_AUTHORIZE_URL"); authURL != "" {
		c.Auth.AuthorizeURL = authURL
	}

	if tokenURL := os.Getenv("OAUTH2_TOKEN_URL"); tokenURL != "" {
		c.Auth.TokenURL = tokenURL
	}

	if clientID := os.Getenv("OAUTH2_CLIENT_ID"); clientID != "" {
		c.Auth.ClientID = clientID
	}

	if clientSecret := os.Getenv("OAUTH2_CLIENT_SECRET"); clientSecret != "" {
		c.Auth.ClientSecret = clientSecret
	}

	if scope := os.Getenv("OAUTH2_SCOPE"); scope != "" {
		c.Auth.Scope = scope
	}

	if credDir := os.Getenv("CREDENTIALS_DIR"); credDir != "" {
		c.Auth.CredentialsDir = credDir
	}

	if screenTokens := os.Getenv("SCREEN_TOKENS"); screenTokens != "" {
		c.Auth.ScreenTokens = strings.ToLower(screenTokens) == "true"
	}

	// Storage configuration
	if storageType := os.Getenv("STORAGE_TYPE"); storageType != "" {
		c.Storage.Type = storageType
	}

	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		c.Storage.RedisURL = redisURL
	}

	if redisDB := os.Getenv("REDIS_DB"); redisDB != "" {
		if db, err := strconv.Atoi(redisDB); err == nil {
			c.Storage.RedisDB = db
		} else {
			return fmt.Errorf("invalid REDIS_DB: %v", err)
		}
	}

	// UI configuration
	if urlTemplate := os.Getenv("URL_TEMPLATE"); urlTemplate != "" {
		c.UI.URLTemplate = urlTemplate
	}

	if nodeLinkURL := os.Getenv("NODE_LINK_URL_TEMPLATE"); nodeLinkURL != "" {
		c.UI.NodeLinkURL = nodeLinkURL
	}

	if podLinkURL := os.Getenv("POD_LINK_URL_TEMPLATE"); podLinkURL != "" {
		c.UI.PodLinkURL = podLinkURL
	}

	if theme := os.Getenv("THEME"); theme != "" {
		c.UI.Theme = theme
	}

	if showCapacity := os.Getenv("SHOW_CAPACITY"); showCapacity != "" {
		c.UI.ShowCapacity = strings.ToLower(showCapacity) == "true"
	}

	if showRequests := os.Getenv("SHOW_REQUESTS"); showRequests != "" {
		c.UI.ShowRequests = strings.ToLower(showRequests) == "true"
	}

	if showLimits := os.Getenv("SHOW_LIMITS"); showLimits != "" {
		c.UI.ShowLimits = strings.ToLower(showLimits) == "true"
	}

	if showUsage := os.Getenv("SHOW_USAGE"); showUsage != "" {
		c.UI.ShowUsage = strings.ToLower(showUsage) == "true"
	}

	// Logging configuration
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		c.Logging.Level = logLevel
	}

	if logFormat := os.Getenv("LOG_FORMAT"); logFormat != "" {
		c.Logging.Format = logFormat
	}

	if logOutput := os.Getenv("LOG_OUTPUT"); logOutput != "" {
		c.Logging.Output = logOutput
	}

	if logAddSource := os.Getenv("LOG_ADD_SOURCE"); logAddSource != "" {
		c.Logging.AddSource = strings.ToLower(logAddSource) == "true"
	}

	if logTimeFormat := os.Getenv("LOG_TIME_FORMAT"); logTimeFormat != "" {
		c.Logging.TimeFormat = logTimeFormat
	}

	return nil
}

// Validate validates the configuration and returns an error if invalid
func (c *Config) Validate() error {
	var errors []string

	// Validate server configuration
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errors = append(errors, "server port must be between 1 and 65535")
	}

	if c.Server.SecretKey == "" {
		errors = append(errors, "server secret key cannot be empty")
	}

	if c.Server.RoutePrefix != "" && !strings.HasPrefix(c.Server.RoutePrefix, "/") {
		errors = append(errors, "route prefix must start with '/'")
	}

	// Validate cluster configuration
	if len(c.Clusters.URLs) == 0 && c.Clusters.RegistryURL == "" && c.Clusters.KubeconfigPath == "" && !c.Clusters.Mock {
		errors = append(errors, "at least one cluster source must be configured (URLs, registry URL, kubeconfig path, or mock mode)")
	}

	if c.Clusters.QueryInterval < time.Second {
		errors = append(errors, "query interval must be at least 1 second")
	}

	if c.Clusters.ConnectTimeout < time.Second {
		errors = append(errors, "connect timeout must be at least 1 second")
	}

	if c.Clusters.ReadTimeout < time.Second {
		errors = append(errors, "read timeout must be at least 1 second")
	}

	// Validate auth configuration
	if c.Auth.Enabled {
		if c.Auth.AuthorizeURL == "" {
			errors = append(errors, "OAuth2 authorize URL is required when authentication is enabled")
		}
		if c.Auth.TokenURL == "" {
			errors = append(errors, "OAuth2 token URL is required when authentication is enabled")
		}
		if c.Auth.ClientID == "" {
			errors = append(errors, "OAuth2 client ID is required when authentication is enabled")
		}
		if c.Auth.ClientSecret == "" {
			errors = append(errors, "OAuth2 client secret is required when authentication is enabled")
		}
	}

	// Validate storage configuration
	if c.Storage.Type != "memory" && c.Storage.Type != "redis" {
		errors = append(errors, "storage type must be 'memory' or 'redis'")
	}

	if c.Storage.Type == "redis" && c.Storage.RedisURL == "" {
		errors = append(errors, "Redis URL is required when using Redis storage")
	}

	if c.Storage.RedisDB < 0 {
		errors = append(errors, "Redis DB must be non-negative")
	}

	// Validate UI configuration
	validThemes := []string{"default", "dark", "light", "colorblind"}
	themeValid := false
	for _, theme := range validThemes {
		if c.UI.Theme == theme {
			themeValid = true
			break
		}
	}
	if !themeValid {
		errors = append(errors, fmt.Sprintf("theme must be one of: %s", strings.Join(validThemes, ", ")))
	}

	// Validate logging configuration
	validLogLevels := []string{"debug", "info", "warn", "error"}
	logLevelValid := false
	for _, level := range validLogLevels {
		if c.Logging.Level == level {
			logLevelValid = true
			break
		}
	}
	if !logLevelValid {
		errors = append(errors, fmt.Sprintf("log level must be one of: %s", strings.Join(validLogLevels, ", ")))
	}

	validLogFormats := []string{"json", "text"}
	logFormatValid := false
	for _, format := range validLogFormats {
		if c.Logging.Format == format {
			logFormatValid = true
			break
		}
	}
	if !logFormatValid {
		errors = append(errors, fmt.Sprintf("log format must be one of: %s", strings.Join(validLogFormats, ", ")))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}

// GetClusterSources returns all configured cluster sources
func (c *Config) GetClusterSources() []string {
	var sources []string

	if len(c.Clusters.URLs) > 0 {
		sources = append(sources, fmt.Sprintf("Static URLs: %s", strings.Join(c.Clusters.URLs, ", ")))
	}

	if c.Clusters.RegistryURL != "" {
		sources = append(sources, fmt.Sprintf("Registry: %s", c.Clusters.RegistryURL))
	}

	if c.Clusters.KubeconfigPath != "" {
		contexts := "all contexts"
		if len(c.Clusters.Contexts) > 0 {
			contexts = strings.Join(c.Clusters.Contexts, ", ")
		}
		sources = append(sources, fmt.Sprintf("Kubeconfig: %s (%s)", c.Clusters.KubeconfigPath, contexts))
	}

	if c.Clusters.Mock {
		sources = append(sources, "Mock clusters")
	}

	return sources
}

// IsAuthEnabled returns true if authentication is enabled
func (c *Config) IsAuthEnabled() bool {
	return c.Auth.Enabled
}

// IsRedisEnabled returns true if Redis storage is configured
func (c *Config) IsRedisEnabled() bool {
	return c.Storage.Type == "redis"
}

// GetServerAddress returns the full server address
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
