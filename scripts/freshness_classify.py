#!/usr/bin/env python3
"""Classify public-profile freshness. Prints one class and exits 0.

Classes: ok, collection_stale, wall_stale, mac_offline, publisher_stuck, workflow_silent, check_failed.

The actions surface sees only the public wall. The local surface can also
see the publisher state file and the last Actions run time.
"""

import argparse
import json
import sys
from datetime import datetime, timezone

ACTIONS_WALL_S = 36 * 3600
LOCAL_WALL_S = 48 * 3600
# The owner's 00:00 -> 10:30 scheduled gap is 10.5 hours. Include 90
# minutes for wake-up, the bounded scan, and its limited retries.
RECENT_CHECK_S = 12 * 3600
SCHEDULE_GAP_S = 6 * 3600


def parse_time(value):
    if not value or not isinstance(value, str):
        return None
    text = value.strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        stamp = datetime.fromisoformat(text)
    except ValueError:
        return None
    if stamp.tzinfo is None:
        return None
    return stamp.astimezone(timezone.utc)


def load_json(path):
    try:
        with open(path, encoding="utf-8") as fh:
            return json.load(fh)
    except (OSError, json.JSONDecodeError):
        return None


def age_seconds(now, stamp):
    if stamp is None:
        return None
    return (now - stamp).total_seconds()


def collection_degraded(checks):
    if not isinstance(checks, list):
        return False
    return any(isinstance(s, dict) and s.get("status") not in ("FRESH", "CONFIRMED_ZERO")
               for s in checks)


# Printed in this order. check_failed is exclusive. A wall failure does not
# hide a collector or an Actions gap, and one class recovering does not clear
# the others.
CLASS_ORDER = (
    "check_failed",
    "wall_stale",
    "collection_stale",
    "publisher_stuck",
    "mac_offline",
    "workflow_silent",
    "pages_stale",
)

# Last refresh codes that mean the collector did not finish. A later calendar
# day or a fresh wall does not make these healthy.
LOCAL_FAILURE_CODES = {
    "WILL_RETRY",
    "BLOCKED_DATA",
    "BLOCKED_REGRESSION",
    "BLOCKED_AUTH",
    "NOT_SIGNED_IN",
}


def join_classes(found):
    ordered = [name for name in CLASS_ORDER if name in found]
    return " ".join(ordered) if ordered else "ok"


def classify(surface, wall, state, workflow_at, now, workflow_missing=False):
    if surface not in ("actions", "local"):
        return "check_failed"
    if not isinstance(wall, dict):
        return "check_failed"
    verified = parse_time((wall.get("wall") or {}).get("verified_at"))
    if verified is None:
        return "check_failed"
    wall_age = age_seconds(now, verified)
    if wall_age is None:
        return "check_failed"
    found = set()
    if surface == "actions":
        if wall_age > ACTIONS_WALL_S:
            found.add("wall_stale")
        snap = wall.get("snapshot")
        if isinstance(snap, dict) and collection_degraded(snap.get("source_status")):
            found.add("collection_stale")
        return join_classes(found)
    if wall_age > LOCAL_WALL_S:
        checked = parse_time(state.get("checked_at")) if isinstance(state, dict) else None
        check_age = age_seconds(now, checked)
        if check_age is not None and check_age <= RECENT_CHECK_S:
            found.add("publisher_stuck")
        else:
            found.add("mac_offline")
    if isinstance(state, dict):
        if state.get("phase") == "failed" or state.get("code") in LOCAL_FAILURE_CODES:
            found.add("collection_stale")
        collection = state.get("collection")
        if isinstance(collection, dict):
            checks = collection.get("sources")
            if collection_degraded(checks):
                found.add("collection_stale")
            elif isinstance(checks, list):
                for check in checks:
                    stamp = parse_time(check.get("last_successful_observation_at")) if isinstance(check, dict) else None
                    if stamp is None or not 0 <= age_seconds(now, stamp) < 24 * 3600:
                        found.add("collection_stale")
                        break
    if workflow_missing:
        found.add("workflow_silent")
    else:
        run_age = age_seconds(now, parse_time(workflow_at) if isinstance(workflow_at, str) else workflow_at)
        if run_age is not None and run_age > SCHEDULE_GAP_S:
            found.add("workflow_silent")
    return join_classes(found)


def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("--surface", required=True, choices=("actions", "local"))
    parser.add_argument("--wall", required=True)
    parser.add_argument("--state")
    parser.add_argument("--workflow-at")
    parser.add_argument("--workflow-missing", action="store_true")
    parser.add_argument("--now")
    args = parser.parse_args(argv)
    now = parse_time(args.now) if args.now else datetime.now(timezone.utc)
    if now is None:
        print("check_failed")
        return 0
    wall = load_json(args.wall)
    if args.state:
        state = load_json(args.state)
        if state is None:
            print("check_failed")
            return 0
    else:
        state = None
    print(classify(args.surface, wall, state, args.workflow_at, now, args.workflow_missing))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
