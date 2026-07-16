#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://127.0.0.1:8080}"
UI_URL="${UI_URL:-http://127.0.0.1:8081}"
SCENARIO_TIMEOUT_SECONDS="${SCENARIO_TIMEOUT_SECONDS:-90}"
SCENARIO_POLL_SECONDS="${SCENARIO_POLL_SECONDS:-2}"
WORKLOAD_TYPE="${WORKLOAD_TYPE:-db_write}"
TARGET_QUEUE="${TARGET_QUEUE:-workload.high}"

require_command() {
  local command="$1"
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing required command: $command" >&2
    exit 1
  fi
}

curl_json() {
  curl -fsS "$@"
}

require_command curl
require_command jq

echo "smoke: api health"
curl_json "${API_URL}/healthz" | jq -e '.status == "ok"' >/dev/null

echo "smoke: api status"
curl_json "${API_URL}/api/v1/status" | jq -e '.status == "ok" and (.counters | type == "object")' >/dev/null

echo "smoke: ui dashboard"
curl -fsS "${UI_URL}/dashboard" | grep -q "Dashboard"

scenario_name="smoke-$(date +%s)"
scenario_payload="$(
  jq -n \
    --arg name "$scenario_name" \
    --arg queue "$TARGET_QUEUE" \
    --arg workload "$WORKLOAD_TYPE" \
    '{
      name: $name,
      profile: {
        rps: 1,
        duration: 3000000000,
        payload_size_bytes: 0,
        concurrency: 1,
        target_queue: $queue,
        workload_type: $workload
      }
    }'
)"

echo "smoke: create scenario ${scenario_name}"
scenario_id="$(
  curl_json \
    -H "Content-Type: application/json" \
    -X POST \
    --data-binary "$scenario_payload" \
    "${API_URL}/api/v1/scenarios" | jq -r '.id'
)"

if [[ -z "$scenario_id" || "$scenario_id" == "null" ]]; then
  echo "scenario id missing" >&2
  exit 1
fi

echo "smoke: poll scenario ${scenario_id}"
deadline=$((SECONDS + SCENARIO_TIMEOUT_SECONDS))
scenario_status=""
while (( SECONDS < deadline )); do
  scenario_body="$(curl_json "${API_URL}/api/v1/scenarios/${scenario_id}")"
  scenario_status="$(jq -r '.status // ""' <<<"$scenario_body")"
  echo "smoke: scenario status=${scenario_status}"

  if [[ -z "$scenario_status" ]]; then
    echo "scenario status blank: ${scenario_body}" >&2
    exit 1
  fi

  case "$scenario_status" in
    completed|failed|stopped)
      break
      ;;
  esac

  sleep "$SCENARIO_POLL_SECONDS"
done

if [[ "$scenario_status" != "completed" ]]; then
  echo "scenario did not complete successfully: ${scenario_status}" >&2
  exit 1
fi

echo "smoke: metrics snapshot"
curl_json "${API_URL}/api/v1/metrics/snapshot" | jq -e '.snapshot.scenario_id == "'"${scenario_id}"'"' >/dev/null

stop_scenario_name="smoke-stop-$(date +%s)"
stop_scenario_payload="$(
  jq -n \
    --arg name "$stop_scenario_name" \
    --arg workload "$WORKLOAD_TYPE" \
    '{
      name: $name,
      profile: {
        rps: 1,
        duration: 20000000000,
        payload_size_bytes: 0,
        concurrency: 1,
        target_queue: "workload.low",
        workload_type: $workload
      }
    }'
)"

echo "smoke: create stoppable scenario ${stop_scenario_name}"
stop_scenario_id="$(
  curl_json \
    -H "Content-Type: application/json" \
    -X POST \
    --data-binary "$stop_scenario_payload" \
    "${API_URL}/api/v1/scenarios" | jq -r '.id'
)"

if [[ -z "$stop_scenario_id" || "$stop_scenario_id" == "null" ]]; then
  echo "stoppable scenario id missing" >&2
  exit 1
fi

echo "smoke: stop scenario ${stop_scenario_id}"
curl_json \
  -H "Content-Type: application/json" \
  -X PUT \
  "${API_URL}/api/v1/scenarios/${stop_scenario_id}/stop" |
  jq -e '.status == "stopped" and .name == "'"${stop_scenario_name}"'" and .profile.target_queue == "workload.low"' >/dev/null

curl_json "${API_URL}/api/v1/scenarios/${stop_scenario_id}" |
  jq -e '.status == "stopped" and .name == "'"${stop_scenario_name}"'"' >/dev/null

echo "smoke: logs stream"
logs_output="$(curl --max-time 5 -fsS -N "${UI_URL}/logs/stream" 2>/dev/null || true)"
grep -q "event: logs" <<<"$logs_output"

echo "smoke: scenarios UI includes scenario"
curl -fsS "${UI_URL}/scenarios" | grep -q "$scenario_id"

echo "smoke: scenarios UI includes stopped scenario"
curl -fsS "${UI_URL}/scenarios" | grep -q "$stop_scenario_id"
curl -fsS "${UI_URL}/scenarios" | grep -q "stopped"

echo "smoke: passed"
