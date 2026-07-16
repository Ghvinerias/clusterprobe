#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

API_URL="${API_URL:-http://127.0.0.1:8080}"
UI_URL="${UI_URL:-http://127.0.0.1:8081}"
HELM_NAMESPACE="${HELM_NAMESPACE:-cluster-probe}"
KUSTOMIZE_DIR="${KUSTOMIZE_DIR:-deploy/kustomize/overlays/self-contained}"
KUSTOMIZE_FLAGS="${KUSTOMIZE_FLAGS:---load-restrictor=LoadRestrictionsNone}"

if [[ -z "${DOCKER_HOST:-}" && -S "$HOME/.colima/default/docker.sock" ]]; then
  export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
fi

if [[ -n "${DOCKER_HOST:-}" && "${TESTCONTAINERS_RYUK_DISABLED:-}" == "" ]]; then
  export TESTCONTAINERS_RYUK_DISABLED=true
fi

if [[ -z "${CHROMIUM_EXECUTABLE_PATH:-}" && -x /Applications/Chromium.app/Contents/MacOS/Chromium ]]; then
  export CHROMIUM_EXECUTABLE_PATH=/Applications/Chromium.app/Contents/MacOS/Chromium
fi

export API_URL
export UI_URL

run_step() {
  local name="$1"
  shift
  printf "\n==> %s\n" "$name"
  "$@"
}

run_step "go test" go test ./...
run_step "go build" go build ./...
run_step "go race test" go test -vet=off ./... -race
run_step "integration test" go test -tags=integration ./integration ./internal/db ./internal/messaging
run_step "helm lint" helm lint deploy/helm/clusterprobe
run_step "helm template" bash -c "helm template clusterprobe deploy/helm/clusterprobe --namespace \"$HELM_NAMESPACE\" >/tmp/clusterprobe-rendered.yaml"
run_step "kustomize render" kubectl kustomize "$KUSTOMIZE_DIR" $KUSTOMIZE_FLAGS >/tmp/clusterprobe-kustomize-rendered.yaml
run_step "product smoke" ./scripts/smoke-local.sh
run_step "browser smoke" npm run test:ui
run_step "diff check" git diff --check

printf "\nLocal validation: PASS\n"
