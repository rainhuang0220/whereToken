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


FAKE_GH = r"""#!/usr/bin/env python3
import json, os, sys
store = os.environ["GH_FAKE_STORE"]
args = sys.argv[1:]
cmd, sub = (args + ["", ""])[:2]

def flag(name):
    if name in args:
        return args[args.index(name) + 1]
    return ""

def rows():
    text = open(store, encoding="utf-8").read().splitlines()
    out = []
    for line in text:
        if not line.strip():
            continue
        number, state, title, classes = (line.split("\t", 3) + [""])[:4]
        out.append((number, state, title, classes))
    return out

def write(items):
    body = "".join("%s\t%s\t%s\t%s\n" % item for item in items)
    open(store, "w", encoding="utf-8").write(body)

if cmd == "issue" and sub == "list":
    want = flag("--state") or "open"
    print(json.dumps([{"number": int(n), "title": title} for n, state, title, _ in rows() if state == want]))
    sys.exit(0)
if cmd == "issue" and sub == "create":
    title = flag("--title")
    classes = ""
    for line in flag("--body").splitlines():
        if line.startswith("classes:"):
            classes = line.split(":", 1)[1].strip()
    number = str(len(rows()) + 1)
    write(rows() + [(number, "open", title, classes)])
    print("https://example.test/issues/%s" % number)
    sys.exit(0)
if cmd == "issue" and sub == "close":
    number = args[2]
    write([(n, "closed" if n == number else state, title, classes) for n, state, title, classes in rows()])
    sys.exit(0)
if cmd == "issue" and sub == "edit":
    number = args[2]
    classes = ""
    for line in flag("--body").splitlines():
        if line.startswith("classes:"):
            classes = line.split(":", 1)[1].strip()
    write([(n, state, title, classes if n == number else old) for n, state, title, old in rows()])
    sys.exit(0)
if cmd == "issue" and sub == "view":
    number = args[2]
    classes = ""
    for n, state, title, old in rows():
        if n == number:
            classes = old
    print(json.dumps({"body": "classes: %s\n" % classes}))
    sys.exit(0)
if cmd == "workflow" and sub == "run":
    sys.exit(0)
sys.stderr.write("unexpected gh %s\n" % " ".join(args))
sys.exit(1)
"""


def issue_rows(path):
    rows = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            rows.append((line.split("\t", 3) + [""])[:4])
    return rows


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

    def test_actions_lookup_failure_is_not_workflow_silence(self):
        server = serve(wall("2026-11-18T11:00:00Z"))
        self.addCleanup(server.shutdown)
        now = "2026-11-18T12:00:00Z"
        cases = {
            "lookup": (None, "1", "lookup_failed", "check_failed", False),
            "silent": ("[]", "0", "silent", "workflow_silent", False),
            "failed": (json.dumps([{"event": "schedule", "createdAt": "2026-11-18T11:00:00Z",
                                    "status": "completed", "conclusion": "failure"}]),
                       "0", "failed", "workflow_failed", False),
            "ok": (json.dumps([{"event": "schedule", "createdAt": "2026-11-18T11:00:00Z",
                                "status": "completed", "conclusion": "success"}]),
                   "0", "ok", "ok", True),
        }
        for name, (payload, code, kind, local_class, healthy) in cases.items():
            with self.subTest(name=name):
                with TemporaryDirectory() as tmp:
                    directory = Path(tmp)
                    gh = directory / "gh"
                    gh.write_text(
                        "#!/bin/sh\n"
                        "if [ \"${GH_RUNS_CODE:-0}\" != 0 ]; then exit \"$GH_RUNS_CODE\"; fi\n"
                        "printf '%s\\n' \"$GH_RUNS\"\n",
                        encoding="utf-8",
                    )
                    gh.chmod(0o755)
                    env = os.environ.copy()
                    env["PATH"] = str(directory) + os.pathsep + env["PATH"]
                    env["GH_RUNS_CODE"] = code
                    if payload is not None:
                        env["GH_RUNS"] = payload
                    cmd = [sys.executable, str(SCRIPT), "--wall-url",
                           f"http://127.0.0.1:{server.server_address[1]}/wall",
                           "--state", str(directory / "state.json"),
                           "--alerts", str(directory / "alerts.json"),
                           "--now", now, "--timeout", "2"]
                    done = subprocess.run(cmd, text=True, capture_output=True, check=False, env=env)
                    if healthy:
                        self.assertEqual(done.returncode, 0, done.stdout + done.stderr)
                    else:
                        self.assertEqual(done.returncode, 1, done.stdout + done.stderr)
                    state = json.loads((directory / "state.json").read_text())
                    self.assertEqual(state["actions"], kind)
                    self.assertIn(local_class, state["local_classes"].split())
                    if kind == "lookup_failed":
                        self.assertNotIn("workflow_silent", state["local_classes"])
                        self.assertNotIn("workflow_failed", state["local_classes"])
                    if kind == "failed":
                        self.assertNotIn("workflow_silent", state["local_classes"])
                    if kind == "silent":
                        self.assertNotIn("check_failed", state["local_classes"])
                        self.assertNotIn("workflow_failed", state["local_classes"])

    def test_delivery_failure_is_not_recorded_as_delivered(self):
        server = serve(wall("2026-11-18T11:00:00Z"))
        self.addCleanup(server.shutdown)
        with TemporaryDirectory() as tmp:
            directory = Path(tmp)
            command = directory / "alert.sh"
            command.write_text("#!/bin/sh\nexit 1\n", encoding="utf-8")
            command.chmod(0o755)
            cmd = [sys.executable, str(SCRIPT), "--wall-url",
                   f"http://127.0.0.1:{server.server_address[1]}/wall",
                   "--state", str(directory / "state.json"),
                   "--alerts", str(directory / "alerts.json"),
                   "--now", NOW, "--timeout", "2", "--skip-actions",
                   "--alert-command", str(command), "--alert-timeout", "2"]
            done = subprocess.run(cmd, text=True, capture_output=True, check=False)
            self.assertEqual(done.returncode, 3, done.stdout + done.stderr)
            self.assertIn("delivery_failed", done.stdout)
            alerts = json.loads((directory / "alerts.json").read_text())
            self.assertTrue(alerts)
            self.assertTrue(all(row.get("delivered") is False for row in alerts))
            sleeper = directory / "sleep.sh"
            sleeper.write_text("#!/bin/sh\nsleep 5\n", encoding="utf-8")
            sleeper.chmod(0o755)
            late = subprocess.run(
                [sys.executable, str(SCRIPT), "--wall-url",
                 f"http://127.0.0.1:{server.server_address[1]}/wall",
                 "--state", str(directory / "state.json"),
                 "--alerts", str(directory / "late.json"),
                 "--now", NOW, "--timeout", "2", "--skip-actions",
                 "--alert-command", str(sleeper), "--alert-timeout", "0.2"],
                text=True, capture_output=True, check=False,
            )
            self.assertEqual(late.returncode, 3, late.stdout + late.stderr)
            late_alerts = json.loads((directory / "late.json").read_text())
            self.assertEqual(late_alerts[0]["delivery_error"], "TimeoutExpired")
            self.assertFalse(any(row.get("delivered") is True for row in late_alerts))

    def test_recovery_closes_only_that_surface(self):
        stale = serve(wall("2026-11-16T00:00:00Z"))
        self.addCleanup(stale.shutdown)
        with TemporaryDirectory() as tmp:
            directory = Path(tmp)
            store = directory / "issues"
            store.write_text("", encoding="utf-8")
            bin_dir = directory / "bin"
            bin_dir.mkdir()
            gh = bin_dir / "gh"
            gh.write_text(FAKE_GH, encoding="utf-8")
            gh.chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["HOME"] = str(directory)
            env["GH_FAKE_STORE"] = str(store)
            env["GITHUB_REPOSITORY"] = "example/whereToken"
            env["WHERETOKEN_DISPATCH_STAMP"] = str(directory / "stamp")
            mac = directory / "mac.json"
            mac.write_text(json.dumps({"checked_at": "2026-11-17T00:00:00Z", "code": "VERIFIED"}), encoding="utf-8")
            url = f"http://127.0.0.1:{stale.server_address[1]}/wall"

            def once(extra_url, mac_doc):
                mac.write_text(json.dumps(mac_doc), encoding="utf-8")
                cmd = [sys.executable, str(SCRIPT), "--wall-url", extra_url,
                       "--state", str(directory / "state.json"),
                       "--alerts", str(directory / "alerts.json"),
                       "--mac-state", str(mac), "--now", NOW, "--timeout", "2",
                       "--skip-actions", "--workflow-missing",
                       "--alert-command", "bash", str(ROOT / "scripts" / "freshness-alert.sh")]
                return subprocess.run(cmd, text=True, capture_output=True, check=False, env=env)

            first = once(url, {"checked_at": "2026-11-17T00:00:00Z", "code": "VERIFIED"})
            self.assertEqual(first.returncode, 1, first.stdout + first.stderr)
            opened = issue_rows(store)
            self.assertEqual(sorted(row[2] for row in opened if row[1] == "open"),
                             ["profile-freshness alert (actions)", "profile-freshness alert (local)"])
            again = once(url, {"checked_at": "2026-11-17T00:00:00Z", "code": "VERIFIED"})
            self.assertEqual(again.returncode, 1, again.stdout + again.stderr)
            self.assertIn("already-open", again.stdout)
            self.assertEqual(len(issue_rows(store)), 2)
            changed = once(url, {"checked_at": "2026-11-18T11:30:00Z", "code": "VERIFIED"})
            self.assertEqual(changed.returncode, 1, changed.stdout + changed.stderr)
            self.assertIn("updated", changed.stdout)
            local = [row for row in issue_rows(store) if row[2].endswith("(local)")][0]
            self.assertEqual(local[1], "open")
            self.assertIn("publisher_stuck", local[3])
            actions = [row for row in issue_rows(store) if row[2].endswith("(actions)")][0]
            self.assertIn("wall_stale", actions[3])
            fresh = serve(wall("2026-11-18T11:00:00Z"))
            self.addCleanup(fresh.shutdown)
            fresh_url = f"http://127.0.0.1:{fresh.server_address[1]}/wall"
            recovered = once(fresh_url, {"checked_at": "2026-11-18T11:30:00Z", "code": "VERIFIED"})
            self.assertEqual(recovered.returncode, 1, recovered.stdout + recovered.stderr)
            self.assertIn("class=ok surface=actions", recovered.stdout)
            rows = issue_rows(store)
            actions = [row for row in rows if row[2].endswith("(actions)")][0]
            local = [row for row in rows if row[2].endswith("(local)")][0]
            self.assertEqual(actions[1], "closed")
            self.assertEqual(local[1], "open")
            self.assertIn("workflow_silent", local[3])
            alerts = json.loads((directory / "alerts.json").read_text())
            self.assertTrue(all(row.get("delivered") is True for row in alerts))

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
