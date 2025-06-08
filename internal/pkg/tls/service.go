package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Service struct {
	certDir  string
	certPath string
	keyPath  string
}

func NewService(certDir string) *Service {
	return &Service{
		certDir:  certDir,
		certPath: filepath.Join(certDir, "server.crt"),
		keyPath:  filepath.Join(certDir, "server.key"),
	}
}

func (s *Service) GenerateSelfSignedCert() error {
	// Ensure certificate directory exists
	if err := os.MkdirAll(s.certDir, 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}

	// Check if certificate already exists and is valid
	if s.certificateExists() && s.certificateValid() {
		return nil // Certificate already exists and is valid
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:       []string{"NodeProbe"},
			OrganizationalUnit: []string{"Distributed Network"},
			Country:            []string{"US"},
			Province:           []string{""},
			Locality:           []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add local network addresses to certificate
	if err := s.addNetworkAddresses(&template); err != nil {
		return fmt.Errorf("failed to add network addresses: %w", err)
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate to file
	certOut, err := os.Create(s.certPath)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to encode certificate: %w", err)
	}

	// Save private key to file
	keyOut, err := os.Create(s.keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	// Set restrictive permissions on private key
	if err := keyOut.Chmod(0600); err != nil {
		return fmt.Errorf("failed to set key file permissions: %w", err)
	}

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	return nil
}

func (s *Service) GetCertPath() (string, string, error) {
	// Check if certificate files exist
	if !s.certificateExists() {
		return "", "", fmt.Errorf("certificate files do not exist")
	}

	return s.certPath, s.keyPath, nil
}

func (s *Service) certificateExists() bool {
	_, certErr := os.Stat(s.certPath)
	_, keyErr := os.Stat(s.keyPath)
	return certErr == nil && keyErr == nil
}

func (s *Service) certificateValid() bool {
	// Load certificate
	certPEM, err := os.ReadFile(s.certPath)
	if err != nil {
		return false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	// Check if certificate is still valid (not expired and valid for at least 30 days)
	now := time.Now()
	return cert.NotAfter.After(now.Add(30 * 24 * time.Hour))
}

func (s *Service) addNetworkAddresses(template *x509.Certificate) error {
	// Add localhost addresses
	template.IPAddresses = append(template.IPAddresses, net.IPv4(127, 0, 0, 1))
	template.IPAddresses = append(template.IPAddresses, net.IPv6loopback)
	template.DNSNames = append(template.DNSNames, "localhost")

	// Get hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		template.DNSNames = append(template.DNSNames, hostname)
	}

	// Get local network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				// Only add non-loopback addresses
				if !v.IP.IsLoopback() {
					template.IPAddresses = append(template.IPAddresses, v.IP)
				}
			case *net.IPAddr:
				if !v.IP.IsLoopback() {
					template.IPAddresses = append(template.IPAddresses, v.IP)
				}
			}
		}
	}

	return nil
}

// RenewCertificate forces renewal of the certificate
func (s *Service) RenewCertificate() error {
	// Remove existing certificate files
	os.Remove(s.certPath)
	os.Remove(s.keyPath)

	// Generate new certificate
	return s.GenerateSelfSignedCert()
}
