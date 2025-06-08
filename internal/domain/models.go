package domain

import (
	"time"
)

// Node represents a node in the distributed network
type Node struct {
	ID           string    `json:"id" db:"id"`
	FQDN         string    `json:"fqdn" db:"fqdn"`
	IP           string    `json:"ip" db:"ip"`
	DiscoveredBy string    `json:"discovered_by" db:"discovered_by"`
	FirstSeen    time.Time `json:"first_seen" db:"first_seen"`
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
	IsActive     bool      `json:"is_active" db:"is_active"`
}

// PollResult represents the result of polling a node
type PollResult struct {
	ID         int64     `json:"id" db:"id"`
	NodeID     string    `json:"node_id" db:"node_id"`
	PollTime   time.Time `json:"poll_time" db:"poll_time"`
	Success    bool      `json:"success" db:"success"`
	ResponseMs int64     `json:"response_ms" db:"response_ms"`
	Error      string    `json:"error,omitempty" db:"error"`
	PathMTU    int       `json:"path_mtu,omitempty" db:"path_mtu"`
}

// NetworkSnapshot represents a snapshot of all known nodes
type NetworkSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"node_id"`
	Nodes     []Node    `json:"nodes"`
}

// SeedConfig represents the seed.json configuration
type SeedConfig struct {
	Nodes []SeedNode `json:"nodes"`
}

// SeedNode represents a node in the seed configuration
type SeedNode struct {
	FQDN string `json:"fqdn"`
	IP   string `json:"ip"`
}

// ReportingConfig represents the reportingserver.json configuration
type ReportingConfig struct {
	ServerFQDN string `json:"server_fqdn"`
	ServerIP   string `json:"server_ip"`
}

// NodeInfo represents the information this node exposes via JSON API
type NodeInfo struct {
	ID    string `json:"id"`
	FQDN  string `json:"fqdn"`
	IP    string `json:"ip"`
	Nodes []Node `json:"nodes"`
}

// Constants
const (
	PollInterval      = 30 * time.Second
	ReportInterval    = 5 * time.Minute
	MaxDatabaseSizeMB = 10
	DefaultPort       = 443
)
