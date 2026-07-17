#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

REGISTRY="${REGISTRY:-docker.io/slickg}"
COMMIT_SHA="${COMMIT_SHA:-$(git rev-parse --short HEAD)}"
TAG="${TAG:-$COMMIT_SHA}"
SERVICES="${SERVICES:-api worker ui chaos-ctrl}"
VERIFY_PULL="${VERIFY_PULL:-false}"

require_command() {
  local command="$1"
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing required command: $command" >&2
    exit 1
  fi
}

image_ref() {
  local service="$1"
  printf "%s/clusterprobe-%s:%s" "$REGISTRY" "$service" "$TAG"
}

manifest_exists() {
  local image="$1"
  docker manifest inspect "$image" >/dev/null 2>&1
}

local_image_exists() {
  local image="$1"
  docker image inspect "$image" >/dev/null 2>&1
}

ensure_local_image() {
  local image="$1"
  if local_image_exists "$image"; then
    return 0
  fi
  if [[ "$VERIFY_PULL" == "true" ]]; then
    docker pull "$image" >/dev/null
    return 0
  fi
  return 1
}

inspect_label() {
  local image="$1"
  local label="$2"
  docker image inspect \
    --format "{{ index .Config.Labels \"$label\" }}" \
    "$image"
}

verify_local_labels() {
  local image="$1"
  local revision
  revision="$(inspect_label "$image" "org.opencontainers.image.revision")"
  if [[ "$revision" != "$COMMIT_SHA" ]]; then
    echo "image $image has revision label $revision, expected $COMMIT_SHA" >&2
    exit 1
  fi
}

verify_chaos_binary_metadata() {
  local image="$1"
  local output
  output="$(docker run --rm "$image" version -o json 2>/dev/null || true)"
  if [[ -z "$output" ]]; then
    echo "warning: unable to read chaos-ctrl binary metadata from $image" >&2
    return 0
  fi
  if ! grep -Eq "\"commit_sha\"[[:space:]]*:[[:space:]]*\"$COMMIT_SHA\"" <<<"$output"; then
    echo "image $image reports unexpected chaos-ctrl metadata: $output" >&2
    exit 1
  fi
}

require_command docker

printf "Verifying ClusterProbe release images:\n"
printf -- "- registry: %s\n" "$REGISTRY"
printf -- "- tag: %s\n" "$TAG"
printf -- "- commit_sha: %s\n" "$COMMIT_SHA"

for service in $SERVICES; do
  image="$(image_ref "$service")"
  printf "\n==> %s\n" "$image"

  if ensure_local_image "$image"; then
    echo "local image: present"
    verify_local_labels "$image"
    echo "revision label: ok"
    if [[ "$service" == "chaos-ctrl" ]]; then
      verify_chaos_binary_metadata "$image"
      echo "chaos-ctrl metadata: ok"
    fi
    continue
  fi

  if manifest_exists "$image"; then
    echo "remote manifest: present"
    echo "local labels: skipped; set VERIFY_PULL=true to pull and validate OCI labels"
    continue
  fi

  echo "image not found locally or in remote registry: $image" >&2
  exit 1
done

printf "\nRelease image verification: PASS\n"
