# Notifications

ClusterProbe includes `scripts/notify.sh` for lightweight ntfy notifications from CI or local release scripts.

## Environment variables

- `CP_NTFY_URL`: Base ntfy server URL, for example `https://ntfy.sh`.
- `CP_NTFY_TOPIC`: Topic name to publish to.
- `CP_NTFY_TOKEN`: Optional bearer token for authenticated topics.

If `CP_NTFY_URL` is unset, the script exits successfully and prints `ntfy not configured, skipping`.

## Example

```bash
export CP_NTFY_URL=https://ntfy.sh
export CP_NTFY_TOPIC=clusterprobe-builds

scripts/notify.sh \
  --phase phase-13 \
  --event review_pass \
  --message "Helm, Kustomize, and Go validation passed" \
  --commits "$(git rev-parse --short HEAD)"
```

Supported event labels are `agent_done`, `review_pass`, and `review_fail`. Other labels are allowed and use the default informational priority.
