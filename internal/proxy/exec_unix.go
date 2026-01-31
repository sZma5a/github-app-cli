//go:build !windows

package proxy

import (
	"syscall"
)

// Exec replaces the current process with gh, injecting the token via GH_TOKEN.
// Does not return on success.
func Exec(args []string, token string) error {
	if err := validateToken(token); err != nil {
		return err
	}

	ghPath, err := resolveGh()
	if err != nil {
		return err
	}

	env := buildEnv(token)
	return syscall.Exec(ghPath, append([]string{ghPath}, args...), env)
}
