//go:build windows

package proxy

import (
	"os"
	"os/exec"
)

// Exec runs gh as a child process on Windows (no syscall.Exec available).
// Forwards stdin/stdout/stderr and exits with gh's exit code.
func Exec(args []string, token string) error {
	if err := validateToken(token); err != nil {
		return err
	}

	ghPath, err := resolveGh()
	if err != nil {
		return err
	}

	cmd := exec.Command(ghPath, args...)
	cmd.Env = buildEnv(token)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.Exit(cmd.ProcessState.ExitCode())
	}
	os.Exit(0)
	return nil
}
