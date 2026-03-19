---
title: Troubleshooting
description: Common issues and solutions for TemporalCI.
---

## Helm Install Fails

### Temporal server won't start

**Symptom:** `temporalci-temporal-frontend` pod is in CrashLoopBackOff.

**Cause:** Usually a database connection issue.

```bash
kubectl logs -l app.kubernetes.io/component=frontend -n temporalci
```

**Fix (local):** Ensure PostgreSQL subchart is running:
```bash
kubectl get pods -l app.kubernetes.io/name=postgresql
```

**Fix (production):** Check that the ACK `DBInstance` is ready:
```bash
kubectl get dbinstance temporalci-db -o jsonpath='{.status.dbInstanceStatus}'
# Should be "available"
```

### Worker can't connect to Temporal

**Symptom:** Worker logs show `connection refused` or `context deadline exceeded`.

**Fix:** Verify the Temporal frontend service is reachable:
```bash
kubectl exec -it deploy/temporalci-worker -- \
  nc -zv temporalci-temporal-frontend 7233
```

## Webhooks Not Triggering

### No workflow starts on push

1. **Check webhook delivery** in GitHub → Settings → Webhooks → Recent Deliveries
2. **Check webhook server logs:**
   ```bash
   kubectl logs -l app=temporalci-webhook
   ```
3. **Common causes:**
   - Wrong webhook secret (signature validation fails)
   - Webhook URL not reachable from GitHub
   - Wrong event types selected (need Push and Pull Request)

### Signature validation failed

**Symptom:** Webhook server returns 401, logs show "invalid signature".

**Fix:** Ensure the webhook secret matches between GitHub and the K8s secret:
```bash
# Check what's in the secret
kubectl get secret temporalci-secrets -o jsonpath='{.data.github-webhook-secret}' | base64 -d
```

## CI Pods

### Pods stuck in Pending

**Cause:** Usually insufficient resources or missing node pool.

```bash
kubectl describe pod ci-<workflow-id>-<step>
```

Look for events like:
- `FailedScheduling` — no nodes match tolerations/selectors
- `Insufficient cpu/memory` — node pool needs to scale

**Fix (local):** Increase minikube resources:
```bash
minikube start --cpus=4 --memory=4096
```

### Pods fail with OOMKilled

**Cause:** Step exceeds memory limits.

**Fix:** Increase resource limits in `.temporalci.yaml` or Helm values:
```yaml
# In Helm values
worker:
  ciPod:
    resources:
      limits:
        memory: "4Gi"
```

## Temporal Web UI

### Can't access the UI

```bash
kubectl port-forward svc/temporalci-temporal-web 8088:8088 -n temporalci
```

Open [http://localhost:8088](http://localhost:8088).

### Workflow shows "timed out"

The workflow-level timeout was exceeded. Check:
1. Which activity was running when it timed out
2. Whether the activity's own timeout is too short
3. Whether the K8s pod was actually running (vs stuck in Pending)

## Logs

### Where are the logs?

| Environment | CI step logs | Infrastructure logs |
|-------------|-------------|-------------------|
| Local | `kubectl logs <pod-name>` | `kubectl logs -l app=temporalci-worker` |
| Production | S3 presigned URL (in PR comment) | CloudWatch Logs |

### S3 log upload fails

**Symptom:** PR comment shows results but no "Full log" link.

**Check:** Worker Pod Identity has S3 write permissions:
```bash
kubectl describe sa temporalci-worker -n temporalci
# Look for eks.amazonaws.com/role-arn annotation
```
