#!/usr/bin/env bash
set -euo pipefail
# Usage:
#   ./start-task.sh DEVENG-42 "add-grpc-timeout"
#
# Required env vars:
#   GITHUB_APP_ID
#   GITHUB_APP_INSTALLATION_ID
#   GITHUB_APP_PRIVATE_KEY   (full PEM content)
#   LINEAR_API_KEY           (lin_api_...)
#
# Optional:
#   REPO="kubediscovery/platform-kubediscovery"
#   TEAM_KEY="DEVENG"
#   RETRY_ATTEMPTS="5"
#   RETRY_DELAY_SECONDS="3"
ISSUE_KEY="${1:?missing issue key, ex: DEVENG-42}"
SLUG="${2:-task}"
REPO="${REPO:-kubediscovery/platform-kubediscovery}"
TEAM_KEY="${TEAM_KEY:-DEVENG}"
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

if ! command -v jq >/dev/null 2>&1; then
  echo "jq not found. Install jq before running this script." >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl not found. Install openssl before running this script." >&2
  exit 1
fi

# 1) Generate GitHub App installation token
GITHUB_APP_TOKEN="$(retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" github_app_token)"

if [[ -z "$GITHUB_APP_TOKEN" || "$GITHUB_APP_TOKEN" == "null" ]]; then
  echo "Failed to obtain GitHub App installation token." >&2
  exit 1
fi
# 2) Auth gh with app token
if command -v gh >/dev/null 2>&1; then
  retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" gh auth login --with-token <<< "$GITHUB_APP_TOKEN" >/dev/null
  retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" gh repo view "$REPO" >/dev/null
  DEFAULT_BRANCH="$(gh repo view "$REPO" --json defaultBranchRef -q '.defaultBranchRef.name')"
else
  DEFAULT_BRANCH="$(retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" curl -sS --max-time 30 "https://api.github.com/repos/${REPO}" \
    -H "Authorization: Bearer ${GITHUB_APP_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" | jq -r '.default_branch')"
  if [[ -z "$DEFAULT_BRANCH" || "$DEFAULT_BRANCH" == "null" ]]; then
    echo "Could not resolve default branch from GitHub API for ${REPO}" >&2
    exit 1
  fi
fi
# 3) Move Linear issue to In Progress
LINEAR_QUERY='
query($teamKey:String!){
  teams(filter:{key:{eq:$teamKey}}){
    nodes{
      id
      states{
        nodes{ id name type }
      }
    }
  }
}'
TEAM_JSON="$(retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" curl -sS --max-time 30 https://api.linear.app/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -d "$(jq -cn --arg q "$LINEAR_QUERY" --arg teamKey "$TEAM_KEY" '{query:$q,variables:{teamKey:$teamKey}}')")"
IN_PROGRESS_STATE_ID="$(jq -r '.data.teams.nodes[0].states.nodes[] | select((.name=="In Progress") or (.type=="started")) | .id' <<< "$TEAM_JSON" | head -n1)"
if [[ -z "${IN_PROGRESS_STATE_ID}" || "${IN_PROGRESS_STATE_ID}" == "null" ]]; then
  echo "Could not find In Progress state for team ${TEAM_KEY}"
  exit 1
fi
UPDATE_MUTATION='
mutation($id:String!,$stateId:String!){
  issueUpdate(id:$id,input:{stateId:$stateId}){ success }
}'
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" curl -sS --max-time 30 https://api.linear.app/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -d "$(jq -cn --arg q "$UPDATE_MUTATION" --arg id "$ISSUE_KEY" --arg stateId "$IN_PROGRESS_STATE_ID" '{query:$q,variables:{id:$id,stateId:$stateId}}')" \
  | jq -e '.data.issueUpdate.success == true' >/dev/null
# 4) Create branch
BRANCH="feat/${ISSUE_KEY}-${SLUG}"
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" git fetch origin
git checkout "$DEFAULT_BRANCH"
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" git pull --ff-only origin "$DEFAULT_BRANCH"
git checkout -b "$BRANCH"
# 5) Git identity
git config user.name "kubediscovery-codex-app[bot]"
git config user.email "kubediscovery-codex-app[bot]@users.noreply.github.com"
echo "OK: ${ISSUE_KEY} moved to In Progress"
echo "OK: branch created -> ${BRANCH}"
