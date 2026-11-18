#!/usr/bin/env python3
"""Host-side freshness watchdog.

This process is meant to run on the hosted machine, which is neither the
Mac nor GitHub Actions. It reads a saved public wall and an optional Actions
timestamp. It prints classes and does not open issues, change DNS, or dial.

It is not installed. Turning it on is a separate production change.
"""

import argparse
import importlib.util
import sys
from datetime import datetime, timezone
from pathlib import Path


def load_classify():
    path = Path(__file__).with_name("freshness_classify.py")
    spec = importlib.util.spec_from_file_location("freshness_classify", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--wall", required=True, help="public profile JSON already fetched")
    parser.add_argument("--workflow-at", help="timestamp of the last Actions schedule or dispatch")
    parser.add_argument("--workflow-missing", action="store_true")
    parser.add_argument("--now")
    args = parser.parse_args(argv)
    classify = load_classify()
    now = classify.parse_time(args.now) if args.now else datetime.now(timezone.utc)
    wall = classify.load_json(args.wall)
    # The host sees the public wall the way Actions does, and the Actions
    # heartbeat the way the Mac does. One of those can fail without the other.
    wall_classes = classify.classify("actions", wall, None, None, now)
    local_classes = classify.classify(
        "local", wall, None, args.workflow_at, now, workflow_missing=args.workflow_missing or not args.workflow_at,
    )
    print("actions", wall_classes)
    print("watchdog", local_classes)
    failed = wall_classes != "ok" or local_classes != "ok"
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
