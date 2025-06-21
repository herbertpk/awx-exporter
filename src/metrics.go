package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics with improved naming and descriptions
var (
	hostInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "awx",
			Name:      "host_info",
			Help:      "Information about AWX hosts including inventory and enabled status",
		},
		[]string{"id", "name", "inventory_id", "inventory_name", "enabled", "instance_id"},
	)

	hostStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "awx",
			Name:      "host_status",
			Help:      "Status metrics for AWX hosts including failures and inventory sources",
		},
		[]string{"id", "name", "metric", "group"},
	)

	hostTimestamps = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "awx",
			Name:      "host_timestamps",
			Help:      "Unix timestamps for AWX host events (created, modified, facts modified)",
		},
		[]string{"id", "name", "event"},
	)

	hostGroupMembership = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "awx",
			Name:      "host_group_membership",
			Help:      "Host to group membership relationships in AWX (1 = member, 0 = not member)",
		},
		[]string{"host_id", "host_name", "group_id", "group_name"},
	)

	groupInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "awx",
			Name:      "group_info",
			Help:      "Information about AWX groups and their inventory associations",
		},
		[]string{"group_id", "group_name", "inventory_id"},
	)

	// Additional metrics for monitoring the exporter itself
	scrapeDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "awx_exporter",
			Name:      "scrape_duration_seconds",
			Help:      "Duration of AWX API scrape operations",
			Buckets:   prometheus.DefBuckets,
		},
	)

	scrapeErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "awx_exporter",
			Name:      "scrape_errors_total",
			Help:      "Total number of errors during AWX API scraping",
		},
	)

	hostsProcessed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "awx_exporter",
			Name:      "hosts_processed_total",
			Help:      "Total number of hosts processed by the exporter",
		},
	)
)

// init registers all Prometheus metrics
func init() {
	// Register host-related metrics
	prometheus.MustRegister(hostInfo)
	prometheus.MustRegister(hostStatus)
	prometheus.MustRegister(hostTimestamps)
	prometheus.MustRegister(hostGroupMembership)
	prometheus.MustRegister(groupInfo)

	// Register exporter monitoring metrics
	prometheus.MustRegister(scrapeDuration)
	prometheus.MustRegister(scrapeErrors)
	prometheus.MustRegister(hostsProcessed)
}

// ResetAllMetrics resets all metrics before each update
func ResetAllMetrics() {
	hostInfo.Reset()
	hostStatus.Reset()
	hostTimestamps.Reset()
	hostGroupMembership.Reset()
	groupInfo.Reset()
}

// IncrementHostsProcessed increments the hosts processed counter
func IncrementHostsProcessed(count int) {
	hostsProcessed.Add(float64(count))
}

// RecordScrapeError increments the scrape errors counter
func RecordScrapeError() {
	scrapeErrors.Inc()
}
