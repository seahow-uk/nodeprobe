package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"nodeprobe/internal/domain"
)

type WebServer struct {
	nodeService      domain.NodeService
	reportingService domain.ReportingService
	configSvc        domain.ConfigService
	tlsService       domain.TLSService
	server           *http.Server
	receivedReports  []domain.NetworkSnapshot // Store received reports for dashboard
}

func NewWebServer(
	nodeService domain.NodeService,
	reportingService domain.ReportingService,
	configSvc domain.ConfigService,
	tlsService domain.TLSService,
) *WebServer {
	return &WebServer{
		nodeService:      nodeService,
		reportingService: reportingService,
		configSvc:        configSvc,
		tlsService:       tlsService,
		receivedReports:  make([]domain.NetworkSnapshot, 0),
	}
}

func (ws *WebServer) Start(ctx context.Context) error {
	// Generate TLS certificate if needed
	if err := ws.tlsService.GenerateSelfSignedCert(); err != nil {
		return fmt.Errorf("failed to generate TLS certificate: %w", err)
	}

	// Get certificate paths
	certPath, keyPath, err := ws.tlsService.GetCertPath()
	if err != nil {
		return fmt.Errorf("failed to get certificate paths: %w", err)
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	ws.setupRoutes(mux)

	// Create HTTPS server
	ws.server = &http.Server{
		Addr:         ":443",
		Handler:      mux,
		TLSConfig:    nil, // Will use cert files
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting HTTPS server on port 443...")

	// Start server in a goroutine
	go func() {
		if err := ws.server.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTPS server error: %v", err)
		}
	}()

	return nil
}

func (ws *WebServer) Stop(ctx context.Context) error {
	if ws.server == nil {
		return nil
	}

	log.Println("Shutting down HTTPS server...")

	// Create a timeout context for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := ws.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to gracefully shutdown server: %w", err)
	}

	return nil
}

func (ws *WebServer) setupRoutes(mux *http.ServeMux) {
	// Node info endpoint - returns this node's information and known nodes
	mux.HandleFunc("/nodeinfo", ws.handleNodeInfo)

	// Report endpoint - accepts network snapshots from other nodes
	mux.HandleFunc("/report", ws.handleReport)

	// Dashboard endpoint - serves HTML report for humans
	mux.HandleFunc("/dashboard", ws.handleDashboard)

	// Health check endpoint
	mux.HandleFunc("/health", ws.handleHealth)

	// Default to dashboard
	mux.HandleFunc("/", ws.handleDashboard)
}

func (ws *WebServer) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current node info
	nodeInfo, err := ws.configSvc.GetNodeInfo()
	if err != nil {
		log.Printf("Failed to get node info: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get known nodes and update the node info
	ctx := r.Context()
	nodes, err := ws.nodeService.GetKnownNodes(ctx)
	if err != nil {
		log.Printf("Failed to get known nodes: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	nodeInfo.Nodes = nodes

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if err := json.NewEncoder(w).Encode(nodeInfo); err != nil {
		log.Printf("Failed to encode node info: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse network snapshot from request body
	var snapshot domain.NetworkSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		log.Printf("Failed to decode network snapshot: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Store the received report (for dashboard purposes)
	ws.receivedReports = append(ws.receivedReports, snapshot)

	// Keep only the last 100 reports to avoid memory issues
	if len(ws.receivedReports) > 100 {
		ws.receivedReports = ws.receivedReports[1:]
	}

	log.Printf("Received network snapshot from node %s with %d nodes",
		snapshot.NodeID, len(snapshot.Nodes))

	// Merge the node information from the snapshot
	ctx := r.Context()
	nodeInfo := &domain.NodeInfo{
		ID:    snapshot.NodeID,
		Nodes: snapshot.Nodes,
	}

	if err := ws.nodeService.MergeNodeInfo(ctx, nodeInfo, "report"); err != nil {
		log.Printf("Failed to merge node info from report: %v", err)
		// Don't return an error to the client as we still received the report
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Network snapshot received",
	})
}

func (ws *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate HTML report
	html, err := ws.reportingService.GenerateHTMLReport()
	if err != nil {
		log.Printf("Failed to generate HTML report: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return HTML response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Failed to write HTML response: %v", err)
	}
}

func (ws *WebServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get basic health information
	ctx := r.Context()
	nodes, err := ws.nodeService.GetKnownNodes(ctx)
	if err != nil {
		log.Printf("Failed to get nodes for health check: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	nodeInfo, err := ws.configSvc.GetNodeInfo()
	if err != nil {
		log.Printf("Failed to get node info for health check: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	health := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().Format(time.RFC3339),
		"node_id":     nodeInfo.ID,
		"node_fqdn":   nodeInfo.FQDN,
		"node_ip":     nodeInfo.IP,
		"known_nodes": len(nodes),
		"uptime":      time.Since(time.Now()).String(), // This is just a placeholder
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(health); err != nil {
		log.Printf("Failed to encode health response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (ws *WebServer) loggingMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log request
		duration := time.Since(start)
		log.Printf("%s %s %d %v %s",
			r.Method, r.URL.Path, wrapped.statusCode, duration, r.RemoteAddr)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// GetReceivedReports returns the recent network snapshots received from other nodes
func (ws *WebServer) GetReceivedReports() []domain.NetworkSnapshot {
	return ws.receivedReports
}
