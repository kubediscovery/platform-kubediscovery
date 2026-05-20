#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./finish-task.sh DEVENG-42 "implement grpc timeout"
#
# Required env vars:
#   GITHUB_APP_ID
#   GITHUB_APP_INSTALLATION_ID
#   GITHUB_APP_PRIVATE_KEY or GITHUB_APP_PRIVATE_KEY_B64
#   LINEAR_API_KEY
#
# Optional:
#   REPO="kubediscovery/platform-kubediscovery"
#   RETRY_ATTEMPTS="5"
#   RETRY_DELAY_SECONDS="3"

ISSUE_KEY="${1:?missing issue key, ex: DEVENG-42}"
SUMMARY="${2:-task implementation}"
REPO="${REPO:-kubediscovery/platform-kubediscovery}"
RETRY_ATTEMPTS="${RETRY_ATTEMPTS:-5}"
RETRY_DELAY_SECONDS="${RETRY_DELAY_SECONDS:-3}"

retry() {
  local attempts="$1"
  local delay="$2"
  shift 2
  local n=1
  while true; do
    if "$@"; then
      return 0
    fi
    if [[ "$n" -ge "$attempts" ]]; then
      echo "Command failed after ${attempts} attempts: $*" >&2
      return 1
    fi
    echo "Attempt ${n}/${attempts} failed. Retrying in ${delay}s..." >&2
    sleep "$delay"
    n=$((n + 1))
  done
}

if [[ -z "${GITHUB_APP_PRIVATE_KEY:-}" && -n "${GITHUB_APP_PRIVATE_KEY_B64:-}" ]]; then
  GITHUB_APP_PRIVATE_KEY="$(printf '%s' "$GITHUB_APP_PRIVATE_KEY_B64" | base64 -d)"
  export GITHUB_APP_PRIVATE_KEY
fi

if [[ -n "${GITHUB_APP_PRIVATE_KEY:-}" ]] && [[ "${GITHUB_APP_PRIVATE_KEY}" != *"BEGIN "*"PRIVATE KEY"* ]]; then
  GITHUB_APP_PRIVATE_KEY="$(printf '%s' "$GITHUB_APP_PRIVATE_KEY" | base64 -d)"
  export GITHUB_APP_PRIVATE_KEY
fi

github_app_token() {
  local now exp header payload header_b64 payload_b64 signing_input signature jwt
  now="$(date +%s)"
  exp="$((now + 540))"
  header='{"alg":"RS256","typ":"JWT"}'
  payload="{\"iat\":$((now - 60)),\"exp\":${exp},\"iss\":\"${GITHUB_APP_ID}\"}"

  header_b64="$(printf '%s' "$header" | openssl base64 -A | tr '+/' '-_' | tr -d '=')"
  payload_b64="$(printf '%s' "$payload" | openssl base64 -A | tr '+/' '-_' | tr -d '=')"
  signing_input="${header_b64}.${payload_b64}"
  signature="$(printf '%s' "$signing_input" | openssl dgst -sha256 -sign <(printf '%s' "$GITHUB_APP_PRIVATE_KEY") -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')"
  jwt="${signing_input}.${signature}"

  curl -sS --max-time 30 -X POST "https://api.github.com/app/installations/${GITHUB_APP_INSTALLATION_ID}/access_tokens" \
    -H "Authorization: Bearer ${jwt}" \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    -d '{}' | jq -r '.token'
}

if ! command -v gh >/dev/null 2>&1; then
  echo "gh CLI not found. Install GitHub CLI before running this script." >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq not found. Install jq before running this script." >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl not found. Install openssl before running this script." >&2
  exit 1
fi

GITHUB_APP_TOKEN="$(retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" github_app_token)"
if [[ -z "$GITHUB_APP_TOKEN" || "$GITHUB_APP_TOKEN" == "null" ]]; then
  echo "Failed to obtain GitHub App installation token." >&2
  exit 1
fi

retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" gh auth login --with-token <<< "$GITHUB_APP_TOKEN" >/dev/null
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" gh repo view "$REPO" >/dev/null

BRANCH="$(git branch --show-current)"
if [[ -z "$BRANCH" ]]; then
  echo "No current branch found." >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  git add -A
  git commit -m "${ISSUE_KEY}: ${SUMMARY}"
fi

retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" git push -u origin "$BRANCH"

DEFAULT_BRANCH="$(gh repo view "$REPO" --json defaultBranchRef -q '.defaultBranchRef.name')"
PR_TITLE="${ISSUE_KEY}: ${SUMMARY}"
PR_BODY=$(cat <<EOF
Closes ${ISSUE_KEY}

What changed:
- Implemented task scope for ${ISSUE_KEY}.

How to validate:
- Run relevant tests/lint for touched modules.

Risks/notes:
- Verify CI and integration behavior before merge.

Repository target: kubediscovery/platform-kubediscovery
Codex environment: platform-kubediscovery
Do not execute this task in any other repository.
EOF
)

PR_URL="$(retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" gh pr create --repo "$REPO" --base "$DEFAULT_BRANCH" --head "$BRANCH" --title "$PR_TITLE" --body "$PR_BODY")"
echo "PR created: $PR_URL"

COMMENT_MUTATION='mutation($id:String!,$body:String!){ commentCreate(input:{issueId:$id, body:$body}){ success } }'
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" curl -sS --max-time 30 https://api.linear.app/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -d "$(jq -cn --arg q "$COMMENT_MUTATION" --arg id "$ISSUE_KEY" --arg body "PR opened: ${PR_URL}" '{query:$q,variables:{id:$id,body:$body}}')" \
  | jq -e '.data.commentCreate.success == true' >/dev/null

echo "Linear updated: ${ISSUE_KEY}"
