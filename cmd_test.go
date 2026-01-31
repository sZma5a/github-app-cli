package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sZma5a/github-app-cli/internal/config"
)

func TestRun_NoArgs(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"gha"}, strings.NewReader(""), &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr = %q, want usage info", stderr.String())
	}
}

func TestRun_Version(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"gha", "--version"}, strings.NewReader(""), &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "gha") {
		t.Errorf("stderr = %q, want version string", stderr.String())
	}
}

func TestRun_VersionShort(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"gha", "-v"}, strings.NewReader(""), &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), version) {
		t.Errorf("stderr = %q, want %q", stderr.String(), version)
	}
}

func TestRun_Help(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"gha", "--help"}, strings.NewReader(""), &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	out := stderr.String()
	if !strings.Contains(out, "gha configure") {
		t.Errorf("missing configure in help: %s", out)
	}
	if !strings.Contains(out, "gha <gh subcommand>") {
		t.Errorf("missing proxy usage in help: %s", out)
	}
}

func TestRun_HelpShort(t *testing.T) {
	var stderr bytes.Buffer
	code := run([]string{"gha", "-h"}, strings.NewReader(""), &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr = %q, want usage info", stderr.String())
	}
}

func TestRun_Configure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	keyPath := generateTestKeyFile(t)

	input := strings.Join([]string{
		"12345",
		"67890",
		keyPath,
	}, "\n") + "\n"

	var stderr bytes.Buffer
	code := run([]string{"gha", "configure"}, strings.NewReader(input), &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.AppID != 12345 {
		t.Errorf("AppID = %d, want 12345", cfg.AppID)
	}
	if cfg.InstallationID != 67890 {
		t.Errorf("InstallationID = %d, want 67890", cfg.InstallationID)
	}
	if cfg.PrivateKeyPath != keyPath {
		t.Errorf("PrivateKeyPath = %q, want %q", cfg.PrivateKeyPath, keyPath)
	}
}

func TestRun_ConfigureInvalidAppID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	input := "not-a-number\n"
	var stderr bytes.Buffer
	code := run([]string{"gha", "configure"}, strings.NewReader(input), &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "invalid App ID") {
		t.Errorf("stderr = %q, want invalid App ID error", stderr.String())
	}
}

func TestRun_ConfigureMissingKeyFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	input := "1\n2\n/nonexistent/key.pem\n"
	var stderr bytes.Buffer
	code := run([]string{"gha", "configure"}, strings.NewReader(input), &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("stderr = %q, want file not found error", stderr.String())
	}
}

func TestRun_ProxyWithoutConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	var stderr bytes.Buffer
	code := run([]string{"gha", "pr", "list"}, strings.NewReader(""), &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "configuration not found") {
		t.Errorf("stderr = %q, want config not found error", stderr.String())
	}
}

func TestRun_ConfigureTildeExpansion(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	keyDir := filepath.Join(tmp, ".ssh")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(keyDir, "app.pem")
	writeTestKey(t, keyPath)

	input := "1\n2\n~/.ssh/app.pem\n"
	var stderr bytes.Buffer
	code := run([]string{"gha", "configure"}, strings.NewReader(input), &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(cfg.PrivateKeyPath) {
		t.Errorf("PrivateKeyPath = %q, want absolute path", cfg.PrivateKeyPath)
	}
}

func generateTestKeyFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-key.pem")
	writeTestKey(t, path)
	return path
}

func writeTestKey(t *testing.T, path string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pemData, 0o600); err != nil {
		t.Fatal(err)
	}
}
