#!/usr/bin/env bash
set -euo pipefail

# TemporalCI E2E Validation Script
# Run after deployment to verify all Q1-Q3 features work end-to-end.
#
# Prerequisites:
#   - kubectl configured for the temporalci cluster
#   - AWS CLI configured
#   - Webhook LB hostname known

NAMESPACE="${NAMESPACE:-temporalci}"
WEBHOOK_HOST="${WEBHOOK_HOST:-}"
TIMEOUT=120

echo "=== TemporalCI E2E Validation ==="
echo "Namespace: $NAMESPACE"
echo ""

# --- 1. Cluster Health ---
echo "--- 1. Cluster Health ---"
kubectl get nodes -o wide
echo ""

# --- 2. Pod Status ---
echo "--- 2. Pod Status ---"
kubectl get pods -n "$NAMESPACE" -o wide
echo ""

# Check all pods are Running
NOT_RUNNING=$(kubectl get pods -n "$NAMESPACE" --no-headers | grep -v Running | grep -v Completed | wc -l)
if [ "$NOT_RUNNING" -gt 0 ]; then
  echo "WARNING: $NOT_RUNNING pods not in Running state"
  kubectl get pods -n "$NAMESPACE" | grep -v Running | grep -v Completed
fi

# --- 3. Worker Health ---
echo "--- 3. Worker Health ---"
WORKER_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=ci-worker -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$WORKER_POD" ]; then
  echo "Worker pod: $WORKER_POD"
  kubectl logs "$WORKER_POD" -n "$NAMESPACE" --tail=5
else
  echo "ERROR: No worker pod found"
fi
echo ""

# --- 4. Webhook Health ---
echo "--- 4. Webhook Health ---"
WEBHOOK_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=webhook -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$WEBHOOK_POD" ]; then
  echo "Webhook pod: $WEBHOOK_POD"
  # Port-forward and test health endpoint
  kubectl port-forward "$WEBHOOK_POD" -n "$NAMESPACE" 18080:8080 &
  PF_PID=$!
  sleep 2
  curl -s http://localhost:18080/health && echo ""
  curl -s http://localhost:18080/status | python3 -m json.tool 2>/dev/null || true
  kill $PF_PID 2>/dev/null || true
fi
echo ""

# --- 5. Metrics Endpoint ---
echo "--- 5. Metrics Endpoint ---"
if [ -n "$WORKER_POD" ]; then
  kubectl port-forward "$WORKER_POD" -n "$NAMESPACE" 19090:9090 &
  PF_PID=$!
  sleep 2
  METRICS=$(curl -s http://localhost:19090/metrics 2>/dev/null | grep -c "ci_" || echo "0")
  echo "Prometheus metrics found: $METRICS ci_* metrics"
  kill $PF_PID 2>/dev/null || true
fi
echo ""

# --- 6. Temporal Server ---
echo "--- 6. Temporal Server ---"
kubectl get svc -n "$NAMESPACE" | grep -E "frontend|web"
echo ""

# --- 7. RBAC ---
echo "--- 7. RBAC Verification ---"
kubectl get role,rolebinding -n "$NAMESPACE" | grep temporalci
echo ""

# --- 8. ServiceAccounts ---
echo "--- 8. ServiceAccounts ---"
kubectl get sa -n "$NAMESPACE" | grep temporalci
echo ""

# --- 9. Repo Registration API ---
echo "--- 9. Repo Registration API ---"
if [ -n "$WEBHOOK_POD" ]; then
  kubectl port-forward "$WEBHOOK_POD" -n "$NAMESPACE" 18080:8080 &
  PF_PID=$!
  sleep 2

  # Register a test repo
  echo "Registering test repo..."
  curl -s -X POST http://localhost:18080/api/repos \
    -H "Content-Type: application/json" \
    -d '{"fullName":"test/repo","defaultBranch":"main"}' | python3 -m json.tool 2>/dev/null || true

  # List repos
  echo "Listing repos..."
  curl -s http://localhost:18080/api/repos | python3 -m json.tool 2>/dev/null || true

  # Delete test repo
  curl -s -X DELETE http://localhost:18080/api/repos/test/repo

  # Dashboard
  echo "Dashboard accessible:"
  curl -s -o /dev/null -w "%{http_code}" http://localhost:18080/dashboard
  echo ""

  kill $PF_PID 2>/dev/null || true
fi
echo ""

# --- 10. Rate Limiting ---
echo "--- 10. Rate Limiting ---"
if [ -n "$WEBHOOK_POD" ]; then
  kubectl port-forward "$WEBHOOK_POD" -n "$NAMESPACE" 18080:8080 &
  PF_PID=$!
  sleep 2

  echo "Sending 65 rapid requests to test rate limiting..."
  RATE_LIMITED=0
  for i in $(seq 1 65); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:18080/webhook -d '{}' -H "X-GitHub-Event: ping" 2>/dev/null)
    if [ "$CODE" = "429" ]; then
      RATE_LIMITED=1
      echo "Rate limited at request $i (429 Too Many Requests) ✅"
      break
    fi
  done
  if [ "$RATE_LIMITED" = "0" ]; then
    echo "WARNING: Rate limiting did not trigger"
  fi

  kill $PF_PID 2>/dev/null || true
fi
echo ""

# --- Summary ---
echo "=== Validation Complete ==="
echo ""
echo "Manual checks needed:"
echo "  - Push to a test repo to trigger full CI pipeline"
echo "  - Verify Check Runs appear on GitHub PR"
echo "  - Verify S3 log upload (requires LOG_BUCKET configured)"
echo "  - Verify Slack notification (requires webhook URL configured)"
echo "  - Verify pod scheduling on ci-jobs NodePool (requires Auto Mode)"
