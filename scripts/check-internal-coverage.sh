#!/usr/bin/env bash
set -euo pipefail

threshold="${INTERNAL_COVERAGE_THRESHOLD:-70.0}"
profile_owned=0

if [[ -n "${COVERPROFILE:-}" ]]; then
  profile="$COVERPROFILE"
else
  profile="$(mktemp -t clusterprobe-internal-cover.XXXXXX)"
  profile_owned=1
fi

cleanup() {
  if [[ "$profile_owned" -eq 1 ]]; then
    rm -f "$profile"
  fi
}
trap cleanup EXIT

GOTOOLCHAIN="${GOTOOLCHAIN:-go1.25.12}" go test ./internal/... -coverprofile="$profile"

total_line="$(go tool cover -func="$profile" | awk '/^total:/ {print $3}')"
if [[ -z "$total_line" ]]; then
  echo "unable to read total internal coverage" >&2
  exit 1
fi

total="${total_line%%%}"
if ! awk -v total="$total" -v threshold="$threshold" 'BEGIN { exit !(total + 0 >= threshold + 0) }'; then
  printf 'internal coverage %.1f%% is below required %.1f%%\n' "$total" "$threshold" >&2
  exit 1
fi

printf 'internal coverage %.1f%% meets required %.1f%%\n' "$total" "$threshold"
