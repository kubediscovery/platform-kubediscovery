#!/usr/bin/env bash
set -euo pipefail

required_commands=(gh jq openssl git curl)
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
