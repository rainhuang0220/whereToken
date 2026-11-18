#!/usr/bin/env python3
"""Independent freshness watchdog for the hosted machine.

It fetches the public wall and the latest profile-freshness Actions run,
persists that observation, and delivers one alert record per surface.
It does not open a GitHub issue unless --alert-command is set.
Installing it on the server is a separate change.
"""

import argparse
import importlib.util
import json
import os
import subprocess
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path


def load_classify():
    path = Path(__file__).with_name("freshness_classify.py")
    spec = importlib.util.spec_from_file_location("freshness_classify", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def fetch_bytes(url, timeout):
    request = urllib.request.Request(url, headers={"Cache-Control": "no-cache", "User-Agent": "wheretoken-watchdog"})
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return response.read(1 << 20)


class DeliveryError(Exception):
    pass


def observe_actions(repo, timeout, now, classify):
    """Return (kind, created_at). A failed gh invocation is lookup_failed.

    lookup_failed is not evidence that the workflow stopped.
    """
    proc = subprocess.run(
        ["gh", "run", "list", "--repo", repo, "--workflow", "profile-freshness.yml",
         "--limit", "10", "--json", "event,createdAt,status,conclusion"],
        check=False, capture_output=True, text=True, timeout=timeout,
    )
    if proc.returncode != 0:
        return "lookup_failed", None
    try:
        rows = json.loads(proc.stdout or "null")
    except json.JSONDecodeError:
        return "lookup_failed", None
    return classify.interpret_actions(rows, now)


def load_state(path):
    try:
        return json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}


def save_json(path, payload):
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    tmp = target.with_suffix(target.suffix + ".tmp")
    tmp.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    os.replace(tmp, target)


def append_alert(alert_path, record):
    previous = []
    path = Path(alert_path)
    if path.exists():
        try:
            previous = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            previous = []
    if not isinstance(previous, list):
        previous = []
    previous.append(record)
    save_json(path, previous)


def deliver(alert_path, surface, classes, command, timeout):
    record = {
        "at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "surface": surface,
        "classes": classes,
    }
    if not command:
        append_alert(alert_path, record)
        return record
    try:
        subprocess.run(command + ["--surface", surface, "--classes", classes], check=True, timeout=timeout)
    except (subprocess.SubprocessError, OSError) as exc:
        record["delivered"] = False
        record["delivery_error"] = exc.__class__.__name__
        append_alert(alert_path, record)
        raise DeliveryError(exc.__class__.__name__)
    record["delivered"] = True
    append_alert(alert_path, record)
    return record


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--wall-url", default="https://wheretoken.plainlist.space/api/v1/public-profile/rainhuang0220")
    parser.add_argument("--repo", default="rainhuang0220/whereToken")
    parser.add_argument("--state", required=True)
    parser.add_argument("--alerts", required=True)
    parser.add_argument("--mac-state")
    parser.add_argument("--timeout", type=float, default=8)
    parser.add_argument("--now")
    parser.add_argument("--alert-command", nargs="*")
    parser.add_argument("--alert-timeout", type=float, default=30)
    parser.add_argument("--skip-actions", action="store_true")
    parser.add_argument("--workflow-missing", action="store_true")
    parser.add_argument("--workflow-at")
    args = parser.parse_args(argv)
    classify = load_classify()
    now = classify.parse_time(args.now) if args.now else datetime.now(timezone.utc)
    if now is None:
        print("watchdog_failed")
        return 3
    observation = {"fetched_at": now.strftime("%Y-%m-%dT%H:%M:%SZ")}
    try:
        wall = json.loads(fetch_bytes(args.wall_url, args.timeout))
        observation["wall"] = "ok"
    except (OSError, urllib.error.URLError, json.JSONDecodeError, TimeoutError):
        wall = None
        observation["wall"] = "unreachable"
    workflow_at = args.workflow_at
    actions_kind = "skipped"
    if workflow_at:
        actions_kind = "provided"
    elif args.workflow_missing:
        actions_kind = "silent"
    elif not args.skip_actions:
        try:
            actions_kind, found_at = observe_actions(args.repo, args.timeout, now, classify)
            if found_at:
                workflow_at = found_at
        except (OSError, subprocess.SubprocessError, json.JSONDecodeError):
            actions_kind = "lookup_failed"
            workflow_at = None
    observation["actions"] = actions_kind
    actions_status = ""
    if actions_kind == "lookup_failed":
        actions_status = "lookup_failed"
    elif actions_kind == "failed":
        actions_status = "failed"
    elif actions_kind == "silent":
        actions_status = "silent"
    mac_state = classify.load_json(args.mac_state) if args.mac_state else None
    if wall is None:
        actions_classes = "check_failed"
        local = ["check_failed"]
        if actions_status == "failed":
            local.append("workflow_failed")
        elif actions_status == "silent":
            local.append("workflow_silent")
        elif actions_status != "lookup_failed":
            stamp = classify.parse_time(workflow_at) if isinstance(workflow_at, str) else None
            age = classify.age_seconds(now, stamp)
            if age is not None and age > classify.SCHEDULE_GAP_S:
                local.append("workflow_silent")
        local_classes = " ".join(local)
    else:
        actions_classes = classify.classify("actions", wall, None, None, now)
        local_classes = classify.classify(
            "local", wall, mac_state, workflow_at, now,
            actions_status == "silent", actions_status,
        )
        if actions_status == "lookup_failed" and actions_classes == "ok":
            actions_classes = "check_failed"
        elif actions_status == "lookup_failed" and "check_failed" not in actions_classes.split():
            actions_classes = "check_failed " + actions_classes
    observation["actions_classes"] = actions_classes
    observation["local_classes"] = local_classes
    try:
        save_json(args.state, observation)
    except OSError:
        print("watchdog_failed")
        return 3
    try:
        deliver(args.alerts, "actions", actions_classes, args.alert_command, args.alert_timeout)
        deliver(args.alerts, "local", local_classes, args.alert_command, args.alert_timeout)
    except DeliveryError:
        print("delivery_failed")
        return 3
    except OSError:
        print("watchdog_failed")
        return 3
    print(f"actions={actions_classes}")
    print(f"local={local_classes}")
    if actions_classes != "ok" or local_classes != "ok":
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
