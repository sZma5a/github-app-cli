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

	"github.com/haribote-lab/github-app-cli/internal/config"
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
	if !strings.Contains(stdout, "gha [flags] <gh subcommand>") {
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

func TestRun_ConfigureAutoDetect(t *testing.T) {
	setupTestEnv(t)

	keyPath := generateTestKeyFile(t)
	input := "12345\n\n" + keyPath + "\n"

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
	if cfg.InstallationID != 0 {
		t.Errorf("InstallationID = %d, want 0 (auto-detect)", cfg.InstallationID)
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

// --- Tests for parseInstallationFlags ---

func TestParseInstallationFlags_InstallationID(t *testing.T) {
	override, remaining := parseInstallationFlags([]string{"--installation-id", "12345", "pr", "list"})
	if override.id != 12345 {
		t.Errorf("id = %d, want 12345", override.id)
	}
	if len(remaining) != 2 || remaining[0] != "pr" || remaining[1] != "list" {
		t.Errorf("remaining = %v, want [pr list]", remaining)
	}
}

func TestParseInstallationFlags_InstallationIDEquals(t *testing.T) {
	override, remaining := parseInstallationFlags([]string{"--installation-id=12345", "pr", "list"})
	if override.id != 12345 {
		t.Errorf("id = %d, want 12345", override.id)
	}
	if len(remaining) != 2 || remaining[0] != "pr" || remaining[1] != "list" {
		t.Errorf("remaining = %v, want [pr list]", remaining)
	}
}

func TestParseInstallationFlags_Org(t *testing.T) {
	override, remaining := parseInstallationFlags([]string{"--org", "myorg", "repo", "list"})
	if override.org != "myorg" {
		t.Errorf("org = %q, want %q", override.org, "myorg")
	}
	if len(remaining) != 2 || remaining[0] != "repo" || remaining[1] != "list" {
		t.Errorf("remaining = %v, want [repo list]", remaining)
	}
}

func TestParseInstallationFlags_OrgEquals(t *testing.T) {
	override, remaining := parseInstallationFlags([]string{"--org=myorg", "repo", "list"})
	if override.org != "myorg" {
		t.Errorf("org = %q, want %q", override.org, "myorg")
	}
	if len(remaining) != 2 {
		t.Errorf("remaining = %v, want [repo list]", remaining)
	}
}

func TestParseInstallationFlags_IDTakesPrecedenceOverOrg(t *testing.T) {
	override, _ := parseInstallationFlags([]string{"--installation-id", "99", "--org", "myorg", "pr", "list"})
	if override.id != 99 {
		t.Errorf("id = %d, want 99", override.id)
	}
	if override.org != "myorg" {
		t.Errorf("org = %q, want %q", override.org, "myorg")
	}
}

func TestParseInstallationFlags_NoFlags(t *testing.T) {
	override, remaining := parseInstallationFlags([]string{"pr", "list", "--repo", "foo/bar"})
	if override.id != 0 {
		t.Errorf("id = %d, want 0", override.id)
	}
	if override.org != "" {
		t.Errorf("org = %q, want empty", override.org)
	}
	if len(remaining) != 4 {
		t.Errorf("remaining = %v, want [pr list --repo foo/bar]", remaining)
	}
}

func TestParseInstallationFlags_InvalidID(t *testing.T) {
	override, remaining := parseInstallationFlags([]string{"--installation-id", "notanumber", "pr", "list"})
	if override.id != 0 {
		t.Errorf("id = %d, want 0 (invalid input ignored)", override.id)
	}
	if len(remaining) != 2 {
		t.Errorf("remaining = %v, want [pr list]", remaining)
	}
}

func TestParseInstallationFlags_FlagAtEnd(t *testing.T) {
	override, remaining := parseInstallationFlags([]string{"pr", "list", "--installation-id"})
	if override.id != 0 {
		t.Errorf("id = %d, want 0", override.id)
	}
	if len(remaining) != 3 {
		t.Errorf("remaining = %v, want [pr list --installation-id]", remaining)
	}
}

// --- Tests for resolveInstallationFromEnv ---

func TestResolveInstallationFromEnv_ID(t *testing.T) {
	t.Setenv("GHA_INSTALLATION_ID", "54321")
	t.Setenv("GHA_ORG", "")
	override := resolveInstallationFromEnv()
	if override.id != 54321 {
		t.Errorf("id = %d, want 54321", override.id)
	}
}

func TestResolveInstallationFromEnv_Org(t *testing.T) {
	t.Setenv("GHA_INSTALLATION_ID", "")
	t.Setenv("GHA_ORG", "testorg")
	override := resolveInstallationFromEnv()
	if override.org != "testorg" {
		t.Errorf("org = %q, want %q", override.org, "testorg")
	}
}

func TestResolveInstallationFromEnv_InvalidID(t *testing.T) {
	t.Setenv("GHA_INSTALLATION_ID", "bad")
	t.Setenv("GHA_ORG", "")
	override := resolveInstallationFromEnv()
	if override.id != 0 {
		t.Errorf("id = %d, want 0 (invalid env ignored)", override.id)
	}
}

func TestResolveInstallationFromEnv_Empty(t *testing.T) {
	t.Setenv("GHA_INSTALLATION_ID", "")
	t.Setenv("GHA_ORG", "")
	override := resolveInstallationFromEnv()
	if override.id != 0 || override.org != "" {
		t.Errorf("expected empty override, got id=%d org=%q", override.id, override.org)
	}
}

// --- Tests for resolveInstallation precedence ---

func TestResolveInstallation_FlagIDWins(t *testing.T) {
	flag := installationOverride{id: 100}
	env := installationOverride{id: 200}
	configID := int64(300)

	id, err := resolveInstallation("fake-jwt", flag, env, configID)
	if err != nil {
		t.Fatal(err)
	}
	if id != 100 {
		t.Errorf("id = %d, want 100 (flag should win)", id)
	}
}

func TestResolveInstallation_EnvIDWins(t *testing.T) {
	flag := installationOverride{}
	env := installationOverride{id: 200}
	configID := int64(300)

	id, err := resolveInstallation("fake-jwt", flag, env, configID)
	if err != nil {
		t.Fatal(err)
	}
	if id != 200 {
		t.Errorf("id = %d, want 200 (env should win over config)", id)
	}
}

func TestResolveInstallation_ConfigIDFallback(t *testing.T) {
	flag := installationOverride{}
	env := installationOverride{}
	configID := int64(300)

	id, err := resolveInstallation("fake-jwt", flag, env, configID)
	if err != nil {
		t.Fatal(err)
	}
	if id != 300 {
		t.Errorf("id = %d, want 300 (config fallback)", id)
	}
}

// --- Tests for help text content ---

func TestRun_HelpContainsFlags(t *testing.T) {
	stdout, _, code := runCmd(t, []string{"gha", "--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"--installation-id",
		"--org",
		"GHA_INSTALLATION_ID",
		"GHA_ORG",
		"Resolution Order",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q", want)
		}
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
