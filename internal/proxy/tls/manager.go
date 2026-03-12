package tls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Certificate holds metadata about a TLS certificate managed by the Manager.
type Certificate struct {
	Domain    string    `json:"domain"`
	CertPath  string    `json:"cert_path"`
	KeyPath   string    `json:"key_path"`
	ExpiresAt time.Time `json:"expires_at"`
	AutoRenew bool      `json:"auto_renew"`
}

// Manager handles TLS certificate lifecycle including loading from disk,
// generating self-signed certificates for development, and automatic renewal.
type Manager struct {
	certDir string
	email   string
	domains map[string]*Certificate
	certs   map[string]*tls.Certificate
	mu      sync.RWMutex
}

// NewManager creates a new TLS Manager. certDir is the directory where certificates
// are stored on disk, and email is the contact email for ACME registration.
func NewManager(certDir, email string) *Manager {
	return &Manager{
		certDir: certDir,
		email:   email,
		domains: make(map[string]*Certificate),
		certs:   make(map[string]*tls.Certificate),
	}
}

// GetCertificate is the callback used by tls.Config to dynamically select a
// certificate during the TLS handshake. It checks the in-memory cache first,
// attempts to load from disk, and falls back to generating a self-signed cert.
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello == nil || hello.ServerName == "" {
		return nil, fmt.Errorf("no server name provided in TLS handshake")
	}

	domain := strings.ToLower(hello.ServerName)

	// Check in-memory cache
	m.mu.RLock()
	if cert, exists := m.certs[domain]; exists {
		m.mu.RUnlock()
		return cert, nil
	}
	m.mu.RUnlock()

	// Try loading from disk
	cert, err := m.loadFromDisk(domain)
	if err == nil {
		m.mu.Lock()
		m.certs[domain] = cert
		m.mu.Unlock()
		return cert, nil
	}

	// Try wildcard match (e.g., *.example.com)
	parts := strings.SplitN(domain, ".", 2)
	if len(parts) == 2 {
		wildcardDomain := "*." + parts[1]
		m.mu.RLock()
		if cert, exists := m.certs[wildcardDomain]; exists {
			m.mu.RUnlock()
			return cert, nil
		}
		m.mu.RUnlock()

		cert, err := m.loadFromDisk(wildcardDomain)
		if err == nil {
			m.mu.Lock()
			m.certs[wildcardDomain] = cert
			m.mu.Unlock()
			return cert, nil
		}
	}

	// Generate self-signed certificate for development
	selfSigned, err := m.GenerateSelfSigned(domain)
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed certificate for %s: %w", domain, err)
	}

	m.mu.Lock()
	m.certs[domain] = selfSigned
	m.mu.Unlock()

	return selfSigned, nil
}

// loadFromDisk attempts to load a certificate and key pair from the cert directory.
func (m *Manager) loadFromDisk(domain string) (*tls.Certificate, error) {
	safeName := sanitizeDomainForPath(domain)
	certPath := filepath.Join(m.certDir, safeName, "cert.pem")
	keyPath := filepath.Join(m.certDir, safeName, "key.pem")

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate for %s: %w", domain, err)
	}

	// Parse the leaf certificate to check expiration
	if len(cert.Certificate) > 0 {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err == nil {
			cert.Leaf = leaf
			if time.Now().After(leaf.NotAfter) {
				return nil, fmt.Errorf("certificate for %s has expired at %v", domain, leaf.NotAfter)
			}
		}
	}

	return &cert, nil
}

// AddDomain registers a domain for TLS certificate management.
func (m *Manager) AddDomain(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.domains[domain]; exists {
		return fmt.Errorf("domain %q is already registered", domain)
	}

	safeName := sanitizeDomainForPath(domain)
	m.domains[domain] = &Certificate{
		Domain:    domain,
		CertPath:  filepath.Join(m.certDir, safeName, "cert.pem"),
		KeyPath:   filepath.Join(m.certDir, safeName, "key.pem"),
		AutoRenew: true,
	}
	return nil
}

// RemoveDomain unregisters a domain and removes its cached certificate.
func (m *Manager) RemoveDomain(domain string) {
	domain = strings.ToLower(strings.TrimSpace(domain))

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.domains, domain)
	delete(m.certs, domain)
}

// LoadCertificates loads all certificates from the cert directory into memory.
// It walks the certDir looking for cert.pem/key.pem pairs in subdirectories.
func (m *Manager) LoadCertificates() error {
	if m.certDir == "" {
		return fmt.Errorf("certificate directory not configured")
	}

	entries, err := os.ReadDir(m.certDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No certs directory yet, not an error
		}
		return fmt.Errorf("failed to read certificate directory %s: %w", m.certDir, err)
	}

	var loadErrors []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		domain := entry.Name()
		certPath := filepath.Join(m.certDir, domain, "cert.pem")
		keyPath := filepath.Join(m.certDir, domain, "key.pem")

		// Check both files exist
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			continue
		}

		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", domain, err))
			continue
		}

		var expiresAt time.Time
		if len(cert.Certificate) > 0 {
			leaf, err := x509.ParseCertificate(cert.Certificate[0])
			if err == nil {
				cert.Leaf = leaf
				expiresAt = leaf.NotAfter
			}
		}

		m.mu.Lock()
		m.certs[domain] = &cert
		m.domains[domain] = &Certificate{
			Domain:    domain,
			CertPath:  certPath,
			KeyPath:   keyPath,
			ExpiresAt: expiresAt,
			AutoRenew: true,
		}
		m.mu.Unlock()
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load some certificates: %s", strings.Join(loadErrors, "; "))
	}
	return nil
}

// GenerateSelfSigned creates a self-signed TLS certificate for development use.
// The certificate is valid for 365 days and is also persisted to disk.
func (m *Manager) GenerateSelfSigned(domain string) (*tls.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NexusOps Development"},
			CommonName:   domain,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	// If wildcard, also add the base domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain := domain[2:]
		template.DNSNames = append(template.DNSNames, baseDomain)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Persist to disk
	if m.certDir != "" {
		if writeErr := m.writeCertToDisk(domain, certDER, privateKey); writeErr != nil {
			// Log but don't fail — the in-memory cert is still usable
			_ = writeErr
		}
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS certificate: %w", err)
	}

	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err == nil {
		tlsCert.Leaf = leaf
	}

	// Update domain metadata
	m.mu.Lock()
	safeName := sanitizeDomainForPath(domain)
	m.domains[domain] = &Certificate{
		Domain:    domain,
		CertPath:  filepath.Join(m.certDir, safeName, "cert.pem"),
		KeyPath:   filepath.Join(m.certDir, safeName, "key.pem"),
		ExpiresAt: notAfter,
		AutoRenew: false, // Self-signed certs are not auto-renewed via ACME
	}
	m.mu.Unlock()

	return &tlsCert, nil
}

// writeCertToDisk saves a certificate and private key to the certDir.
func (m *Manager) writeCertToDisk(domain string, certDER []byte, key *ecdsa.PrivateKey) error {
	safeName := sanitizeDomainForPath(domain)
	dir := filepath.Join(m.certDir, safeName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	certPath := filepath.Join(dir, "cert.pem")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write cert PEM: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal key: %w", err)
	}
	keyPath := filepath.Join(dir, "key.pem")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("failed to write key PEM: %w", err)
	}

	return nil
}

// RenewCertificates checks all managed domains and renews certificates that
// are expiring within the next 30 days. In production, this would use the ACME
// protocol; for now, it regenerates self-signed certificates as a placeholder.
func (m *Manager) RenewCertificates(ctx context.Context) error {
	m.mu.RLock()
	domainsToRenew := make([]string, 0)
	renewalThreshold := time.Now().Add(30 * 24 * time.Hour)

	for domain, certInfo := range m.domains {
		if !certInfo.AutoRenew {
			continue
		}
		if certInfo.ExpiresAt.IsZero() || certInfo.ExpiresAt.Before(renewalThreshold) {
			domainsToRenew = append(domainsToRenew, domain)
		}
	}
	m.mu.RUnlock()

	if len(domainsToRenew) == 0 {
		return nil
	}

	var renewErrors []string

	for _, domain := range domainsToRenew {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// In production, this would perform ACME challenge-response with Let's Encrypt.
		// For now, generate a new self-signed certificate as a placeholder.
		cert, err := m.GenerateSelfSigned(domain)
		if err != nil {
			renewErrors = append(renewErrors, fmt.Sprintf("%s: %v", domain, err))
			continue
		}

		m.mu.Lock()
		m.certs[domain] = cert
		if info, exists := m.domains[domain]; exists {
			info.ExpiresAt = time.Now().Add(365 * 24 * time.Hour)
		}
		m.mu.Unlock()
	}

	if len(renewErrors) > 0 {
		return fmt.Errorf("failed to renew some certificates: %s", strings.Join(renewErrors, "; "))
	}
	return nil
}

// GetTLSConfig returns a tls.Config configured to use this Manager's
// GetCertificate callback for dynamic certificate selection.
func (m *Manager) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: m.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}
}

// sanitizeDomainForPath converts a domain name to a safe directory name.
func sanitizeDomainForPath(domain string) string {
	return strings.ReplaceAll(domain, "*", "_wildcard")
}
