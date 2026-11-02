# Security and project status

TemporalCI is an experimental POC. It is not a hardened multi-tenant CI service and does not have a production support or security-response SLA.

## Reporting a vulnerability

Send security reports privately to **andrekurait@gmail.com**. Include the affected commit, a minimal reproduction, impact, and a suggested fix if available. Do not include live credentials or private build logs in a public issue.

## Operator boundary

- GitHub webhooks require HMAC-SHA256 verification with `GITHUB_WEBHOOK_SECRET`. An unset secret rejects webhook requests.
- Dashboard sessions use GitHub OAuth. Configure `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and a comma-separated `GITHUB_ALLOWED_USERS` list. An empty allowlist grants no operator access.
- OAuth state is bound to the initiating browser, expires after ten minutes, and can be used once. Session cookies are `HttpOnly`, `SameSite=Lax`, and `Secure` by default. Set `SESSION_COOKIE_SECURE=false` only for an isolated loopback HTTP development session.
- Management endpoints, custom triggers, and repository configuration always require an allowed operator, even if public build reads are enabled. Allowed operators can manage the whole instance; there is no per-repository role model.
- `PUBLIC_READ=true` exposes build metadata, parameters, and log links. Enable it only for builds whose contents are intended to be public. Status badges are public independently.
- Sessions, OAuth state, and notifications are held in memory. Restarting the webhook process invalidates sessions. Multiple replicas require shared session storage or another authentication design.

## Execution and secrets

Run only repositories and pipeline definitions you trust, using disposable infrastructure and narrowly scoped credentials. Do not connect an instance with privileged credentials to untrusted pull-request code.

The worker can run commands locally when no in-cluster Kubernetes client is available. Kubernetes steps using `privileged: true` or `docker: true` require the repository in `PRIVILEGED_REPOS` (`worker.privilegedRepos` in Helm); an empty list denies both. Kubernetes pods, optional network policies, and privileged-repository controls are not a complete tenant isolation boundary. Review the effective RBAC, pod configuration, CNI enforcement, and AWS permissions for your environment.

Activity arguments and results are recorded in Temporal history. The current secret-fetching and AWS-role activities can place credential values in that history. Log masking does not remove those values from history. Protect Temporal and its database, and do not use production secrets with this POC. Credential delivery outside workflow payloads or payload encryption requires additional design.

Keep Temporal, PostgreSQL, LocalStack, and worker metrics private. Expose the webhook/dashboard through an HTTPS reverse proxy only after configuring OAuth and the operator allowlist. The native webhook binds to loopback by default; the Helm container binds inside its pod behind a `ClusterIP` service.

## Deployment examples

The checked-in EKS and load-balancer examples are not a production security baseline. Load-balancer exposure is disabled by default; enabling it does not provision TLS or shared authentication state for you. Temporal and PostgreSQL use the existing `temporalci-database` Secret (`postgres-password` key); create it through your secret-management process before deploying. The local setup script generates development credentials automatically. No deployment is needed to run the unit tests.
