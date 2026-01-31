# gha

[![CI](https://github.com/sZma5a/github-app-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/sZma5a/github-app-cli/actions/workflows/ci.yml)
[![Release](https://github.com/sZma5a/github-app-cli/actions/workflows/release.yml/badge.svg)](https://github.com/sZma5a/github-app-cli/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Proxy `gh` CLI commands with GitHub App authentication. Transparently generates an installation token from your GitHub App credentials and injects it into any `gh` command.

## Why?

GitHub Apps provide fine-grained, scoped permissions — ideal for CI/CD, automation, and bot workflows. But using them with the `gh` CLI requires manually generating JWTs, exchanging them for installation tokens, and passing them around. `gha` does all of that for you.

## Install

### From GitHub Releases

Download the latest binary from [Releases](https://github.com/sZma5a/github-app-cli/releases) and place it in your `PATH`.

```bash
# Example: Linux amd64
curl -sL https://github.com/sZma5a/github-app-cli/releases/latest/download/gha_linux_amd64.tar.gz | tar xz
sudo mv gha /usr/local/bin/
```

### From Source

```bash
go install github.com/sZma5a/github-app-cli@latest
```

## Setup

```bash
gha configure
```

You will be prompted for:

| Field | Description |
|---|---|
| **App ID** | Your GitHub App's ID (Settings → Developer settings → GitHub Apps) |
| **Installation ID** | The installation ID for the target org/repo |
| **Private Key Path** | Absolute path to the `.pem` private key file |

Configuration is saved to `~/.config/github-app-cli/config.yaml` (respects `XDG_CONFIG_HOME`).

## Usage

Use `gha` exactly like `gh` — all arguments are passed through:

```bash
gha pr list
gha issue create --title "Bug" --body "Details"
gha api repos/{owner}/{repo}
gha repo clone owner/repo
```

Under the hood, `gha`:

1. Reads your GitHub App credentials from the config
2. Generates a short-lived JWT (RS256, 10-minute expiry)
3. Exchanges the JWT for an installation access token via the GitHub API
4. Sets `GH_TOKEN` and execs `gh` with your arguments

## How It Works

```
gha pr list
  │
  ├─ Load config (~/.config/github-app-cli/config.yaml)
  ├─ Read private key (.pem)
  ├─ Generate JWT (iat: now-30s, exp: now+10m, iss: app_id)
  ├─ POST /app/installations/{id}/access_tokens → installation token
  └─ exec gh pr list  (with GH_TOKEN=<installation_token>)
```

## License

MIT
