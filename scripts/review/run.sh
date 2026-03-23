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

run_step "golangci-lint" golangci-lint run
run_step "gosec" gosec ./...
run_step "go test" go test ./... -race -coverprofile=coverage.out

printf "\nReview summary:\n"
for line in "${summary[@]}"; do
  printf "- %s\n" "$line"
done

if [[ $status -eq 0 ]]; then
  printf "\nReview: PASS\n"
else
  printf "\nReview: FAIL\n"
fi

exit $status
