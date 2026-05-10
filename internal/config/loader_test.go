package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
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
  debug: true
clusters:
  urls:
    - "https://k8s.example.com"
`

	err = ioutil.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Save and set environment variable
	originalPort := os.Getenv("SERVER_PORT")
	defer func() {
		if originalPort == "" {
			os.Unsetenv("SERVER_PORT")
		} else {
			os.Setenv("SERVER_PORT", originalPort)
		}
	}()
	os.Setenv("SERVER_PORT", "8888")

	// Load configuration
	config, err := Load(LoadOptions{
		ConfigFile: configFile,
		Validate:   true,
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Environment variable should override file configuration
	if config.Server.Port != 8888 {
		t.Errorf("Expected port 8888 (from env), got %d", config.Server.Port)
	}

	// File configuration should be loaded
	if config.Server.Debug != true {
		t.Errorf("Expected debug true (from file), got %t", config.Server.Debug)
	}

	// Should have cluster from file
	if len(config.Clusters.URLs) != 1 || config.Clusters.URLs[0] != "https://k8s.example.com" {
		t.Errorf("Expected cluster URL from file, got %v", config.Clusters.URLs)
	}
}

func TestLoadSkipEnv(t *testing.T) {
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
clusters:
  urls:
    - "https://k8s.example.com"
`

	err = ioutil.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set environment variable
	originalPort := os.Getenv("SERVER_PORT")
	defer func() {
		if originalPort == "" {
			os.Unsetenv("SERVER_PORT")
		} else {
			os.Setenv("SERVER_PORT", originalPort)
		}
	}()
	os.Setenv("SERVER_PORT", "8888")

	// Load configuration with SkipEnv
	config, err := Load(LoadOptions{
		ConfigFile: configFile,
		SkipEnv:    true,
		Validate:   true,
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should use file configuration, not environment
	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090 (from file), got %d", config.Server.Port)
	}
}

func TestLoadSkipDefaultFile(t *testing.T) {
	// Change to a directory with a config file
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  port: 9090
clusters:
  urls:
    - "https://k8s.example.com"
`

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

	// Load configuration with SkipDefaultFile
	config, err := Load(LoadOptions{
		SkipDefaultFile: true,
		Validate:        true,
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should use default values, not file
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}
}

func TestLoadValidationFailure(t *testing.T) {
	// Create a config file with invalid configuration
	tmpDir, err := ioutil.TempDir("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  port: 0  # Invalid port
  secret_key: ""  # Empty secret key
`

	err = ioutil.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load configuration with validation
	_, err = Load(LoadOptions{
		ConfigFile: configFile,
		Validate:   true,
	})
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
}

func TestLoadDefault(t *testing.T) {
	config, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault failed: %v", err)
	}

	// Should have default values
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}

	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got '%s'", config.Server.Host)
	}
}

func TestLoadWithFile(t *testing.T) {
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
clusters:
  urls:
    - "https://k8s.example.com"
`

	err = ioutil.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadWithFile(configFile)
	if err != nil {
		t.Fatalf("LoadWithFile failed: %v", err)
	}

	// Should have loaded from file
	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}

	if len(config.Clusters.URLs) != 1 || config.Clusters.URLs[0] != "https://k8s.example.com" {
		t.Errorf("Expected cluster URL from file, got %v", config.Clusters.URLs)
	}
}

func TestLoadForTesting(t *testing.T) {
	config, err := LoadForTesting()
	if err != nil {
		t.Fatalf("LoadForTesting failed: %v", err)
	}

	// Should have test-friendly values
	if config.Server.Port != 0 {
		t.Errorf("Expected port 0 for testing, got %d", config.Server.Port)
	}

	if config.Server.SecretKey != "test-secret-key" {
		t.Errorf("Expected test secret key, got '%s'", config.Server.SecretKey)
	}

	if config.Clusters.Mock != true {
		t.Errorf("Expected mock clusters for testing, got %t", config.Clusters.Mock)
	}

	if config.Auth.Enabled != false {
		t.Errorf("Expected auth disabled for testing, got %t", config.Auth.Enabled)
	}

	if config.Storage.Type != "memory" {
		t.Errorf("Expected memory storage for testing, got '%s'", config.Storage.Type)
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"short", "***"},
		{"12345678", "***"},
		{"123456789", "1234***6789"},
		{"very-long-secret-key-here", "very***here"},
	}

	for _, tt := range tests {
		result := maskSecret(tt.input)
		if result != tt.expected {
			t.Errorf("maskSecret(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
