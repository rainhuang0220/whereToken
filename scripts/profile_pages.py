#!/usr/bin/env python3
"""Verify the deployed Pages fallback against the committed profile bundle."""

import argparse
from dataclasses import dataclass
from datetime import date, datetime, timezone
import http.client
import json
import os
from pathlib import Path
import re
import subprocess
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen
import xml.etree.ElementTree as ET

NAMES = ("manifest.json", "profile.json", "preview-light.svg", "preview-dark.svg")
SVG = "{http://www.w3.org/2000/svg}"
DIGEST = re.compile(r"sha256:[0-9a-f]{64}")
SCHEMA = "wheretoken.public-profile"
PROVENANCE = "local_sanitized_snapshot"
MAX_BYTES = 2 * 1024 * 1024


@dataclass(frozen=True)
class Identity:
    snapshot_id: str
    as_of: date
    generated_at: datetime
    asset_revision: str
    total: int


def identity(files):
    manifest = json.loads(files["manifest.json"])
    profile = json.loads(files["profile.json"])
    snapshot_id = profile["snapshot_id"]
    revision = manifest["asset_revision"]
    provenance = profile["provenance"]
    total = profile["periods"]["all"]["totals"]["total"]["value"]
    if (
        manifest["schema"] != SCHEMA or profile["schema"] != SCHEMA
        or manifest["provenance"] != PROVENANCE or provenance["kind"] != PROVENANCE
        or provenance["refresh_mode"] != "manual_publish" or provenance["live_sync"] is not False
        or not DIGEST.fullmatch(snapshot_id) or not DIGEST.fullmatch(revision)
        or type(total) is not int or total < 0
    ):
        raise ValueError("profile provenance or identity")
    for key in ("snapshot_id", "as_of_date", "generated_at"):
        if manifest[key] != profile[key]:
            raise ValueError("manifest and profile differ")
    as_of = date.fromisoformat(profile["as_of_date"])
    generated_at = datetime.fromisoformat(profile["generated_at"].replace("Z", "+00:00"))
    if generated_at.tzinfo is None:
        raise ValueError("generation time requires a zone")
    for name in NAMES[2:]:
        svg = ET.fromstring(files[name])
        metadata = svg.find(f'{SVG}metadata[@id="wheretoken-snapshot-id"]')
        desc = svg.findtext(f"{SVG}desc", "")
        if metadata is None or metadata.text != snapshot_id or not desc.startswith(f"Coding-agent token usage through {as_of}."):
            raise ValueError("preview and profile differ")
    return Identity(snapshot_id, as_of, generated_at, revision, total)


INVALID_BUNDLE = (json.JSONDecodeError, UnicodeError, ValueError, AttributeError, TypeError, KeyError, ET.ParseError)


def fetch(url, name, nonce):
    # Cache-Control alone is advisory to the Pages CDN.
    req = Request(f"{url.rstrip('/')}/{name}?verify={nonce}", headers={"Cache-Control": "no-cache"})
    with urlopen(req, timeout=8) as response:
        data = response.read(MAX_BYTES + 1)
        if len(data) > MAX_BYTES:
            raise ValueError("oversized public file")
        return data


def inspect_pages(url, expected_files, expected):
    nonce = time.time_ns()
    deployed_files = {}
    for name in NAMES:
        try:
            deployed_files[name] = fetch(url, name, nonce)
        except HTTPError as exc:
            return ("stale" if exc.code == 404 else "unavailable"), f"{name}_http_{exc.code}", None
        except (URLError, OSError, http.client.HTTPException):
            return "unavailable", f"{name}_network", None
        except ValueError:
            return "stale", f"{name}_invalid", None

    if deployed_files == expected_files:
        return "verified", "", expected
    try:
        deployed = identity(deployed_files)
    except INVALID_BUNDLE:
        return "stale", "bundle_mismatch", None
    # A later deployment may win a race with this check. Retain that bundle
    # and avoid dispatching a workflow that could restore an older revision.
    later = deployed.as_of > expected.as_of or (
        deployed.as_of == expected.as_of and deployed.generated_at > expected.generated_at
    )
    if later and deployed.total >= expected.total:
        return "superseded", "", deployed
    if later or deployed.total > expected.total:
        return "conflict", "publication_watermarks_disagree", deployed
    changed = next(name for name in NAMES if deployed_files[name] != expected_files[name])
    return "stale", f"{changed}_mismatch", deployed


def dispatch_repair(repo):
    try:
        result = subprocess.run(
            ["gh", "run", "list", "--repo", repo, "--branch", "main", "--workflow", "pages.yml",
             "--limit", "10", "--json", "status"],
            check=True, capture_output=True, text=True, timeout=20,
        )
        runs = json.loads(result.stdout)
        if any(run["status"] != "completed" for run in runs):
            return "deploy_pending", "active_pages_workflow"
    except (OSError, subprocess.SubprocessError, ValueError, TypeError, KeyError):
        return "dispatch_failed", "workflow_status_unavailable"
    try:
        subprocess.run(
            ["gh", "workflow", "run", "pages.yml", "--repo", repo, "--ref", "main"],
            check=True, capture_output=True, timeout=20,
        )
    except (OSError, subprocess.SubprocessError):
        return "dispatch_failed", "workflow_dispatch_failed"
    return "deploy_requested", "waiting_for_readback"


def report(status, reason, deployed=None):
    stamp = datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")
    lines = [f"pages_mirror={status}", f"pages_checked_at={stamp}"]
    if reason:
        lines.append(f"pages_reason={reason}")
    if deployed:
        lines.extend((f"pages_snapshot_id={deployed.snapshot_id}", f"pages_as_of_date={deployed.as_of}"))
    if status in ("verified", "superseded"):
        lines.append(f"pages_verified_at={stamp}")
    print("\n".join(lines))
    if path := os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(path, "a", encoding="utf-8") as summary:
            summary.write("### Pages mirror\n\n```text\n" + "\n".join(lines) + "\n```\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dir", type=Path, required=True, help="committed public-profile bundle")
    parser.add_argument("--url", required=True, help="deployed profile directory URL")
    parser.add_argument("--repair", action="store_true", help="request Pages deployment for confirmed drift")
    parser.add_argument("--attempts", type=int, choices=range(1, 6), default=1, help="bounded post-deploy readback attempts")
    args = parser.parse_args()
    repo = os.environ.get("GITHUB_REPOSITORY")
    if args.repair and not repo:
        parser.error("--repair requires GITHUB_REPOSITORY")

    try:
        expected_files = {name: (args.dir / name).read_bytes() for name in NAMES}
        expected = identity(expected_files)
    except (OSError, *INVALID_BUNDLE):
        report("check_failed", "invalid_committed_bundle")
        return 1
    for attempt in range(args.attempts):
        status, reason, deployed = inspect_pages(args.url, expected_files, expected)
        if status in ("verified", "superseded"):
            break
        if attempt + 1 < args.attempts:
            time.sleep(2 ** attempt)
    if status == "stale" and args.repair:
        dispatch_status, dispatch_reason = dispatch_repair(repo)
        reason = f"{reason},{dispatch_reason}"
        status = dispatch_status
    report(status, reason, deployed)
    return 0 if status in ("verified", "superseded") else 1


if __name__ == "__main__":
    raise SystemExit(main())
