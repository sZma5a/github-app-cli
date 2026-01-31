package proxy

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRun_InvokesGhWithToken(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not found, skipping integration test")
	}

	out, err := RunCapture([]string{"--version"}, "ghs_dummy_token")
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if !strings.Contains(out, "gh version") {
		t.Errorf("output = %q, want substring %q", out, "gh version")
	}
}

func TestRun_TokenSetInEnv(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not found, skipping integration test")
	}

	token := "ghs_test_token_verify"

	script := `#!/bin/sh
echo "GH_TOKEN=$GH_TOKEN"
`
	dir := t.TempDir()
	scriptPath := dir + "/gh"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+origPath)

	out, err := RunCapture([]string{}, token)
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if !strings.Contains(out, "GH_TOKEN="+token) {
		t.Errorf("output = %q, want GH_TOKEN=%s", out, token)
	}
}

func TestRun_GhNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := RunCapture([]string{"--version"}, "token")
	if err == nil {
		t.Fatal("expected error when gh not in PATH")
	}
	if !strings.Contains(err.Error(), "gh") {
		t.Errorf("error = %q, want mention of gh", err.Error())
	}
}
