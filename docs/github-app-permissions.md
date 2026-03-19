# GitHub App Permissions

TemporalCI uses a GitHub App for authentication. The required permissions depend on which features you use.

## Required Permissions

| Permission | Access | Features |
|-----------|--------|----------|
| `checks` | write | Check Runs API (step results on PRs) |
| `statuses` | write | Commit status updates |
| `pull_requests` | write | PR comments, backport PR creation |
| `contents` | write | Branch deletion, cherry-pick, tag operations |
| `issues` | write | Label management (add-untriaged workflow) |
| `security_events` | write | CodeQL SARIF upload |
| `metadata` | read | Repository metadata (always required) |

## Webhook Events

Subscribe to these events:
- `push` — branch and tag pushes
- `pull_request` — PR opened, synchronized, closed, labeled
- `issues` — issue opened, reopened, transferred
- `release` — release published, created

## Least Privilege

TemporalCI requests only the permissions needed for the current operation:
- CI pipeline runs: `checks:write`, `statuses:write`, `pull_requests:write`
- CodeQL scans: adds `security_events:write`
- Backport workflows: adds `contents:write`
- Issue labeling: adds `issues:write`
