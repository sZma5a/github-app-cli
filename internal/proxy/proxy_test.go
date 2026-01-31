package proxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeGh(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunCapture_InvokesGhWithToken(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not found")
	}

	out, err := RunCapture([]string{"--version"}, "ghs_dummy_token")
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if !strings.Contains(out, "gh version") {
		t.Errorf("output = %q, want substring %q", out, "gh version")
	}
}

func TestRunCapture_TokenSetInEnv(t *testing.T) {
	token := "ghs_test_token_verify"
	dir := writeFakeGh(t, "#!/bin/sh\necho \"GH_TOKEN=$GH_TOKEN\"\n")
	t.Setenv("PATH", dir)

	out, err := RunCapture([]string{}, token)
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if !strings.Contains(out, "GH_TOKEN="+token) {
		t.Errorf("output = %q, want GH_TOKEN=%s", out, token)
	}
}

func TestRunCapture_GithubTokenAlsoFiltered(t *testing.T) {
	dir := writeFakeGh(t, "#!/bin/sh\necho \"GH=$GH_TOKEN GITHUB=$GITHUB_TOKEN\"\n")
	t.Setenv("PATH", dir)
	t.Setenv("GITHUB_TOKEN", "should_be_removed")

	out, err := RunCapture([]string{}, "app_token")
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if strings.Contains(out, "should_be_removed") {
		t.Errorf("GITHUB_TOKEN was not filtered: %s", out)
	}
	if !strings.Contains(out, "GH=app_token") {
		t.Errorf("GH_TOKEN not set correctly: %s", out)
	}
}

func TestRunCapture_ExistingGhTokenOverridden(t *testing.T) {
	dir := writeFakeGh(t, "#!/bin/sh\necho \"GH_TOKEN=$GH_TOKEN\"\n")
	t.Setenv("PATH", dir)
	t.Setenv("GH_TOKEN", "old_token_should_be_replaced")

	out, err := RunCapture([]string{}, "new_app_token")
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if strings.Contains(out, "old_token_should_be_replaced") {
		t.Errorf("old GH_TOKEN not overridden: %s", out)
	}
	if !strings.Contains(out, "GH_TOKEN=new_app_token") {
		t.Errorf("new token not set: %s", out)
	}
}

func TestRunCapture_EmptyToken(t *testing.T) {
	_, err := RunCapture([]string{"--version"}, "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want mention of empty", err.Error())
	}
}

func TestRunCapture_WhitespaceOnlyToken(t *testing.T) {
	_, err := RunCapture([]string{"--version"}, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only token")
	}
}

func TestRunCapture_GhNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := RunCapture([]string{"--version"}, "token")
	if err == nil {
		t.Fatal("expected error when gh not in PATH")
	}
	if !strings.Contains(err.Error(), "gh") {
		t.Errorf("error = %q, want mention of gh", err.Error())
	}
}

func TestRunCapture_ArgsPassedThrough(t *testing.T) {
	dir := writeFakeGh(t, "#!/bin/sh\necho \"ARGS=$*\"\n")
	t.Setenv("PATH", dir)

	out, err := RunCapture([]string{"pr", "list", "--repo", "org/repo"}, "tok")
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if !strings.Contains(out, "ARGS=pr list --repo org/repo") {
		t.Errorf("args not passed through: %s", out)
	}
}

func TestRunCapture_NonZeroExitCode(t *testing.T) {
	dir := writeFakeGh(t, "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir)

	_, err := RunCapture([]string{}, "tok")
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestFilterEnv(t *testing.T) {
	env := []string{
		"HOME=/home/user",
		"GH_TOKEN=old",
		"GITHUB_TOKEN=old2",
		"PATH=/usr/bin",
		"GH_TOKEN_EXTRA=keep",
	}

	got := filterEnv(env, "GH_TOKEN", "GITHUB_TOKEN")

	for _, e := range got {
		if strings.HasPrefix(e, "GH_TOKEN=") || strings.HasPrefix(e, "GITHUB_TOKEN=") {
			t.Errorf("should have been filtered: %s", e)
		}
	}

	found := map[string]bool{}
	for _, e := range got {
		found[e] = true
	}
	if !found["HOME=/home/user"] {
		t.Error("HOME should be preserved")
	}
	if !found["PATH=/usr/bin"] {
		t.Error("PATH should be preserved")
	}
	if !found["GH_TOKEN_EXTRA=keep"] {
		t.Error("GH_TOKEN_EXTRA should be preserved (prefix match only)")
	}
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid", "ghs_abc123", false},
		{"empty", "", true},
		{"whitespace", "   ", true},
		{"tab", "\t", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateToken(%q) err = %v, wantErr = %v", tt.token, err, tt.wantErr)
			}
		})
	}
}
