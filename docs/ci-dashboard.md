# CI Dashboard

## Overview

The CI Dashboard is a SvelteKit web application that provides real-time visibility into TemporalCI builds. It queries Temporal workflow history through the webhook server's CI API layer — no separate database required.

Source: `ui/` directory. Built with `@sveltejs/adapter-node` for server-side rendering.

## Pages

| Page | Route | Description |
|------|-------|-------------|
| Builds | `/ci/builds` | List of recent builds with status, repo, ref, duration. Filterable by repo, branch, status |
| Build Detail | `/ci/builds/{workflowId}` | Step-by-step view with status, duration, log links. Shows parameters, DAG visualization |
| Repos | `/ci/repos` | Registered repositories with latest build status, default branch, pipeline names |
| Triggers | `/ci/triggers` | Manual pipeline trigger form — select repo, pipeline, ref, and parameters |
| Analytics | `/ci/analytics` | Per-repo metrics: success rate, avg duration, failing steps, slowest steps, daily trend |

The navbar also links to the Temporal UI for direct workflow inspection.

## Authentication

GitHub OAuth, handled by the webhook server:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/github` | GET | Redirects to GitHub OAuth authorize |
| `/auth/github/callback` | GET | Exchanges code, creates session, redirects to `/dashboard` |
| `/auth/me` | GET | Returns current user (login, name, avatar) |
| `/auth/logout` | POST | Clears session |

Configure `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and a comma-separated `GITHUB_ALLOWED_USERS` operator allowlist. No users are allowed by default. OAuth state expires after ten minutes and is bound to the initiating browser.

Sessions are stored in-memory with 7-day expiry. Cookies are `HttpOnly`, `SameSite=Lax`, and `Secure` by default. For isolated loopback HTTP development only, set `SESSION_COOKIE_SECURE=false`. Restarting the process invalidates all sessions.

### PUBLIC_READ Mode

Build reads are private by default. Set `PUBLIC_READ=true` only when build metadata, parameters, and log links are intended to be public. Management endpoints (including repository configuration and manual/custom triggers) and write operations always require an allowed operator.

## API Endpoints

All endpoints are served by the webhook server under `/api/ci/`.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/ci/builds` | GET | List builds. Query params: `repo`, `branch`, `status`, `limit` (max 200) |
| `/api/ci/builds/{workflowId}` | GET | Build detail with steps, parameters, timing |
| `/api/ci/builds/{workflowId}/steps/{stepName}/log` | GET | Redirects to step log (S3 presigned URL) |
| `/api/ci/repos` | GET | List repos with latest build status and pipeline names |
| `/api/ci/repos/{owner}/{repo}/badge.svg` | GET | SVG build status badge (no auth required) |
| `/api/ci/analytics` | GET | Repo analytics. Query params: `repo` (required), `days` (default 30, max 365) |
| `/api/ci/notifications` | GET | Unread notifications. Query param: `limit` (default 20) |
| `/api/ci/notifications/read` | POST | Mark notifications read. Body: `{"ids": ["..."]}` |

### Build Status Badge

Embed in README:

```markdown
![Build Status](https://your-temporalci.example.com/api/ci/repos/owner/repo/badge.svg)
```

## Local Development

```bash
make dashboard-install   # npm ci in ui/
make dashboard-dev       # npm run dev in ui/
```

The dev server runs on the default SvelteKit port. API calls proxy to the webhook server — ensure it's running locally or configure `API_URL`.

Build for production:

```bash
make dashboard-build     # npm run build in ui/
```

## Deployment

The dashboard deploys as a separate container via the helm chart. Enable it in values:

```yaml
ciDashboard:
  enabled: true
  replicas: 1
  port: 3000
  publicRead: false
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 128Mi

githubOAuth:
  clientId: "your-github-oauth-app-id"
  existingSecret: "temporalci-oauth"
  allowedUsers: ["your-github-login"]
  secureCookies: true
```

Provide the OAuth client secret in the Kubernetes Secret named by `githubOAuth.existingSecret`, under `github-oauth-secret`; do not commit it in values files. The chart does not create this key for you. If `existingSecret` is unset, it uses `<release>-secrets`. Use an HTTPS proxy and a callback URL ending in `/auth/github/callback`.

Allowed operators can manage the entire POC instance. This is not per-repository authorization. See [SECURITY.md](../SECURITY.md) for remaining limitations.

The helm chart creates:
- A `Deployment` running the `temporalci-dashboard` image
- A `ClusterIP` Service on the configured port
- Nginx ConfigMap routing `/ci/*` to the dashboard and `/api/ci/*`, `/auth/*` to the webhook server

The dashboard container receives `API_URL` pointing to the webhook server's internal service URL for SSR API calls.

## Notifications

Two notification channels:

| Channel | Trigger | Configuration |
|---------|---------|---------------|
| Slack | Pipeline completion (pass/fail) | Per-repo `notifySlack` webhook URL set during repo registration |
| In-app | Build failed / recovered | Automatic. Stored in-memory (100 max). Polled by dashboard |

In-app notifications appear as a badge count on the navbar bell icon. Click to view, then mark as read.
