package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/haribote-lab/github-app-cli/internal/auth"
	"github.com/haribote-lab/github-app-cli/internal/config"
	"github.com/haribote-lab/github-app-cli/internal/proxy"
)

// Set via -ldflags "-X main.version=..."
var version = "dev"

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (exitCode int) {
	if len(args) < 2 {
		printUsage(stdout)
		return 1
	}

	switch args[1] {
	case "configure":
		if err := runConfigure(stdin, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "--version", "-v":
		fmt.Fprintf(stdout, "gha %s\n", version)
	case "--help", "-h":
		printUsage(stdout)
	default:
		if err := runProxy(args[1:]); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `gha - proxy gh commands with GitHub App authentication

Usage:
  gha configure            Set up GitHub App credentials
  gha <gh subcommand>      Proxy any gh command with App token
  gha --version            Show version
  gha --help               Show this help

Examples:
  gha configure
  gha pr list
  gha api repos/{owner}/{repo}
  gha issue create --title "Bug" --body "Details"

Configuration is stored in ~/.config/github-app-cli/config.yaml
`)
}

func runConfigure(stdin io.Reader, stderr io.Writer) error {
	reader := bufio.NewReader(stdin)

	appIDStr, err := prompt(reader, stderr, "GitHub App ID: ")
	if err != nil {
		return fmt.Errorf("reading App ID: %w", err)
	}
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil || appID <= 0 {
		return fmt.Errorf("invalid App ID %q: must be a positive integer", appIDStr)
	}

	installIDStr, err := prompt(reader, stderr, "Installation ID (empty to auto-detect): ")
	if err != nil {
		return fmt.Errorf("reading Installation ID: %w", err)
	}
	var installID int64
	if installIDStr != "" {
		installID, err = strconv.ParseInt(installIDStr, 10, 64)
		if err != nil || installID <= 0 {
			return fmt.Errorf("invalid Installation ID %q: must be a positive integer", installIDStr)
		}
	}

	keyPath, err := prompt(reader, stderr, "Private Key Path: ")
	if err != nil {
		return fmt.Errorf("reading Private Key Path: %w", err)
	}
	if keyPath == "" {
		return fmt.Errorf("private key path must not be empty")
	}

	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			keyPath = filepath.Join(home, keyPath[2:])
		}
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		return fmt.Errorf("private key file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("private key path is not a regular file: %s", keyPath)
	}

	cfg := &config.Config{
		AppID:          appID,
		InstallationID: installID,
		PrivateKeyPath: keyPath,
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	dir, _ := config.Dir()
	fmt.Fprintf(stderr, "Configuration saved to %s/config.yaml\n", dir)
	return nil
}

func prompt(reader *bufio.Reader, w io.Writer, msg string) (string, error) {
	fmt.Fprint(w, msg)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("unexpected end of input")
	}
	return strings.TrimSpace(line), nil
}

func runProxy(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	jwtToken, err := auth.GenerateJWT(cfg.AppID, cfg.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("generating JWT: %w", err)
	}

	installationID := cfg.InstallationID
	if installationID == 0 {
		installationID, err = resolveInstallationID(jwtToken)
		if err != nil {
			return err
		}
	}

	installToken, err := auth.GetInstallationToken(jwtToken, installationID)
	if err != nil {
		return fmt.Errorf("getting installation token: %w", err)
	}

	return proxy.Exec(args, installToken)
}

func resolveInstallationID(jwtToken string) (int64, error) {
	installations, err := auth.GetInstallations(jwtToken)
	if err != nil {
		return 0, fmt.Errorf("listing installations: %w", err)
	}

	switch len(installations) {
	case 0:
		return 0, fmt.Errorf("no installations found for this GitHub App")
	case 1:
		return installations[0].ID, nil
	default:
		lines := make([]string, 0, len(installations))
		for _, inst := range installations {
			lines = append(lines, fmt.Sprintf("  %d (%s)", inst.ID, inst.Account.Login))
		}
		return 0, fmt.Errorf("multiple installations found, set installation_id in config:\n%s", strings.Join(lines, "\n"))
	}
}
