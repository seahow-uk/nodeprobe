package app

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"strings"
	"sync"
	"time"

	"nodeprobe/internal/domain"
)

type ReportingService struct {
	nodeService domain.NodeService
	httpClient  domain.HTTPClient
	configSvc   domain.ConfigService
	pollRepo    domain.PollRepository
	running     bool
	stopChan    chan struct{}
	mu          sync.RWMutex
}

func NewReportingService(
	nodeService domain.NodeService,
	httpClient domain.HTTPClient,
	configSvc domain.ConfigService,
	pollRepo domain.PollRepository,
) *ReportingService {
	return &ReportingService{
		nodeService: nodeService,
		httpClient:  httpClient,
		configSvc:   configSvc,
		pollRepo:    pollRepo,
		stopChan:    make(chan struct{}),
	}
}

func (rs *ReportingService) Start(ctx context.Context) error {
	rs.mu.Lock()
	if rs.running {
		rs.mu.Unlock()
		return fmt.Errorf("reporting service is already running")
	}
	rs.running = true
	rs.mu.Unlock()

	log.Println("Starting reporting service...")

	// Start the reporting loop in a separate goroutine
	go rs.reportingLoop(ctx)

	return nil
}

func (rs *ReportingService) Stop() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.running {
		return fmt.Errorf("reporting service is not running")
	}

	log.Println("Stopping reporting service...")
	rs.running = false
	close(rs.stopChan)

	return nil
}

func (rs *ReportingService) reportingLoop(ctx context.Context) {
	ticker := time.NewTicker(domain.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Reporting service stopped due to context cancellation")
			return
		case <-rs.stopChan:
			log.Println("Reporting service stopped")
			return
		case <-ticker.C:
			if err := rs.SendReport(ctx); err != nil {
				log.Printf("Error sending report: %v", err)
			}
		}
	}
}

func (rs *ReportingService) SendReport(ctx context.Context) error {
	// Check if reporting server is configured
	reportingConfig, err := rs.configSvc.LoadReportingConfig()
	if err != nil {
		return fmt.Errorf("failed to load reporting config: %w", err)
	}

	if reportingConfig == nil {
		// No reporting server configured, skip
		return nil
	}

	// Get current node information
	nodeInfo, err := rs.configSvc.GetNodeInfo()
	if err != nil {
		return fmt.Errorf("failed to get node info: %w", err)
	}

	// Get all known nodes
	nodes, err := rs.nodeService.GetKnownNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get known nodes: %w", err)
	}

	// Create network snapshot
	snapshot := &domain.NetworkSnapshot{
		Timestamp: time.Now(),
		NodeID:    nodeInfo.ID,
		Nodes:     nodes,
	}

	// Send snapshot to reporting server
	reportingURL := fmt.Sprintf("https://%s:443", reportingConfig.ServerFQDN)
	if reportingConfig.ServerFQDN == "" || reportingConfig.ServerFQDN == "unknown" {
		reportingURL = fmt.Sprintf("https://%s:443", reportingConfig.ServerIP)
	}

	reportCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := rs.httpClient.SendNetworkSnapshot(reportCtx, reportingURL, snapshot); err != nil {
		return fmt.Errorf("failed to send network snapshot: %w", err)
	}

	log.Printf("Successfully sent network snapshot to %s", reportingURL)
	return nil
}

func (rs *ReportingService) GenerateHTMLReport() (string, error) {
	// Get all known nodes
	ctx := context.Background()
	nodes, err := rs.nodeService.GetKnownNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get known nodes: %w", err)
	}

	// Get node info
	nodeInfo, err := rs.configSvc.GetNodeInfo()
	if err != nil {
		return "", fmt.Errorf("failed to get node info: %w", err)
	}

	// Get recent poll results
	since := time.Now().Add(-24 * time.Hour) // Last 24 hours
	pollResults, err := rs.pollRepo.GetRecentPollResults(ctx, since)
	if err != nil {
		log.Printf("Warning: failed to get recent poll results: %v", err)
		pollResults = []domain.PollResult{}
	}

	// Create report data structure
	reportData := struct {
		GeneratedAt   string
		ReportingNode domain.NodeInfo
		Nodes         []domain.Node
		PollResults   []domain.PollResult
		TotalNodes    int
		ActiveNodes   int
		InactiveNodes int
		SuccessRate   float64
	}{
		GeneratedAt:   time.Now().Format("2006-01-02 15:04:05 UTC"),
		ReportingNode: *nodeInfo,
		Nodes:         nodes,
		PollResults:   pollResults,
		TotalNodes:    len(nodes),
	}

	// Calculate statistics
	activeCount := 0
	for _, node := range nodes {
		if node.IsActive {
			activeCount++
		}
	}
	reportData.ActiveNodes = activeCount
	reportData.InactiveNodes = len(nodes) - activeCount

	// Calculate success rate from recent polls
	if len(pollResults) > 0 {
		successCount := 0
		for _, result := range pollResults {
			if result.Success {
				successCount++
			}
		}
		reportData.SuccessRate = float64(successCount) / float64(len(pollResults)) * 100
	}

	// Generate HTML report
	html, err := rs.generateHTMLFromTemplate(reportData)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML from template: %w", err)
	}

	return html, nil
}

func (rs *ReportingService) generateHTMLFromTemplate(data interface{}) (string, error) {
	const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NodeProbe Network Report</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            padding: 30px;
        }
        h1 {
            color: #333;
            border-bottom: 3px solid #007acc;
            padding-bottom: 10px;
        }
        h2 {
            color: #555;
            margin-top: 30px;
        }
        .stats {
            display: flex;
            gap: 20px;
            margin: 20px 0;
            flex-wrap: wrap;
        }
        .stat-card {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 8px;
            text-align: center;
            min-width: 150px;
            flex: 1;
        }
        .stat-value {
            font-size: 2em;
            font-weight: bold;
            display: block;
        }
        .stat-label {
            font-size: 0.9em;
            opacity: 0.9;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background-color: #f8f9fa;
            font-weight: 600;
            color: #333;
        }
        tr:hover {
            background-color: #f8f9fa;
        }
        .status-active {
            color: #28a745;
            font-weight: bold;
        }
        .status-inactive {
            color: #dc3545;
            font-weight: bold;
        }
        .success {
            color: #28a745;
        }
        .failure {
            color: #dc3545;
        }
        .timestamp {
            color: #666;
            font-size: 0.9em;
        }
        .node-id {
            font-family: monospace;
            background-color: #f8f9fa;
            padding: 2px 6px;
            border-radius: 4px;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🌐 NodeProbe Network Report</h1>
        
        <div class="timestamp">
            <strong>Generated:</strong> {{.GeneratedAt}}<br>
            <strong>Reporting Node:</strong> {{.ReportingNode.ID}} ({{.ReportingNode.FQDN}})
        </div>

        <div class="stats">
            <div class="stat-card">
                <span class="stat-value">{{.TotalNodes}}</span>
                <span class="stat-label">Total Nodes</span>
            </div>
            <div class="stat-card">
                <span class="stat-value">{{.ActiveNodes}}</span>
                <span class="stat-label">Active Nodes</span>
            </div>
            <div class="stat-card">
                <span class="stat-value">{{.InactiveNodes}}</span>
                <span class="stat-label">Inactive Nodes</span>
            </div>
            <div class="stat-card">
                <span class="stat-value">{{printf "%.1f%%" .SuccessRate}}</span>
                <span class="stat-label">Success Rate (24h)</span>
            </div>
        </div>

        <h2>📊 Network Nodes</h2>
        <table>
            <thead>
                <tr>
                    <th>Node ID</th>
                    <th>FQDN</th>
                    <th>IP Address</th>
                    <th>Status</th>
                    <th>Discovered By</th>
                    <th>First Seen</th>
                    <th>Last Seen</th>
                </tr>
            </thead>
            <tbody>
                {{range .Nodes}}
                <tr>
                    <td><span class="node-id">{{.ID}}</span></td>
                    <td>{{.FQDN}}</td>
                    <td>{{.IP}}</td>
                    <td>
                        {{if .IsActive}}
                            <span class="status-active">●&nbsp;Active</span>
                        {{else}}
                            <span class="status-inactive">●&nbsp;Inactive</span>
                        {{end}}
                    </td>
                    <td>{{.DiscoveredBy}}</td>
                    <td>{{.FirstSeen.Format "2006-01-02 15:04"}}</td>
                    <td>{{.LastSeen.Format "2006-01-02 15:04"}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>

        <h2>🔍 Recent Poll Results (Last 24 Hours)</h2>
        <table>
            <thead>
                <tr>
                    <th>Time</th>
                    <th>Node ID</th>
                    <th>Status</th>
                    <th>Response Time</th>
                    <th>Path MTU</th>
                    <th>Error</th>
                </tr>
            </thead>
            <tbody>
                {{range .PollResults}}
                <tr>
                    <td>{{.PollTime.Format "01-02 15:04:05"}}</td>
                    <td><span class="node-id">{{.NodeID}}</span></td>
                    <td>
                        {{if .Success}}
                            <span class="success">✓ Success</span>
                        {{else}}
                            <span class="failure">✗ Failed</span>
                        {{end}}
                    </td>
                    <td>{{.ResponseMs}}ms</td>
                    <td>
                        {{if .PathMTU}}
                            {{.PathMTU}} bytes
                        {{else}}
                            -
                        {{end}}
                    </td>
                    <td>{{.Error}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>

        <div class="timestamp" style="margin-top: 40px; text-align: center; border-top: 1px solid #ddd; padding-top: 20px;">
            <em>NodeProbe Distributed Network Monitor</em>
        </div>
    </div>
</body>
</html>
`

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// IsRunning returns whether the reporting service is currently running
func (rs *ReportingService) IsRunning() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.running
}
