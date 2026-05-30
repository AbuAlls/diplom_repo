#!/usr/bin/env bash
set -euo pipefail

API_BASE="${API_BASE:-http://127.0.0.1:8080}"
EMAIL="${EMAIL:-test@example.com}"
PASSWORD="${PASSWORD:-password123}"
FULL_NAME="${FULL_NAME:-Test User}"
UPLOAD_FILE="${UPLOAD_FILE:-/tmp/docsapp-minio-check.txt}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

request() {
  local method="$1"
  local url="$2"
  local output="$3"
  local code
  shift 3

  if ! code="$(curl -sS -o "$output" -w "%{http_code}" -X "$method" "$url" "$@")"; then
    printf '000'
    return 0
  fi
  printf '%s' "$code"
}

require_json_field() {
  local file="$1"
  local expr="$2"
  local value

  value="$(jq -er "$expr" "$file")"
  printf '%s' "$value"
}

print_failure() {
  local label="$1"
  local code="$2"
  local file="$3"

  printf '%s failed with HTTP %s\n' "$label" "$code" >&2
  if [[ -s "$file" ]]; then
    jq . "$file" >&2 || cat "$file" >&2
  fi
}

health_body="$tmp_dir/health.json"
health_code="000"
for _ in $(seq 1 60); do
  health_code="$(request GET "$API_BASE/healthz" "$health_body")"
  if [[ "$health_code" == "200" ]]; then
    break
  fi
  sleep 1
done
if [[ "$health_code" != "200" ]]; then
  print_failure "health check" "$health_code" "$health_body"
  exit 1
fi

auth_body="$tmp_dir/auth.json"
auth_code="$(request POST "$API_BASE/v0/auth/register" "$auth_body" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"full_name\":\"$FULL_NAME\"}")"

if [[ "$auth_code" != "201" && "$auth_code" != "200" ]]; then
  auth_code="$(request POST "$API_BASE/v0/auth/token" "$auth_body" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    --data-urlencode "grant_type=password" \
    --data-urlencode "username=$EMAIL" \
    --data-urlencode "password=$PASSWORD")"
fi

if [[ "$auth_code" != "200" && "$auth_code" != "201" ]]; then
  print_failure "auth" "$auth_code" "$auth_body"
  exit 1
fi

token="$(require_json_field "$auth_body" '.access_token')"

plan_body="$tmp_dir/plan.json"
plan_code="$(request POST "$API_BASE/v0/plans" "$plan_body" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"name":"MinIO API check plan"}')"
if [[ "$plan_code" != "201" ]]; then
  print_failure "create plan" "$plan_code" "$plan_body"
  exit 1
fi
plan_id="$(require_json_field "$plan_body" '.id')"

goal_body="$tmp_dir/goal.json"
goal_code="$(request POST "$API_BASE/v0/plans/$plan_id/goals" "$goal_body" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"name":"MinIO API check goal"}')"
if [[ "$goal_code" != "201" ]]; then
  print_failure "create goal" "$goal_code" "$goal_body"
  exit 1
fi
goal_id="$(require_json_field "$goal_body" '.id')"

item_body="$tmp_dir/item.json"
item_code="$(request POST "$API_BASE/v0/plans/$plan_id/goals/$goal_id/items" "$item_body" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"name":"MinIO API check item"}')"
if [[ "$item_code" != "201" ]]; then
  print_failure "create item" "$item_code" "$item_body"
  exit 1
fi
item_id="$(require_json_field "$item_body" '.id')"

printf 'docsapp minio api check\n' > "$UPLOAD_FILE"

upload_body="$tmp_dir/upload.json"
upload_code="$(request POST "$API_BASE/v0/documents/upload/$item_id" "$upload_body" \
  -H "Authorization: Bearer $token" \
  -F "file=@$UPLOAD_FILE;type=text/plain")"
if [[ "$upload_code" != "200" ]]; then
  print_failure "upload document" "$upload_code" "$upload_body"
  exit 1
fi
document_id="$(require_json_field "$upload_body" '.id')"

storage_body="$tmp_dir/storage.json"
storage_code="$(request GET "$API_BASE/v0/documents/$document_id/storage" "$storage_body" \
  -H "Authorization: Bearer $token")"
if [[ "$storage_code" != "200" ]]; then
  print_failure "storage check" "$storage_code" "$storage_body"
  exit 1
fi

printf 'TOKEN=%s\n' "$token"
printf 'PLAN_ID=%s GOAL_ID=%s ITEM_ID=%s DOCUMENT_ID=%s\n' "$plan_id" "$goal_id" "$item_id" "$document_id"
printf 'HTTP %s\n' "$storage_code"
jq . "$storage_body"
