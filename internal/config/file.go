package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadFromFile loads configuration from a YAML file
func (c *Config) LoadFromFile(filename string) error {
	if filename == "" {
		return nil // No file specified, skip
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found: %s", filename)
	}

	// Read file content
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read configuration file %s: %v", filename, err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse configuration file %s: %v", filename, err)
	}

	return nil
}

// SaveToFile saves the current configuration to a YAML file
func (c *Config) SaveToFile(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %v", err)
	}

	// Write to file
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file %s: %v", filename, err)
	}

	return nil
}

// LoadFromDefaultLocations attempts to load configuration from default locations
func (c *Config) LoadFromDefaultLocations() error {
	// List of default configuration file locations
	defaultLocations := []string{
		"./config.yaml",
		"./config.yml",
		"./kube-ops-view.yaml",
		"./kube-ops-view.yml",
		"/etc/kube-ops-view/config.yaml",
		"/etc/kube-ops-view/config.yml",
	}

	// Try to find and load from home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		defaultLocations = append(defaultLocations,
			filepath.Join(homeDir, ".kube-ops-view.yaml"),
			filepath.Join(homeDir, ".kube-ops-view.yml"),
			filepath.Join(homeDir, ".config", "kube-ops-view", "config.yaml"),
			filepath.Join(homeDir, ".config", "kube-ops-view", "config.yml"),
		)
	}

	// Try each location
	for _, location := range defaultLocations {
		if _, err := os.Stat(location); err == nil {
			return c.LoadFromFile(location)
		}
	}

	// No configuration file found, which is okay
	return nil
}
