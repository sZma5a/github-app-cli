package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sZma5a/github-app-cli/internal/auth"
	"github.com/sZma5a/github-app-cli/internal/config"
	"github.com/sZma5a/github-app-cli/internal/proxy"
)

const version = "0.1.0"

func run(args []string, stdin io.Reader, stderr io.Writer) (exitCode int) {
	if len(args) < 2 {
		printUsage(stderr)
		return 1
	}

	switch args[1] {
	case "configure":
		if err := runConfigure(stdin, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "--version", "-v":
		fmt.Fprintf(stderr, "gha %s\n", version)
	case "--help", "-h":
		printUsage(stderr)
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

	fmt.Fprint(stderr, "GitHub App ID: ")
	appIDStr, _ := reader.ReadString('\n')
	appIDStr = strings.TrimSpace(appIDStr)
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid App ID %q: %w", appIDStr, err)
	}

	fmt.Fprint(stderr, "Installation ID: ")
	installIDStr, _ := reader.ReadString('\n')
	installIDStr = strings.TrimSpace(installIDStr)
	installID, err := strconv.ParseInt(installIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Installation ID %q: %w", installIDStr, err)
	}

	fmt.Fprint(stderr, "Private Key Path: ")
	keyPath, _ := reader.ReadString('\n')
	keyPath = strings.TrimSpace(keyPath)

	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			keyPath = filepath.Join(home, keyPath[2:])
		}
	}

	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("private key file not found: %s", keyPath)
	}

	cfg := &config.Config{
		AppID:          appID,
		InstallationID: installID,
		PrivateKeyPath: keyPath,
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	dir, _ := config.Dir()
	fmt.Fprintf(stderr, "Configuration saved to %s/config.yaml\n", dir)
	return nil
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

	installToken, err := auth.GetInstallationToken(jwtToken, cfg.InstallationID)
	if err != nil {
		return fmt.Errorf("getting installation token: %w", err)
	}

	return proxy.Exec(args, installToken)
}
