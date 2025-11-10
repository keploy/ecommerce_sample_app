#!/usr/bin/env bash
# User Service flow using curl based on postman/collection.microservices.json
# Endpoints covered:
# - POST /login (get JWT)
# - POST /users (create user)
# - POST /users/{id}/addresses (create default address)
# - GET  /users/{id}/addresses (list addresses)
# - GET  /users/{id} (get user)
# - DELETE /users/{id} (delete user)
#
# Requirements: curl, jq
#
# Env overrides (with sensible defaults):
#   USER_BASE      - default http://localhost:8082/api/v1
#   USERNAME       - default alice
#   EMAIL          - default alice@example.com
#   PASSWORD       - default p@ssw0rd
#   ADDRESS_* vars - override address fields if needed
#   SKIP_DELETE=1  - if set, skip deleting the created user
#
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "Error: curl is required" >&2; exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "Error: jq is required (e.g. sudo apt-get install jq)" >&2; exit 1
fi

USER_BASE=${USER_BASE:-"http://localhost:8082/api/v1"}
USERNAME=${USERNAME:-"alice"}
EMAIL=${EMAIL:-"alice@example.com"}
PASSWORD=${PASSWORD:-"p@ssw0rd"}

# Address defaults (matches Postman example)
ADDRESS_LINE1=${ADDRESS_LINE1:-"1 Main St"}
ADDRESS_CITY=${ADDRESS_CITY:-"NYC"}
ADDRESS_STATE=${ADDRESS_STATE:-"NY"}
ADDRESS_POSTAL=${ADDRESS_POSTAL:-"10001"}
ADDRESS_COUNTRY=${ADDRESS_COUNTRY:-"US"}
ADDRESS_PHONE=${ADDRESS_PHONE:-"+1-555-0000"}
ADDRESS_IS_DEFAULT=${ADDRESS_IS_DEFAULT:-true}

# Slight randomness to avoid collisions when the backend enforces unique usernames/emails
RAND_SUFFIX=$(printf "%04d" $((RANDOM % 10000)))
RAND_EMAIL_TAG=$(date +%s)
USERNAME_UNIQ=${USERNAME}-${RAND_SUFFIX}
EMAIL_UNIQ=${EMAIL/@@/@} # noop if not placeholder
if [[ "$EMAIL" == *"@"* ]]; then
  EMAIL_UNIQ="${EMAIL%%@*}+${RAND_EMAIL_TAG}@${EMAIL##*@}"
fi

SEP="__HTTP_STATUS__SEPARATOR__"

req() {
  # Usage: req METHOD URL DATA_JSON [AUTH_TOKEN]
  local method="$1" url="$2" data="${3:-}" token="${4:-}"
  local headers=("-H" "Accept: application/json")
  if [[ -n "$data" ]]; then
    headers+=("-H" "Content-Type: application/json" "--data" "$data")
  fi
  if [[ -n "$token" ]]; then
    headers+=("-H" "Authorization: Bearer $token")
  fi
  # -sS silent but show errors, include http code with delimiter
  curl -sS -X "$method" "$url" "${headers[@]}" -w "${SEP}%{http_code}"
}

handle_response() {
  # Split body and status using our delimiter
  local resp="$1"
  local http_code body
  http_code="${resp##*${SEP}}"
  body="${resp%${SEP}*}"
  echo "$http_code" "$body"
}

say() { printf "\n==> %s\n" "$*"; }

say "User base: $USER_BASE"

# 1) Login to get JWT
say "Logging in to get JWT"
LOGIN_PAYLOAD=$(jq -n --arg u "$USERNAME" --arg p "$PASSWORD" '{username: $u, password: $p}')
resp=$(req POST "$USER_BASE/login" "$LOGIN_PAYLOAD")
read -r code body < <(handle_response "$resp")
echo "HTTP $code"
echo "$body" | jq '.' || true
if [[ "$code" != "200" && "$code" != "201" ]]; then
  echo "Error: login failed (HTTP $code)" >&2; exit 1
fi
JWT=$(echo "$body" | jq -r '.token // empty')
if [[ -z "${JWT}" || "$JWT" == "null" ]]; then
  echo "Error: token not found in login response" >&2; exit 1
fi
say "JWT acquired"

# 2) Create user (authorized)
say "Creating user: $USERNAME_UNIQ ($EMAIL_UNIQ)"
CREATE_USER_PAYLOAD=$(jq -n \
  --arg u "$USERNAME_UNIQ" \
  --arg e "$EMAIL_UNIQ" \
  --arg p "$PASSWORD" \
  '{username: $u, email: $e, password: $p}')
resp=$(req POST "$USER_BASE/users" "$CREATE_USER_PAYLOAD" "$JWT")
read -r code body < <(handle_response "$resp")
echo "HTTP $code"
echo "$body" | jq '.' || true
if [[ "$code" != "200" && "$code" != "201" ]]; then
  echo "Warning: create user returned HTTP $code; continuing" >&2
fi
USER_ID=$(echo "$body" | jq -r '.id // empty')
if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
  # Try to login again with the unique username to fetch a user profile if backend supports it
  USER_ID=$(echo "$body" | jq -r '.user.id // empty')
fi
if [[ -z "$USER_ID" || "$USER_ID" == "null" ]]; then
  echo "Warning: user id not returned; attempting to fetch via GET /users (may not be supported)" >&2
fi

# 3) Add default address if we have a user id
if [[ -n "${USER_ID:-}" ]]; then
  say "Adding default address for user $USER_ID"
  ADDR_PAYLOAD=$(jq -n \
    --arg l1 "$ADDRESS_LINE1" \
    --arg city "$ADDRESS_CITY" \
    --arg st "$ADDRESS_STATE" \
    --arg pc "$ADDRESS_POSTAL" \
    --arg ctry "$ADDRESS_COUNTRY" \
    --arg ph "$ADDRESS_PHONE" \
    --argjson def "$ADDRESS_IS_DEFAULT" \
    '{line1: $l1, city: $city, state: $st, postal_code: $pc, country: $ctry, phone: $ph, is_default: $def}')
  resp=$(req POST "$USER_BASE/users/$USER_ID/addresses" "$ADDR_PAYLOAD" "$JWT")
  read -r code body < <(handle_response "$resp")
  echo "HTTP $code"
  echo "$body" | jq '.' || true
  ADDRESS_ID=$(echo "$body" | jq -r '.id // empty')
else
  echo "Skipping address creation (no USER_ID)" >&2
fi

# 4) List addresses (if we have a user id)
if [[ -n "${USER_ID:-}" ]]; then
  say "Listing addresses for user $USER_ID"
  resp=$(req GET "$USER_BASE/users/$USER_ID/addresses" "" "$JWT")
  read -r code body < <(handle_response "$resp")
  echo "HTTP $code"
  echo "$body" | jq '.' || true
fi

# 5) Get user (if we have a user id)
if [[ -n "${USER_ID:-}" ]]; then
  say "Fetching user $USER_ID"
  resp=$(req GET "$USER_BASE/users/$USER_ID" "" "$JWT")
  read -r code body < <(handle_response "$resp")
  echo "HTTP $code"
  echo "$body" | jq '.' || true
fi

# 6) Optionally delete the user
if [[ -z "${SKIP_DELETE:-}" && -n "${USER_ID:-}" ]]; then
  say "Deleting user $USER_ID (set SKIP_DELETE=1 to keep)"
  resp=$(req DELETE "$USER_BASE/users/$USER_ID" "" "$JWT")
  read -r code body < <(handle_response "$resp")
  echo "HTTP $code"
  if [[ "$code" == "200" || "$code" == "404" ]]; then
    echo "Delete completed (ok or not found)"
  else
    echo "$body" | jq '.' || true
  fi
else
  say "Skip delete (SKIP_DELETE=1 or missing USER_ID)"
fi

say "Done."
