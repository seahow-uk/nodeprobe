package app

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"nodeprobe/internal/domain"
)

type NodeService struct {
	nodeRepo   domain.NodeRepository
	configSvc  domain.ConfigService
	mu         sync.RWMutex
	knownNodes map[string]*domain.Node
}

func NewNodeService(nodeRepo domain.NodeRepository, configSvc domain.ConfigService) *NodeService {
	return &NodeService{
		nodeRepo:   nodeRepo,
		configSvc:  configSvc,
		knownNodes: make(map[string]*domain.Node),
	}
}

func (ns *NodeService) Initialize(ctx context.Context) error {
	// Load existing nodes from database
	nodes, err := ns.nodeRepo.GetAllNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to load existing nodes: %w", err)
	}

	ns.mu.Lock()
	for i := range nodes {
		ns.knownNodes[nodes[i].ID] = &nodes[i]
	}
	ns.mu.Unlock()

	// Load seed nodes and add them to the registry
	if err := ns.loadSeedNodes(ctx); err != nil {
		log.Printf("Warning: failed to load seed nodes: %v", err)
	}

	return nil
}

func (ns *NodeService) DiscoverNodes(ctx context.Context) error {
	// This method is called periodically to refresh node information
	// For now, it just updates the last seen timestamp for active nodes

	ns.mu.RLock()
	var activeNodes []domain.Node
	for _, node := range ns.knownNodes {
		if node.IsActive {
			activeNodes = append(activeNodes, *node)
		}
	}
	ns.mu.RUnlock()

	// Update last seen for active nodes in database
	now := time.Now()
	for _, node := range activeNodes {
		node.LastSeen = now
		if err := ns.nodeRepo.UpdateNode(ctx, &node); err != nil {
			log.Printf("Failed to update node %s last seen: %v", node.ID, err)
		}
	}

	return nil
}

func (ns *NodeService) MergeNodeInfo(ctx context.Context, nodeInfo *domain.NodeInfo, discoveredBy string) error {
	if nodeInfo == nil {
		return fmt.Errorf("nodeInfo cannot be nil")
	}

	myNodeID, err := ns.configSvc.GetNodeID()
	if err != nil {
		return fmt.Errorf("failed to get own node ID: %w", err)
	}

	now := time.Now()

	// Add the source node itself if it's not already known
	if nodeInfo.ID != myNodeID {
		if err := ns.addOrUpdateNode(ctx, &domain.Node{
			ID:           nodeInfo.ID,
			FQDN:         nodeInfo.FQDN,
			IP:           nodeInfo.IP,
			DiscoveredBy: discoveredBy,
			FirstSeen:    now,
			LastSeen:     now,
			IsActive:     true,
		}); err != nil {
			log.Printf("Failed to add/update source node %s: %v", nodeInfo.ID, err)
		}
	}

	// Process all nodes in the nodeInfo
	for _, node := range nodeInfo.Nodes {
		// Skip our own node
		if node.ID == myNodeID {
			continue
		}

		// Check if we already know about this node
		ns.mu.RLock()
		existingNode, exists := ns.knownNodes[node.ID]
		ns.mu.RUnlock()

		if !exists {
			// This is a new node, add it
			newNode := &domain.Node{
				ID:           node.ID,
				FQDN:         node.FQDN,
				IP:           node.IP,
				DiscoveredBy: nodeInfo.ID, // The node that told us about this
				FirstSeen:    now,
				LastSeen:     now,
				IsActive:     true,
			}

			if err := ns.addOrUpdateNode(ctx, newNode); err != nil {
				log.Printf("Failed to add new node %s: %v", node.ID, err)
				continue
			}

			log.Printf("Discovered new node %s (%s) via %s", node.ID, node.FQDN, nodeInfo.ID)
		} else {
			// Update existing node information if needed
			updated := false

			if existingNode.FQDN != node.FQDN {
				existingNode.FQDN = node.FQDN
				updated = true
			}

			if existingNode.IP != node.IP {
				existingNode.IP = node.IP
				updated = true
			}

			// Always update last seen
			existingNode.LastSeen = now
			updated = true

			if updated {
				if err := ns.addOrUpdateNode(ctx, existingNode); err != nil {
					log.Printf("Failed to update existing node %s: %v", node.ID, err)
				}
			}
		}
	}

	return nil
}

func (ns *NodeService) GetKnownNodes(ctx context.Context) ([]domain.Node, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	nodes := make([]domain.Node, 0, len(ns.knownNodes))
	for _, node := range ns.knownNodes {
		nodes = append(nodes, *node)
	}

	return nodes, nil
}

func (ns *NodeService) UpdateNodeStatus(ctx context.Context, nodeID string, isActive bool) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	node, exists := ns.knownNodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.IsActive = isActive

	if err := ns.nodeRepo.UpdateNode(ctx, node); err != nil {
		return fmt.Errorf("failed to update node status in database: %w", err)
	}

	return nil
}

func (ns *NodeService) addOrUpdateNode(ctx context.Context, node *domain.Node) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Check if node exists in database
	existingNode, err := ns.nodeRepo.GetNode(ctx, node.ID)
	if err != nil {
		return fmt.Errorf("failed to check existing node: %w", err)
	}

	if existingNode == nil {
		// Create new node
		if err := ns.nodeRepo.CreateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to create node: %w", err)
		}
	} else {
		// Update existing node but preserve first seen time
		node.FirstSeen = existingNode.FirstSeen
		if err := ns.nodeRepo.UpdateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to update node: %w", err)
		}
	}

	// Update in-memory cache
	ns.knownNodes[node.ID] = node

	return nil
}

func (ns *NodeService) loadSeedNodes(ctx context.Context) error {
	seedConfig, err := ns.configSvc.LoadSeedConfig()
	if err != nil {
		return fmt.Errorf("failed to load seed config: %w", err)
	}

	if seedConfig == nil || len(seedConfig.Nodes) == 0 {
		return nil // No seed nodes configured
	}

	myNodeID, err := ns.configSvc.GetNodeID()
	if err != nil {
		return fmt.Errorf("failed to get own node ID: %w", err)
	}

	now := time.Now()

	for _, seedNode := range seedConfig.Nodes {
		// Generate a deterministic ID for seed nodes based on their FQDN/IP
		// This ensures seed nodes get consistent IDs across restarts
		nodeID := fmt.Sprintf("seed-%s-%s", seedNode.FQDN, seedNode.IP)

		// Skip if this is somehow our own node
		if nodeID == myNodeID {
			continue
		}

		ns.mu.RLock()
		_, exists := ns.knownNodes[nodeID]
		ns.mu.RUnlock()

		if !exists {
			newNode := &domain.Node{
				ID:           nodeID,
				FQDN:         seedNode.FQDN,
				IP:           seedNode.IP,
				DiscoveredBy: "seed",
				FirstSeen:    now,
				LastSeen:     now,
				IsActive:     true,
			}

			if err := ns.addOrUpdateNode(ctx, newNode); err != nil {
				log.Printf("Failed to add seed node %s: %v", nodeID, err)
				continue
			}

			log.Printf("Added seed node %s (%s)", nodeID, seedNode.FQDN)
		}
	}

	return nil
}

// GetActiveNodes returns only the active nodes
func (ns *NodeService) GetActiveNodes(ctx context.Context) ([]domain.Node, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	var activeNodes []domain.Node
	for _, node := range ns.knownNodes {
		if node.IsActive {
			activeNodes = append(activeNodes, *node)
		}
	}

	return activeNodes, nil
}

// GetNodeByID returns a specific node by its ID
func (ns *NodeService) GetNodeByID(ctx context.Context, nodeID string) (*domain.Node, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	node, exists := ns.knownNodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	// Return a copy to prevent external modifications
	nodeCopy := *node
	return &nodeCopy, nil
}
