# Task Automation Scripts

This directory contains helper scripts to automate Linear + GitHub flow for Codex.

## Scripts

- `check-deps.sh`: validates required CLI tools are installed.
- `start-task.sh`: moves a Linear issue to `In Progress`, authenticates with GitHub App, creates a branch.
- `finish-task.sh`: runs pre-PR checks (`gofmt` + `golangci-lint` on changed modules), pushes branch, opens PR, comments PR URL in Linear.

## Required environment variables

- `LINEAR_API_KEY`
- `GITHUB_APP_ID`
- `GITHUB_APP_INSTALLATION_ID`
- `GITHUB_APP_PRIVATE_KEY` (PEM multiline) or `GITHUB_APP_PRIVATE_KEY_B64` (base64)

## Required commands

- `jq`
- `openssl`
- `git`
- `curl`
- `gofmt`
- `golangci-lint`

Optional:

- `gh` (if missing, scripts use GitHub REST API fallback)

## Pre-PR validation behavior

`finish-task.sh` enforces local validation before creating PR:

- Runs `gofmt -w` on changed `.go` files.
- Runs `golangci-lint run ./...` only in Go module directories affected by changes.
- Aborts PR creation if formatting/lint fails.

## Usage

Load environment variables (example using `.env`):

```bash
set -a; source .env; set +a
```

Validate dependencies:

```bash
bash scripts/check-deps.sh
```

Start task:

```bash
bash scripts/start-task.sh DEVENG-5 go-service-template
```

Finish task:

```bash
bash scripts/finish-task.sh DEVENG-5 "criar template base de servico Go"
```
