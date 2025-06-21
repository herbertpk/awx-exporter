package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTP client configuration
const (
	defaultTimeout = 30 * time.Second
	maxIdleConns   = 100
	idleTimeout    = 90 * time.Second
)

// buildURL constructs a full URL for AWX API requests
// It handles both relative paths and full URLs from pagination
func buildURL(config *AWXConfig, pathOrURL string) string {
	// If it's already a full URL, return it as is
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL
	}

	// If it's just "/" or empty, this indicates no more pages
	if pathOrURL == "/" || pathOrURL == "" {
		return ""
	}

	// If it's a relative URL starting with /, use it as is
	if strings.HasPrefix(pathOrURL, "/") {
		protocol := "https"
		if config.UseHTTP {
			protocol = "http"
		}
		return fmt.Sprintf("%s://%s%s", protocol, config.Host, pathOrURL)
	}

	// If it's a relative URL without /, add it
	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}

	protocol := "https"
	if config.UseHTTP {
		protocol = "http"
	}
	return fmt.Sprintf("%s://%s%s", protocol, config.Host, pathOrURL)
}

// makeAWXRequest performs an authenticated request to the AWX API with timeout
func makeAWXRequest(client *http.Client, config *AWXConfig, urlStr string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", urlStr, err)
	}

	req.SetBasicAuth(config.User, config.Password)
	req.Header.Set("User-Agent", "awx-exporter/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed for %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s, body: %s", resp.StatusCode, resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", urlStr, err)
	}

	// Check if response looks like HTML (common error case)
	if len(body) > 0 && strings.Contains(string(body), "<html") {
		return nil, fmt.Errorf("received HTML response instead of JSON from %s (likely an error page)", urlStr)
	}

	return body, nil
}

// createHTTPClient creates an HTTP client with optimized settings
func createHTTPClient(config *AWXConfig) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.TLSInsecure,
		},
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     idleTimeout,
		DisableCompression:  false,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   defaultTimeout,
	}
}
