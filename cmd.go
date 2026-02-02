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
	"github.com/haribote-lab/github-app-cli/internal/update"
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
		checkForUpdate(stderr)
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
  gha configure                          Set up GitHub App credentials
  gha [flags] <gh subcommand>            Proxy any gh command with App token
  gha --version                          Show version
  gha --help                             Show this help

Flags:
  --installation-id <id>    Use specific installation (overrides config & env)
  --org <name>              Resolve installation by org/user name

Environment Variables:
  GHA_INSTALLATION_ID       Installation ID (overrides config, overridden by flags)
  GHA_ORG                   Org/user name to resolve (overrides config, overridden by flags)

Resolution Order (highest to lowest precedence):
  1. --installation-id / --org flag
  2. GHA_INSTALLATION_ID / GHA_ORG environment variable
  3. installation_id in config.yaml
  4. Auto-detect (works only with single installation)

Examples:
  gha configure
  gha pr list
  gha --org myorg repo list
  gha --installation-id 12345 issue create --title "Bug"
  GHA_ORG=myorg gha pr list

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

func checkForUpdate(w io.Writer) {
	dir, err := config.Dir()
	if err != nil {
		return
	}
	if result := update.Check(version, dir); result != nil {
		fmt.Fprint(w, update.FormatNotice(result))
	}
}

// installationOverride holds per-command installation selection parsed from flags or env vars.
type installationOverride struct {
	id  int64
	org string
}

// parseInstallationFlags extracts --installation-id and --org from args,
// returning the override and the remaining args to pass to gh.
func parseInstallationFlags(args []string) (installationOverride, []string) {
	var override installationOverride
	var remaining []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--installation-id" && i+1 < len(args):
			if id, err := strconv.ParseInt(args[i+1], 10, 64); err == nil && id > 0 {
				override.id = id
			}
			i++ // skip the value
		case strings.HasPrefix(args[i], "--installation-id="):
			val := strings.TrimPrefix(args[i], "--installation-id=")
			if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
				override.id = id
			}
		case args[i] == "--org" && i+1 < len(args):
			override.org = args[i+1]
			i++ // skip the value
		case strings.HasPrefix(args[i], "--org="):
			override.org = strings.TrimPrefix(args[i], "--org=")
		default:
			remaining = append(remaining, args[i])
		}
	}

	return override, remaining
}

// resolveInstallationFromEnv reads GHA_INSTALLATION_ID and GHA_ORG environment variables.
func resolveInstallationFromEnv() installationOverride {
	var override installationOverride
	if envID := os.Getenv("GHA_INSTALLATION_ID"); envID != "" {
		if id, err := strconv.ParseInt(envID, 10, 64); err == nil && id > 0 {
			override.id = id
		}
	}
	if envOrg := os.Getenv("GHA_ORG"); envOrg != "" {
		override.org = envOrg
	}
	return override
}

// resolveInstallationByOrg finds the installation ID for a given org/user login.
func resolveInstallationByOrg(jwtToken string, org string, opts ...auth.Option) (int64, error) {
	installations, err := auth.GetInstallations(jwtToken, opts...)
	if err != nil {
		return 0, fmt.Errorf("listing installations: %w", err)
	}

	for _, inst := range installations {
		if strings.EqualFold(inst.Account.Login, org) {
			return inst.ID, nil
		}
	}

	available := make([]string, 0, len(installations))
	for _, inst := range installations {
		available = append(available, fmt.Sprintf("  %d (%s)", inst.ID, inst.Account.Login))
	}
	return 0, fmt.Errorf("no installation found for org %q, available:\n%s", org, strings.Join(available, "\n"))
}

func runProxy(args []string) error {
	// 1. Parse flags (highest precedence)
	flagOverride, ghArgs := parseInstallationFlags(args)

	// 2. Read env vars (middle precedence)
	envOverride := resolveInstallationFromEnv()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	jwtToken, err := auth.GenerateJWT(cfg.AppID, cfg.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("generating JWT: %w", err)
	}

	// 3. Resolve installation ID with precedence: flag > env > config > auto-detect
	installationID, err := resolveInstallation(jwtToken, flagOverride, envOverride, cfg.InstallationID)
	if err != nil {
		return err
	}

	installToken, err := auth.GetInstallationToken(jwtToken, installationID)
	if err != nil {
		return fmt.Errorf("getting installation token: %w", err)
	}

	return proxy.Exec(ghArgs, installToken)
}

// resolveInstallation determines the installation ID using the precedence chain:
// flag > env > config > auto-detect.
func resolveInstallation(jwtToken string, flag, env installationOverride, configID int64) (int64, error) {
	// Flag --installation-id takes highest precedence
	if flag.id > 0 {
		return flag.id, nil
	}
	// Flag --org
	if flag.org != "" {
		return resolveInstallationByOrg(jwtToken, flag.org)
	}
	// Env GHA_INSTALLATION_ID
	if env.id > 0 {
		return env.id, nil
	}
	// Env GHA_ORG
	if env.org != "" {
		return resolveInstallationByOrg(jwtToken, env.org)
	}
	// Config file
	if configID > 0 {
		return configID, nil
	}
	// Auto-detect
	return resolveInstallationID(jwtToken)
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
