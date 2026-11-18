#!/usr/bin/env python3
"""Exercise Pages verification and repair against a real local HTTP origin."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread
import unittest
from urllib.parse import urlsplit


SCRIPT = Path(__file__).with_name("profile_pages.py")


def bundle(revision, day, total):
    identity = {"schema": "wheretoken.public-profile",
                "snapshot_id": "sha256:" + revision * 64,
                "as_of_date": day, "generated_at": day + "T12:00:00Z"}
    profile = {**identity, "schema_version": 2,
               "provenance": {"kind": "local_sanitized_snapshot", "refresh_mode": "manual_publish", "live_sync": False},
               "periods": {"all": {"totals": {"total": {"value": total}}}}}
    manifest = {**identity, "asset_revision": "sha256:" + revision * 64,
                "provenance": "local_sanitized_snapshot"}
    preview = (
        '<svg xmlns="http://www.w3.org/2000/svg">'
        f"<desc>Coding-agent token usage through {day}. No prompts, paths, or identifiers.</desc>"
        f'<metadata id="wheretoken-snapshot-id">{identity["snapshot_id"]}</metadata></svg>'
    ).encode()
    return {"manifest.json": json.dumps(manifest).encode(), "profile.json": json.dumps(profile).encode(),
            "preview-light.svg": preview, "preview-dark.svg": preview}


class VerifyPages(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.directory = Path(tmp.name)
        self.current = bundle("a", "2026-11-16", 140)
        self.old = bundle("9", "2026-11-12", 100)
        self.published = dict(self.current)
        self.pages_requests = []
        for name, payload in self.current.items():
            (self.directory / name).write_bytes(payload)

        testcase = self

        class PageHandler(BaseHTTPRequestHandler):
            def do_GET(self):
                name = urlsplit(self.path).path.rsplit("/", 1)[-1]
                testcase.pages_requests.append(name)
                response = testcase.published.get(name, 404)
                if isinstance(response, list):
                    response = response.pop(0)
                if isinstance(response, int):
                    self.send_response(response)
                    self.end_headers()
                    return
                self.send_response(200)
                self.end_headers()
                self.wfile.write(response)

            def log_message(self, *_):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), PageHandler)
        thread = Thread(target=server.serve_forever, args=(0.05,), daemon=True)
        thread.start()
        self.addCleanup(thread.join, 2)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        self.origin = f"http://127.0.0.1:{server.server_port}/whereToken/profile/"
        self.dispatches = self.directory / "dispatches"
        self.env = dict(os.environ, GITHUB_REPOSITORY="fixture/whereToken", PATH=f"{self.directory}{os.pathsep}{os.environ['PATH']}",
                        TEST_DISPATCH_LOG=str(self.dispatches))
        gh = self.directory / "gh"
        gh.write_text("""#!/usr/bin/env python3
import json
import os
import sys
from pathlib import Path

match sys.argv[1:3]:
    case ["run", "list"]:
        print(json.dumps([{"status": os.environ.get("TEST_PAGE_RUN_STATUS", "completed")}]))
    case ["workflow", "run"]:
        assert "pages.yml" in sys.argv and "main" in sys.argv
        with Path(os.environ["TEST_DISPATCH_LOG"]).open("a", encoding="utf-8") as log:
            log.write("dispatch\\n")
        sys.exit(int(os.environ.get("TEST_DISPATCH_FAILURE", 0)))
    case _:
        sys.exit(2)
""", encoding="utf-8")
        gh.chmod(0o755)

    def run_check(self, *extra):
        return subprocess.run([sys.executable, str(SCRIPT), "--dir", str(self.directory),
                               "--url", self.origin, *extra], env=self.env, capture_output=True, text=True, timeout=10)

    def expect_result(self, run, code, status):
        self.assertEqual(run.returncode, code, run.stdout + run.stderr)
        self.assertIn("pages_mirror=" + status, run.stdout)

    def test_retries_failed_dispatch_with_unchanged_verified_commit(self):
        self.published = self.old
        self.env["TEST_DISPATCH_FAILURE"] = "1"
        self.expect_result(self.run_check("--repair"), 1, "dispatch_failed")
        self.env["TEST_DISPATCH_FAILURE"] = "0"
        self.expect_result(self.run_check("--repair"), 1, "deploy_requested")
        self.published = self.current
        self.expect_result(self.run_check("--repair"), 0, "verified")
        self.assertEqual(self.dispatches.read_text(encoding="utf-8"), "dispatch\ndispatch\n")
        self.assertEqual((self.directory / "manifest.json").read_bytes(), self.current["manifest.json"])

    def test_a_matching_manifest_does_not_hide_a_mismatched_svg(self):
        self.published["preview-dark.svg"] = self.old["preview-dark.svg"]
        self.expect_result(self.run_check(), 1, "stale")

    def test_a_matching_manifest_does_not_hide_mismatched_json(self):
        self.published["profile.json"] = self.old["profile.json"]
        self.expect_result(self.run_check(), 1, "stale")

    def test_active_pages_run_prevents_duplicate_dispatch(self):
        self.published = self.old
        self.env["TEST_PAGE_RUN_STATUS"] = "in_progress"
        self.expect_result(self.run_check("--repair"), 1, "deploy_pending")
        self.assertFalse(self.dispatches.exists())

    def test_unavailable_pages_does_not_start_redundant_deploy(self):
        self.published["manifest.json"] = 503
        self.expect_result(self.run_check("--repair"), 1, "unavailable")
        self.assertFalse(self.dispatches.exists())

    def test_newer_deployment_cannot_trigger_a_rollback(self):
        self.published = bundle("b", "2026-11-17", 150)
        self.expect_result(self.run_check("--repair"), 0, "superseded")
        self.assertFalse(self.dispatches.exists())

    def test_verify_retries_delayed_cdn_readback(self):
        self.published["manifest.json"] = [404, self.current["manifest.json"]]
        self.expect_result(self.run_check("--attempts", "2"), 0, "verified")
        self.assertEqual(self.pages_requests.count("manifest.json"), 2)


if __name__ == "__main__":
    unittest.main()
