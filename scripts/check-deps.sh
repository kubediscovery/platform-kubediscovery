#!/usr/bin/env bash
set -euo pipefail

required_commands=(jq openssl git curl gofmt golangci-lint)
optional_commands=(gh)
missing=()

for cmd in "${required_commands[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    missing+=("$cmd")
  fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "Missing required commands: ${missing[*]}" >&2
  echo "Install them in the Cloud Agent setup script before running task automation." >&2
  exit 1
fi

echo "All required commands are available: ${required_commands[*]}"

for cmd in "${optional_commands[@]}"; do
  if command -v "$cmd" >/dev/null 2>&1; then
    echo "Optional command available: $cmd"
  else
    echo "Optional command missing: $cmd (script will use GitHub API fallback)"
  fi
done
