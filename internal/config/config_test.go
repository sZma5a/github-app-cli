package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	// Use a temp dir as config root.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	want := &Config{
		AppID:          12345,
		InstallationID: 67890,
		PrivateKeyPath: "/tmp/test-key.pem",
	}

	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.AppID != want.AppID {
		t.Errorf("AppID = %d, want %d", got.AppID, want.AppID)
	}
	if got.InstallationID != want.InstallationID {
		t.Errorf("InstallationID = %d, want %d", got.InstallationID, want.InstallationID)
	}
	if got.PrivateKeyPath != want.PrivateKeyPath {
		t.Errorf("PrivateKeyPath = %q, want %q", got.PrivateKeyPath, want.PrivateKeyPath)
	}
}

func TestLoad_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	_, err := Load()
	if err == nil {
		t.Fatal("Load: expected error for missing config, got nil")
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing app_id",
			yaml:    "installation_id: 1\nprivate_key_path: /tmp/k.pem\n",
			wantErr: "app_id is required",
		},
		{
			name:    "missing installation_id",
			yaml:    "app_id: 1\nprivate_key_path: /tmp/k.pem\n",
			wantErr: "installation_id is required",
		},
		{
			name:    "missing private_key_path",
			yaml:    "app_id: 1\ninstallation_id: 1\n",
			wantErr: "private_key_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)

			dir := filepath.Join(tmp, ".config", "github-app-cli")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &Config{
		AppID:          1,
		InstallationID: 2,
		PrivateKeyPath: "/tmp/k.pem",
	}

	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmp, ".config", "github-app-cli", "config.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Config file should not be world-readable.
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("config file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(tmp, ".config", "github-app-cli")
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
