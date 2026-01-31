package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Exec replaces the current process with gh, injecting the token via GH_TOKEN.
// This function does not return on success (process is replaced).
func Exec(args []string, token string) error {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh CLI not found in PATH - install it from https://cli.github.com: %w", err)
	}

	env := filterEnv(os.Environ(), "GH_TOKEN")
	env = append(env, "GH_TOKEN="+token)

	return syscall.Exec(ghPath, append([]string{"gh"}, args...), env)
}

// RunCapture runs gh as a child process and returns combined output.
// Used for testing; production code uses Exec.
func RunCapture(args []string, token string) (string, error) {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh CLI not found in PATH - install it from https://cli.github.com: %w", err)
	}

	cmd := exec.Command(ghPath, args...)
	cmd.Env = filterEnv(os.Environ(), "GH_TOKEN")
	cmd.Env = append(cmd.Env, "GH_TOKEN="+token)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func filterEnv(env []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}
