#!/usr/bin/env python3
"""Prepare local Secrets without rotating credentials or changing an existing PVC."""

import base64
import json
import secrets
import subprocess
import sys

NAMESPACE = "temporalci"


def kubectl(*args, document=None):
    result = subprocess.run(
        ["kubectl", *args, "--context", "minikube", "-n", NAMESPACE],
        input=json.dumps(document) if document is not None else None,
        text=True,
        capture_output=True,
        check=True,
    )
    if args[0] == "get" and result.stdout.strip():
        return json.loads(result.stdout)
    return None


def get_secret(name):
    return kubectl("get", "secret", name, "--ignore-not-found", "-o", "json")


def encoded_random():
    return base64.b64encode(secrets.token_hex(32).encode()).decode()


def require_keys(secret, *keys):
    data = secret.get("data", {})
    for key in keys:
        if key not in data:
            raise ValueError(f"{secret['metadata']['name']} is missing {key}; refusing to replace credentials")
        try:
            decoded = base64.b64decode(data[key], validate=True)
        except (ValueError, TypeError) as error:
            raise ValueError(f"{secret['metadata']['name']} has invalid {key} data") from error
        if key != "github-token" and not decoded:
            raise ValueError(f"{secret['metadata']['name']} has empty {key}; refusing to replace credentials")
    return data


def prepare():
    database = get_secret("temporalci-database")
    old_database = get_secret("temporalci-postgresql")
    webhook = get_secret("temporalci-local-secrets")
    old_webhook = get_secret("temporalci-secrets")
    pvcs = kubectl("get", "pvc", "-o", "json")
    releases = kubectl("get", "secret", "-l", "owner=helm,name=temporalci", "-o", "json")
    existing_install = bool(pvcs["items"] or releases["items"] or old_database or old_webhook)
    pending = []

    if database:
        require_keys(database, "postgres-password")
    elif old_database:
        data = require_keys(old_database, "postgres-password")
        pending.append(("temporalci-database", {"postgres-password": data["postgres-password"]}))
    elif existing_install:
        raise ValueError(
            "Existing local state has no recoverable PostgreSQL Secret. Restore the current "
            "database password into temporalci-database (key postgres-password) before retrying. "
            "No password was generated and no PVC was changed."
        )
    else:
        pending.append(("temporalci-database", {"postgres-password": encoded_random()}))

    if webhook:
        require_keys(webhook, "github-webhook-secret", "github-token")
    elif old_webhook:
        # Copy only data, never Helm ownership metadata. Helm may delete the old
        # release-owned Secret during the upgrade; the new reference survives.
        data = require_keys(old_webhook, "github-webhook-secret", "github-token")
        pending.append(("temporalci-local-secrets", data))
    elif existing_install:
        raise ValueError(
            "Existing local state has no webhook Secret. Restore temporalci-local-secrets "
            "(keys github-webhook-secret and github-token) before retrying."
        )
    else:
        pending.append(("temporalci-local-secrets", {
            "github-webhook-secret": encoded_random(),
            "github-token": "",
        }))

    # Validate every source before creating anything. Never update an existing
    # Secret: a concurrent creation fails safely and a rerun reuses that Secret.
    for name, data in pending:
        kubectl("create", "-f", "-", document={
            "apiVersion": "v1",
            "kind": "Secret",
            "metadata": {"name": name, "namespace": NAMESPACE},
            "type": "Opaque",
            "data": data,
        })
        print(f"Prepared {name}")


if __name__ == "__main__":
    try:
        prepare()
    except subprocess.CalledProcessError:
        # Do not include command output: kubectl errors may echo Secret data.
        sys.exit("Local Secret preparation failed: kubectl could not read or create a resource. No upgrade was started.")
    except (ValueError, KeyError) as error:
        sys.exit(str(error))
