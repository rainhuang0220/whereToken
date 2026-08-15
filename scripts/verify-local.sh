#!/usr/bin/env bash
# Cross-check wheretoken scan --json against independent Python sums.
# Does not write anything under $HOME.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "== fixture: testdata/adapters/kimi"
python3 - <<'PY'
import json, subprocess
raw = subprocess.check_output(["python3", "scripts/sum_kimi.py", "testdata/adapters/kimi"], text=True)
d = json.loads(raw)
want = {"miss": 150, "cache_read": 1000, "cache_create": 20, "output": 15, "total": 1185}
if d != want:
    raise SystemExit(f"fixture mismatch {d}")
print("ok kimi fixture", d)
PY

echo "== build"
mkdir -p bin
go build -o bin/wheretoken ./cmd/wheretoken

echo "== scan --json"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
./bin/wheretoken scan --json >"$tmp"

python3 - "$tmp" <<'PY'
import json, subprocess, sys
from pathlib import Path

scan = json.loads(Path(sys.argv[1]).read_text())

def by_id(key: str, id_: str):
    for row in scan.get(key) or []:
        if row.get("id") == id_:
            return row
    return None

def check(name: str, py: dict, sl: dict | None) -> None:
    if sl is None:
        raise SystemExit(f"{name}: present on disk but missing in scan JSON")
    for field in ("miss", "cache_read", "cache_create", "output"):
        if int(py[field]) != int(sl[field]):
            raise SystemExit(f"{name} {field}: python={py[field]} scan={sl[field]}")
    print(f"ok {name} total={py['total']}")

home = Path.home()
kimi = home / ".kimi-code"
if kimi.is_dir():
    py = json.loads(subprocess.check_output(["python3", "scripts/sum_kimi.py", str(kimi)], text=True))
    check("kimi", py, by_id("by_source", "kimi"))
else:
    print("skip kimi: ~/.kimi-code absent")

oc_dir = home / ".local" / "share" / "opencode"
db = oc_dir / "opencode.db"
if not db.is_file():
    db = oc_dir / "opencode-stable.db"
if db.is_file():
    py = json.loads(subprocess.check_output(["python3", "scripts/sum_opencode.py", str(db)], text=True))
    check("opencode", py, by_id("by_source", "opencode"))
else:
    print("skip opencode: db absent")

print("--- claude ---")
print(json.dumps(by_id("by_source", "claude"), ensure_ascii=False, indent=2))
print("--- codex ---")
print(json.dumps(by_id("by_source", "codex"), ensure_ascii=False, indent=2))
print("--- all ---")
print(json.dumps(scan.get("all"), ensure_ascii=False, indent=2))

src = sum(int(s["total"]) for s in scan.get("by_source") or [])
vend = sum(int(s["total"]) for s in scan.get("by_vendor") or [])
all_t = int(scan["all"]["total"])
if src != all_t or vend != all_t:
    raise SystemExit(f"conservation fail src={src} vend={vend} all={all_t}")
print(f"ok conservation all={all_t}")

days = ((scan.get("calendar") or {}).get("all") or {}).get("days") or []
day_sum = sum(int(d["total"]) for d in days)
if day_sum != all_t:
    raise SystemExit(f"calendar conservation fail days={day_sum} all={all_t}")
print(f"ok calendar days={day_sum}")
if (scan.get("calendar") or {}).get("week_start") != "monday":
    raise SystemExit(f"week_start={(scan.get('calendar') or {}).get('week_start')}")
print("ok calendar week_start=monday")
errs = scan.get("errors") or []
if errs:
    print("scan errors:", errs)
PY
