package config_test

import (
	"fmt"
	"log"
	"os"

	"kube-ops-view/internal/config"

	"github.com/spf13/cobra"
)

// ExampleLoad demonstrates how to load configuration from multiple sources
func ExampleLoad() {
	// Load configuration with default options
	cfg, err := config.LoadDefault()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Server will run on %s\n", cfg.GetServerAddress())
	fmt.Printf("Mock mode: %t\n", cfg.Clusters.Mock)

	// Output:
	// Server will run on 0.0.0.0:8080
	// Mock mode: true
}

// ExampleConfig_LoadFromEnv demonstrates loading from environment variables
func ExampleConfig_LoadFromEnv() {
	// Set some environment variables
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("DEBUG", "true")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("DEBUG")
	}()

	cfg := config.New()
	err := cfg.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Port: %d\n", cfg.Server.Port)
	fmt.Printf("Debug: %t\n", cfg.Server.Debug)

	// Output:
	// Port: 9090
	// Debug: true
}

// ExampleConfig_AddFlags demonstrates how to add CLI flags
func ExampleConfig_AddFlags() {
	cfg := config.New()

	cmd := &cobra.Command{
		Use: "kube-ops-view",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Port: %d\n", cfg.Server.Port)
			fmt.Printf("Debug: %t\n", cfg.Server.Debug)
		},
	}

	// Add configuration flags to the command
	cfg.AddFlags(cmd)

	// Simulate command line arguments
	cmd.SetArgs([]string{"--port", "8888", "--debug"})
	cmd.Execute()

	// Output:
	// Port: 8888
	// Debug: true
}

// ExampleLoadForTesting demonstrates loading test configuration
func ExampleLoadForTesting() {
	cfg, err := config.LoadForTesting()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Port: %d (random)\n", cfg.Server.Port)
	fmt.Printf("Mock: %t\n", cfg.Clusters.Mock)
	fmt.Printf("Auth: %t\n", cfg.Auth.Enabled)

	// Output:
	// Port: 0 (random)
	// Mock: true
	// Auth: false
}
