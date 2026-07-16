#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RELEASE_NAME="${RELEASE_NAME:-clusterprobe}"
NAMESPACE="${NAMESPACE:-cluster-probe}"
REGISTRY="${REGISTRY:-docker.io/slickg}"
TAG="${TAG:-local-$(git rev-parse --short HEAD)-$(date -u +%Y%m%d%H%M%S)}"
VERSION="${VERSION:-dev}"
COMMIT_SHA="${COMMIT_SHA:-$(git rev-parse --short HEAD)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
STORAGE_CLASS="${STORAGE_CLASS:-local-path}"
API_PORT="${API_PORT:-18080}"
UI_PORT="${UI_PORT:-18081}"
HELM_TIMEOUT="${HELM_TIMEOUT:-10m}"
CHART_DIR="${CHART_DIR:-deploy/helm/clusterprobe}"

if [[ -z "${DOCKER_HOST:-}" && -S "$HOME/.colima/default/docker.sock" ]]; then
  export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
fi

if [[ -z "${CHROMIUM_EXECUTABLE_PATH:-}" && -x /Applications/Chromium.app/Contents/MacOS/Chromium ]]; then
  export CHROMIUM_EXECUTABLE_PATH=/Applications/Chromium.app/Contents/MacOS/Chromium
fi

require_command() {
  local command="$1"
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing required command: $command" >&2
    exit 1
  fi
}

port_available() {
  local port="$1"
  ! nc -z 127.0.0.1 "$port" >/dev/null 2>&1
}

choose_port() {
  local start="$1"
  local port="$start"
  while ! port_available "$port"; do
    port=$((port + 1))
  done
  printf "%s" "$port"
}

run_step() {
  local name="$1"
  shift
  printf "\n==> %s\n" "$name"
  "$@"
}

rollout_status_all() {
  local kind="$1"
  local resources
  resources="$(kubectl -n "$NAMESPACE" get "$kind" -o name)"
  if [[ -z "$resources" ]]; then
    return 0
  fi
  while IFS= read -r resource; do
    [[ -z "$resource" ]] && continue
    kubectl -n "$NAMESPACE" rollout status "$resource" --timeout=180s
  done <<<"$resources"
}

cleanup() {
  if [[ "${KEEP_PORT_FORWARDS:-false}" == "true" ]]; then
    if [[ -n "${API_PF_PID:-}" && -n "${UI_PF_PID:-}" ]]; then
      printf "\nPort-forwards left running:\n"
      printf -- "- API: http://127.0.0.1:%s (pid %s)\n" "$API_PORT" "$API_PF_PID"
      printf -- "- UI:  http://127.0.0.1:%s (pid %s)\n" "$UI_PORT" "$UI_PF_PID"
    fi
    return
  fi
  if [[ -n "${API_PF_PID:-}" ]]; then
    kill "$API_PF_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "${UI_PF_PID:-}" ]]; then
    kill "$UI_PF_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

require_command docker
require_command helm
require_command kubectl
require_command nc
require_command npm

printf "Local Kubernetes validation configuration:\n"
printf -- "- release: %s\n" "$RELEASE_NAME"
printf -- "- namespace: %s\n" "$NAMESPACE"
printf -- "- registry: %s\n" "$REGISTRY"
printf -- "- tag: %s\n" "$TAG"
printf -- "- storageClass: %s\n" "$STORAGE_CLASS"
printf -- "- docker host: %s\n" "${DOCKER_HOST:-default}"

run_step "build api image" docker build \
  -f cmd/api/Dockerfile \
  -t "$REGISTRY/clusterprobe-api:$TAG" \
  --build-arg VERSION="$VERSION" \
  --build-arg COMMIT_SHA="$COMMIT_SHA" \
  --build-arg BUILD_DATE="$BUILD_DATE" \
  .

run_step "build worker image" docker build \
  -f cmd/worker/Dockerfile \
  -t "$REGISTRY/clusterprobe-worker:$TAG" \
  --build-arg VERSION="$VERSION" \
  --build-arg COMMIT_SHA="$COMMIT_SHA" \
  --build-arg BUILD_DATE="$BUILD_DATE" \
  .

run_step "build ui image" docker build \
  -f cmd/ui/Dockerfile \
  -t "$REGISTRY/clusterprobe-ui:$TAG" \
  --build-arg VERSION="$VERSION" \
  --build-arg COMMIT_SHA="$COMMIT_SHA" \
  --build-arg BUILD_DATE="$BUILD_DATE" \
  .

run_step "helm upgrade" helm upgrade --install "$RELEASE_NAME" "$CHART_DIR" \
  -n "$NAMESPACE" \
  --create-namespace \
  --set global.storageClass="$STORAGE_CLASS" \
  --set global.imagePullPolicy=IfNotPresent \
  --set api.replicas=1 \
  --set worker.replicas=1 \
  --set ui.replicas=1 \
  --set api.image.repository="$REGISTRY/clusterprobe-api" \
  --set api.image.tag="$TAG" \
  --set worker.image.repository="$REGISTRY/clusterprobe-worker" \
  --set worker.image.tag="$TAG" \
  --set ui.image.repository="$REGISTRY/clusterprobe-ui" \
  --set ui.image.tag="$TAG" \
  --set chaos.enabled=true \
  --wait \
  --timeout "$HELM_TIMEOUT"

run_step "deployment rollout status" rollout_status_all deployment
run_step "statefulset rollout status" rollout_status_all statefulset
run_step "daemonset rollout status" rollout_status_all daemonset

API_PORT="$(choose_port "$API_PORT")"
UI_PORT="$(choose_port "$UI_PORT")"

run_step "start port-forwards" bash -c "
  kubectl -n '$NAMESPACE' port-forward svc/${RELEASE_NAME}-clusterprobe-api '$API_PORT':8080 >/tmp/clusterprobe-api-port-forward.log 2>&1 &
  echo \$! >/tmp/clusterprobe-api-port-forward.pid
  kubectl -n '$NAMESPACE' port-forward svc/${RELEASE_NAME}-clusterprobe-ui '$UI_PORT':8081 >/tmp/clusterprobe-ui-port-forward.log 2>&1 &
  echo \$! >/tmp/clusterprobe-ui-port-forward.pid
"
API_PF_PID="$(cat /tmp/clusterprobe-api-port-forward.pid)"
UI_PF_PID="$(cat /tmp/clusterprobe-ui-port-forward.pid)"

API_URL="http://127.0.0.1:$API_PORT"
UI_URL="http://127.0.0.1:$UI_PORT"
export API_URL
export UI_URL

run_step "wait for forwarded endpoints" bash -c "
  for _ in {1..60}; do
    if curl -fsS '$API_URL/healthz' >/dev/null 2>&1 && curl -fsS '$UI_URL/dashboard' >/dev/null 2>&1; then
      exit 0
    fi
    sleep 1
  done
  echo 'timed out waiting for port-forwarded endpoints' >&2
  echo 'api port-forward log:' >&2
  cat /tmp/clusterprobe-api-port-forward.log >&2 || true
  echo 'ui port-forward log:' >&2
  cat /tmp/clusterprobe-ui-port-forward.log >&2 || true
  exit 1
"

printf "\nForwarded endpoints:\n"
printf -- "- API: %s\n" "$API_URL"
printf -- "- UI:  %s\n" "$UI_URL"

run_step "full local validation" ./scripts/validate-local.sh

printf "\nLocal Kubernetes validation: PASS\n"
