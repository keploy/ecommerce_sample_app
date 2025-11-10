#!/usr/bin/env bash
# Replay user_service/keploy/manual/tests (test-1..test-5) as curl commands
#
# test-1: POST /login (admin/admin123)
# test-2: POST /users (alice)
# test-3: POST /users/{userId}/addresses (default)
# test-4: GET  /users/{userId}/addresses
# test-5: GET  /users/{userId}
#
# Requirements: curl, jq
# Overrides via env:
#   USER_BASE=http://localhost:8082/api/v1
#   ADMIN_USER=admin
#   ADMIN_PASS=admin123
#   NEW_USERNAME=alice
#   NEW_EMAIL=alice@example.com
#   NEW_PASSWORD='p@ssw0rd'
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then echo "curl required" >&2; exit 1; fi
if ! command -v jq >/dev/null 2>&1; then echo "jq required (sudo apt-get install jq)" >&2; exit 1; fi

USER_BASE=${USER_BASE:-"http://localhost:8082/api/v1"}
ADMIN_USER=${ADMIN_USER:-"admin"}
ADMIN_PASS=${ADMIN_PASS:-"admin123"}
NEW_USERNAME=${NEW_USERNAME:-"alice"}
NEW_EMAIL=${NEW_EMAIL:-"alice@example.com"}
NEW_PASSWORD=${NEW_PASSWORD:-"p@ssw0rd"}

say(){ printf "\n==> %s\n" "$*"; }
SEP="__SEP__HTTP__"

req(){
  # req METHOD URL DATA_JSON [AUTH_TOKEN]
  local method="$1" url="$2" data="${3:-}" token="${4:-}";
  local args=( -sS -X "$method" "$url" -H "Accept: application/json" );
  if [[ -n "$data" ]]; then
    args+=( -H "Content-Type: application/json" --data "$data" );
  fi
  if [[ -n "$token" ]]; then
    args+=( -H "Authorization: Bearer $token" );
  fi
  curl "${args[@]}" -w "${SEP}%{http_code}"
}

parse(){
  local resp="$1"; local code body;
  code="${resp##*${SEP}}"; body="${resp%${SEP}*}";
  echo "$code" "$body";
}

echo "USER_BASE=$USER_BASE"

# test-1: Login
say "test-1: POST /login"
LOGIN_PAYLOAD=$(jq -n --arg u "$ADMIN_USER" --arg p "$ADMIN_PASS" '{username:$u,password:$p}')
resp=$(req POST "$USER_BASE/login" "$LOGIN_PAYLOAD")
read -r code body < <(parse "$resp"); echo "HTTP $code"; echo "$body" | jq '.' || true
if [[ "$code" != 200 && "$code" != 201 ]]; then echo "Login failed" >&2; exit 1; fi
TOKEN=$(echo "$body" | jq -r '.token // empty'); [[ -n "$TOKEN" ]] || { echo "No token" >&2; exit 1; }

# test-2: Create user
say "test-2: POST /users (create $NEW_USERNAME)"
# Avoid unique constraint conflicts by default; insert a short suffix.
# Disable by setting DISABLE_RANDOM_SUFFIX=1
if [[ -z "${DISABLE_RANDOM_SUFFIX:-}" ]]; then
  TS=$(date +%s)
  SUFFIX="-${TS}-${RANDOM}"
  USERNAME_EFF="${NEW_USERNAME}${SUFFIX}"
  if [[ "$NEW_EMAIL" == *"@"* ]]; then
    EMAIL_EFF="${NEW_EMAIL%%@*}${SUFFIX}@${NEW_EMAIL##*@}"
  else
    EMAIL_EFF="${NEW_EMAIL}${SUFFIX}"
  fi
else
  USERNAME_EFF="$NEW_USERNAME"; EMAIL_EFF="$NEW_EMAIL"
fi
echo "Using username: $USERNAME_EFF"
echo "Using email   : $EMAIL_EFF"
CREATE_PAYLOAD=$(jq -n --arg u "$USERNAME_EFF" --arg e "$EMAIL_EFF" --arg p "$NEW_PASSWORD" '{username:$u,email:$e,password:$p}')
resp=$(req POST "$USER_BASE/users" "$CREATE_PAYLOAD" "$TOKEN")
read -r code body < <(parse "$resp"); echo "HTTP $code"; echo "$body" | jq '.' || true
if [[ "$code" != 200 && "$code" != 201 ]]; then
  echo "Create user returned $code (continuing)" >&2
  # Common cause: unique constraint violation on username/email when reusing same inputs.
  # You can set DISABLE_RANDOM_SUFFIX= to keep inputs stable, or change NEW_USERNAME/NEW_EMAIL.
fi
USER_ID=$(echo "$body" | jq -r '.id // empty');
if [[ -z "$USER_ID" || "$USER_ID" == null ]]; then echo "No user id in response, cannot continue" >&2; exit 1; fi

# test-3: Add address
say "test-3: POST /users/$USER_ID/addresses (default)"
ADDR_PAYLOAD='{"line1":"1 Main St","city":"NYC","state":"NY","postal_code":"10001","country":"US","phone":"+1-555-0000","is_default":true}'
resp=$(req POST "$USER_BASE/users/$USER_ID/addresses" "$ADDR_PAYLOAD" "$TOKEN")
read -r code body < <(parse "$resp"); echo "HTTP $code"; echo "$body" | jq '.' || true
ADDR_ID=$(echo "$body" | jq -r '.id // empty') || true

# test-4: List addresses
say "test-4: GET /users/$USER_ID/addresses"
resp=$(req GET "$USER_BASE/users/$USER_ID/addresses" "" "$TOKEN")
read -r code body < <(parse "$resp"); echo "HTTP $code"; echo "$body" | jq '.' || true

# test-5: Get user
say "test-5: GET /users/$USER_ID"
resp=$(req GET "$USER_BASE/users/$USER_ID" "" "$TOKEN")
read -r code body < <(parse "$resp"); echo "HTTP $code"; echo "$body" | jq '.' || true

say "Done (manual tests 1-5 replayed)."
