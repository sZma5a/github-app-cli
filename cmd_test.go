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

func setupTestEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")
	return tmp
}

func runCmd(t *testing.T, args []string, input string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code = run(args, strings.NewReader(input), &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func TestRun_NoArgs(t *testing.T) {
	stdout, _, code := runCmd(t, []string{"gha"}, "")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("stdout = %q, want usage info", stdout)
	}
}

func TestRun_Version(t *testing.T) {
	stdout, _, code := runCmd(t, []string{"gha", "--version"}, "")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "gha") {
		t.Errorf("stdout = %q, want version string", stdout)
	}
}

func TestRun_VersionShort(t *testing.T) {
	stdout, _, code := runCmd(t, []string{"gha", "-v"}, "")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, version) {
		t.Errorf("stdout = %q, want %q", stdout, version)
	}
}

func TestRun_Help(t *testing.T) {
	stdout, _, code := runCmd(t, []string{"gha", "--help"}, "")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "gha configure") {
		t.Errorf("missing configure in help: %s", stdout)
	}
	if !strings.Contains(stdout, "gha <gh subcommand>") {
		t.Errorf("missing proxy usage in help: %s", stdout)
	}
}

func TestRun_HelpShort(t *testing.T) {
	stdout, _, code := runCmd(t, []string{"gha", "-h"}, "")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("stdout = %q, want usage info", stdout)
	}
}

func TestRun_HelpToStdout_ErrorsToStderr(t *testing.T) {
	stdout, stderr, _ := runCmd(t, []string{"gha", "--help"}, "")
	if !strings.Contains(stdout, "Usage:") {
		t.Error("help should go to stdout")
	}
	if strings.Contains(stderr, "Usage:") {
		t.Error("help should NOT go to stderr")
	}
}

func TestRun_Configure(t *testing.T) {
	setupTestEnv(t)

	keyPath := generateTestKeyFile(t)
	input := "12345\n67890\n" + keyPath + "\n"

	_, stderr, code := runCmd(t, []string{"gha", "configure"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
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
	if !strings.Contains(stderr, "Configuration saved") {
		t.Errorf("stderr = %q, want confirmation message", stderr)
	}
}

func TestRun_ConfigureInvalidAppID(t *testing.T) {
	setupTestEnv(t)

	_, stderr, code := runCmd(t, []string{"gha", "configure"}, "not-a-number\n")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid App ID") {
		t.Errorf("stderr = %q, want invalid App ID error", stderr)
	}
}

func TestRun_ConfigureNegativeAppID(t *testing.T) {
	setupTestEnv(t)

	_, stderr, code := runCmd(t, []string{"gha", "configure"}, "-5\n")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "positive integer") {
		t.Errorf("stderr = %q, want positive integer error", stderr)
	}
}

func TestRun_ConfigureEOF(t *testing.T) {
	setupTestEnv(t)

	_, stderr, code := runCmd(t, []string{"gha", "configure"}, "")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "error") {
		t.Errorf("stderr = %q, want error message", stderr)
	}
}

func TestRun_ConfigureMissingKeyFile(t *testing.T) {
	setupTestEnv(t)

	_, stderr, code := runCmd(t, []string{"gha", "configure"}, "1\n2\n/nonexistent/key.pem\n")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "private key file") {
		t.Errorf("stderr = %q, want file not found error", stderr)
	}
}

func TestRun_ConfigureKeyPathIsDirectory(t *testing.T) {
	setupTestEnv(t)

	dirPath := t.TempDir()
	_, stderr, code := runCmd(t, []string{"gha", "configure"}, "1\n2\n"+dirPath+"\n")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "not a regular file") {
		t.Errorf("stderr = %q, want not-a-file error", stderr)
	}
}

func TestRun_ConfigureEmptyKeyPath(t *testing.T) {
	setupTestEnv(t)

	_, stderr, code := runCmd(t, []string{"gha", "configure"}, "1\n2\n\n")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "empty") {
		t.Errorf("stderr = %q, want empty path error", stderr)
	}
}

func TestRun_ProxyWithoutConfig(t *testing.T) {
	setupTestEnv(t)

	_, stderr, code := runCmd(t, []string{"gha", "pr", "list"}, "")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "configuration not found") {
		t.Errorf("stderr = %q, want config not found error", stderr)
	}
}

func TestRun_ConfigureTildeExpansion(t *testing.T) {
	tmp := setupTestEnv(t)

	keyDir := filepath.Join(tmp, ".ssh")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(keyDir, "app.pem")
	writeTestKey(t, keyPath)

	_, _, code := runCmd(t, []string{"gha", "configure"}, "1\n2\n~/.ssh/app.pem\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
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
