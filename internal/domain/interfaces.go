package domain

import (
	"context"
	"time"
)

// NodeRepository defines the interface for node data storage operations
type NodeRepository interface {
	GetAllNodes(ctx context.Context) ([]Node, error)
	GetNode(ctx context.Context, id string) (*Node, error)
	CreateNode(ctx context.Context, node *Node) error
	UpdateNode(ctx context.Context, node *Node) error
	DeleteNode(ctx context.Context, id string) error
	GetActiveNodes(ctx context.Context) ([]Node, error)
}

// PollRepository defines the interface for poll result storage operations
type PollRepository interface {
	CreatePollResult(ctx context.Context, result *PollResult) error
	GetPollResults(ctx context.Context, nodeID string, limit int) ([]PollResult, error)
	GetRecentPollResults(ctx context.Context, since time.Time) ([]PollResult, error)
	CleanupOldResults(ctx context.Context, maxSizeMB int) error
	GetDatabaseSize(ctx context.Context) (int64, error)
}

// HTTPClient defines the interface for making HTTP requests to other nodes
type HTTPClient interface {
	GetNodeInfo(ctx context.Context, nodeURL string) (*NodeInfo, error)
	SendNetworkSnapshot(ctx context.Context, reportingURL string, snapshot *NetworkSnapshot) error
	TestPathMTU(ctx context.Context, nodeURL string) (int, error)
}

// ConfigService defines the interface for configuration management
type ConfigService interface {
	LoadSeedConfig() (*SeedConfig, error)
	LoadReportingConfig() (*ReportingConfig, error)
	GetNodeID() (string, error)
	GetNodeInfo() (*NodeInfo, error)
	SaveNodeID(id string) error
}

// TLSService defines the interface for TLS certificate management
type TLSService interface {
	GenerateSelfSignedCert() error
	GetCertPath() (string, string, error) // returns cert path, key path, error
}

// PollingService defines the interface for the polling service
type PollingService interface {
	Start(ctx context.Context) error
	Stop() error
	PollNode(ctx context.Context, node *Node) (*PollResult, error)
}

// ReportingService defines the interface for the reporting service
type ReportingService interface {
	Start(ctx context.Context) error
	Stop() error
	SendReport(ctx context.Context) error
	GenerateHTMLReport() (string, error)
}

// WebServer defines the interface for the web server
type WebServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// NodeService defines the interface for node management operations
type NodeService interface {
	DiscoverNodes(ctx context.Context) error
	MergeNodeInfo(ctx context.Context, nodeInfo *NodeInfo, discoveredBy string) error
	GetKnownNodes(ctx context.Context) ([]Node, error)
	GetActiveNodes(ctx context.Context) ([]Node, error)
	UpdateNodeStatus(ctx context.Context, nodeID string, isActive bool) error
}
