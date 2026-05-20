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
# 1) Generate GitHub App installation token
GITHUB_APP_TOKEN="$(
python3 - <<'PY'
import os, time, json, base64, urllib.request
import jwt
app_id = os.environ["GITHUB_APP_ID"]
inst_id = os.environ["GITHUB_APP_INSTALLATION_ID"]
pem = os.environ["GITHUB_APP_PRIVATE_KEY"].encode()
now = int(time.time())
payload = {"iat": now - 60, "exp": now + 540, "iss": app_id}
app_jwt = jwt.encode(payload, pem, algorithm="RS256")
url = f"https://api.github.com/app/installations/{inst_id}/access_tokens"
req = urllib.request.Request(
    url,
    data=b"{}",
    headers={
        "Authorization": f"Bearer {app_jwt}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    },
    method="POST",
)
with urllib.request.urlopen(req, timeout=30) as r:
    data = json.loads(r.read().decode())
print(data["token"])
PY
)"
# 2) Auth gh with app token
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" gh auth login --with-token <<< "$GITHUB_APP_TOKEN" >/dev/null
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" gh repo view "$REPO" >/dev/null
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
DEFAULT_BRANCH="$(gh repo view "$REPO" --json defaultBranchRef -q '.defaultBranchRef.name')"
git checkout "$DEFAULT_BRANCH"
retry "$RETRY_ATTEMPTS" "$RETRY_DELAY_SECONDS" git pull --ff-only origin "$DEFAULT_BRANCH"
git checkout -b "$BRANCH"
# 5) Git identity
git config user.name "kubediscovery-codex-app[bot]"
git config user.email "kubediscovery-codex-app[bot]@users.noreply.github.com"
echo "OK: ${ISSUE_KEY} moved to In Progress"
echo "OK: branch created -> ${BRANCH}"
