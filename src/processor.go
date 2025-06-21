package main

import (
	"log"
	"strconv"
	"time"
)

// processHosts processes a list of hosts and updates all metrics
func processHosts(hosts []Host) {

	hostCount := len(hosts)

	if hostCount == 0 {
		return
	}

	for i, host := range hosts {
		if err := processHost(host); err != nil {
			log.Printf("Error processing host %s (ID: %d): %v", host.Name, host.ID, err)
		}

		// Log progress for large batches
		if hostCount > 100 && (i+1)%50 == 0 {
			log.Printf("Processed %d/%d hosts", i+1, hostCount)
		}
	}

	// Update the hosts processed counter
	IncrementHostsProcessed(hostCount)
}


func processHost(host Host) error {
	
	idStr := strconv.Itoa(host.ID)
	invIDStr := strconv.Itoa(host.SummaryFields.Inventory.ID)
	enabledStr := boolToString(host.Enabled)

	hostInfo.WithLabelValues(
		idStr,
		host.Name,
		invIDStr,
		host.SummaryFields.Inventory.Name,
		enabledStr,
		host.InstanceID,
	).Set(1)


	if err := processHostStatus(host, idStr); err != nil {
		return err
	}

	if err := processHostTimestamps(host, idStr); err != nil {
		return err
	}

	if err := processHostGroups(host, idStr, invIDStr); err != nil {
		return err
	}

	return nil
}

// processHostStatus handles status metrics for a host
func processHostStatus(host Host, idStr string) error {
	activeFailures := boolToFloat(host.HasActiveFailures)
	inventorySources := boolToFloat(host.HasInventorySources)

	if len(host.SummaryFields.Groups.Results) == 0 {
		// Host has no groups, create metrics with "none" group
		hostStatus.WithLabelValues(idStr, host.Name, "active_failures", "none").Set(activeFailures)
		hostStatus.WithLabelValues(idStr, host.Name, "inventory_sources", "none").Set(inventorySources)
	} else {
		// Host has groups, create separate metrics for each group
		for _, group := range host.SummaryFields.Groups.Results {
			hostStatus.WithLabelValues(idStr, host.Name, "active_failures", group.Name).Set(activeFailures)
			hostStatus.WithLabelValues(idStr, host.Name, "inventory_sources", group.Name).Set(inventorySources)
		}
	}
	return nil
}


func processHostTimestamps(host Host, idStr string) error {
	if err := setTimestampMetric(host.Created, idStr, host.Name, "created"); err != nil {
		return err
	}
	if err := setTimestampMetric(host.Modified, idStr, host.Name, "modified"); err != nil {
		return err
	}
	if host.AnsibleFactsModified != nil {
		if err := setTimestampMetric(*host.AnsibleFactsModified, idStr, host.Name, "ansible_facts_modified"); err != nil {
			return err
		}
	}
	return nil
}


func processHostGroups(host Host, idStr, invIDStr string) error {
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

	return nil
}


func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}


func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func setTimestampMetric(timestampStr, id, name, event string) error {
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		log.Printf("Error parsing timestamp %s for host %s (ID: %s): %v", timestampStr, name, id, err)
		return err
	}
	hostTimestamps.WithLabelValues(id, name, event).Set(float64(timestamp.Unix()))
	return nil
}
