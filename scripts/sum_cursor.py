#!/usr/bin/env python3
"""Sum Cursor state.vscdb usage by key prefix. Read-only. No prompt bodies.

argv[1] is state.vscdb or a Cursor User dir / app-support dir.
"""

from __future__ import annotations

import json
import sqlite3
import sys
from pathlib import Path

THINKING = 30


def db_path(root: Path) -> Path:
    if root.is_file():
        return root
    for rel in (
        Path("User") / "globalStorage" / "state.vscdb",
        Path("globalStorage") / "state.vscdb",
        Path("state.vscdb"),
    ):
        p = root / rel
        if p.is_file():
            return p
    return root / "User" / "globalStorage" / "state.vscdb"


def _num(v) -> int:
    if v is None or v == "":
        return 0
    try:
        return int(v)
    except (TypeError, ValueError):
        try:
            return int(float(v))
        except (TypeError, ValueError):
            return 0


def sum_cursor(path: Path) -> dict[str, int]:
    uri = f"file:{path}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    try:
        sub = set()
        tables = {
            r[0]
            for r in conn.execute("SELECT name FROM sqlite_master WHERE type='table'")
        }
        if "composerHeaders" in tables:
            for cid, flag in conn.execute("SELECT composerId, isSubagent FROM composerHeaders"):
                if flag:
                    sub.add(cid)
        miss = cache_read = cache_create = output = requests = turns = 0
        q = """
        SELECT key,
          json_extract(value, '$.type'),
          json_extract(value, '$.tokenCount.inputTokens'),
          json_extract(value, '$.tokenCount.outputTokens'),
          json_extract(value, '$.tokenCount.cacheReadTokens'),
          json_extract(value, '$.tokenCount.cacheWriteTokens'),
          json_extract(value, '$.capabilityType')
        FROM cursorDiskKV
        WHERE key LIKE 'bubbleId:%'
        """
        for key, typ, inn, out, cr, cw, cap in conn.execute(q):
            parts = str(key).split(":", 2)
            cid = parts[1] if len(parts) >= 3 else ""
            t = _num(typ)
            if t == 1:
                if cid not in sub:
                    turns += 1
                continue
            if t != 2 or _num(cap) == THINKING:
                continue
            requests += 1
            miss += _num(inn)
            output += _num(out)
            cache_read += _num(cr)
            cache_create += _num(cw)
        return {
            "miss": miss,
            "cache_read": cache_read,
            "cache_create": cache_create,
            "output": output,
            "total": miss + cache_read + cache_create + output,
            "requests": requests,
            "user_turns": turns,
        }
    finally:
        conn.close()


def main() -> None:
    default = (
        Path.home()
        / "Library"
        / "Application Support"
        / "Cursor"
        / "User"
        / "globalStorage"
        / "state.vscdb"
    )
    root = Path(sys.argv[1]) if len(sys.argv) > 1 else default
    json.dump(sum_cursor(db_path(root)), sys.stdout)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
