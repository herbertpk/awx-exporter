package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// Default configuration values
const (
	defaultScrapeInterval = 5 * time.Minute
	defaultPort           = "8080"
)

// loadConfig loads and validates configuration from environment variables
func loadConfig() *AWXConfig {
	config := &AWXConfig{
		Host:        getEnvOrDefault("AWX_HOST", ""),
		User:        getEnvOrDefault("AWX_USER", ""),
		Password:    getEnvOrDefault("AWX_PASSWORD", ""),
		UseHTTP:     getEnvAsBool("HTTP", false),
		TLSInsecure: getEnvAsBool("TLS_INSECURE", false),
	}

	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	return config
}

// validateConfig ensures all required configuration is present and valid
func validateConfig(config *AWXConfig) error {
	if config.Host == "" {
		return fmt.Errorf("AWX_HOST environment variable is required")
	}
	if config.User == "" {
		return fmt.Errorf("AWX_USER environment variable is required")
	}
	if config.Password == "" {
		return fmt.Errorf("AWX_PASSWORD environment variable is required")
	}
	return nil
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsBool gets an environment variable as a boolean
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true"
	}
	return defaultValue
}

// getScrapeInterval gets the scrape interval from environment or returns default
func getScrapeInterval() time.Duration {
	if value := os.Getenv("SCRAPE_INTERVAL"); value != "" {
		if minutes, err := strconv.Atoi(value); err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute
		}
	}
	return defaultScrapeInterval
}

// getPort gets the server port from environment or returns default
func getPort() string {
	return getEnvOrDefault("PORT", defaultPort)
}
