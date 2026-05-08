#!/usr/bin/env python3

import importlib.util
import hashlib
import hmac
import os
import unittest
from pathlib import Path


def load_relay_module():
    os.environ.setdefault("BUILDKITE_ORGANIZATION", "test-org")
    os.environ.setdefault("BUILDKITE_PIPELINE", "test-pipeline")
    os.environ.setdefault("BUILDKITE_API_TOKEN", "test-token")
    os.environ.setdefault("GITCODE_WEBHOOK_TOKEN", "test-webhook-token")

    app_path = (
        Path(__file__).resolve().parents[1]
        / "files"
        / "gitcode-webhook-relay"
        / "app.py"
    )
    spec = importlib.util.spec_from_file_location("gitcode_webhook_relay", app_path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


relay = load_relay_module()


class GitCodeWebhookRelayTest(unittest.TestCase):
    def setUp(self):
        self.original_settings = relay.SETTINGS.copy()
        self.original_trigger_buildkite = relay.trigger_buildkite
        relay.SETTINGS.update(
            {
                "trigger_mr": True,
                "mr_actions": {"merge"},
                "mr_target_branches": {"feature/sandbox"},
                "skip_wip": True,
                "use_virtual_merge_ref": False,
                "message_prefix": "Triggered by GitCode",
                "buildkite_env": {},
            }
        )
        self.triggered = []

        def fake_trigger_buildkite(branch, commit, message, extra_env):
            self.triggered.append(
                {
                    "branch": branch,
                    "commit": commit,
                    "message": message,
                    "extra_env": extra_env,
                }
            )
            return {"web_url": "https://buildkite.example/build/1", "number": 1}

        relay.trigger_buildkite = fake_trigger_buildkite

    def tearDown(self):
        relay.SETTINGS.clear()
        relay.SETTINGS.update(self.original_settings)
        relay.trigger_buildkite = self.original_trigger_buildkite

    def test_merged_update_triggers_target_branch_commit(self):
        code, result = relay.handle_merge_request(
            {
                "object_attributes": {
                    "action": "update",
                    "state": "merged",
                    "target_branch": "feature/sandbox",
                    "source_branch": "wip/webhook-macos-default",
                    "target_branch_commit": {"id": "56336d2c21f5"},
                    "last_commit": {"id": "98d3d2e45c12"},
                    "iid": 637,
                    "work_in_progress": False,
                },
                "project": {"path_with_namespace": "openeuler/yuanrong"},
                "git_target_branch_commit_no": "56336d2c21f5",
            }
        )

        self.assertEqual(code, 200)
        self.assertEqual(result["status"], "triggered")
        self.assertEqual(result["branch"], "feature/sandbox")
        self.assertEqual(result["commit"], "56336d2c21f5")
        self.assertEqual(self.triggered[0]["extra_env"]["GITCODE_MR_ACTION"], "merge")

    def test_open_update_is_still_filtered(self):
        code, result = relay.handle_merge_request(
            {
                "object_attributes": {
                    "action": "update",
                    "state": "opened",
                    "target_branch": "feature/sandbox",
                    "source_branch": "wip/webhook-macos-default",
                    "last_commit": {"id": "98d3d2e45c12"},
                    "iid": 637,
                    "work_in_progress": False,
                },
                "project": {"path_with_namespace": "openeuler/yuanrong"},
            }
        )

        self.assertEqual(code, 202)
        self.assertEqual(result["reason"], "merge request action filtered")
        self.assertEqual(result["details"]["action"], "update")
        self.assertEqual(self.triggered, [])

    def test_webhook_token_is_valid_when_signature_secret_is_also_set(self):
        relay.SETTINGS["webhook_token"] = "gitcode-password"
        relay.SETTINGS["signature_secret"] = "configured-signature-secret"

        self.assertTrue(
            relay.verify_request(
                b"{}",
                {"X-GitCode-Token": "gitcode-password"},
            )
        )

    def test_valid_signature_is_accepted_before_token_fallback(self):
        body = b'{"object_kind":"merge_request"}'
        relay.SETTINGS["webhook_token"] = "gitcode-password"
        relay.SETTINGS["signature_secret"] = "configured-signature-secret"
        signature = "sha256=" + hmac.new(
            relay.SETTINGS["signature_secret"].encode("utf-8"),
            body,
            hashlib.sha256,
        ).hexdigest()

        self.assertTrue(
            relay.verify_request(
                body,
                {
                    "X-GitCode-Signature-256": signature,
                    "X-GitCode-Token": "wrong-token",
                },
            )
        )


if __name__ == "__main__":
    unittest.main()
