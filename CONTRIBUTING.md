# Contributing to TemporalCI

TemporalCI is a proof of concept. Small, tested changes that clarify behavior or make the experiment reproducible are especially useful.

## Development

1. Fork the repository and create a branch.
2. Install Go 1.24.1 or newer. Install Node.js 24 and npm if working on `ui/` or `site/`.
3. Run `make check`. These checks use local mocks and Temporal's test environment; no cloud deployment is required.
4. For dashboard changes, run `make dashboard-install dashboard-build`. For documentation-site changes, run `npm --prefix site ci` and `npm --prefix site run build`.

Format changed Go files with `gofmt`. Commit `go.sum` or the relevant npm lockfile when changing dependencies. Keep checksum verification enabled, and make dependency installation failures visible.

## Tests and pull requests

Include a regression test when fixing execution, authorization, or configuration behavior. Prefer testing the real handler or workflow boundary over copying its implementation into a test helper. Describe the triggering condition, resulting behavior, validation performed, and any compatibility changes in the pull request.

Do not run `make bootstrap`, `make deploy`, Terraform, or the EKS scripts as part of routine verification. Those commands can create resources or change a deployment. The main-branch Docker workflow can also publish images and update GitOps state; branch and pull-request CI is the review path.

Do not commit credentials, real webhook payloads containing private data, or environment-specific infrastructure identifiers. See [SECURITY.md](SECURITY.md) for private vulnerability reporting and the current trust model.
