package app

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"nodeprobe/internal/domain"
)

type PollingService struct {
	nodeService domain.NodeService
	pollRepo    domain.PollRepository
	httpClient  domain.HTTPClient
	configSvc   domain.ConfigService
	running     bool
	stopChan    chan struct{}
	mu          sync.RWMutex
	nodeIndex   int
	firstPolls  map[string]bool // Track first polls for path MTU testing
}

func NewPollingService(
	nodeService domain.NodeService,
	pollRepo domain.PollRepository,
	httpClient domain.HTTPClient,
	configSvc domain.ConfigService,
) *PollingService {
	return &PollingService{
		nodeService: nodeService,
		pollRepo:    pollRepo,
		httpClient:  httpClient,
		configSvc:   configSvc,
		stopChan:    make(chan struct{}),
		firstPolls:  make(map[string]bool),
	}
}

func (ps *PollingService) Start(ctx context.Context) error {
	ps.mu.Lock()
	if ps.running {
		ps.mu.Unlock()
		return fmt.Errorf("polling service is already running")
	}
	ps.running = true
	ps.mu.Unlock()

	log.Println("Starting polling service...")

	// Start the polling loop in a separate goroutine
	go ps.pollingLoop(ctx)

	return nil
}

func (ps *PollingService) Stop() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.running {
		return fmt.Errorf("polling service is not running")
	}

	log.Println("Stopping polling service...")
	ps.running = false
	close(ps.stopChan)

	return nil
}

func (ps *PollingService) pollingLoop(ctx context.Context) {
	ticker := time.NewTicker(domain.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Polling service stopped due to context cancellation")
			return
		case <-ps.stopChan:
			log.Println("Polling service stopped")
			return
		case <-ticker.C:
			if err := ps.pollNextNode(ctx); err != nil {
				log.Printf("Error during polling: %v", err)
			}
		}
	}
}

func (ps *PollingService) pollNextNode(ctx context.Context) error {
	// Get active nodes
	nodes, err := ps.nodeService.GetActiveNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil // No nodes to poll
	}

	// Get our own node ID to avoid polling ourselves
	myNodeID, err := ps.configSvc.GetNodeID()
	if err != nil {
		return fmt.Errorf("failed to get own node ID: %w", err)
	}

	// Filter out our own node
	var filteredNodes []domain.Node
	for _, node := range nodes {
		if node.ID != myNodeID {
			filteredNodes = append(filteredNodes, node)
		}
	}

	if len(filteredNodes) == 0 {
		return nil // No other nodes to poll
	}

	// Rotate through nodes
	ps.mu.Lock()
	if ps.nodeIndex >= len(filteredNodes) {
		ps.nodeIndex = 0
	}
	nodeToPolI := ps.nodeIndex
	ps.nodeIndex++
	ps.mu.Unlock()

	nodeToPoll := &filteredNodes[nodeToPolI]

	// Poll the selected node
	result, err := ps.PollNode(ctx, nodeToPoll)
	if err != nil {
		log.Printf("Failed to poll node %s (%s): %v", nodeToPoll.ID, nodeToPoll.FQDN, err)
		return nil // Don't return error to keep polling loop running
	}

	// Store the poll result
	if err := ps.pollRepo.CreatePollResult(ctx, result); err != nil {
		log.Printf("Failed to store poll result for node %s: %v", nodeToPoll.ID, err)
	}

	// Update node status based on poll result
	if err := ps.nodeService.UpdateNodeStatus(ctx, nodeToPoll.ID, result.Success); err != nil {
		log.Printf("Failed to update node status for %s: %v", nodeToPoll.ID, err)
	}

	return nil
}

func (ps *PollingService) PollNode(ctx context.Context, node *domain.Node) (*domain.PollResult, error) {
	startTime := time.Now()

	result := &domain.PollResult{
		NodeID:     node.ID,
		PollTime:   startTime,
		Success:    false,
		ResponseMs: 0,
	}

	// Construct the node URL
	nodeURL := fmt.Sprintf("https://%s:443", node.FQDN)
	if node.FQDN == "" || node.FQDN == "unknown" {
		nodeURL = fmt.Sprintf("https://%s:443", node.IP)
	}

	// Create a timeout context for this poll
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Check if this is the first poll for path MTU testing
	ps.mu.Lock()
	isFirstPoll := !ps.firstPolls[node.ID]
	if isFirstPoll {
		ps.firstPolls[node.ID] = true
	}
	ps.mu.Unlock()

	// Perform path MTU test on first poll
	if isFirstPoll {
		if mtu, err := ps.httpClient.TestPathMTU(pollCtx, nodeURL); err == nil {
			result.PathMTU = mtu
			log.Printf("Path MTU to node %s (%s): %d", node.ID, node.FQDN, mtu)
		} else {
			log.Printf("Failed to test path MTU to node %s: %v", node.ID, err)
		}
	}

	// Get node information from the target node
	nodeInfo, err := ps.httpClient.GetNodeInfo(pollCtx, nodeURL)
	endTime := time.Now()

	// Calculate response time
	responseMs := endTime.Sub(startTime).Milliseconds()
	result.ResponseMs = responseMs

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		log.Printf("Poll failed for node %s (%s): %v (response time: %dms)",
			node.ID, node.FQDN, err, responseMs)
		return result, nil
	}

	result.Success = true
	log.Printf("Poll successful for node %s (%s): %dms",
		node.ID, node.FQDN, responseMs)

	// Merge the discovered node information
	if err := ps.nodeService.MergeNodeInfo(ctx, nodeInfo, node.ID); err != nil {
		log.Printf("Failed to merge node info from %s: %v", node.ID, err)
	}

	return result, nil
}

// GetPollHistory returns recent poll results for a specific node
func (ps *PollingService) GetPollHistory(ctx context.Context, nodeID string, limit int) ([]domain.PollResult, error) {
	return ps.pollRepo.GetPollResults(ctx, nodeID, limit)
}

// GetRecentPollResults returns all poll results since a given time
func (ps *PollingService) GetRecentPollResults(ctx context.Context, since time.Time) ([]domain.PollResult, error) {
	return ps.pollRepo.GetRecentPollResults(ctx, since)
}

// CleanupOldResults removes old poll results to keep database size under control
func (ps *PollingService) CleanupOldResults(ctx context.Context) error {
	return ps.pollRepo.CleanupOldResults(ctx, domain.MaxDatabaseSizeMB)
}

// GetDatabaseSize returns the current database size in bytes
func (ps *PollingService) GetDatabaseSize(ctx context.Context) (int64, error) {
	return ps.pollRepo.GetDatabaseSize(ctx)
}

// IsRunning returns whether the polling service is currently running
func (ps *PollingService) IsRunning() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.running
}
