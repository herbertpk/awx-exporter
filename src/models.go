package main

// AWXConfig holds configuration for AWX connection
type AWXConfig struct {
	Host        string `json:"host"`
	User        string `json:"user"`
	Password    string `json:"-"` // Don't expose in JSON
	UseHTTP     bool   `json:"use_http"`
	TLSInsecure bool   `json:"tls_insecure"`
}

// HostResponse represents the paginated response from AWX API
type HostResponse struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []Host `json:"results"`
}

// Inventory represents inventory information
type Inventory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Group represents group information
type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Groups represents a collection of groups
type Groups struct {
	Count   int     `json:"count"`
	Results []Group `json:"results"`
}

// SummaryFields contains summary information for a host
type SummaryFields struct {
	Inventory Inventory `json:"inventory"`
	Groups    Groups    `json:"groups"`
}

// Host represents an AWX host
type Host struct {
	ID                   int           `json:"id"`
	Created              string        `json:"created"`
	Modified             string        `json:"modified"`
	Name                 string        `json:"name"`
	Inventory            int           `json:"inventory"`
	Enabled              bool          `json:"enabled"`
	InstanceID           string        `json:"instance_id"`
	HasActiveFailures    bool          `json:"has_active_failures"`
	HasInventorySources  bool          `json:"has_inventory_sources"`
	AnsibleFactsModified *string       `json:"ansible_facts_modified"`
	SummaryFields        SummaryFields `json:"summary_fields"`
}
