package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Setup structured logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	config := loadConfig()
	scrapeInterval := getScrapeInterval()
	port := getPort()

	log.Printf("Starting AWX exporter - Host: %s, Port: %s, Interval: %v", config.Host, port, scrapeInterval)

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start metrics collection in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go updateMetrics(ctx, config, scrapeInterval)

	// Setup graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during server shutdown: %v", err)
		}
	}()

	log.Printf("Server started on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// healthHandler provides a simple health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s","service":"awx-exporter"}`, time.Now().Format(time.RFC3339))
}

// updateMetrics runs the metrics collection loop
func updateMetrics(ctx context.Context, config *AWXConfig, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial collection immediately
	if err := fetchAndUpdateMetrics(config); err != nil {
		log.Printf("Initial collection failed: %v", err)
		RecordScrapeError()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fetchAndUpdateMetrics(config); err != nil {
				log.Printf("Collection failed: %v", err)
				RecordScrapeError()
			}
		}
	}
}

// fetchAndUpdateMetrics fetches data from AWX API and updates metrics
func fetchAndUpdateMetrics(config *AWXConfig) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		scrapeDuration.Observe(duration.Seconds())
		log.Printf("Metrics collection completed in %v", duration)
	}()

	// Reset metrics before each update
	ResetAllMetrics()

	client := createHTTPClient(config)

	nextURL := buildURL(config, "/api/v2/hosts/?format=json")
	pageCount := 0
	totalHosts := 0

	for nextURL != "" {
		resp, err := makeAWXRequest(client, config, nextURL)
		if err != nil {
			RecordScrapeError()
			return fmt.Errorf("failed to fetch data from %s: %w", nextURL, err)
		}

		pageCount++
		var hostResponse HostResponse
		if err := json.Unmarshal(resp, &hostResponse); err != nil {
			RecordScrapeError()
			// Log the first 200 characters of the response for debugging
			responsePreview := string(resp)
			if len(responsePreview) > 200 {
				responsePreview = responsePreview[:200] + "..."
			}
			log.Printf("Response preview: %s", responsePreview)
			return fmt.Errorf("error parsing JSON from %s: %w", nextURL, err)
		}

		hostCount := len(hostResponse.Results)
		totalHosts += hostCount
		log.Printf("Page %d: %d hosts", pageCount, hostCount)

		processHosts(hostResponse.Results)
		nextURL = buildURL(config, hostResponse.Next)
	}

	log.Printf("Completed: %d pages, %d total hosts", pageCount, totalHosts)
	return nil
}
