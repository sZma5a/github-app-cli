package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var errEmptyToken = fmt.Errorf("token must not be empty")

// GhBinary is the name of the gh CLI binary to look up in PATH.
const GhBinary = "gh"

func resolveGh() (string, error) {
	p, err := exec.LookPath(GhBinary)
	if err != nil {
		return "", fmt.Errorf("gh CLI not found in PATH - install it from https://cli.github.com: %w", err)
	}
	return p, nil
}

func buildEnv(token string) []string {
	env := filterEnv(os.Environ(), "GH_TOKEN", "GITHUB_TOKEN")
	return append(env, "GH_TOKEN="+token)
}

func validateToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return errEmptyToken
	}
	return nil
}

// RunCapture runs gh as a child process and returns combined output.
// Intended for testing; production code uses Exec.
func RunCapture(args []string, token string) (string, error) {
	if err := validateToken(token); err != nil {
		return "", err
	}

	ghPath, err := resolveGh()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(ghPath, args...)
	cmd.Env = buildEnv(token)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func filterEnv(env []string, keys ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, key := range keys {
			if strings.HasPrefix(e, key+"=") {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
