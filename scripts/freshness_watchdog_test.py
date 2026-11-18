#!/usr/bin/env python3
"""Watchdog delivery tests. The HTTP server and alert command are local."""

import json
import os
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import TestCase, main

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "hosted_freshness_watchdog.py"
NOW = "2026-11-18T12:00:00Z"


def wall(verified):
    return json.dumps({"wall": {"verified_at": verified, "phase": "verified"}, "snapshot": {}}).encode()


class Handler(BaseHTTPRequestHandler):
    body = wall("2026-11-18T11:00:00Z")
    fail = False

    def do_GET(self):
        if self.fail:
            self.send_error(503)
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(self.body)

    def log_message(self, fmt, *args):
        return


def serve(body, fail=False):
    Handler.body = body
    Handler.fail = fail
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


def run(tmp, url, extra=None):
    cmd = [sys.executable, str(SCRIPT), "--wall-url", url, "--state", str(tmp / "state.json"),
           "--alerts", str(tmp / "alerts.json"), "--now", NOW, "--timeout", "2", "--skip-actions"]
    if extra:
        cmd.extend(extra)
    return subprocess.run(cmd, text=True, capture_output=True, check=False)


class WatchdogTest(TestCase):
    def test_fresh_wall_delivers_ok(self):
        server = serve(wall("2026-11-18T11:00:00Z"))
        self.addCleanup(server.shutdown)
        with TemporaryDirectory() as tmp:
            directory = Path(tmp)
            done = run(directory, f"http://127.0.0.1:{server.server_address[1]}/wall")
            self.assertEqual(done.returncode, 0, done.stderr)
            alerts = json.loads((directory / "alerts.json").read_text())
            self.assertEqual([row["classes"] for row in alerts], ["ok", "ok"])
            state = json.loads((directory / "state.json").read_text())
            self.assertEqual(state["wall"], "ok")

    def test_hosted_down_delivers_check_failed(self):
        with TemporaryDirectory() as tmp:
            directory = Path(tmp)
            done = run(directory, "http://127.0.0.1:1/wall")
            self.assertEqual(done.returncode, 1)
            alerts = json.loads((directory / "alerts.json").read_text())
            self.assertEqual(alerts[0]["classes"], "check_failed")
            self.assertEqual(alerts[1]["surface"], "local")
            stale = run(directory, "http://127.0.0.1:1/wall",
                        extra=["--alerts", str(directory / "both.json"),
                               "--workflow-at", "2026-11-18T02:00:00Z", "--skip-actions"])
            self.assertEqual(stale.returncode, 1, stale.stdout)
            both = json.loads((directory / "both.json").read_text())
            self.assertIn("workflow_silent", both[1]["classes"])
            self.assertIn("check_failed", both[1]["classes"])

    def test_actions_silence_and_mac_offline_are_delivered(self):
        server = serve(wall("2026-11-16T00:00:00Z"))
        self.addCleanup(server.shutdown)
        with TemporaryDirectory() as tmp:
            directory = Path(tmp)
            mac = directory / "mac.json"
            mac.write_text(json.dumps({"checked_at": "2026-11-17T00:00:00Z", "code": "VERIFIED"}), encoding="utf-8")
            marker = directory / "alert-command"
            command = directory / "alert.sh"
            command.write_text("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$WATCHDOG_ALERT_LOG\"\n", encoding="utf-8")
            command.chmod(0o755)
            env = os.environ.copy()
            env["WATCHDOG_ALERT_LOG"] = str(marker)
            cmd = [sys.executable, str(SCRIPT), "--wall-url", f"http://127.0.0.1:{server.server_address[1]}/wall",
                   "--state", str(directory / "state.json"), "--alerts", str(directory / "alerts.json"),
                   "--mac-state", str(mac), "--now", NOW, "--timeout", "2",
                   "--alert-command", str(command), "--skip-actions", "--workflow-missing"]
            done = subprocess.run(cmd, text=True, capture_output=True, check=False, env=env)
            self.assertEqual(done.returncode, 1, done.stdout + done.stderr)
            text = marker.read_text(encoding="utf-8")
            self.assertIn("--surface actions", text)
            self.assertIn("wall_stale", text)
            self.assertIn("--surface local", text)
            self.assertIn("mac_offline", text)
            self.assertIn("workflow_silent", text)

    def test_watchdog_storage_failure_is_not_ok(self):
        server = serve(wall("2026-11-18T11:00:00Z"))
        self.addCleanup(server.shutdown)
        with TemporaryDirectory() as tmp:
            directory = Path(tmp)
            blocked = directory / "not-a-directory"
            blocked.write_text("x", encoding="utf-8")
            done = run(directory, f"http://127.0.0.1:{server.server_address[1]}/wall",
                       extra=["--state", str(blocked / "state.json")])
            self.assertEqual(done.returncode, 3)
            self.assertIn("watchdog_failed", done.stdout)


if __name__ == "__main__":
    main()
