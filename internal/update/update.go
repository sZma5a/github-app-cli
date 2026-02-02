package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	cacheFile     = "update-check.json"
	checkInterval = 24 * time.Hour
	httpTimeout   = 3 * time.Second
	maxResponse   = 1 << 20
	releaseURL    = "https://api.github.com/repos/haribote-lab/github-app-cli/releases/latest"
)

type options struct {
	baseURL string
}

// Option configures update check behaviour.
type Option func(*options)

// WithBaseURL overrides the GitHub API release URL (used for testing).
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

func buildOpts(opts []Option) options {
	o := options{baseURL: releaseURL}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

type state struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

// Result holds the latest version info when an update is available.
type Result struct {
	Latest  string
	Current string
}

// Check returns non-nil Result if a newer version is available.
// It caches the result for 24 hours. Returns nil on any error or if up-to-date.
func Check(currentVersion, cacheDir string, opts ...Option) *Result {
	if currentVersion == "" || currentVersion == "dev" {
		return nil
	}

	cachePath := filepath.Join(cacheDir, cacheFile)
	cached := readCache(cachePath)

	if cached != nil && time.Since(cached.CheckedAt) < checkInterval {
		if isNewer(cached.LatestVersion, currentVersion) {
			return &Result{Latest: cached.LatestVersion, Current: currentVersion}
		}
		return nil
	}

	o := buildOpts(opts)
	latest := fetchLatestVersion(o.baseURL)
	if latest == "" {
		return nil
	}

	writeCache(cachePath, &state{LatestVersion: latest, CheckedAt: time.Now()})

	if isNewer(latest, currentVersion) {
		return &Result{Latest: latest, Current: currentVersion}
	}
	return nil
}

func fetchLatestVersion(url string) string {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return ""
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return ""
	}

	return strings.TrimPrefix(release.TagName, "v")
}

func readCache(path string) *state {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

func writeCache(path string, s *state) {
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func isNewer(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")

	lParts := strings.Split(latest, ".")
	cParts := strings.Split(current, ".")

	for i := 0; i < 3; i++ {
		l := part(lParts, i)
		c := part(cParts, i)
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

func part(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, _ := strconv.Atoi(parts[i])
	return n
}

// FormatNotice returns the update notification message.
func FormatNotice(r *Result) string {
	return fmt.Sprintf(
		"A new version of gha is available: v%s → v%s\nRun `brew upgrade gha` or visit https://github.com/haribote-lab/github-app-cli/releases\n",
		r.Current, r.Latest,
	)
}
