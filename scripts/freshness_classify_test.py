#!/usr/bin/env python3
"""Fixture tests for scripts/freshness_classify.py. No live GitHub calls."""

import json
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "freshness_classify.py"
NOW = datetime(2026, 10, 27, 19, 30, tzinfo=timezone.utc)


def stamp(delta):
    return (NOW + delta).strftime("%Y-%m-%dT%H:%M:%SZ")


def wall(delta):
    return {"wall": {"verified_at": stamp(delta), "phase": "verified"}}


class ClassifyTest(unittest.TestCase):
    def classify(self, surface, wall_doc, state=None, workflow_at=None):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            wall_path = directory / "wall.json"
            wall_path.write_text(json.dumps(wall_doc), encoding="utf-8")
            cmd = [sys.executable, str(SCRIPT), "--surface", surface, "--wall", str(wall_path), "--now", stamp(timedelta(0))]
            if state is not None:
                state_path = directory / "state.json"
                state_path.write_text(json.dumps(state), encoding="utf-8")
                cmd.extend(["--state", str(state_path)])
            if workflow_at is not None:
                cmd.extend(["--workflow-at", workflow_at])
            out = subprocess.check_output(cmd, text=True).strip()
            return out

    def classify_cmd(self, extra):
        with tempfile.TemporaryDirectory() as tmp:
            wall_path = Path(tmp) / "wall.json"
            wall_path.write_text(json.dumps(wall(timedelta(hours=-2))), encoding="utf-8")
            cmd = [sys.executable, str(SCRIPT), "--surface", "local", "--wall", str(wall_path), "--now", stamp(timedelta(0)), *extra]
            return subprocess.check_output(cmd, text=True).strip()

    def test_actions_thresholds(self):
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-35, minutes=-59))), "ok")
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-36))), "ok")
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-36, seconds=-1))), "wall_stale")

    def test_actions_ignores_local_state(self):
        state = {"checked_at": stamp(timedelta(hours=-1))}
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-1)), state=state), "ok")
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-40)), state=state), "wall_stale")

    def test_actions_reports_collection_degradation_separately_from_publication(self):
        doc = wall(timedelta(hours=-1))
        doc["snapshot"] = {"source_status": [
            {"source": "cursor", "status": "AUTH_REQUIRED"},
            {"source": "kimi", "status": "CONFIRMED_ZERO"},
        ]}
        self.assertEqual(self.classify("actions", doc), "collection_stale")
        doc["wall"]["verified_at"] = stamp(timedelta(hours=-40))
        self.assertEqual(self.classify("actions", doc), "wall_stale collection_stale")
        doc["wall"]["verified_at"] = stamp(timedelta(hours=-1))
        doc["snapshot"]["source_status"][0]["status"] = "FRESH"
        self.assertEqual(self.classify("actions", doc), "ok")

    def test_broken_wall_is_a_failed_check(self):
        self.assertEqual(self.classify("actions", {}), "check_failed")
        self.assertEqual(self.classify("actions", {"wall": {"verified_at": "yesterday"}}), "check_failed")
        self.assertEqual(self.classify("local", {"wall": {}}), "check_failed")

    def test_local_wall_and_mac(self):
        fresh = wall(timedelta(hours=-20))
        self.assertEqual(self.classify("local", fresh), "ok")
        self.assertEqual(self.classify("local", wall(timedelta(hours=-48))), "ok")
        stale = wall(timedelta(hours=-49))
        recent = {"checked_at": "2026-10-28T01:30:00+08:00"}
        self.assertEqual(self.classify("local", stale, state=recent), "publisher_stuck")
        old = {"checked_at": stamp(timedelta(hours=-13))}
        self.assertEqual(self.classify("local", stale, state=old), "mac_offline")
        self.assertEqual(self.classify("local", stale), "mac_offline")

    def test_sleep_between_daily_slots_does_not_imply_mac_offline(self):
        stale = wall(timedelta(hours=-49))
        # The 00:00 -> 10:30 slot gap is 10.5 hours. Allow 90 minutes
        # beyond that for wake-up and a bounded refresh attempt.
        for age in (timedelta(hours=10, minutes=30), timedelta(hours=12)):
            with self.subTest(age=age):
                state = {"checked_at": stamp(-age)}
                self.assertEqual(self.classify("local", stale, state=state), "publisher_stuck")
        state = {"checked_at": stamp(timedelta(hours=-12, seconds=-1))}
        self.assertEqual(self.classify("local", stale, state=state), "mac_offline")

    def test_schedule_gap_only_when_the_wall_is_inside_the_local_window(self):
        fresh = wall(timedelta(hours=-2))
        self.assertEqual(self.classify("local", fresh, workflow_at=stamp(timedelta(hours=-6))), "ok")
        self.assertEqual(self.classify("local", fresh, workflow_at=stamp(timedelta(hours=-6, seconds=-1))), "workflow_silent")
        self.assertEqual(self.classify("local", fresh), "ok")
        stale = wall(timedelta(hours=-50))
        self.assertEqual(
            self.classify("local", stale, state={"checked_at": stamp(timedelta(hours=-1))}, workflow_at=stamp(timedelta(hours=-9))),
            "publisher_stuck workflow_silent",
        )

    def test_local_retry_is_visible_while_the_wall_is_fresh(self):
        state = {"phase": "failed", "code": "WILL_RETRY", "checked_at": stamp(timedelta(minutes=-5))}
        self.assertEqual(self.classify("local", wall(timedelta(hours=-2)), state=state), "collection_stale")
        quiet = {"phase": "published", "code": "VERIFIED", "checked_at": stamp(timedelta(minutes=-5)),
                 "collection": {"sources": [{"source": "cursor", "status": "CONFIRMED_ZERO",
                                             "last_successful_observation_at": stamp(timedelta(hours=-1))}]}}
        self.assertEqual(self.classify("local", wall(timedelta(hours=-2)), state=quiet), "ok")

    def test_unreadable_state_is_not_ok(self):
        self.assertEqual(self.classify_cmd(["--state", str(SCRIPT)]), "check_failed")

    def test_missing_workflow_evidence_is_silent(self):
        self.assertEqual(self.classify_cmd(["--workflow-missing"]), "workflow_silent")

    def test_actions_lookup_failure_is_not_a_stopped_workflow(self):
        self.assertEqual(self.classify_cmd(["--actions-lookup-failed"]), "check_failed")
        self.assertNotIn("workflow_silent", self.classify_cmd(["--actions-lookup-failed"]))
        self.assertEqual(self.classify_cmd(["--workflow-failed"]), "workflow_failed")

    def test_actions_list_kinds_stay_distinct(self):
        now = "2026-11-18T12:00:00Z"
        rows = {
            "silent": [],
            "old": [{"event": "schedule", "createdAt": "2026-11-18T02:00:00Z", "status": "completed", "conclusion": "success"}],
            "failed": [{"event": "schedule", "createdAt": "2026-11-18T11:00:00Z", "status": "completed", "conclusion": "failure"}],
            "ok": [{"event": "schedule", "createdAt": "2026-11-18T11:00:00Z", "status": "completed", "conclusion": "success"}],
        }
        expect = {"silent": "silent", "old": "silent", "failed": "failed", "ok": "ok"}
        for name, payload in rows.items():
            with self.subTest(name=name):
                out = subprocess.run(
                    [sys.executable, str(SCRIPT), "--classify-actions", "--now", now],
                    input=json.dumps(payload), text=True, capture_output=True, check=True,
                ).stdout.strip().split()[0]
                self.assertEqual(out, expect[name])

    def test_future_stamp_is_not_stale(self):
        self.assertEqual(self.classify("actions", wall(timedelta(minutes=5))), "ok")


if __name__ == "__main__":
    unittest.main()
