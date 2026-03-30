#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://127.0.0.1:18577}
CONFIG_PATH=${CONFIG_PATH:-/Users/hoonzi/Documents/docker_v/go-commu-bin-data/config.yml}
BOOTSTRAP_USERNAME=${BOOTSTRAP_USERNAME:-admin}
BOOTSTRAP_PASSWORD=${BOOTSTRAP_PASSWORD:-commu-admin-1q2w#E\$R!}

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "config file not found: $CONFIG_PATH" >&2
  exit 1
fi

json_escape() {
  local value="$1"
  value=${value//\\/\\\\}
  value=${value//\"/\\\"}
  printf '%s' "$value"
}

payload=$(printf '{"username":"%s","password":"%s"}' \
  "$(json_escape "$BOOTSTRAP_USERNAME")" \
  "$(json_escape "$BOOTSTRAP_PASSWORD")")

tmp_headers=$(mktemp)
tmp_body=$(mktemp)
trap 'rm -f "$tmp_headers" "$tmp_body"' EXIT

http_code=$(curl -sS -o "$tmp_body" -D "$tmp_headers" -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  --data "$payload")

if [[ "$http_code" != "200" ]]; then
  echo "bootstrap admin login failed (http $http_code)" >&2
  cat "$tmp_body" >&2
  exit 1
fi

auth_header=$(awk -F': ' 'BEGIN { IGNORECASE = 1 } /^Authorization:/ { print $2; exit }' "$tmp_headers")
if [[ -z "$auth_header" ]]; then
  echo "bootstrap admin login succeeded but Authorization header is missing" >&2
  exit 1
fi

echo "bootstrap admin ok: $BOOTSTRAP_USERNAME"
