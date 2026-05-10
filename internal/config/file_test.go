package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  port: 9090
  host: "127.0.0.1"
  route_prefix: "/test"
  debug: true
  secret_key: "test-secret"

clusters:
  urls:
    - "https://k8s1.example.com"
    - "https://k8s2.example.com"
  registry_url: "https://registry.example.com"
  kubeconfig_path: "/home/user/.kube/config"
  contexts:
    - "context1"
    - "context2"
  query_interval: "10s"
  connect_timeout: "15s"
  read_timeout: "20s"
  mock: true

auth:
  enabled: true
  authorize_url: "https://auth.example.com/authorize"
  token_url: "https://auth.example.com/token"
  client_id: "test-client-id"
  client_secret: "test-client-secret"
  scope: "read write"
  credentials_dir: "/etc/credentials"
  screen_tokens: true

storage:
  type: "redis"
  redis_url: "redis://localhost:6379"
  redis_db: 1

ui:
  url_template: "https://dashboard.example.com/{cluster}"
  node_link_url: "https://dashboard.example.com/node/{node}"
  pod_link_url: "https://dashboard.example.com/pod/{pod}"
  theme: "dark"
  show_capacity: false
  show_requests: false
  show_limits: false
  show_usage: false
`

	err = ioutil.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config := New()
	err = config.LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Test server configuration
	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}

	if config.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", config.Server.Host)
	}

	if config.Server.RoutePrefix != "/test" {
		t.Errorf("Expected route prefix '/test', got '%s'", config.Server.RoutePrefix)
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

	if config.Clusters.QueryInterval != 10*time.Second {
		t.Errorf("Expected query interval 10s, got %s", config.Clusters.QueryInterval)
	}

	// Test auth configuration
	if config.Auth.Enabled != true {
		t.Errorf("Expected auth enabled true, got %t", config.Auth.Enabled)
	}

	if config.Auth.ClientID != "test-client-id" {
		t.Errorf("Expected client ID 'test-client-id', got '%s'", config.Auth.ClientID)
	}

	// Test storage configuration
	if config.Storage.Type != "redis" {
		t.Errorf("Expected storage type 'redis', got '%s'", config.Storage.Type)
	}

	if config.Storage.RedisDB != 1 {
		t.Errorf("Expected Redis DB 1, got %d", config.Storage.RedisDB)
	}

	// Test UI configuration
	if config.UI.Theme != "dark" {
		t.Errorf("Expected theme 'dark', got '%s'", config.UI.Theme)
	}

	if config.UI.ShowCapacity != false {
		t.Errorf("Expected show capacity false, got %t", config.UI.ShowCapacity)
	}
}

func TestLoadFromFileNotFound(t *testing.T) {
	config := New()
	err := config.LoadFromFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestLoadFromFileEmpty(t *testing.T) {
	config := New()
	err := config.LoadFromFile("")
	if err != nil {
		t.Errorf("Expected no error for empty filename, got %v", err)
	}
}

func TestLoadFromFileInvalidYAML(t *testing.T) {
	// Create a temporary config file with invalid YAML
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	invalidYAML := `
server:
  port: 8080
  invalid yaml structure
    missing colon
`

	err = ioutil.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config := New()
	err = config.LoadFromFile(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestSaveToFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := New()
	config.Server.Port = 9090
	config.Server.Host = "127.0.0.1"
	config.Server.Debug = true
	config.Clusters.URLs = []string{"https://k8s.example.com"}

	configFile := filepath.Join(tmpDir, "config.yaml")
	err = config.SaveToFile(configFile)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load the saved config and verify
	newConfig := New()
	err = newConfig.LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if newConfig.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", newConfig.Server.Port)
	}

	if newConfig.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", newConfig.Server.Host)
	}

	if newConfig.Server.Debug != true {
		t.Errorf("Expected debug true, got %t", newConfig.Server.Debug)
	}

	if len(newConfig.Clusters.URLs) != 1 || newConfig.Clusters.URLs[0] != "https://k8s.example.com" {
		t.Errorf("Expected cluster URL 'https://k8s.example.com', got %v", newConfig.Clusters.URLs)
	}
}

func TestSaveToFileCreateDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := New()

	// Save to a nested directory that doesn't exist
	configFile := filepath.Join(tmpDir, "nested", "dir", "config.yaml")
	err = config.SaveToFile(configFile)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created in nested directory")
	}
}

func TestLoadFromDefaultLocations(t *testing.T) {
	// Create a temporary directory to simulate a home directory
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a config file in the current directory
	configContent := `
server:
  port: 9999
  debug: true
`

	configFile := filepath.Join(tmpDir, "config.yaml")
	err = ioutil.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Change to the temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config := New()
	err = config.LoadFromDefaultLocations()
	if err != nil {
		t.Fatalf("LoadFromDefaultLocations failed: %v", err)
	}

	// Should have loaded the config from ./config.yaml
	if config.Server.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", config.Server.Port)
	}

	if config.Server.Debug != true {
		t.Errorf("Expected debug true, got %t", config.Server.Debug)
	}
}

func TestLoadFromDefaultLocationsNoFile(t *testing.T) {
	// Create a temporary directory with no config files
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to the temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config := New()
	err = config.LoadFromDefaultLocations()
	if err != nil {
		t.Errorf("LoadFromDefaultLocations should not fail when no config file is found, got: %v", err)
	}

	// Should still have default values
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}
}
