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
	keyPath, privKey := generateTestKey(t)

	token, err := GenerateJWT(12345, keyPath)
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	parsed, err := jwt.Parse(token, func(tok *jwt.Token) (any, error) {
		return &privKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("parsing JWT: %v", err)
	}

	iss, _ := parsed.Claims.GetIssuer()
	if iss != "12345" {
		t.Errorf("issuer = %q, want %q", iss, "12345")
	}

	iat, _ := parsed.Claims.GetIssuedAt()
	if iat == nil || time.Since(iat.Time) > 45*time.Second {
		t.Error("iat should be ~30 seconds in the past")
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

func TestGetInstallations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations" {
			t.Errorf("path = %s, want /app/installations", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization = %q, want Bearer prefix", auth)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 111, "account": map[string]string{"login": "org-a"}},
			{"id": 222, "account": map[string]string{"login": "org-b"}},
		})
	}))
	defer srv.Close()

	got, err := GetInstallations("fake-jwt", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("GetInstallations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != 111 || got[0].Account.Login != "org-a" {
		t.Errorf("got[0] = %+v, want id=111 login=org-a", got[0])
	}
	if got[1].ID != 222 || got[1].Account.Login != "org-b" {
		t.Errorf("got[1] = %+v, want id=222 login=org-b", got[1])
	}
}

func TestGetInstallations_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	got, err := GetInstallations("fake-jwt", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("GetInstallations: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestGetInstallations_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	_, err := GetInstallations("bad-jwt", WithBaseURL(srv.URL))
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want substring %q", err.Error(), "401")
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

		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q, want %q", got, "application/vnd.github+json")
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
			t.Errorf("X-GitHub-Api-Version = %q, want %q", got, "2022-11-28")
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization = %q, want Bearer prefix", auth)
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"token":      wantToken,
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
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

func TestGetInstallationToken_EmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"token":      "",
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		})
	}))
	defer srv.Close()

	_, err := GetInstallationToken("jwt", 1, WithBaseURL(srv.URL))
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !strings.Contains(err.Error(), "empty token") {
		t.Errorf("error = %q, want substring %q", err.Error(), "empty token")
	}
}
