#!/usr/bin/env bash
# Complete CLI example:
#   create -> connect -> files mkdir/write -> exec -> verify -> kill
#
# Required environment variables:
#   WORKSPACE_ID, BRANCH_ID
#
# Optional environment variables:
#   CLI, SANDBOX_TEMPLATE, SANDBOX_TIMEOUT, SANDBOX_PROJECT_DIR,
#   SANDBOX_REMOTE_DIR, SANDBOX_COMMAND, SANDBOX_ARGS
#
# This script is intentionally a CLI-only example. Do not pass its
# envdAccessToken to the E2B SDK.
set -euo pipefail

: "${WORKSPACE_ID:?set WORKSPACE_ID}"
: "${BRANCH_ID:?set BRANCH_ID}"

CLI="${CLI:-byted-supabase-cli}"
TEMPLATE="${SANDBOX_TEMPLATE:-base}"
PROJECT_DIR="${SANDBOX_PROJECT_DIR:-}"
REMOTE_DIR="${SANDBOX_REMOTE_DIR:-/home/user/project}"
COMMAND="${SANDBOX_COMMAND:-echo}"
ARGS="${SANDBOX_ARGS:-sandbox-ok}"
SANDBOX_ID=""
SANDBOX_ACCESS_TOKEN=""
read -r -a COMMAND_ARGS <<< "$ARGS"

command -v "$CLI" >/dev/null 2>&1 || {
  echo "CLI not found: $CLI" >&2
  exit 1
}
command -v jq >/dev/null 2>&1 || {
  echo "jq is required" >&2
  exit 1
}

cleanup() {
  if [[ -n "$SANDBOX_ID" ]]; then
    "$CLI" sandbox kill \
      --workspace-id "$WORKSPACE_ID" \
      --branch-id "$BRANCH_ID" \
      --sandbox-id "$SANDBOX_ID" \
      --yes \
      -o json || true
  fi
}
trap cleanup EXIT

create_json="$("$CLI" sandbox create \
  --workspace-id "$WORKSPACE_ID" \
  --branch-id "$BRANCH_ID" \
  --template-name "$TEMPLATE" \
  --timeout "${SANDBOX_TIMEOUT:-600}" \
  --detach \
  -o json)"
SANDBOX_ID="$(printf '%s' "$create_json" | jq -r '.sandboxID')"
[[ -n "$SANDBOX_ID" && "$SANDBOX_ID" != "null" ]] || {
  echo "sandbox create did not return sandboxID" >&2
  exit 1
}
unset create_json

connect_json="$("$CLI" sandbox connect \
  --workspace-id "$WORKSPACE_ID" \
  --branch-id "$BRANCH_ID" \
  --sandbox-id "$SANDBOX_ID" \
  --timeout "${SANDBOX_TIMEOUT:-600}" \
  -o json)"
SANDBOX_ACCESS_TOKEN="$(printf '%s' "$connect_json" | jq -r '.envdAccessToken')"
[[ -n "$SANDBOX_ACCESS_TOKEN" && "$SANDBOX_ACCESS_TOKEN" != "null" ]] || {
  echo "sandbox connect did not return envdAccessToken" >&2
  exit 1
}
unset connect_json

"$CLI" sandbox files mkdir \
  --workspace-id "$WORKSPACE_ID" \
  --branch-id "$BRANCH_ID" \
  --sandbox-id "$SANDBOX_ID" \
  --sandbox-access-token "$SANDBOX_ACCESS_TOKEN" \
  --path "$REMOTE_DIR" \
  -o json

if [[ -n "$PROJECT_DIR" ]]; then
  [[ -d "$PROJECT_DIR" ]] || {
    echo "SANDBOX_PROJECT_DIR does not exist: $PROJECT_DIR" >&2
    exit 1
  }
  while IFS= read -r -d '' file; do
    relative="${file#"$PROJECT_DIR"/}"
    remote="$REMOTE_DIR/$relative"
    remote_parent="${remote%/*}"
    "$CLI" sandbox files mkdir \
      --workspace-id "$WORKSPACE_ID" \
      --branch-id "$BRANCH_ID" \
      --sandbox-id "$SANDBOX_ID" \
      --sandbox-access-token "$SANDBOX_ACCESS_TOKEN" \
      --path "$remote_parent" \
      -o json
    "$CLI" sandbox files write \
      --workspace-id "$WORKSPACE_ID" \
      --branch-id "$BRANCH_ID" \
      --sandbox-id "$SANDBOX_ID" \
      --sandbox-access-token "$SANDBOX_ACCESS_TOKEN" \
      --path "$remote" \
      --file "$file" \
      -o json
  done < <(find "$PROJECT_DIR" -type f -print0)
fi

echo "sandbox id: $SANDBOX_ID"
exec_json="$("$CLI" sandbox exec \
  --workspace-id "$WORKSPACE_ID" \
  --branch-id "$BRANCH_ID" \
  --sandbox-id "$SANDBOX_ID" \
  --sandbox-access-token "$SANDBOX_ACCESS_TOKEN" \
  --command "$COMMAND" \
  "${COMMAND_ARGS[@]}" \
  -o json)"
printf '%s\n' "$exec_json"

exit_code="$(printf '%s' "$exec_json" | jq -r '.exit_code // empty')"
if [[ -n "$exit_code" && "$exit_code" != "0" ]]; then
  echo "sandbox command failed with exit_code=$exit_code" >&2
  exit 1
fi

status="$(printf '%s' "$exec_json" | jq -r '.status // empty')"
stdout="$(printf '%s' "$exec_json" | jq -r '.stdout // empty')"
stderr="$(printf '%s' "$exec_json" | jq -r '.stderr // empty')"
[[ -n "$stdout" ]] && printf '%s\n' "$stdout"
[[ -n "$stderr" ]] && printf '%s\n' "$stderr" >&2

if [[ -n "$status" && "$status" != "exit status 0" && -z "$exit_code" ]]; then
  echo "sandbox command did not report success: $status" >&2
  exit 1
fi
