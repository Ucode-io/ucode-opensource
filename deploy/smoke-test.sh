#!/usr/bin/env bash
#
# Walks the path a new user takes, against a running stack.
#
# This exists because every failure found so far surfaced at runtime and not in
# any test: a service panicking on an unset optional dependency, and plan-limit
# checks calling a billing service that had been removed. All of them compiled
# and passed `go test`. Only an actual request found them.
#
#   deploy/smoke-test.sh
#
# Expects the stack to be up (`ucode start`). Exits non-zero on the first
# failure, and says which step failed.

set -euo pipefail

GATEWAY="${UCODE_GATEWAY_URL:-http://127.0.0.1:8000}"
AUTH="${UCODE_AUTH_URL:-http://127.0.0.1:9104}"
LOGIN="${UCODE_ADMIN_LOGIN:-admin@ucode.local}"
PASSWORD="${UCODE_ADMIN_PASSWORD:-UcodeAdmin1!}"

# A slug that cannot collide with a real table and is easy to spot if cleanup
# ever fails to run.
SLUG="smoke_$(date +%s)"

step()  { printf '\n→ %s\n' "$1"; }
fail()  { printf '\n✗ %s\n' "$1" >&2; exit 1; }
ok()    { printf '  ✓ %s\n' "$1"; }

# Reads a value out of a JSON body by path, e.g. `json data.response.token`.
# Prints nothing when the path is missing, so callers can check for empty
# rather than parsing an error.
json() {
    python3 -c '
import sys, json
try:
    node = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for key in sys.argv[1].split("."):
    if isinstance(node, list):
        try:
            node = node[int(key)]
        except (ValueError, IndexError):
            sys.exit(0)
    elif isinstance(node, dict) and key in node:
        node = node[key]
    else:
        sys.exit(0)
print(node)
' "$1"
}

# ---------------------------------------------------------------- sign in --
step "Signing in as $LOGIN"

login_response=$(curl -sS -X POST "$AUTH/v3/multicompany/default-login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$LOGIN\",\"password\":\"$PASSWORD\"}")

token=$(printf '%s' "$login_response" | json data.response.token.access_token)
project=$(printf '%s' "$login_response" | json data.project_data.project_id)

[ -n "$token" ]   || fail "no access token in the login response: $login_response"
[ -n "$project" ] || fail "no project id in the login response"
ok "signed in, project $project"

# ------------------------------------------------------------ environment --
step "Reading the environment"

environment=$(curl -sS "$GATEWAY/v1/environment?project_id=$project&limit=1" \
    -H "Authorization: Bearer $token" -H "project-id: $project" \
    | json data.environments.0.id)

[ -n "$environment" ] || fail "the project has no environment"
ok "environment $environment"

auth_headers=(
    -H "Authorization: Bearer $token"
    -H "Content-Type: application/json"
    -H "project-id: $project"
    -H "environment-id: $environment"
)

# ------------------------------------------------------------ create table --
step "Creating table $SLUG"

table=$(curl -sS -X POST "$GATEWAY/v1/table" "${auth_headers[@]}" \
    -d "{\"label\":\"$SLUG\",\"slug\":\"$SLUG\",\"show_in_menu\":false,\"attributes\":{}}" \
    | json data.id)

[ -n "$table" ] || fail "the table was not created — this is where a plan-limit check used to fail"
ok "table $table"

cleanup() {
    curl -sS -X DELETE "$GATEWAY/v1/table/$table" "${auth_headers[@]}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# ------------------------------------------------------------ create item --
step "Creating a record"

curl -sS -X POST "$GATEWAY/v2/items/$SLUG" "${auth_headers[@]}" -d '{"data":{}}' >/dev/null \
    || fail "the record was not created"
ok "record created"

# -------------------------------------------------------------- read back --
step "Reading it back"

count=$(curl -sS "$GATEWAY/v2/items/$SLUG" "${auth_headers[@]}" | json data.data.count)

[ "$count" = "1" ] || fail "expected exactly one record, got: ${count:-nothing}"
ok "one record returned"

printf '\nSmoke test passed: sign-in, table, record, read-back.\n'
