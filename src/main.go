package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// --- Configuration ---
type Config struct {
	AWXURL             string
	AWXUsername        string
	AWXPassword        string
	ScrapeInterval     time.Duration
	ListenPort         string
	InsecureSkipVerify bool
	RequestTimeout     time.Duration
}

// loadConfig loads configuration from environment variables.
func loadConfig() (*Config, error) {
	username := os.Getenv("AWX_USERNAME")
	password := os.Getenv("AWX_PASSWORD")

	if username == "" || password == "" {
		return nil, fmt.Errorf("AWX_USERNAME and AWX_PASSWORD must be set")
	}

	scrapeIntervalStr := getEnv("SCRAPE_INTERVAL", "24h")
	scrapeInterval, err := time.ParseDuration(scrapeIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SCRAPE_INTERVAL duration: %w", err)
	}

	return &Config{
		AWXURL:             getEnv("AWX_URL", "https://awx"),
		AWXUsername:        username,
		AWXPassword:        password,
		ScrapeInterval:     scrapeInterval,
		ListenPort:         ":" + getEnv("EXPORTER_PORT", "8080"),
		InsecureSkipVerify: getEnv("INSECURE_SKIP_VERIFY", "false") == "true",
		RequestTimeout:     30 * time.Second,
	}, nil
}

// --- Prometheus Metrics Definition ---
const namespace = "awx"

var (
	jobLastRunTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "job_last_run_timestamp_seconds",
			Help:      "The finish time of the last job run for a template, as a Unix timestamp.",
		},
		[]string{"job_template_id", "job_template_name"},
	)
	inventoryFailedHosts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "template_inventory_failed_hosts",
			Help:      "Number of hosts with active failures in the inventory associated with a job template.",
		},
		[]string{"job_template_id", "job_template_name"},
	)
	inventoryTotalHosts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "template_inventory_total_hosts",
			Help:      "Number of active hosts in the inventory associated with a job template.",
		},
		[]string{"job_template_id", "job_template_name"},
	)
)

// --- Data Structures for AWX API JSON ---
type JobTemplateResponse struct {
	Next    string        `json:"next"`
	Results []JobTemplate `json:"results"`
}

type JobTemplate struct {
	ID            int           `json:"id"`
	Name          string        `json:"name"`
	LastJobRun    string        `json:"last_job_run"`
	SummaryFields SummaryFields `json:"summary_fields"`
}

type SummaryFields struct {
	Inventory struct {
		HostsWithFailures int `json:"hosts_with_active_failures"`
		TotalHosts        int `json:"total_hosts"`
	} `json:"inventory"`
}

// Global HTTP client and configuration
var (
	httpClient *http.Client
	appConfig  *Config
)

// --- Helper Functions ---
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// --- Exporter Logic ---
func scrapeAWX() {
	log.Println("Starting scrape of AWX job templates...")

	nextPagePath := "/api/v2/job_templates"
	// Reset metrics at the start of a scrape to remove stale entries
	jobLastRunTimestamp.Reset()
	inventoryFailedHosts.Reset()
	inventoryTotalHosts.Reset()

	// Use url.Parse to handle the base URL properly
	base, err := url.Parse(appConfig.AWXURL)
	if err != nil {
		log.Printf("Error: Invalid AWX_URL: %v", err)
		return
	}

	for nextPagePath != "" {
		// Resolve the next page path relative to the base URL
		reqURL := base.ResolveReference(&url.URL{Path: nextPagePath})

		req, err := http.NewRequest("GET", reqURL.String(), nil)
		if err != nil {
			log.Printf("Failed to create request for %s: %v", reqURL, err)
			break
		}
		req.SetBasicAuth(appConfig.AWXUsername, appConfig.AWXPassword)

		resp, err := httpClient.Do(req)
		if err != nil {
			log.Printf("Failed to execute request for %s: %v", reqURL, err)
			break
		}
		if resp.StatusCode != http.StatusOK {
			log.Printf("Received non-200 status code from %s: %d", reqURL, resp.StatusCode)
			resp.Body.Close()
			break
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read response body from %s: %v", reqURL, err)
			resp.Body.Close()
			break
		}
		resp.Body.Close()

		var response JobTemplateResponse
		if err := json.Unmarshal(body, &response); err != nil {
			log.Printf("Failed to unmarshal JSON from %s: %v", reqURL, err)
			break
		}

		for _, template := range response.Results {
			labels := prometheus.Labels{
				"job_template_id":   fmt.Sprintf("%d", template.ID),
				"job_template_name": template.Name,
			}

			if template.LastJobRun != "" {
				finishedTime, err := time.Parse(time.RFC3339, template.LastJobRun)
				if err == nil {
					jobLastRunTimestamp.With(labels).Set(float64(finishedTime.Unix()))
				} else {
					log.Printf("Could not parse timestamp '%s' for template '%s'", template.LastJobRun, template.Name)
				}
			}

			failedHosts := template.SummaryFields.Inventory.HostsWithFailures
			totalHosts := template.SummaryFields.Inventory.TotalHosts

			inventoryFailedHosts.With(labels).Set(float64(failedHosts))
			inventoryTotalHosts.With(labels).Set(float64(totalHosts))
		}

		nextPagePath = response.Next
	}

	log.Println("Scrape finished.")
}

// --- Main Execution ---
func main() {
	var err error
	appConfig, err = loadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize the global HTTP client
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: appConfig.InsecureSkipVerify},
		},
		Timeout: appConfig.RequestTimeout,
	}

	// Register metrics
	reg := prometheus.NewRegistry()
	reg.MustRegister(jobLastRunTimestamp)
	reg.MustRegister(inventoryFailedHosts)
	reg.MustRegister(inventoryTotalHosts)

	// Start the scraping loop in a background goroutine
	go func() {
		for {
			scrapeAWX()
			log.Printf("Sleeping for %v...", appConfig.ScrapeInterval)
			time.Sleep(appConfig.ScrapeInterval)
		}
	}()

	// Expose the registered metrics via HTTP
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	log.Printf("Prometheus metrics server started on http://localhost%s", appConfig.ListenPort)
	if err := http.ListenAndServe(appConfig.ListenPort, nil); err != nil {
		log.Fatalf("Error starting HTTP server: %v", err)
	}
}
