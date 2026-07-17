#!/usr/bin/env bash
set -euo pipefail

status=0
summary=()

run_step() {
  local name="$1"
  shift
  if "$@"; then
    summary+=("${name}: PASS")
  else
    summary+=("${name}: FAIL")
    status=1
  fi
}

require_command() {
  local command="$1"
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing required command: $command" >&2
    return 1
  fi
}

run_required() {
  local name="$1"
  local command="$2"
  shift 2
  if require_command "$command"; then
    run_step "$name" "$command" "$@"
  else
    summary+=("${name}: FAIL")
    status=1
  fi
}

run_required "golangci-lint" golangci-lint run
run_required "gosec" gosec ./...
if require_command govulncheck; then
  run_step "govulncheck" env GOTOOLCHAIN=go1.25.12 govulncheck ./...
else
  summary+=("govulncheck: FAIL")
  status=1
fi
run_required "secret scan" gitleaks detect --source . --no-git --redact
run_step "go race coverage" env GOTOOLCHAIN=go1.25.12 go test ./... -race -coverprofile=coverage.out
run_step "internal coverage threshold" ./scripts/check-internal-coverage.sh
run_step "diff check" git diff --check

printf "\nReview summary:\n"
for line in "${summary[@]}"; do
  printf -- "- %s\n" "$line"
done

if [[ $status -eq 0 ]]; then
  printf "\nReview: PASS\n"
else
  printf "\nReview: FAIL\n"
fi

exit $status
