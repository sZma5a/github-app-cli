package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, tagName string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{"tag_name": tagName})
	}))
}

func TestCheck_NewerVersionAvailable(t *testing.T) {
	srv := newTestServer(t, "v1.2.0", http.StatusOK)
	defer srv.Close()

	dir := t.TempDir()
	result := Check("1.0.0", dir, WithBaseURL(srv.URL))
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Latest != "1.2.0" {
		t.Errorf("Latest = %q, want %q", result.Latest, "1.2.0")
	}
	if result.Current != "1.0.0" {
		t.Errorf("Current = %q, want %q", result.Current, "1.0.0")
	}
}

func TestCheck_AlreadyUpToDate(t *testing.T) {
	srv := newTestServer(t, "v1.0.0", http.StatusOK)
	defer srv.Close()

	dir := t.TempDir()
	result := Check("1.0.0", dir, WithBaseURL(srv.URL))
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestCheck_CurrentIsNewer(t *testing.T) {
	srv := newTestServer(t, "v0.9.0", http.StatusOK)
	defer srv.Close()

	dir := t.TempDir()
	result := Check("1.0.0", dir, WithBaseURL(srv.URL))
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestCheck_DevVersion(t *testing.T) {
	result := Check("dev", t.TempDir())
	if result != nil {
		t.Errorf("expected nil for dev version, got %+v", result)
	}
}

func TestCheck_EmptyVersion(t *testing.T) {
	result := Check("", t.TempDir())
	if result != nil {
		t.Errorf("expected nil for empty version, got %+v", result)
	}
}

func TestCheck_APIError(t *testing.T) {
	srv := newTestServer(t, "", http.StatusInternalServerError)
	defer srv.Close()

	dir := t.TempDir()
	result := Check("1.0.0", dir, WithBaseURL(srv.URL))
	if result != nil {
		t.Errorf("expected nil on API error, got %+v", result)
	}
}

func TestCheck_UsesCache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer srv.Close()

	dir := t.TempDir()

	result1 := Check("1.0.0", dir, WithBaseURL(srv.URL))
	if result1 == nil {
		t.Fatal("first check: expected non-nil")
	}

	result2 := Check("1.0.0", dir, WithBaseURL(srv.URL))
	if result2 == nil {
		t.Fatal("second check: expected non-nil (from cache)")
	}

	if callCount != 1 {
		t.Errorf("API called %d times, want 1 (second should use cache)", callCount)
	}
}

func TestCheck_StaleCache(t *testing.T) {
	srv := newTestServer(t, "v3.0.0", http.StatusOK)
	defer srv.Close()

	dir := t.TempDir()
	stale := &state{
		LatestVersion: "2.0.0",
		CheckedAt:     time.Now().Add(-25 * time.Hour),
	}
	data, _ := json.Marshal(stale)
	os.WriteFile(filepath.Join(dir, cacheFile), data, 0o600)

	result := Check("1.0.0", dir, WithBaseURL(srv.URL))
	if result == nil {
		t.Fatal("expected non-nil result after stale cache refresh")
	}
	if result.Latest != "3.0.0" {
		t.Errorf("Latest = %q, want %q (refreshed from API)", result.Latest, "3.0.0")
	}
}

func TestCheck_FreshCacheNoUpdate(t *testing.T) {
	dir := t.TempDir()
	fresh := &state{
		LatestVersion: "1.0.0",
		CheckedAt:     time.Now(),
	}
	data, _ := json.Marshal(fresh)
	os.WriteFile(filepath.Join(dir, cacheFile), data, 0o600)

	result := Check("1.0.0", dir)
	if result != nil {
		t.Errorf("expected nil for same version in fresh cache, got %+v", result)
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.0", true},
		{"2.0.0", "1.9.9", true},
		{"1.0.0", "1.0.0", false},
		{"0.9.0", "1.0.0", false},
		{"v1.0.1", "v1.0.0", true},
		{"1.0.0", "1.0.1", false},
		{"0.0.2", "0.0.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.latest+"_vs_"+tt.current, func(t *testing.T) {
			got := isNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func TestFormatNotice(t *testing.T) {
	r := &Result{Latest: "2.0.0", Current: "1.0.0"}
	notice := FormatNotice(r)
	if !strings.Contains(notice, "v1.0.0") || !strings.Contains(notice, "v2.0.0") {
		t.Errorf("notice = %q, want both versions", notice)
	}
	if !strings.Contains(notice, "brew upgrade") {
		t.Errorf("notice = %q, want brew upgrade instruction", notice)
	}
}
