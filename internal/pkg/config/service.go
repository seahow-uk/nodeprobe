package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"nodeprobe/internal/domain"

	"github.com/google/uuid"
)

type Service struct {
	configDir string
	nodeID    string
	nodeInfo  *domain.NodeInfo
}

func NewService(configDir string) (*Service, error) {
	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	service := &Service{
		configDir: configDir,
	}

	// Load or generate node ID
	nodeID, err := service.loadOrGenerateNodeID()
	if err != nil {
		return nil, fmt.Errorf("failed to load/generate node ID: %w", err)
	}
	service.nodeID = nodeID

	// Initialize node info
	nodeInfo, err := service.initializeNodeInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize node info: %w", err)
	}
	service.nodeInfo = nodeInfo

	return service, nil
}

func (s *Service) LoadSeedConfig() (*domain.SeedConfig, error) {
	seedPath := filepath.Join(s.configDir, "seed.json")

	// Check if seed.json exists
	if _, err := os.Stat(seedPath); os.IsNotExist(err) {
		// Create empty seed config if it doesn't exist
		emptySeed := &domain.SeedConfig{Nodes: []domain.SeedNode{}}
		return emptySeed, nil
	}

	data, err := os.ReadFile(seedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read seed config: %w", err)
	}

	var config domain.SeedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal seed config: %w", err)
	}

	return &config, nil
}

func (s *Service) LoadReportingConfig() (*domain.ReportingConfig, error) {
	reportingPath := filepath.Join(s.configDir, "reportingserver.json")

	// Check if reportingserver.json exists
	if _, err := os.Stat(reportingPath); os.IsNotExist(err) {
		// Return nil if no reporting server is configured
		return nil, nil
	}

	data, err := os.ReadFile(reportingPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read reporting config: %w", err)
	}

	var config domain.ReportingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reporting config: %w", err)
	}

	return &config, nil
}

func (s *Service) GetNodeID() (string, error) {
	return s.nodeID, nil
}

func (s *Service) GetNodeInfo() (*domain.NodeInfo, error) {
	return s.nodeInfo, nil
}

func (s *Service) SaveNodeID(id string) error {
	nodeIDPath := filepath.Join(s.configDir, "node.id")

	if err := os.WriteFile(nodeIDPath, []byte(id), 0644); err != nil {
		return fmt.Errorf("failed to save node ID: %w", err)
	}

	s.nodeID = id
	return nil
}

func (s *Service) loadOrGenerateNodeID() (string, error) {
	nodeIDPath := filepath.Join(s.configDir, "node.id")

	// Try to load existing node ID
	if data, err := os.ReadFile(nodeIDPath); err == nil {
		nodeID := strings.TrimSpace(string(data))
		if nodeID != "" {
			return nodeID, nil
		}
	}

	// Generate new 32-bit UUID (actually using full UUID for better uniqueness)
	nodeID := uuid.New().String()

	// Save the generated ID
	if err := s.SaveNodeID(nodeID); err != nil {
		return "", err
	}

	return nodeID, nil
}

func (s *Service) initializeNodeInfo() (*domain.NodeInfo, error) {
	// Get local network information
	fqdn, ip, err := s.getLocalNetworkInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get local network info: %w", err)
	}

	nodeInfo := &domain.NodeInfo{
		ID:    s.nodeID,
		FQDN:  fqdn,
		IP:    ip,
		Nodes: []domain.Node{}, // Will be populated by the node service
	}

	return nodeInfo, nil
}

func (s *Service) getLocalNetworkInfo() (string, string, error) {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Get local IP address by connecting to a remote address
	// This doesn't actually send any packets
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return hostname, "127.0.0.1", nil
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	localIP := localAddr.IP.String()

	return hostname, localIP, nil
}

// UpdateNodeInfo updates the nodes list in the node info
func (s *Service) UpdateNodeInfo(nodes []domain.Node) {
	s.nodeInfo.Nodes = nodes
}

// CreateSampleSeedConfig creates a sample seed.json file
func (s *Service) CreateSampleSeedConfig() error {
	sampleConfig := &domain.SeedConfig{
		Nodes: []domain.SeedNode{
			{
				FQDN: "node1.example.com",
				IP:   "192.168.1.100",
			},
			{
				FQDN: "node2.example.com",
				IP:   "192.168.1.101",
			},
		},
	}

	data, err := json.MarshalIndent(sampleConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sample seed config: %w", err)
	}

	seedPath := filepath.Join(s.configDir, "seed.json.example")
	if err := os.WriteFile(seedPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sample seed config: %w", err)
	}

	return nil
}

// CreateSampleReportingConfig creates a sample reportingserver.json file
func (s *Service) CreateSampleReportingConfig() error {
	sampleConfig := &domain.ReportingConfig{
		ServerFQDN: "reporting.example.com",
		ServerIP:   "192.168.1.10",
	}

	data, err := json.MarshalIndent(sampleConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sample reporting config: %w", err)
	}

	reportingPath := filepath.Join(s.configDir, "reportingserver.json.example")
	if err := os.WriteFile(reportingPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sample reporting config: %w", err)
	}

	return nil
}
