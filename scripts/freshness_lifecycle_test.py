#!/usr/bin/env python3
"""Virtual 7, 14 and 30 day incident lifecycle. No network and no clock."""

import importlib.util
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("freshness_classify", ROOT / "freshness_classify.py")
classify_mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(classify_mod)

START = datetime(2026, 1, 1, tzinfo=timezone.utc)


def stamp(day, hours=0):
    return (START + timedelta(days=day, hours=hours)).strftime("%Y-%m-%dT%H:%M:%SZ")


def wall(day, hours=-2):
    return {"wall": {"verified_at": stamp(day, hours), "phase": "verified"}}


def classes(surface, day, **kwargs):
    text = classify_mod.classify(
        surface,
        kwargs.get("wall", wall(day)),
        kwargs.get("state"),
        kwargs.get("workflow_at"),
        START + timedelta(days=day),
        kwargs.get("workflow_missing", False),
    )
    return set() if text == "ok" else set(text.split())


class LifecycleTest(unittest.TestCase):
    def test_horizons_do_not_hide_or_double_count_incidents(self):
        for horizon in (7, 14, 30):
            with self.subTest(horizon=horizon):
                self.run_horizon(horizon)

    def run_horizon(self, horizon):
        actions_open = set()
        local_open = set()
        for day in range(horizon):
            # Days 0-1: collector retry while the wall and Actions are fine.
            if day < 2:
                state = {"phase": "failed", "code": "WILL_RETRY", "checked_at": stamp(day, -1)}
                local = classes("local", day, state=state, workflow_at=stamp(day, -1))
                remote = classes("actions", day)
            # Day 2: recovery. Confirmed zero is not an incident.
            elif day == 2:
                state = {"phase": "published", "code": "VERIFIED", "checked_at": stamp(day, -1),
                         "collection": {"sources": [{"source": "cursor", "status": "CONFIRMED_ZERO",
                                                     "last_successful_observation_at": stamp(day, -1)}]}}
                local = classes("local", day, state=state, workflow_at=stamp(day, -1))
                remote = classes("actions", day)
            # Days 3-4: Actions stopped. The wall is still fresh.
            elif day < 5:
                local = classes("local", day, workflow_missing=True)
                remote = classes("actions", day)
            # Middle: wall stale, Mac recently checked, Actions also stale.
            elif day < horizon - 2:
                state = {"checked_at": stamp(day, -1), "code": "VERIFIED", "phase": "published"}
                local = classes("local", day, wall=wall(day, hours=-50), state=state, workflow_at=stamp(day, -10))
                snap = wall(day, hours=-40)
                snap["snapshot"] = {"source_status": [{"source": "cursor", "status": "AUTH_REQUIRED"},
                                                      {"source": "kimi", "status": "CONFIRMED_ZERO"}]}
                remote = classes("actions", day, wall=snap)
            else:
                # Last two days recover the wall and Actions. Cursor auth on the
                # public snapshot stays an Actions incident. Local is quiet.
                state = {"checked_at": stamp(day, -1), "code": "VERIFIED", "phase": "published"}
                local = classes("local", day, state=state, workflow_at=stamp(day, -1))
                snap = wall(day)
                snap["snapshot"] = {"source_status": [{"source": "cursor", "status": "AUTH_REQUIRED"}]}
                remote = classes("actions", day, wall=snap)

            self.assertNotIn("CONFIRMED_ZERO", local | remote)
            if day < 2:
                self.assertIn("collection_stale", local)
                self.assertNotIn("wall_stale", remote)
            elif day == 2:
                self.assertEqual(local, set())
                self.assertEqual(remote, set())
            elif day < 5:
                self.assertIn("workflow_silent", local)
                self.assertNotIn("wall_stale", local)
            elif day < horizon - 2:
                self.assertIn("publisher_stuck", local)
                self.assertIn("workflow_silent", local)
                self.assertIn("wall_stale", remote)
                self.assertIn("collection_stale", remote)
            else:
                self.assertEqual(local, set())
                self.assertEqual(remote, {"collection_stale"})
                self.assertNotIn("wall_stale", remote)

            # A local recovery never clears the Actions set, and the reverse.
            if not local:
                local_open.clear()
            else:
                local_open = set(local)
            if not remote:
                actions_open.clear()
            else:
                actions_open = set(remote)
            self.assertFalse(local_open & {"wall_stale"} and "collection_stale" in actions_open and not remote)
        if horizon == 30:
            self.assertEqual(actions_open, {"collection_stale"})
            self.assertEqual(local_open, set())


if __name__ == "__main__":
    unittest.main()
