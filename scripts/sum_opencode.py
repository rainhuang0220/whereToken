#!/usr/bin/env python3
"""Sum OpenCode message.data tokens. Read-only. argv[1] is a db file or data dir."""

from __future__ import annotations

import json
import sqlite3
import sys
from pathlib import Path


def db_path(root: Path) -> Path:
    if root.is_file():
        return root
    for name in ("opencode.db", "opencode-stable.db"):
        p = root / name
        if p.is_file():
            return p
    return root / "opencode.db"


def sum_opencode(path: Path) -> dict[str, int]:
    uri = f"file:{path}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    try:
        miss = cache_read = cache_create = output = 0
        for (raw,) in conn.execute("SELECT data FROM message"):
            if not raw:
                continue
            try:
                rec = json.loads(raw)
            except json.JSONDecodeError:
                continue
            tokens = rec.get("tokens")
            if not tokens:
                continue
            cache = tokens.get("cache") or {}
            reasoning = int(tokens.get("reasoning") or 0)
            miss += int(tokens.get("input") or 0)
            cache_read += int(cache.get("read") or 0)
            cache_create += int(cache.get("write") or 0)
            output += int(tokens.get("output") or 0) + reasoning
        return {
            "miss": miss,
            "cache_read": cache_read,
            "cache_create": cache_create,
            "output": output,
            "total": miss + cache_read + cache_create + output,
        }
    finally:
        conn.close()


def main() -> None:
    root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path.home() / ".local" / "share" / "opencode"
    json.dump(sum_opencode(db_path(root)), sys.stdout)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
