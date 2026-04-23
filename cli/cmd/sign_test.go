package cmd

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- RSA key helpers ---

func generateRSAPEMPair(t *testing.T) (privPath, pubPath string) {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privPath = filepath.Join(dir, "priv.pem")
	pubPath = filepath.Join(dir, "pub.pem")

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	pubDER, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	if werr := os.WriteFile(privPath, privPEM, 0o600); werr != nil {
		t.Fatal(werr)
	}
	if werr := os.WriteFile(pubPath, pubPEM, 0o600); werr != nil {
		t.Fatal(werr)
	}
	return
}

// --- ECDSA key helpers ---

func generateECDSAPEMPair(t *testing.T) (privPath, pubPath string) {
	t.Helper()
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privPath = filepath.Join(dir, "priv.pem")
	pubPath = filepath.Join(dir, "pub.pem")

	privDER, _ := x509.MarshalECPrivateKey(privKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})
	pubDER, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	if werr := os.WriteFile(privPath, privPEM, 0o600); werr != nil {
		t.Fatal(werr)
	}
	if werr := os.WriteFile(pubPath, pubPEM, 0o600); werr != nil {
		t.Fatal(werr)
	}
	return
}

// --- Ed25519 key helpers ---

func generateEd25519PEMPair(t *testing.T) (privPath, pubPath string) {
	t.Helper()
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privPath = filepath.Join(dir, "priv.pem")
	pubPath = filepath.Join(dir, "pub.pem")

	privDER, _ := x509.MarshalPKCS8PrivateKey(privKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	pubDER, _ := x509.MarshalPKIXPublicKey(pubKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	if werr := os.WriteFile(privPath, privPEM, 0o600); werr != nil {
		t.Fatal(werr)
	}
	if werr := os.WriteFile(pubPath, pubPEM, 0o600); werr != nil {
		t.Fatal(werr)
	}
	return
}

// --- Tests ---

func TestSignVerify_RSA_sha256(t *testing.T) {
	privPath, pubPath := generateRSAPEMPair(t)
	data := []byte("hello rsa")

	out := execute(t, signRootCmd, data, "--key", privPath, "--algo", "rsa-sha256")
	sigB64 := strings.TrimSpace(string(out))
	if sigB64 == "" {
		t.Fatal("expected non-empty signature")
	}

	out2 := execute(t, verifyCmd, data, "--key", pubPath, "--sig", sigB64, "--algo", "rsa-sha256")
	if !strings.Contains(string(out2), "valid") {
		t.Errorf("expected 'valid', got %s", out2)
	}
}

func TestSignVerify_RSA_sha512(t *testing.T) {
	privPath, pubPath := generateRSAPEMPair(t)
	data := []byte("hello rsa sha512")

	out := execute(t, signRootCmd, data, "--key", privPath, "--algo", "rsa-sha512")
	sigB64 := strings.TrimSpace(string(out))

	out2 := execute(t, verifyCmd, data, "--key", pubPath, "--sig", sigB64, "--algo", "rsa-sha512")
	if !strings.Contains(string(out2), "valid") {
		t.Errorf("expected 'valid', got %s", out2)
	}
}

func TestSignVerify_ECDSA_sha256(t *testing.T) {
	privPath, pubPath := generateECDSAPEMPair(t)
	data := []byte("hello ecdsa")

	out := execute(t, signRootCmd, data, "--key", privPath, "--algo", "ecdsa-sha256")
	sigB64 := strings.TrimSpace(string(out))
	if sigB64 == "" {
		t.Fatal("expected non-empty signature")
	}

	out2 := execute(t, verifyCmd, data, "--key", pubPath, "--sig", sigB64, "--algo", "ecdsa-sha256")
	if !strings.Contains(string(out2), "valid") {
		t.Errorf("expected 'valid', got %s", out2)
	}
}

func TestSignVerify_Ed25519(t *testing.T) {
	privPath, pubPath := generateEd25519PEMPair(t)
	data := []byte("hello ed25519")

	out := execute(t, signRootCmd, data, "--key", privPath, "--algo", "ed25519")
	sigB64 := strings.TrimSpace(string(out))
	if sigB64 == "" {
		t.Fatal("expected non-empty signature")
	}

	out2 := execute(t, verifyCmd, data, "--key", pubPath, "--sig", sigB64, "--algo", "ed25519")
	if !strings.Contains(string(out2), "valid") {
		t.Errorf("expected 'valid', got %s", out2)
	}
}

func TestSignCmd_json(t *testing.T) {
	privPath, _ := generateRSAPEMPair(t)
	out := execute(t, signRootCmd, []byte("data"), "--key", privPath, "--format", "json")
	var result map[string]string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out)
	}
	if result["algorithm"] == "" {
		t.Error("expected algorithm field")
	}
	if result["signature"] == "" {
		t.Error("expected signature field")
	}
}

func TestVerifyCmd_invalid(t *testing.T) {
	privPath, pubPath := generateRSAPEMPair(t)
	data := []byte("hello")

	out := execute(t, signRootCmd, data, "--key", privPath, "--algo", "rsa-sha256")
	sigB64 := strings.TrimSpace(string(out))

	shouldExitWithCode(t, 1, func() string {
		execute(t, verifyCmd, []byte("tampered"), "--key", pubPath, "--sig", sigB64, "--algo", "rsa-sha256")
		return ""
	})
}

func TestSignCmd_missingKey(t *testing.T) {
	shouldExitWithCode(t, 1, func() string {
		execute(t, signRootCmd, []byte("data"))
		return ""
	})
}

func TestVerifyCmd_badBase64(t *testing.T) {
	_, pubPath := generateRSAPEMPair(t)
	shouldExitWithCode(t, 1, func() string {
		execute(t, verifyCmd, []byte("data"), "--key", pubPath, "--sig", "!!!notbase64!!!")
		return ""
	})
}

func TestSignVerify_RSA_json_verify(t *testing.T) {
	privPath, pubPath := generateRSAPEMPair(t)
	data := []byte("test")

	out := execute(t, signRootCmd, data, "--key", privPath, "--algo", "rsa-sha256")
	sigB64 := strings.TrimSpace(string(out))

	out2 := execute(t, verifyCmd, data, "--key", pubPath, "--sig", sigB64, "--algo", "rsa-sha256", "--format", "json")
	var result map[string]interface{}
	if err := json.Unmarshal(out2, &result); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, out2)
	}
	if result["valid"] != true {
		t.Errorf("expected valid=true, got %v", result["valid"])
	}
}

func TestHashForAlgo_badAlgo(t *testing.T) {
	_, _, err := hashForAlgo("unsupported", []byte("x"))
	if err == nil {
		t.Error("expected error for unsupported algo")
	}
}

func TestSignData_wrongKeyType(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, serr := signData([]byte("x"), rsaKey, "ed25519")
	if serr == nil {
		t.Error("expected error for wrong key type (ed25519 algo with RSA key)")
	}
}

func TestVerifyData_badAlgo(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	err := verifyData([]byte("x"), []byte("sig"), pub, "unknown-algo")
	if err == nil {
		t.Error("expected error for unknown algo")
	}
}

func TestLoadPrivateKey_invalidPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	_ = os.WriteFile(path, []byte("not pem"), 0o600)
	_, err := loadPrivateKey(path)
	if err == nil {
		t.Error("expected error for non-PEM file")
	}
}

func TestLoadPublicKey_invalidPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	_ = os.WriteFile(path, []byte("not pem"), 0o600)
	_, err := loadPublicKey(path)
	if err == nil {
		t.Error("expected error for non-PEM file")
	}
}

func TestLoadPublicKey_unsupportedType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "weird.pem")
	block := pem.EncodeToMemory(&pem.Block{Type: "WEIRD KEY", Bytes: []byte("bytes")})
	_ = os.WriteFile(path, block, 0o600)
	_, err := loadPublicKey(path)
	if err == nil {
		t.Error("expected error for unsupported PEM type")
	}
}

func TestSignRSA_wrongKeyType(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, serr := signData([]byte("x"), ecKey, "rsa-sha256")
	if serr == nil {
		t.Error("expected error for EC key with RSA algo")
	}
}

func TestSignECDSA_wrongKeyType(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, serr := signData([]byte("x"), rsaKey, "ecdsa-sha256")
	if serr == nil {
		t.Error("expected error for RSA key with ECDSA algo")
	}
}

func TestVerifyRSA_wrongKeyType(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verErr := verifyData([]byte("x"), []byte("sig"), &ecKey.PublicKey, "rsa-sha256")
	if verErr == nil {
		t.Error("expected error for EC pubkey with RSA algo")
	}
}

func TestVerifyECDSA_wrongKeyType(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	verErr := verifyData([]byte("x"), []byte("sig"), &rsaKey.PublicKey, "ecdsa-sha256")
	if verErr == nil {
		t.Error("expected error for RSA pubkey with ECDSA algo")
	}
}

func TestVerifyEd25519_wrongKeyType(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	verErr := verifyData([]byte("x"), []byte("sig"), &rsaKey.PublicKey, "ed25519")
	if verErr == nil {
		t.Error("expected error for RSA pubkey with Ed25519 algo")
	}
}

func TestLoadPrivateKey_PKCS8_nonSigner(t *testing.T) {
	// Generate a valid PKCS8 key to confirm PKCS8 path works
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(privKey)
	dir := t.TempDir()
	path := filepath.Join(dir, "ed.pem")
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	_ = os.WriteFile(path, block, 0o600)
	signer, err := loadPrivateKey(path)
	if err != nil {
		t.Fatalf("expected no error loading PKCS8 Ed25519, got: %v", err)
	}
	if signer == nil {
		t.Error("expected non-nil signer")
	}
}

func TestSignData_ed25519_notEd25519Key(t *testing.T) {
	// Use a standard ECDSA signer (which doesn't implement ed25519.PrivateKey)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	_, err := signData([]byte("x"), ecKey, "ed25519")
	if err == nil {
		t.Error("expected error for non-ed25519 key with ed25519 algo")
	}
}

func TestVerifyCmd_yamlFormat(t *testing.T) {
	privPath, pubPath := generateRSAPEMPair(t)
	data := []byte("test yaml")
	out := execute(t, signRootCmd, data, "--key", privPath)
	sigB64 := strings.TrimSpace(string(out))

	out2 := execute(t, verifyCmd, data, "--key", pubPath, "--sig", sigB64, "--format", "yaml")
	if !strings.Contains(string(out2), "valid") {
		t.Errorf("expected yaml with valid, got: %s", out2)
	}
}

func TestSignRootCmd_yamlFormat(t *testing.T) {
	privPath, _ := generateRSAPEMPair(t)
	out := execute(t, signRootCmd, []byte("data"), "--key", privPath, "--format", "yaml")
	if !strings.Contains(string(out), "signature") {
		t.Errorf("expected 'signature' in yaml output, got: %s", out)
	}
}

func TestVerifyCmd_jsonInvalid(t *testing.T) {
	_, pubPath := generateRSAPEMPair(t)
	// Build a technically valid base64 but wrong signature
	wrongSig := base64.StdEncoding.EncodeToString([]byte("invalidsig"))
	out := execute(t, verifyCmd, []byte("data"), "--key", pubPath, "--sig", wrongSig, "--format", "json")
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["valid"] != false {
		t.Errorf("expected valid=false, got %v", result["valid"])
	}
}
