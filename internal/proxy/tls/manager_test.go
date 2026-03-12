package tls

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/certs", "admin@example.com")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.certDir != "/tmp/certs" {
		t.Errorf("certDir = %q, want %q", m.certDir, "/tmp/certs")
	}
	if m.email != "admin@example.com" {
		t.Errorf("email = %q, want %q", m.email, "admin@example.com")
	}
	if m.domains == nil {
		t.Fatal("domains map is nil")
	}
	if m.certs == nil {
		t.Fatal("certs map is nil")
	}
	if len(m.domains) != 0 {
		t.Errorf("domains length = %d, want 0", len(m.domains))
	}
	if len(m.certs) != 0 {
		t.Errorf("certs length = %d, want 0", len(m.certs))
	}
}

func TestAddDomain(t *testing.T) {
	tests := []struct {
		name      string
		domain    string
		wantErr   bool
		errSubstr string
	}{
		{"valid domain", "example.com", false, ""},
		{"wildcard domain", "*.example.com", false, ""},
		{"empty domain", "", true, "cannot be empty"},
		{"whitespace domain", "   ", true, "cannot be empty"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManager(t.TempDir(), "test@example.com")
			err := m.AddDomain(tc.domain)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify domain was registered.
			normalizedDomain := strings.ToLower(strings.TrimSpace(tc.domain))
			m.mu.RLock()
			info, exists := m.domains[normalizedDomain]
			m.mu.RUnlock()
			if !exists {
				t.Fatalf("domain %q not found in domains map", normalizedDomain)
			}
			if info.Domain != normalizedDomain {
				t.Errorf("info.Domain = %q, want %q", info.Domain, normalizedDomain)
			}
			if !info.AutoRenew {
				t.Error("AutoRenew should be true for newly added domains")
			}
		})
	}
}

func TestAddDomainDuplicate(t *testing.T) {
	m := NewManager(t.TempDir(), "test@example.com")

	if err := m.AddDomain("example.com"); err != nil {
		t.Fatalf("first AddDomain failed: %v", err)
	}

	err := m.AddDomain("example.com")
	if err == nil {
		t.Fatal("expected error for duplicate domain, got nil")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("error = %q, want it to contain 'already registered'", err.Error())
	}
}

func TestGetCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, "test@example.com")

	// Pre-generate and cache a certificate.
	generated, err := m.GenerateSelfSigned("cached.example.com")
	if err != nil {
		t.Fatalf("GenerateSelfSigned failed: %v", err)
	}
	m.mu.Lock()
	m.certs["cached.example.com"] = generated
	m.mu.Unlock()

	// GetCertificate should return the cached cert.
	hello := &tls.ClientHelloInfo{ServerName: "cached.example.com"}
	cert, err := m.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate returned error: %v", err)
	}
	if cert != generated {
		t.Error("GetCertificate did not return the cached certificate")
	}
}

func TestGetCertificateNilHello(t *testing.T) {
	m := NewManager(t.TempDir(), "test@example.com")

	_, err := m.GetCertificate(nil)
	if err == nil {
		t.Fatal("expected error for nil hello, got nil")
	}
	if !strings.Contains(err.Error(), "no server name") {
		t.Errorf("error = %q, want it to contain 'no server name'", err.Error())
	}
}

func TestGetCertificateEmptyServerName(t *testing.T) {
	m := NewManager(t.TempDir(), "test@example.com")

	hello := &tls.ClientHelloInfo{ServerName: ""}
	_, err := m.GetCertificate(hello)
	if err == nil {
		t.Fatal("expected error for empty server name, got nil")
	}
	if !strings.Contains(err.Error(), "no server name") {
		t.Errorf("error = %q, want it to contain 'no server name'", err.Error())
	}
}

func TestGetCertificateAutoGenerates(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, "test@example.com")

	// No cert cached, no cert on disk -- should auto-generate self-signed.
	hello := &tls.ClientHelloInfo{ServerName: "new.example.com"}
	cert, err := m.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate returned error: %v", err)
	}
	if cert == nil {
		t.Fatal("GetCertificate returned nil cert")
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate failed: %v", err)
	}
	found := false
	for _, dns := range leaf.DNSNames {
		if dns == "new.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DNSNames = %v, want it to contain 'new.example.com'", leaf.DNSNames)
	}
}

func TestGenerateSelfSigned(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, "test@example.com")

	cert, err := m.GenerateSelfSigned("example.com")
	if err != nil {
		t.Fatalf("GenerateSelfSigned failed: %v", err)
	}
	if cert == nil {
		t.Fatal("GenerateSelfSigned returned nil")
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("cert.Certificate is empty")
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate failed: %v", err)
	}

	if leaf.Subject.CommonName != "example.com" {
		t.Errorf("CommonName = %q, want %q", leaf.Subject.CommonName, "example.com")
	}
	found := false
	for _, dns := range leaf.DNSNames {
		if dns == "example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DNSNames = %v, want it to contain 'example.com'", leaf.DNSNames)
	}
	if len(leaf.Subject.Organization) == 0 || leaf.Subject.Organization[0] != "NexusOps Development" {
		t.Errorf("Organization = %v, want [NexusOps Development]", leaf.Subject.Organization)
	}

	// Certificate should be valid now and expire in ~365 days.
	now := time.Now()
	if now.Before(leaf.NotBefore) {
		t.Errorf("certificate NotBefore (%v) is in the future", leaf.NotBefore)
	}
	expectedExpiry := now.Add(365 * 24 * time.Hour)
	diff := leaf.NotAfter.Sub(expectedExpiry)
	if diff > time.Hour || diff < -time.Hour {
		t.Errorf("certificate expiry diff from 365 days = %v, want within 1 hour", diff)
	}

	// Domain metadata should be updated.
	m.mu.RLock()
	domainInfo, exists := m.domains["example.com"]
	m.mu.RUnlock()
	if !exists {
		t.Fatal("domain metadata not found after GenerateSelfSigned")
	}
	if domainInfo.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero")
	}
	if domainInfo.AutoRenew {
		t.Error("AutoRenew should be false for self-signed certs")
	}
}

func TestGenerateSelfSignedWildcard(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, "test@example.com")

	cert, err := m.GenerateSelfSigned("*.example.com")
	if err != nil {
		t.Fatalf("GenerateSelfSigned failed: %v", err)
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate failed: %v", err)
	}

	hasWildcard := false
	hasBase := false
	for _, dns := range leaf.DNSNames {
		if dns == "*.example.com" {
			hasWildcard = true
		}
		if dns == "example.com" {
			hasBase = true
		}
	}
	if !hasWildcard {
		t.Error("DNSNames should contain '*.example.com'")
	}
	if !hasBase {
		t.Error("DNSNames should contain 'example.com' (base domain for wildcard)")
	}
}
