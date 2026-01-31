package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKey(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-key.pem")
	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(path, pemData, 0o600); err != nil {
		t.Fatalf("writing test key: %v", err)
	}

	return path, key
}

func generateTestKeyPKCS8(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshaling PKCS8: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-key-pkcs8.pem")
	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})
	if err := os.WriteFile(path, pemData, 0o600); err != nil {
		t.Fatalf("writing test key: %v", err)
	}

	return path
}

func TestGenerateJWT(t *testing.T) {
	keyPath, pubKey := generateTestKey(t)

	token, err := GenerateJWT(12345, keyPath)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	parsed, err := jwt.Parse(token, func(tok *jwt.Token) (any, error) {
		return &pubKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("parsing JWT: %v", err)
	}

	iss, _ := parsed.Claims.GetIssuer()
	if iss != "12345" {
		t.Errorf("issuer = %q, want %q", iss, "12345")
	}

	exp, _ := parsed.Claims.GetExpirationTime()
	if exp == nil || time.Until(exp.Time) < 9*time.Minute {
		t.Error("expiration should be ~10 minutes from now")
	}
}

func TestGenerateJWT_PKCS8(t *testing.T) {
	keyPath := generateTestKeyPKCS8(t)

	token, err := GenerateJWT(99999, keyPath)
	if err != nil {
		t.Fatalf("GenerateJWT with PKCS8: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestGenerateJWT_FileNotFound(t *testing.T) {
	_, err := GenerateJWT(1, "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
}

func TestGenerateJWT_InvalidPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(path, []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := GenerateJWT(1, path)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestGetInstallationToken(t *testing.T) {
	wantToken := "ghs_test_token_abc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/app/installations/67890/access_tokens") {
			t.Errorf("path = %s, want suffix /app/installations/67890/access_tokens", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization = %q, want Bearer prefix", auth)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"token":      wantToken,
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	got, err := GetInstallationToken("fake-jwt", 67890, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("GetInstallationToken: %v", err)
	}
	if got != wantToken {
		t.Errorf("token = %q, want %q", got, wantToken)
	}
}

func TestGetInstallationToken_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	_, err := GetInstallationToken("bad-jwt", 1, WithBaseURL(srv.URL))
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want substring %q", err.Error(), "401")
	}
}
