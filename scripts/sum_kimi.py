#!/usr/bin/env python3
"""Sum Kimi Code usage.record ledgers. Read-only. argv[1] overrides the root."""

from __future__ import annotations

import json
import sys
from pathlib import Path


def sum_kimi(root: Path) -> dict[str, int]:
    miss = cache_read = cache_create = output = 0
    for path in root.rglob("wire.jsonl"):
        with path.open(encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    rec = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if rec.get("type") != "usage.record":
                    continue
                if str(rec.get("usageScope") or "").lower() == "session":
                    continue
                usage = rec.get("usage") or {}
                miss += int(usage.get("inputOther") or 0)
                cache_read += int(usage.get("inputCacheRead") or 0)
                cache_create += int(usage.get("inputCacheCreation") or 0)
                output += int(usage.get("output") or 0)
    return {
        "miss": miss,
        "cache_read": cache_read,
        "cache_create": cache_create,
        "output": output,
        "total": miss + cache_read + cache_create + output,
    }


def main() -> None:
    root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path.home() / ".kimi-code"
    json.dump(sum_kimi(root), sys.stdout)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
