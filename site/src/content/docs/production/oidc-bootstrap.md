---
title: GitHub AWS Publication Removed
description: The former GitHub Actions AWS publisher and bootstrap integration are no longer active.
---

The former AWS image publisher and automatic GitOps tag updater have been
removed. Repository CI no longer requests AWS credentials or assumes an AWS
role. No AWS role or account setup is needed for the current code checks.

The GitHub Pages workflow retains the identity-token permission required by
GitHub Pages. That permission is used for documentation publication, not AWS
authentication.

This page preserves the former bootstrap guide's URL. Its AWS onboarding
instructions have been withdrawn. Terraform and Helm remain operator-managed
examples; they are not applied by repository CI. Review any infrastructure
example independently before using it in an account you control.
