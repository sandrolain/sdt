package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// generateSelfSignedCert creates a temporary self-signed PEM certificate for testing.
func generateSelfSignedCert(t *testing.T) (certPEM []byte, certFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.example.com"},
		DNSNames:     []string{"test.example.com"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	dir := t.TempDir()
	certFile = filepath.Join(dir, "test.pem")
	if werr := os.WriteFile(certFile, certPEM, 0o600); werr != nil {
		t.Fatal(werr)
	}
	return certPEM, certFile
}

func TestCertParsePEMBytes(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)
	certs, err := parsePEMCertBytes(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if len(certs) != 1 {
		t.Fatalf("expected 1 cert, got %d", len(certs))
	}
}

func TestCertParsePEMBytes_empty(t *testing.T) {
	certs, err := parsePEMCertBytes([]byte("no pem here"))
	if err != nil {
		t.Fatal(err)
	}
	if len(certs) != 0 {
		t.Fatalf("expected 0 certs, got %d", len(certs))
	}
}

func TestCertInfoFromCert(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)
	certs, err := parsePEMCertBytes(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	info := certInfoFromCert(certs[0])
	if info.DaysLeft < 0 {
		t.Error("expected non-negative days_left for a valid cert")
	}
	if info.Expired {
		t.Error("expected non-expired cert")
	}
	if !strings.Contains(info.Subject, "test.example.com") {
		t.Errorf("unexpected subject: %s", info.Subject)
	}
	if len(info.Fingerprint) != 64 {
		t.Errorf("expected 64-char SHA-256 fingerprint, got %d", len(info.Fingerprint))
	}
}

func TestCertInspectCmd_file_json(t *testing.T) {
	certPEM, certFile := generateSelfSignedCert(t)
	_ = certPEM

	out := execute(t, certInspectCmd, nil, "--file", certFile, "--format", "json")
	var infos []CertInfo
	if err := json.Unmarshal(out, &infos); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if len(infos) == 0 {
		t.Error("expected at least one cert info")
	}
}

func TestCertInspectCmd_stdin(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)
	out := execute(t, certInspectCmd, certPEM, "--format", "json")
	var infos []CertInfo
	if err := json.Unmarshal(out, &infos); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if len(infos) == 0 {
		t.Error("expected at least one cert info")
	}
}

func TestCertInspectCmd_noCerts(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, certInspectCmd, []byte("not a pem"))
		return ""
	})
}

func TestCertExpiryCmd_file_json(t *testing.T) {
	_, certFile := generateSelfSignedCert(t)
	out := execute(t, certExpiryCmd, nil, "--file", certFile, "--format", "json")
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if _, ok := result["not_after"]; !ok {
		t.Error("expected not_after field")
	}
}

func TestCertInspectCmd_yaml(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)
	out := execute(t, certInspectCmd, certPEM, "--format", "yaml")
	if !strings.Contains(string(out), "subject") {
		t.Errorf("expected 'subject' in yaml output, got: %s", out)
	}
}

func TestCertExpiryCmd_yaml(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)
	out := execute(t, certExpiryCmd, certPEM, "--format", "yaml")
	if !strings.Contains(string(out), "not_after") {
		t.Errorf("expected 'not_after' in yaml output, got: %s", out)
	}
}

func TestParsePEMCertFile_notFound(t *testing.T) {
	_, err := parsePEMCertFile("/nonexistent/path/cert.pem")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestCertExpiryCmd_text(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)
	out := execute(t, certExpiryCmd, certPEM)
	if !strings.Contains(string(out), "expires") {
		t.Errorf("expected 'expires' in output, got: %s", out)
	}
}
