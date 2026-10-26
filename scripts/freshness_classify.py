#!/usr/bin/env python3
"""Classify public-profile freshness. Prints one class and exits 0.

Classes: ok, wall_stale, mac_offline, publisher_stuck, workflow_silent, check_failed.

The actions surface sees only the public wall. The local surface can also
see the publisher state file and the last Actions run time.
"""

import argparse
import json
import sys
from datetime import datetime, timezone

ACTIONS_WALL_S = 36 * 3600
LOCAL_WALL_S = 48 * 3600
RECENT_CHECK_S = 6 * 3600
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


def classify(surface, wall, state, workflow_at, now):
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
    if surface == "actions":
        if wall_age > ACTIONS_WALL_S:
            return "wall_stale"
        return "ok"
    if wall_age > LOCAL_WALL_S:
        checked = None
        if isinstance(state, dict):
            checked = parse_time(state.get("checked_at"))
        check_age = age_seconds(now, checked)
        if check_age is not None and check_age <= RECENT_CHECK_S:
            return "publisher_stuck"
        return "mac_offline"
    run_age = age_seconds(now, parse_time(workflow_at) if isinstance(workflow_at, str) else workflow_at)
    if run_age is not None and run_age > SCHEDULE_GAP_S:
        return "workflow_silent"
    return "ok"


def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("--surface", required=True, choices=("actions", "local"))
    parser.add_argument("--wall", required=True)
    parser.add_argument("--state")
    parser.add_argument("--workflow-at")
    parser.add_argument("--now")
    args = parser.parse_args(argv)
    now = parse_time(args.now) if args.now else datetime.now(timezone.utc)
    if now is None:
        print("check_failed")
        return 0
    wall = load_json(args.wall)
    state = load_json(args.state) if args.state else None
    print(classify(args.surface, wall, state, args.workflow_at, now))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
