#!/usr/bin/env bash
set -euo pipefail

phase=""
event=""
message=""
commits=""
warnings=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --phase)
      phase="$2"
      shift 2
      ;;
    --event)
      event="$2"
      shift 2
      ;;
    --message)
      message="$2"
      shift 2
      ;;
    --commits)
      commits="$2"
      shift 2
      ;;
    --warnings)
      warnings="$2"
      shift 2
      ;;
    *)
      echo "unknown flag: $1" >&2
      exit 1
      ;;
  esac
 done

if [[ -z "${CP_NTFY_URL:-}" ]]; then
  echo "ntfy not configured, skipping"
  exit 0
fi

priority=3
tags="information_source"
case "$event" in
  agent_done)
    priority=3
    tags="white_check_mark"
    ;;
  review_pass)
    priority=4
    tags="rocket"
    ;;
  review_fail)
    priority=5
    tags="rotating_light"
    ;;
 esac

event_label="$event"
title="[ClusterProbe] ${phase} — ${event_label}"
body="$message"

if [[ -n "$commits" ]]; then
  body+=$'\n'"Commits: ${commits}"
fi
if [[ -n "$warnings" ]]; then
  body+=$'\n'"Soft warnings: ${warnings}"
fi

ntfy_url="${CP_NTFY_URL%/}/${CP_NTFY_TOPIC}"

curl_args=(
  -sS
  -X POST
  -H "Title: ${title}"
  -H "Priority: ${priority}"
  -H "Tags: ${tags}"
  -H "Click: https://github.com/Ghvinerias/clusterprobe/commits/main"
  -d "${body}"
)

if [[ -n "${CP_NTFY_TOKEN:-}" ]]; then
  curl_args+=( -H "Authorization: Bearer ${CP_NTFY_TOKEN}" )
fi

curl "${curl_args[@]}" "${ntfy_url}"
