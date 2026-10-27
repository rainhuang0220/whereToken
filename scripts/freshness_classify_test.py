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

    def test_actions_thresholds(self):
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-35, minutes=-59))), "ok")
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-36))), "ok")
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-36, seconds=-1))), "wall_stale")

    def test_actions_ignores_local_state(self):
        state = {"checked_at": stamp(timedelta(hours=-1))}
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-1)), state=state), "ok")
        self.assertEqual(self.classify("actions", wall(timedelta(hours=-40)), state=state), "wall_stale")

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
        old = {"checked_at": stamp(timedelta(hours=-10))}
        self.assertEqual(self.classify("local", stale, state=old), "mac_offline")
        self.assertEqual(self.classify("local", stale), "mac_offline")
        self.assertEqual(self.classify("local", stale, state={"checked_at": stamp(timedelta(hours=-6))}), "publisher_stuck")
        self.assertEqual(self.classify("local", stale, state={"checked_at": stamp(timedelta(hours=-6, seconds=-1))}), "mac_offline")

    def test_schedule_gap_only_when_the_wall_is_inside_the_local_window(self):
        fresh = wall(timedelta(hours=-2))
        self.assertEqual(self.classify("local", fresh, workflow_at=stamp(timedelta(hours=-6))), "ok")
        self.assertEqual(self.classify("local", fresh, workflow_at=stamp(timedelta(hours=-6, seconds=-1))), "workflow_silent")
        self.assertEqual(self.classify("local", fresh), "ok")
        stale = wall(timedelta(hours=-50))
        self.assertEqual(
            self.classify("local", stale, state={"checked_at": stamp(timedelta(hours=-1))}, workflow_at=stamp(timedelta(hours=-9))),
            "publisher_stuck",
        )

    def test_future_stamp_is_not_stale(self):
        self.assertEqual(self.classify("actions", wall(timedelta(minutes=5))), "ok")


if __name__ == "__main__":
    unittest.main()
