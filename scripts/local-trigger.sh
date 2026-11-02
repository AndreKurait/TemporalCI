#!/usr/bin/env bash
set -euo pipefail
REPO="${1:?Usage: local-trigger.sh owner/repo [ref]}"
REF="${2:-refs/heads/main}"
SECRET="${WEBHOOK_SECRET:-}"
if [[ -z "$SECRET" ]]; then
  SECRET="$(kubectl --context minikube get secret temporalci-local-secrets -n temporalci \
    -o jsonpath='{.data.github-webhook-secret}' | openssl base64 -d -A)"
fi
if [[ -z "$SECRET" ]]; then
  echo "Set WEBHOOK_SECRET or create the local temporalci-local-secrets Secret." >&2
  exit 1
fi

PAYLOAD=$(cat <<EOF
{"ref":"$REF","repository":{"full_name":"$REPO"},"head_commit":{"id":"local-$(date +%s)"},"pusher":{"name":"local-dev"}}
EOF
)

SIG=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')

echo "Triggering workflow for $REPO @ $REF"
curl --fail-with-body -sS -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  -d "$PAYLOAD"
echo ""
