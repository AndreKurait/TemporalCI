"""Offline regression tests; every kubectl invocation is mocked."""

import base64
import importlib.util
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    "local_secrets", Path(__file__).with_name("prepare-local-secrets.py")
)
local_secrets = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(local_secrets)


def secret(name, **values):
    return {
        "metadata": {"name": name, "annotations": {"meta.helm.sh/release-name": "temporalci"}},
        "data": {key: base64.b64encode(value.encode()).decode() for key, value in values.items()},
    }


class LocalSecretsTest(unittest.TestCase):
    def setUp(self):
        self.secrets = {}
        self.pvcs = []
        self.releases = []
        self.created = []
        self.calls = []
        self.failure = None
        self.mock = patch.object(local_secrets.subprocess, "run", side_effect=self.run_kubectl)
        self.mock.start()
        self.addCleanup(self.mock.stop)

    def run_kubectl(self, args, *, input, **kwargs):
        self.calls.append(args)
        self.assertEqual(args[0], "kubectl")
        self.assertEqual(args[-2:], ["-n", "temporalci"])
        self.assertEqual(args[-4:-2], ["--context", "minikube"])
        self.assertTrue(kwargs["check"])
        if self.failure:
            raise self.failure
        command = args[1:-4]
        if command[0] == "create":
            body = json.loads(input)
            self.created.append(body)
            self.secrets[body["metadata"]["name"]] = body
            return subprocess.CompletedProcess(args, 0, "secret/example created\n")
        if command[1] == "pvc":
            output = {"items": self.pvcs}
        elif command[2] == "-l":
            output = {"items": self.releases}
        else:
            self.assertIn("--ignore-not-found", command)
            output = self.secrets.get(command[2])
        return subprocess.CompletedProcess(args, 0, json.dumps(output) if output else "")

    def test_fresh_install_creates_random_external_secrets_and_rerun_preserves_them(self):
        local_secrets.prepare()
        self.assertEqual(len(self.created), 2)
        database, webhook = self.created
        self.assertTrue(base64.b64decode(database["data"]["postgres-password"]))
        self.assertTrue(base64.b64decode(webhook["data"]["github-webhook-secret"]))
        self.assertEqual(webhook["data"]["github-token"], "")
        self.assertNotIn("annotations", webhook["metadata"])
        local_secrets.prepare()
        self.assertEqual(len(self.created), 2)

    def test_old_helm_credentials_are_copied_without_ownership_or_rotation(self):
        old_db = secret("temporalci-postgresql", **{"postgres-password": "existing-pvc-password"})
        old_hook = secret("temporalci-secrets", **{
            "github-webhook-secret": "registered-webhook",
            "github-token": "existing-token",
            "github-oauth-secret": "existing-oauth",
        })
        self.secrets = {
            "temporalci-postgresql": old_db,
            "temporalci-secrets": old_hook,
        }
        self.pvcs = [{"metadata": {"name": "data-temporalci-postgresql-0"}}]
        self.releases = [{"metadata": {"name": "sh.helm.release.v1.temporalci.v1"}}]
        local_secrets.prepare()
        self.assertEqual(self.created[0]["data"], old_db["data"])
        self.assertEqual(self.created[1]["data"], old_hook["data"])
        self.assertEqual(self.created[1]["metadata"], {
            "name": "temporalci-local-secrets", "namespace": "temporalci",
        })

    def test_orphaned_pvc_requires_recovered_database_password(self):
        self.pvcs = [{"metadata": {"name": "data-temporalci-postgresql-0"}}]
        with self.assertRaisesRegex(ValueError, "no recoverable PostgreSQL Secret"):
            local_secrets.prepare()
        self.assertEqual(self.created, [])

    def test_prior_release_without_password_does_not_generate_one(self):
        self.releases = [{"metadata": {"name": "old-release"}}]
        with self.assertRaisesRegex(ValueError, "no recoverable PostgreSQL Secret"):
            local_secrets.prepare()
        self.assertEqual(self.created, [])

    def test_missing_old_webhook_aborts_before_any_creates(self):
        self.secrets["temporalci-postgresql"] = secret(
            "temporalci-postgresql", **{"postgres-password": "test"}
        )
        with self.assertRaisesRegex(ValueError, "no webhook Secret"):
            local_secrets.prepare()
        self.assertEqual(self.created, [])

    def test_invalid_existing_database_does_not_rotate(self):
        self.secrets["temporalci-database"] = secret("temporalci-database", **{"postgres-password": ""})
        with self.assertRaisesRegex(ValueError, "empty postgres-password"):
            local_secrets.prepare()
        self.assertEqual(self.created, [])

    def test_incomplete_old_webhook_is_not_silently_replaced(self):
        self.secrets["temporalci-postgresql"] = secret(
            "temporalci-postgresql", **{"postgres-password": "test"}
        )
        self.secrets["temporalci-secrets"] = secret(
            "temporalci-secrets", **{"github-webhook-secret": "keep-me"}
        )
        with self.assertRaisesRegex(ValueError, "missing github-token"):
            local_secrets.prepare()
        self.assertEqual(self.created, [])

    def test_kubectl_permission_failure_is_not_treated_as_missing_secret(self):
        self.failure = subprocess.CalledProcessError(1, ["kubectl"], stderr="Forbidden")
        with self.assertRaises(subprocess.CalledProcessError):
            local_secrets.prepare()
        self.assertEqual(self.created, [])


if __name__ == "__main__":
    unittest.main()
