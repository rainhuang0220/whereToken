(function () {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const params = new URLSearchParams(location.search);
  const state = {
    range: params.get("range") || "all",
    tab: params.get("tab") || "agents",
    filter: params.get("filter") || "",
    snap: null,
  };

  function applyTheme(mode) {
    if (mode === "light" || mode === "dark") {
      document.documentElement.setAttribute("data-theme", mode);
    } else {
      document.documentElement.removeAttribute("data-theme");
      mode = "system";
    }
    try { localStorage.setItem("wt-theme", mode); } catch (_) {}
    document.querySelectorAll("[data-theme-set]").forEach((b) => {
      b.setAttribute("aria-pressed", b.getAttribute("data-theme-set") === mode ? "true" : "false");
    });
  }

  function periodOf(snap, id) {
    return snap.periods[id] || snap.periods.all;
  }

  function validateSnapshot(snap) {
	if (!snap || typeof snap !== "object") throw new Error("snapshot must be an object");
	if (snap.schema !== "wheretoken.public-profile" || snap.schema_version !== 1) throw new Error("schema mismatch");
	if (!/^sha256:[0-9a-f]{64}$/.test(snap.snapshot_id || "")) throw new Error("snapshot id mismatch");
	const provenance = snap.provenance || {};
	const local = provenance.kind === "local_sanitized_snapshot" && provenance.refresh_mode === "manual_publish";
	const demo = provenance.kind === "synthetic_demo" && provenance.refresh_mode === "committed_fixture";
	if ((!local && !demo) || provenance.live_sync !== false) throw new Error("provenance mismatch");
	for (const id of ["all", "today", "7d", "30d", "53w"]) {
	  const period = snap.periods && snap.periods[id];
	  if (!period || !period.totals || !period.totals.total || !period.range) throw new Error("period mismatch");
	}
	const activity = snap.activity || {};
	if (!Array.isArray(activity.dates) || activity.dates.length !== 371 || !Array.isArray(activity.series)) throw new Error("activity mismatch");
	for (const series of activity.series) {
	  if (![series.values, series.levels, series.states].every((items) => Array.isArray(items) && items.length === activity.dates.length)) {
		throw new Error("activity series mismatch");
	  }
	}
	return snap;
  }

  function writeURL() {
    const q = new URLSearchParams();
    if (state.range !== "all") q.set("range", state.range);
    if (state.tab !== "agents") q.set("tab", state.tab);
    if (state.filter) q.set("filter", state.filter);
    const s = q.toString();
    history.replaceState(null, "", s ? "?" + s : location.pathname);
  }

  function seriesFor(snap) {
    const wantDim = state.tab === "providers" ? "vendor" : state.tab === "models" ? "model" : "agent";
    if (!state.filter) {
      return snap.activity.series.find((s) => s.dimension === "all") || snap.activity.series[0];
    }
    return snap.activity.series.find((s) => s.dimension === wantDim && s.id === state.filter)
      || snap.activity.series.find((s) => s.dimension === "all");
  }

  function render() {
    const snap = state.snap;
    if (!snap) return;
    const p = periodOf(snap, state.range);
	const demo = snap.provenance && snap.provenance.kind === "synthetic_demo";
	$("freshness").textContent = (demo ? "DEMO DATA · synthetic snapshot" : "Public snapshot · generated locally") + " · updated " + (snap.generated_at || snap.as_of_date);
    const owner = snap.owner || {};
    $("identity").textContent = owner.display_name || owner.github_login || "Local public snapshot";
    const kpis = [
      ["All-time tokens", snap.periods.all.totals.total.display, "This page default range is All"],
      ["This range", p.totals.total.display, p.range.label],
      ["Cache hit rate", p.hit_rate.display],
      ["Requests", p.requests.display],
      ["Current streak", p.current_streak.display],
      ["Active days", p.active_days.display],
    ];
    $("kpis").replaceChildren();
    kpis.forEach(([k, v]) => {
      const d = document.createElement("div");
      const dt = document.createElement("dt");
      dt.textContent = k;
      const dd = document.createElement("dd");
      dd.textContent = v;
      d.append(dt, dd);
      $("kpis").append(d);
    });
    if (snap.data_status === "partial") {
      $("status").hidden = false;
      $("status").textContent = "Partial data";
    } else if (snap.data_status === "unavailable") {
      $("status").hidden = false;
      $("status").textContent = "No public activity in this snapshot.";
    } else {
      $("status").hidden = true;
    }

    const ranges = ["today", "7d", "30d", "53w", "all"];
    const labels = { today: "Today", "7d": "7d", "30d": "30d", "53w": "53w", all: "All" };
    $("ranges").replaceChildren();
    ranges.forEach((id) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = labels[id];
      b.setAttribute("role", "tab");
      b.setAttribute("aria-selected", state.range === id ? "true" : "false");
      b.addEventListener("click", () => { state.range = id; writeURL(); render(); });
      $("ranges").append(b);
    });

    const ser = seriesFor(snap);
    const wall = $("wall");
    wall.replaceChildren();
    const dates = snap.activity.dates || [];
    dates.forEach((date, i) => {
      const st = (ser && ser.states[i]) || "empty";
      const val = (ser && ser.values[i]) || 0;
      const lv = (ser && ser.levels[i]) || 0;
      const cell = document.createElement("button");
      cell.type = "button";
      cell.className = "cell " + st;
      cell.style.opacity = st === "active" ? String(0.2 + lv * 0.2) : "";
      cell.dataset.date = date;
      cell.setAttribute("aria-label", date + " " + st);
      cell.addEventListener("focus", (e) => showTip(e, date, val, st, ser));
      cell.addEventListener("mouseenter", (e) => showTip(e, date, val, st, ser));
      cell.addEventListener("blur", hideTip);
      cell.addEventListener("mouseleave", hideTip);
      wall.append(cell);
    });
	if (!wall.dataset.positioned) {
	  wall.dataset.positioned = "true";
	  requestAnimationFrame(() => { wall.scrollLeft = wall.scrollWidth; });
	}

    const tabs = [{ id: "agents", label: "Agents" }, { id: "providers", label: "Providers" }];
    if ((p.by_model || []).length) tabs.push({ id: "models", label: "Models" });
    if (state.tab === "models" && tabs.length < 3) state.tab = "agents";
    $("tabs").replaceChildren();
    tabs.forEach((tab) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = tab.label;
      b.setAttribute("role", "tab");
      b.setAttribute("aria-selected", state.tab === tab.id ? "true" : "false");
      b.addEventListener("click", () => { state.tab = tab.id; state.filter = ""; writeURL(); render(); });
      $("tabs").append(b);
    });
    const rows = state.tab === "providers" ? p.by_vendor : state.tab === "models" ? (p.by_model || []) : p.by_agent;
    $("rows").replaceChildren();
    (rows || []).forEach((row) => {
      const b = document.createElement("button");
      b.type = "button";
      b.setAttribute("aria-pressed", state.filter === row.id ? "true" : "false");
      const left = document.createElement("span");
      left.textContent = row.label;
      const right = document.createElement("span");
      right.textContent = row.totals.total.display + " · " + row.share;
      b.append(left, right);
      b.addEventListener("click", () => {
        state.filter = state.filter === row.id ? "" : row.id;
        writeURL();
        render();
      });
      $("rows").append(b);
    });
  }

  function showTip(ev, date, val, st, ser) {
    const tip = $("tip");
    tip.hidden = false;
    tip.textContent = date + " · " + (st === "future" ? "future" : st === "unknown" ? "unknown" : String(val) + " tokens") + (ser && ser.label ? " · " + ser.label : "");
    const r = ev.target.getBoundingClientRect();
    tip.style.left = Math.min(r.left, window.innerWidth - 220) + "px";
    tip.style.top = (r.bottom + 8) + "px";
  }
  function hideTip() { $("tip").hidden = true; }

  document.querySelectorAll("[data-theme-set]").forEach((b) => {
    b.addEventListener("click", () => applyTheme(b.getAttribute("data-theme-set")));
  });
  try { applyTheme(localStorage.getItem("wt-theme") || "system"); } catch (_) { applyTheme("system"); }

  fetch("./profile.json", { cache: "no-store" })
    .then((r) => { if (!r.ok) throw new Error("snapshot missing"); return r.json(); })
	.then((snap) => {
	  validateSnapshot(snap);
      state.snap = snap;
      render();
    })
    .catch((err) => {
      $("status").hidden = false;
      $("status").textContent = "Could not load public snapshot: " + err.message;
    });
})();
