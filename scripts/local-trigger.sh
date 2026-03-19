#!/usr/bin/env bash
set -euo pipefail
REPO="${1:?Usage: local-trigger.sh owner/repo [ref]}"
REF="${2:-refs/heads/main}"
SECRET="${WEBHOOK_SECRET:-dev-secret}"

PAYLOAD=$(cat <<EOF
{"ref":"$REF","repository":{"full_name":"$REPO"},"head_commit":{"id":"local-$(date +%s)"},"pusher":{"name":"local-dev"}}
EOF
)

SIG=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')

echo "Triggering workflow for $REPO @ $REF"
curl -s -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  -d "$PAYLOAD"
echo ""
