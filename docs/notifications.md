# Notifications

ClusterProbe uses ntfy for lightweight CI notifications via `scripts/notify.sh`.

Environment variables:
- `CP_NTFY_URL`: Base ntfy server URL (for example, https://ntfy.sh)
- `CP_NTFY_TOPIC`: Topic name to publish to
- `CP_NTFY_TOKEN`: Optional bearer token for authenticated topics
