package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
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
			wantErr: "app_id must be a positive integer",
		},
		{
			name:    "negative app_id",
			yaml:    "app_id: -1\ninstallation_id: 1\nprivate_key_path: /tmp/k.pem\n",
			wantErr: "app_id must be a positive integer",
		},
		{
			name:    "missing installation_id",
			yaml:    "app_id: 1\nprivate_key_path: /tmp/k.pem\n",
			wantErr: "installation_id must be a positive integer",
		},
		{
			name:    "negative installation_id",
			yaml:    "app_id: 1\ninstallation_id: -5\nprivate_key_path: /tmp/k.pem\n",
			wantErr: "installation_id must be a positive integer",
		},
		{
			name:    "missing private_key_path",
			yaml:    "app_id: 1\ninstallation_id: 1\n",
			wantErr: "private_key_path is required",
		},
		{
			name:    "whitespace-only private_key_path",
			yaml:    "app_id: 1\ninstallation_id: 1\nprivate_key_path: \"   \"\n",
			wantErr: "private_key_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)

			dir := filepath.Join(tmp, ".config", configDir)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, configFile), []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", configDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(":::invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("error = %q, want substring %q", err.Error(), "parsing config")
	}
}

func TestLoad_UnknownField(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".config", configDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	yml := "app_id: 1\ninstallation_id: 1\nprivate_key_path: /tmp/k.pem\ntypo_field: oops\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unknown field")
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

	configPath := filepath.Join(tmp, ".config", configDir, configFile)
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file permissions = %o, want 0600", perm)
	}

	dirPath := filepath.Join(tmp, ".config", configDir)
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir permissions = %o, want 0700", perm)
	}
}

func TestSave_FixesExistingPermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	configPath := filepath.Join(tmp, ".config", configDir, configFile)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		AppID:          1,
		InstallationID: 2,
		PrivateKeyPath: "/tmp/k.pem",
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file permissions after Save = %o, want 0600", perm)
	}
}

func TestSave_NilConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	err := Save(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(tmp, ".config", configDir)
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}

func TestDir_XDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(tmp, configDir)
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}
