---
title: Quick Start
description: Get TemporalCI running on your repo in 5 minutes.
---

## 1. Add a Pipeline Config

Create `.temporalci.yaml` in your repository root:

```yaml
steps:
  - name: build
    image: golang:1.26
    command: go build ./...
    timeout: 5m

  - name: test
    image: golang:1.26
    command: go test -v ./...
    timeout: 5m
    depends_on: [build]
```

## 2. Register the Webhook

```bash
curl -X POST http://<webhook-url>/api/repos \
  -H "Content-Type: application/json" \
  -d '{"fullName": "owner/repo", "defaultBranch": "main"}'
```

Or configure it manually in GitHub → Settings → Webhooks:
- **Payload URL:** `https://your-webhook-endpoint/webhook`
- **Content type:** `application/json`
- **Secret:** your webhook secret
- **Events:** Pushes and Pull requests

## 3. Push Code

Every push and PR triggers a pipeline. Results appear on the commit and PR:

```
## TemporalCI Results

✅ build (12.5s)
✅ test (20.2s)

2 passed, 0 failed in 32.7s

🔗 View workflow run
```

## What Happens Under the Hood

1. GitHub sends a webhook event to the TemporalCI webhook server
2. The webhook server validates the signature and starts a `CIPipeline` Temporal workflow
3. The workflow clones the repo, reads `.temporalci.yaml`, and executes steps as K8s pods
4. Results are reported back to GitHub as commit status and PR comments
5. Full logs are uploaded to S3 with presigned URLs

If anything crashes mid-pipeline, Temporal replays the workflow and resumes from the last completed activity.
