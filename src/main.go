package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var scrapeInterval int = 5 // Default scrape interval in minutes

// AWX configuration from environment variables
type AWXConfig struct {
	Host        string
	User        string
	Password    string
	UseHTTP     bool
	TLSInsecure bool
}

// JSON structures matching the AWX API response
type HostResponse struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []Host `json:"results"`
}

type Host struct {
	ID                   int     `json:"id"`
	Created              string  `json:"created"`
	Modified             string  `json:"modified"`
	Name                 string  `json:"name"`
	Inventory            int     `json:"inventory"`
	Enabled              bool    `json:"enabled"`
	InstanceID           string  `json:"instance_id"`
	HasActiveFailures    bool    `json:"has_active_failures"`
	HasInventorySources  bool    `json:"has_inventory_sources"`
	AnsibleFactsModified *string `json:"ansible_facts_modified"`
	SummaryFields        struct {
		Inventory struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"inventory"`
		Groups struct {
			Count   int `json:"count"`
			Results []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"results"`
		} `json:"groups"`
	} `json:"summary_fields"`
}

// Prometheus metrics
var (
	hostInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "awx_host_info",
			Help: "Information about AWX hosts",
		},
		[]string{"id", "name", "inventory_id", "inventory_name", "enabled", "instance_id"},
	)

	hostStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "awx_host_status",
			Help: "Status metrics for AWX hosts",
		},
		[]string{"id", "name", "metric"},
	)

	hostTimestamps = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "awx_host_timestamps",
			Help: "Timestamps for AWX host events",
		},
		[]string{"id", "name", "event"},
	)

	hostGroupMembership = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "awx_host_group_membership",
			Help: "Host to group membership in AWX",
		},
		[]string{"host_id", "host_name", "group_id", "group_name"},
	)

	groupInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "awx_group_info",
			Help: "Information about AWX groups",
		},
		[]string{"group_id", "group_name", "inventory_id"},
	)
)

func init() {
	prometheus.MustRegister(hostInfo)
	prometheus.MustRegister(hostStatus)
	prometheus.MustRegister(hostTimestamps)
	prometheus.MustRegister(hostGroupMembership)
	prometheus.MustRegister(groupInfo)
}

func main() {
	config := loadConfig()
	http.Handle("/metrics", promhttp.Handler())
	go updateMetrics(config)
	log.Printf("Starting AWX exporter on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadConfig() *AWXConfig {
	config := &AWXConfig{
		Host:        os.Getenv("AWX_HOST"),
		User:        os.Getenv("AWX_USER"),
		Password:    os.Getenv("AWX_PASSWORD"),
		UseHTTP:     os.Getenv("HTTP") == "true",
		TLSInsecure: os.Getenv("TLS_INSECURE") == "true",
	}

	if config.Host == "" || config.User == "" || config.Password == "" {
		log.Fatal("AWX_HOST, AWX_USER, and AWX_PASSWORD must be set")
	}

	return config
}

func updateMetrics(config *AWXConfig) {
	for {
		if err := fetchAndUpdateMetrics(config); err != nil {
			log.Printf("Error updating metrics: %v", err)
		}
		time.Sleep(time.Duration(scrapeInterval) * time.Minute)
	}
}

func fetchAndUpdateMetrics(config *AWXConfig) error {
	// Reset metrics before each update
	hostInfo.Reset()
	hostStatus.Reset()
	hostTimestamps.Reset()
	hostGroupMembership.Reset()
	groupInfo.Reset()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: config.TLSInsecure},
		},
	}

	nextURL := buildURL(config, "/api/v2/hosts/?format=json")
	for nextURL != "" {
		resp, err := makeAWXRequest(client, config, nextURL)
		if err != nil {
			return err
		}
		fmt.Printf("Fetched data from %s\n", nextURL)
		var hostResponse HostResponse
		if err := json.Unmarshal(resp, &hostResponse); err != nil {
			return fmt.Errorf("error parsing JSON: %w", err)
		}
		processHosts(hostResponse.Results)
		nextURL = buildURL(config, hostResponse.Next)
	}

	return nil
}

func buildURL(config *AWXConfig, path string) string {
	protocol := "https"
	if config.UseHTTP {
		protocol = "http"
	}
	return fmt.Sprintf("%s://%s%s", protocol, config.Host, path)
}

func makeAWXRequest(client *http.Client, config *AWXConfig, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(config.User, config.Password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func processHosts(hosts []Host) {
	for _, host := range hosts {
		// Convert IDs to strings for labels
		idStr := strconv.Itoa(host.ID)
		invIDStr := strconv.Itoa(host.SummaryFields.Inventory.ID)
		enabled := "false"
		if host.Enabled {
			enabled = "true"
		}

		// Host info metric
		hostInfo.WithLabelValues(
			idStr,
			host.Name,
			invIDStr,
			host.SummaryFields.Inventory.Name,
			enabled,
			host.InstanceID,
		).Set(1)

		// Status metrics
		hostStatus.WithLabelValues(idStr, host.Name, "active_failures").Set(boolToFloat(host.HasActiveFailures))
		hostStatus.WithLabelValues(idStr, host.Name, "inventory_sources").Set(boolToFloat(host.HasInventorySources))

		// Timestamps
		setTimestampMetric(host.Created, idStr, host.Name, "created")
		setTimestampMetric(host.Modified, idStr, host.Name, "modified")
		if host.AnsibleFactsModified != nil {
			setTimestampMetric(*host.AnsibleFactsModified, idStr, host.Name, "ansible_facts_modified")
		}

		// Group information
		for _, group := range host.SummaryFields.Groups.Results {
			groupIDStr := strconv.Itoa(group.ID)

			// Record group membership
			hostGroupMembership.WithLabelValues(
				idStr,
				host.Name,
				groupIDStr,
				group.Name,
			).Set(1)

			// Record group info (will deduplicate automatically)
			groupInfo.WithLabelValues(
				groupIDStr,
				group.Name,
				invIDStr,
			).Set(1)
		}
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func setTimestampMetric(timestampStr, id, name, event string) {
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		log.Printf("Error parsing timestamp %s for host %s: %v", timestampStr, name, err)
		return
	}
	hostTimestamps.WithLabelValues(id, name, event).Set(float64(timestamp.Unix()))
}
