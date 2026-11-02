---
title: GitHub and Runtime Credentials
description: Separate repository automation from credentials used by a deployed TemporalCI runtime.
---

## Repository Automation

The current GitHub Actions workflows run code checks and publish the documentation
site to GitHub Pages. They use GitHub's built-in workflow token and do not require
AWS access keys, an AWS role ARN, an AWS account ID, or an ECR registry.

The AWS image publisher and automatic GitOps tag updater have been removed.
There is no Terraform Apply workflow. Do not add AWS credentials or deployment
variables to this repository for routine CI.

## Runtime Credentials

A separately deployed TemporalCI runtime still needs its own configuration:

- `GITHUB_WEBHOOK_SECRET` validates GitHub webhook signatures.
- GitHub API credentials let the worker report build results.
- `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and `GITHUB_ALLOWED_USERS` configure
  operator login for the dashboard and management APIs.

Provide runtime credentials through the secret mechanism selected for that
deployment. They are distinct from repository workflow credentials. For local
development, follow the [local setup guide](/TemporalCI/getting-started/local-dev/).
The Terraform and Helm directories are operator-managed examples; repository CI
does not apply them.
