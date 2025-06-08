package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"nodeprobe/internal/app"
	"nodeprobe/internal/pkg/config"
	"nodeprobe/internal/pkg/http"
	"nodeprobe/internal/pkg/sqlite"
	"nodeprobe/internal/pkg/tls"
)

const (
	dataDir = "/app/data"
	certDir = "/app/certs"
)

func main() {
	log.Println("Starting NodeProbe...")

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize services
	if err := run(ctx); err != nil {
		log.Fatalf("Application failed: %v", err)
	}

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal, gracefully shutting down...")
	cancel()

	// Give services time to shut down gracefully
	time.Sleep(5 * time.Second)
	log.Println("NodeProbe stopped")
}

func run(ctx context.Context) error {
	// Ensure data directories exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Initialize configuration service
	configSvc, err := config.NewService(dataDir)
	if err != nil {
		return fmt.Errorf("failed to create config service: %w", err)
	}

	// Create sample configuration files
	if err := configSvc.CreateSampleSeedConfig(); err != nil {
		log.Printf("Warning: failed to create sample seed config: %v", err)
	}
	if err := configSvc.CreateSampleReportingConfig(); err != nil {
		log.Printf("Warning: failed to create sample reporting config: %v", err)
	}

	// Initialize database
	dbPath := filepath.Join(dataDir, "nodeprobe.db")
	repo, err := sqlite.NewRepository(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			log.Printf("Failed to close repository: %v", err)
		}
	}()

	// Initialize HTTP client
	httpClient := http.NewClient()
	defer func() {
		if err := httpClient.Close(); err != nil {
			log.Printf("Failed to close HTTP client: %v", err)
		}
	}()

	// Initialize TLS service
	tlsService := tls.NewService(certDir)

	// Initialize node service
	nodeService := app.NewNodeService(repo, configSvc)
	if err := nodeService.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize node service: %w", err)
	}

	// Initialize polling service
	pollingService := app.NewPollingService(nodeService, repo, httpClient, configSvc)

	// Initialize reporting service
	reportingService := app.NewReportingService(nodeService, httpClient, configSvc, repo)

	// Initialize web server
	webServer := app.NewWebServer(nodeService, reportingService, configSvc, tlsService)

	// Start all services
	log.Println("Starting services...")

	// Start web server first
	if err := webServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start web server: %w", err)
	}

	// Wait a moment for web server to start
	time.Sleep(2 * time.Second)

	// Start polling service
	if err := pollingService.Start(ctx); err != nil {
		return fmt.Errorf("failed to start polling service: %w", err)
	}

	// Start reporting service
	if err := reportingService.Start(ctx); err != nil {
		return fmt.Errorf("failed to start reporting service: %w", err)
	}

	// Get node information for logging
	nodeInfo, err := configSvc.GetNodeInfo()
	if err != nil {
		log.Printf("Warning: failed to get node info for logging: %v", err)
	} else {
		log.Printf("NodeProbe started successfully!")
		log.Printf("Node ID: %s", nodeInfo.ID)
		log.Printf("Node FQDN: %s", nodeInfo.FQDN)
		log.Printf("Node IP: %s", nodeInfo.IP)
		log.Printf("HTTPS Server: https://%s:443", nodeInfo.FQDN)
		log.Printf("Dashboard: https://%s:443/dashboard", nodeInfo.FQDN)
		log.Printf("Node Info API: https://%s:443/nodeinfo", nodeInfo.FQDN)
		log.Printf("Health Check: https://%s:443/health", nodeInfo.FQDN)
	}

	// Start cleanup routine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := pollingService.CleanupOldResults(ctx); err != nil {
					log.Printf("Failed to cleanup old poll results: %v", err)
				}
			}
		}
	}()

	// Keep the main goroutine alive and handle context cancellation
	<-ctx.Done()

	// Graceful shutdown
	log.Println("Starting graceful shutdown...")

	// Stop services in reverse order
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := reportingService.Stop(); err != nil {
		log.Printf("Error stopping reporting service: %v", err)
	}

	if err := pollingService.Stop(); err != nil {
		log.Printf("Error stopping polling service: %v", err)
	}

	if err := webServer.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping web server: %v", err)
	}

	return nil
}
