package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"nodeprobe/internal/domain"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	// Create HTTP client with TLS configuration that accepts self-signed certificates
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Accept self-signed certificates
		},
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	return &Client{
		httpClient: client,
	}
}

func (c *Client) GetNodeInfo(ctx context.Context, nodeURL string) (*domain.NodeInfo, error) {
	// Ensure URL has https scheme and proper format
	if !strings.HasPrefix(nodeURL, "https://") {
		nodeURL = "https://" + nodeURL
	}

	// Add the node info endpoint
	if !strings.HasSuffix(nodeURL, "/") {
		nodeURL += "/"
	}
	nodeURL += "nodeinfo"

	req, err := http.NewRequestWithContext(ctx, "GET", nodeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "NodeProbe/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	var nodeInfo domain.NodeInfo
	if err := json.NewDecoder(resp.Body).Decode(&nodeInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &nodeInfo, nil
}

func (c *Client) SendNetworkSnapshot(ctx context.Context, reportingURL string, snapshot *domain.NetworkSnapshot) error {
	// Ensure URL has https scheme and proper format
	if !strings.HasPrefix(reportingURL, "https://") {
		reportingURL = "https://" + reportingURL
	}

	// Add the report endpoint
	if !strings.HasSuffix(reportingURL, "/") {
		reportingURL += "/"
	}
	reportingURL += "report"

	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reportingURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NodeProbe/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) TestPathMTU(ctx context.Context, nodeURL string) (int, error) {
	// Parse the URL to get the host
	if !strings.HasPrefix(nodeURL, "https://") {
		nodeURL = "https://" + nodeURL
	}

	// Extract host from URL
	host := strings.TrimPrefix(nodeURL, "https://")
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx == -1 {
		host += ":443" // Add default HTTPS port
	}

	// Perform Path MTU Discovery
	mtu, err := c.discoverPathMTU(ctx, host)
	if err != nil {
		return 0, fmt.Errorf("failed to discover path MTU: %w", err)
	}

	return mtu, nil
}

func (c *Client) discoverPathMTU(ctx context.Context, host string) (int, error) {
	// Start with common MTU sizes and work our way down
	mtuSizes := []int{1500, 1472, 1460, 1400, 1300, 1200, 1000, 800, 576}

	for _, mtu := range mtuSizes {
		if c.testMTUSize(ctx, host, mtu) {
			return mtu, nil
		}
	}

	// If none of the standard sizes work, try a binary search approach
	return c.binarySearchMTU(ctx, host, 576, 1500)
}

func (c *Client) testMTUSize(ctx context.Context, host string, mtu int) bool {
	// Create a TCP connection to test the path
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Test by attempting to send data of the specified size
	// This is a simplified approach - in production, you might want
	// to use raw sockets and ICMP for true Path MTU Discovery
	testData := make([]byte, mtu-40) // Account for TCP/IP headers

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(testData)

	return err == nil
}

func (c *Client) binarySearchMTU(ctx context.Context, host string, min, max int) (int, error) {
	if min >= max {
		return min, nil
	}

	mid := (min + max) / 2

	if c.testMTUSize(ctx, host, mid) {
		// Try a larger size
		return c.binarySearchMTU(ctx, host, mid+1, max)
	} else {
		// Try a smaller size
		return c.binarySearchMTU(ctx, host, min, mid-1)
	}
}

func (c *Client) Close() error {
	// Close idle connections
	c.httpClient.CloseIdleConnections()
	return nil
}
