#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://127.0.0.1:8080}"
UI_URL="${UI_URL:-http://127.0.0.1:8081}"
NAMESPACE="${NAMESPACE:-cluster-probe}"
CHAOS_TIMEOUT_SECONDS="${CHAOS_TIMEOUT_SECONDS:-60}"
SCENARIO_TIMEOUT_SECONDS="${SCENARIO_TIMEOUT_SECONDS:-90}"
POLL_SECONDS="${POLL_SECONDS:-2}"

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

rollout_healthy() {
  local component="$1"
  kubectl -n "$NAMESPACE" rollout status deployment \
    -l "app.kubernetes.io/component=$component" \
    --timeout=120s >/dev/null
}

api_healthy() {
  curl_json "${API_URL}/healthz" | jq -e '.status == "ok"' >/dev/null
}

status_snapshot() {
  curl_json "${API_URL}/api/v1/status"
}

create_scenario() {
  local name="$1"
  jq -n \
    --arg name "$name" \
    '{
      name: $name,
      profile: {
        rps: 1,
        duration: 12000000000,
        payload_size_bytes: 0,
        concurrency: 1,
        target_queue: "workload.high",
        workload_type: "db_write"
      }
    }' |
    curl_json \
      -H "Content-Type: application/json" \
      -X POST \
      --data-binary @- \
      "${API_URL}/api/v1/scenarios"
}

create_stress_chaos() {
  local name="$1"
  local scenario_id="$2"
  jq -n \
    --arg name "$name" \
    --arg scenario "$scenario_id" \
    '{
      name: $name,
      scenario: $scenario,
      config: {
        type: "stress",
        target: "app.kubernetes.io/component=worker",
        duration: "10s",
        workers: "1",
        load: "10"
      }
    }' |
    curl_json \
      -H "Content-Type: application/json" \
      -X POST \
      --data-binary @- \
      "${API_URL}/api/v1/chaos/experiments"
}

poll_chaos_terminal() {
  local experiment_id="$1"
  local deadline=$((SECONDS + CHAOS_TIMEOUT_SECONDS))
  local body status

  while (( SECONDS < deadline )); do
    body="$(curl_json "${API_URL}/api/v1/chaos/experiments/${experiment_id}")"
    status="$(jq -r '.status // ""' <<<"$body")"
    echo "chaos-smoke: experiment status=${status}"
    case "$status" in
      completed|Completed|finished|Finished)
        return 0
        ;;
      failed|Failed|error|Error)
        echo "chaos experiment failed: ${body}" >&2
        return 1
        ;;
    esac
    sleep "$POLL_SECONDS"
  done

  echo "timed out waiting for chaos experiment ${experiment_id}" >&2
  return 1
}

poll_scenario_terminal_or_stop() {
  local scenario_id="$1"
  local deadline=$((SECONDS + SCENARIO_TIMEOUT_SECONDS))
  local body status

  while (( SECONDS < deadline )); do
    body="$(curl_json "${API_URL}/api/v1/scenarios/${scenario_id}")"
    status="$(jq -r '.status // ""' <<<"$body")"
    echo "chaos-smoke: scenario status=${status}"
    case "$status" in
      completed)
        return 0
        ;;
      failed|stopped)
        echo "scenario reached terminal status ${status}" >&2
        return 0
        ;;
    esac
    sleep "$POLL_SECONDS"
  done

  echo "chaos-smoke: stopping scenario ${scenario_id} after timeout"
  curl_json \
    -H "Content-Type: application/json" \
    -X PUT \
    "${API_URL}/api/v1/scenarios/${scenario_id}/stop" |
    jq -e '.status == "stopped"' >/dev/null
}

cleanup_experiment() {
  if [[ -n "${experiment_id:-}" ]]; then
    curl -fsS -X DELETE "${API_URL}/api/v1/chaos/experiments/${experiment_id}" >/dev/null 2>&1 || true
  fi
}
trap cleanup_experiment EXIT

require_command curl
require_command jq
require_command kubectl

echo "chaos-smoke: baseline API health"
api_healthy
baseline_status="$(status_snapshot)"
jq -e '.status == "ok" and (.counters | type == "object")' <<<"$baseline_status" >/dev/null
rollout_healthy api
rollout_healthy worker

scenario_name="chaos-smoke-$(date +%s)"
echo "chaos-smoke: create scenario ${scenario_name}"
scenario_body="$(create_scenario "$scenario_name")"
scenario_id="$(jq -r '.id' <<<"$scenario_body")"
if [[ -z "$scenario_id" || "$scenario_id" == "null" ]]; then
  echo "scenario id missing: ${scenario_body}" >&2
  exit 1
fi

experiment_name="chaos-smoke-stress-$(date +%s)"
echo "chaos-smoke: create StressChaos ${experiment_name}"
experiment_body="$(create_stress_chaos "$experiment_name" "$scenario_id")"
experiment_id="$(jq -r '.id' <<<"$experiment_body")"
if [[ -z "$experiment_id" || "$experiment_id" == "null" ]]; then
  echo "experiment id missing: ${experiment_body}" >&2
  exit 1
fi

echo "chaos-smoke: verify experiment appears in API and UI"
curl_json "${API_URL}/api/v1/chaos/experiments" |
  jq -e '.[] | select(.id == "'"${experiment_id}"'")' >/dev/null
curl -fsS "${UI_URL}/chaos" | grep -q "$experiment_id"

poll_chaos_terminal "$experiment_id"

echo "chaos-smoke: verify API and Worker health after chaos"
api_healthy
rollout_healthy api
rollout_healthy worker

poll_scenario_terminal_or_stop "$scenario_id"

after_status="$(status_snapshot)"
jq -e '.status == "ok" and (.counters | type == "object")' <<<"$after_status" >/dev/null

echo "chaos-smoke: passed"
