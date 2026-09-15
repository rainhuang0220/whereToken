(function () {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const params = new URLSearchParams(location.search);
  const state = {
    range: params.get("range") || "all",
    tab: params.get("tab") || "agents",
    filter: params.get("filter") || "",
    metric: params.get("metric") === "requests" ? "requests" : "tokens",
    snap: null,
  };
  let tipTimer = 0;
  let tipFrame = 0;
  let tipTarget = null;
  let hoverTarget = null;
  let focusTarget = null;

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

  function metricOf(series) {
    return series && series.metric ? series.metric : "tokens";
  }

  function validateSnapshot(snap) {
    if (!snap || typeof snap !== "object") throw new Error("snapshot must be an object");
    if (snap.schema !== "wheretoken.public-profile" || (snap.schema_version !== 1 && snap.schema_version !== 2)) {
      throw new Error("schema mismatch");
    }
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
    if (state.metric !== "tokens") q.set("metric", state.metric);
    const s = q.toString();
    history.replaceState(null, "", s ? "?" + s : location.pathname);
  }

  function seriesFor(snap) {
    const wantDim = state.tab === "providers" ? "vendor" : state.tab === "models" ? "model" : "agent";
    const series = snap.activity.series || [];
    if (!state.filter) {
      return series.find((s) => s.dimension === "all" && metricOf(s) === state.metric) || null;
    }
    return series.find((s) => s.dimension === wantDim && s.id === state.filter && metricOf(s) === state.metric) || null;
  }

  function coverageOf(row) {
    if (row && row.coverage && row.coverage.tokens) return row.coverage;
    const total = row && row.totals && row.totals.total ? row.totals.total : {};
    const zero = total.value == null || total.value === 0;
    const unavailable = row && row.quality === "degraded" && zero;
    return {
      tokens: unavailable ? "unavailable" : (total.status || "available"),
      requests: row && row.requests ? row.requests.status : "available",
      token_source: "unknown",
      reason: unavailable ? "local_tokens_missing" : "",
    };
  }

  function compact(n) {
    if (n == null || n < 0) return "—";
    if (n < 1000) return String(n);
    const units = [[1e12, "T"], [1e9, "B"], [1e6, "M"], [1e3, "K"]];
    for (const [div, suffix] of units) {
      if (n >= div) {
        const scaled = n / div;
        const text = scaled >= 100 ? scaled.toFixed(1) : scaled >= 10 ? scaled.toFixed(1) : scaled.toFixed(2);
        return String(text).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1") + suffix;
      }
    }
    return String(n);
  }

  function formatDay(iso) {
    if (!iso) return "";
    const d = new Date(iso + "T00:00:00");
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
  }

  function monthName(iso) {
    const d = new Date(iso + "T00:00:00");
    return d.toLocaleDateString("en-US", { month: "short" });
  }

  function unitLabel() {
    return state.metric === "requests" ? "requests" : "tokens";
  }

  function selectedRow(snap) {
    const p = periodOf(snap, state.range);
    const rows = state.tab === "providers" ? p.by_vendor : state.tab === "models" ? (p.by_model || []) : p.by_agent;
    return (rows || []).find((row) => row.id === state.filter) || null;
  }

  function hasMixedCoverage(snap) {
    const rows = (snap.periods && snap.periods.all && snap.periods.all.by_agent) || [];
    return rows.some((row) => coverageOf(row).token_source === "account_api") &&
      rows.some((row) => coverageOf(row).token_source !== "account_api");
  }

  function render() {
    const snap = state.snap;
    if (!snap) return;
    dismissTip();
    const p = periodOf(snap, state.range);
    const demo = snap.provenance && snap.provenance.kind === "synthetic_demo";
    const when = (snap.as_of_date || "").slice(0, 10);
    const pretty = when ? formatDay(when).replace(/,\s+\d{4}$/, "") : "";
    $("freshness").textContent = (demo ? "DEMO DATA · " : "") + (pretty ? "updated " + pretty : "");
    const owner = snap.owner || {};
    const identityNode = $("identity");
    identityNode.textContent = owner.display_name || owner.github_login || "";
    const identity = identityNode.textContent;
    $("identity").hidden = !identity;
    if (snap.links && /^https:\/\//.test(snap.links.project || "")) $("github").href = snap.links.project;

    const total = state.metric === "requests" ? p.requests : p.totals.total;
    $("hero-value").textContent = total.display || "—";
    $("hero-label").textContent = state.metric === "requests" ? "requests" : "tokens";
    const heroCoverage = $("hero-coverage");
    const coverageIssue = hasMixedCoverage(snap) || snap.data_status !== "available";
    heroCoverage.hidden = !coverageIssue;
    const coverageText = heroCoverage.querySelector("span");
    if (coverageText) {
      coverageText.textContent = snap.data_status === "unavailable" ? "Token coverage unavailable." :
        snap.data_status === "partial" ? "Partial coverage." : "Coverage varies by source.";
    }
    const items = [
      [p.requests.display || "—", "requests"],
      [p.active_days.display || "—", "active days"],
      [p.current_streak.display || "—", "day current streak"],
    ];
    $("hero-meta").replaceChildren();
    items.forEach(([value, note]) => {
      const li = document.createElement("li");
      const strong = document.createElement("strong");
      strong.textContent = value;
      li.append(strong, note ? document.createTextNode(" · " + note) : document.createTextNode(""));
      $("hero-meta").append(li);
    });

    $("status").hidden = snap.data_status !== "unavailable";
    if (snap.data_status === "unavailable") {
      $("status").textContent = "No public activity in this snapshot.";
    }

    const rangeLabels = { today: "Today", "7d": "7d", "30d": "30d", "53w": "53w", all: "All" };
    renderSeg($("metrics"), [
      { id: "tokens", label: "Tokens" },
      { id: "requests", label: "Requests" },
    ], state.metric, (id) => { state.metric = id; writeURL(); render(); });
    renderSeg($("ranges"), ["today", "7d", "30d", "53w", "all"].map((id) => ({ id, label: rangeLabels[id] })), state.range, (id) => {
      state.range = id; writeURL(); render();
    });
    $("range-readout").textContent = (p.active_days.display || "—") + " active days";

    const ser = seriesFor(snap);
    const row = selectedRow(snap);
    $("series-heading").textContent = row ? row.label : "All agents";
    renderWall(snap, ser, row);
    renderTrend(snap, ser);
    renderBreakdown(snap, p);
    renderCoverage(snap);
  }

  function renderSeg(root, items, selected, onPick) {
    root.replaceChildren();
    items.forEach((item) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = item.label;
      b.setAttribute("role", "tab");
      b.setAttribute("aria-selected", selected === item.id ? "true" : "false");
      b.addEventListener("click", () => onPick(item.id));
      root.append(b);
    });
  }

  function renderWall(snap, ser, row) {
    const wall = $("wall");
    const empty = $("wall-empty");
    const dates = snap.activity.dates || [];
    wall.replaceChildren();
    $("months").replaceChildren();
    $("weekdays").replaceChildren();
    $("legend").replaceChildren();
    if (!ser) {
      empty.hidden = false;
      empty.replaceChildren();
      const title = document.createElement("p");
      const kind = state.metric === "requests" ? "Request" : "Token";
      title.textContent = kind + " activity unavailable" + (row ? " for " + row.label : "");
      empty.append(title);
      if (state.metric === "tokens") {
        const alt = (snap.activity.series || []).find((s) => s.dimension === (state.filter ? (state.tab === "providers" ? "vendor" : "agent") : "all") && s.id === (state.filter || "all") && metricOf(s) === "requests");
        if (alt) {
          const b = document.createElement("button");
          b.type = "button";
          b.textContent = "View request activity";
          b.addEventListener("click", () => { state.metric = "requests"; writeURL(); render(); });
          empty.append(b);
        }
      }
      return;
    }
    empty.hidden = true;

    const weekdays = document.createElement("span");
    weekdays.style.height = "18px";
    $("weekdays").append(weekdays);
    ["Mon", "", "Wed", "", "Fri", "", ""].forEach((label) => {
      const s = document.createElement("span");
      s.textContent = label;
      $("weekdays").append(s);
    });

    let lastMonth = "";
    for (let col = 0; col < 53; col++) {
      const date = dates[col * 7];
      const label = document.createElement("span");
      if (date) {
        const m = date.slice(0, 7);
        if (m !== lastMonth) {
          label.textContent = monthName(date);
          lastMonth = m;
        }
      }
      $("months").append(label);
    }
    updateMonthLabelVisibility();

    let peak = -1;
    let peakIdx = -1;
    dates.forEach((_, i) => {
      if ((ser.states[i] || "") === "active" && ser.values[i] > peak) {
        peak = ser.values[i];
        peakIdx = i;
      }
    });

    dates.forEach((date, i) => {
      const st = (ser.states[i]) || "empty";
      const val = ser.values[i] || 0;
      const lv = ser.levels[i] || 0;
      const cell = document.createElement("button");
      cell.type = "button";
      cell.className = "cell " + st + (st === "active" ? " lv" + Math.min(Math.max(lv, 1), 5) : "") + (i === peakIdx ? " peak" : "");
      cell.dataset.date = date;
      cell.setAttribute("aria-describedby", "tip");
      cell.setAttribute("aria-label", formatDay(date) + ", " + tipValue(val, st) + (state.filter && ser.label ? ", " + ser.label : ""));
      const show = (e) => showTip(e, date, val, st, ser);
      const enter = (e) => { hoverTarget = cell; show(e); };
      const leave = () => {
        if (hoverTarget === cell) hoverTarget = null;
        scheduleTipHide(cell);
      };
      cell.addEventListener("focus", (e) => { focusTarget = cell; show(e); });
      cell.addEventListener("pointerenter", enter);
      cell.addEventListener("mouseenter", enter);
      cell.addEventListener("blur", () => {
        if (focusTarget === cell) focusTarget = null;
        scheduleTipHide(cell);
      });
      cell.addEventListener("pointerleave", leave);
      cell.addEventListener("mouseleave", leave);
      wall.append(cell);
    });
    if (!wall.dataset.positioned) {
      wall.dataset.positioned = "true";
      requestAnimationFrame(() => {
        const scroller = $("heat-scroll");
        scroller.scrollLeft = scroller.scrollWidth;
      });
    }

    const legend = $("legend");
    const less = document.createElement("span");
    less.textContent = "Less";
    legend.append(less);
    ["empty", "lv1", "lv2", "lv3", "lv4", "lv5"].forEach((cls) => {
      const i = document.createElement("i");
      i.className = "cell " + (cls === "empty" ? "empty" : cls);
      legend.append(i);
    });
    const more = document.createElement("span");
    more.textContent = "More";
    legend.append(more);
  }

  function updateMonthLabelVisibility() {
    const scroller = $("heat-scroll");
    const bounds = scroller.getBoundingClientRect();
    $("months").querySelectorAll("span").forEach((label) => {
      const rect = label.getBoundingClientRect();
      const fullyVisible = rect.left >= bounds.left && rect.right <= bounds.right;
      label.style.visibility = label.textContent && fullyVisible ? "" : "hidden";
    });
  }

  function renderTrend(snap, ser) {
    const host = $("trend");
    const label = $("trend-value");
    host.replaceChildren();
    if (!ser) {
      label.textContent = "";
      return;
    }
    const dates = snap.activity.dates || [];
    let end = dates.findIndex((date) => date > snap.activity.to);
    if (end < 0) end = dates.length;
    const windowDays = { today: 1, "7d": 7, "30d": 30, "53w": 371, all: 371 }[state.range] || 30;
    const start = Math.max(0, end - windowDays);
    const trendLabels = { today: "Today", "7d": "Last 7 days", "30d": "Last 30 days", "53w": "Last 53 weeks", all: "Available 53-week activity" };
    $("trend-label").textContent = trendLabels[state.range];
    host.setAttribute("aria-label", trendLabels[state.range]);
    const pts = [];
    for (let i = start; i < end; i++) {
      if ((ser.states[i] || "") === "future") continue;
      pts.push({ date: dates[i], value: ser.values[i] || 0 });
    }
    const max = Math.max(1, ...pts.map((p) => p.value));
    const sum = pts.reduce((n, p) => n + p.value, 0);
    label.textContent = compact(sum) + " " + unitLabel();
    const w = 1000;
    const h = 72;
    const ns = "http://www.w3.org/2000/svg";
    const svg = document.createElementNS(ns, "svg");
    svg.setAttribute("viewBox", "0 0 " + w + " " + h);
    svg.setAttribute("preserveAspectRatio", "none");
    const pad = 4;
    const xy = pts.map((p, i) => {
      const x = pts.length === 1 ? w / 2 : pad + (i * (w - pad * 2)) / (pts.length - 1);
      const y = h - pad - (p.value / max) * (h - pad * 2);
      return [x, y, p];
    });
    const d = xy.map((p, i) => (i ? "L" : "M") + p[0].toFixed(1) + " " + p[1].toFixed(1)).join(" ");
    const line = document.createElementNS(ns, "path");
    line.setAttribute("class", "line");
    line.setAttribute("d", d);
    svg.append(line);
    const dot = document.createElementNS(ns, "circle");
    dot.setAttribute("class", "dot");
    dot.setAttribute("r", "4");
    dot.setAttribute("visibility", "hidden");
    svg.append(dot);
    svg.addEventListener("mousemove", (ev) => {
      const r = svg.getBoundingClientRect();
      const x = ((ev.clientX - r.left) / r.width) * w;
      let best = 0;
      let dist = Infinity;
      xy.forEach((p, i) => {
        const dx = Math.abs(p[0] - x);
        if (dx < dist) { dist = dx; best = i; }
      });
      const p = xy[best];
      dot.setAttribute("cx", p[0]);
      dot.setAttribute("cy", p[1]);
      dot.setAttribute("visibility", "visible");
      label.textContent = formatDay(p[2].date) + " · " + compact(p[2].value) + " " + unitLabel();
    });
    svg.addEventListener("mouseleave", () => {
      dot.setAttribute("visibility", "hidden");
      label.textContent = compact(sum) + " " + unitLabel();
    });
    host.append(svg);
  }

  function renderBreakdown(snap, p) {
    const tabs = [{ id: "agents", label: "Agents" }, { id: "providers", label: "Providers" }];
    if ((p.by_model || []).length) tabs.push({ id: "models", label: "Models" });
    if (state.tab === "models" && tabs.length < 3) state.tab = "agents";
    renderSeg($("tabs"), tabs, state.tab, (id) => { state.tab = id; state.filter = ""; writeURL(); render(); });
    const rows = state.tab === "providers" ? p.by_vendor : state.tab === "models" ? (p.by_model || []) : p.by_agent;
    $("rows").replaceChildren();
    const max = Math.max(1, ...(rows || []).map((row) => {
      const cov = coverageOf(row);
      if (cov.tokens === "unavailable") return 0;
      return row.totals.total.value || 0;
    }));
    (rows || []).forEach((row, idx) => {
      const cov = coverageOf(row);
      const b = document.createElement("button");
      b.type = "button";
      b.className = "rank-row";
      b.setAttribute("aria-pressed", state.filter === row.id ? "true" : "false");
      const n = document.createElement("span");
      n.className = "rank-n";
      n.textContent = String(idx + 1);
      const body = document.createElement("span");
      const top = document.createElement("span");
      top.className = "rank-top";
      const name = document.createElement("span");
      name.className = "rank-name";
      name.textContent = row.label;
      const value = document.createElement("span");
      value.className = "rank-value";
      value.textContent = cov.tokens === "unavailable" ? "—" : row.totals.total.display;
      top.append(name, value);
      const bar = document.createElement("span");
      bar.className = "rank-bar";
      const fill = document.createElement("i");
      const pct = cov.tokens === "unavailable" ? 0 : Math.max(0, (row.totals.total.value || 0) / max * 100);
      fill.style.width = pct + "%";
      bar.append(fill);
      const meta = document.createElement("span");
      meta.className = "rank-meta";
      const req = compact(row.requests && row.requests.value);
      if (cov.tokens === "unavailable") {
        meta.textContent = req + " requests";
      } else {
        meta.textContent = (row.share || "—") + " · " + req + " requests";
      }
      body.append(top, bar, meta);
      if (cov.tokens === "unavailable") {
        const note = document.createElement("span");
        note.className = "rank-note";
        note.textContent = "Token usage unavailable";
        body.append(note);
      } else if (cov.token_source === "account_api" && cov.token_window) {
        const note = document.createElement("span");
        note.className = "rank-note";
        note.textContent = coverageNote(cov);
        body.append(note);
      }
      b.append(n, body);
      b.addEventListener("click", () => {
        state.filter = state.filter === row.id ? "" : row.id;
        writeURL();
        render();
      });
      $("rows").append(b);
    });
  }

  function renderCoverage(snap) {
    const host = $("coverage-table");
    host.replaceChildren();
    const table = document.createElement("table");
    table.className = "coverage-table";
    const head = document.createElement("tr");
    ["Source", "Tokens", "Requests", "Notes"].forEach((label) => {
      const th = document.createElement("th");
      th.textContent = label;
      head.append(th);
    });
    table.append(head);
    (snap.periods.all.by_agent || []).forEach((row) => {
      const cov = coverageOf(row);
      const tr = document.createElement("tr");
      const cells = [
        row.label,
        cov.tokens === "available" ? "Available" : cov.tokens === "partial" ? "Partial" : "Unavailable",
        cov.requests === "available" ? "Available" : "Unavailable",
        coverageNote(cov),
      ];
      cells.forEach((text, i) => {
        const td = document.createElement("td");
        td.textContent = text;
        if (i === 1 || i === 2) td.className = text === "Unavailable" ? "miss" : "ok";
        tr.append(td);
      });
      table.append(tr);
    });
    host.append(table);
  }

  function coverageNote(cov) {
    switch (cov.reason) {
      case "account_api_skipped": return "Account usage skipped";
      case "auth_missing": return "Sign-in missing";
      case "api_failed": return "Account usage failed";
      case "local_tokens_missing": return "Token ledger missing";
      default:
        if (cov.token_source === "account_api" && cov.token_window) {
          const from = cov.token_window.from ? formatDay(cov.token_window.from) : "Start unavailable";
          return from + " – " + formatDay(cov.token_window.to);
        }
        if (cov.token_source === "account_api") return "Account usage";
        return "";
    }
  }

  function tipValue(val, st) {
    if (st === "future") return "Future";
    if (st === "unknown") return "Unknown";
    return compact(val) + " " + unitLabel();
  }

  function showTip(ev, date, val, st, ser) {
    const tip = $("tip");
    const target = ev.currentTarget;
    clearTimeout(tipTimer);
    cancelAnimationFrame(tipFrame);
    tipTarget = target;
    tip.hidden = false;
    tip.replaceChildren();
    const title = document.createElement("b");
    title.textContent = formatDay(date);
    const body = document.createElement("div");
    body.textContent = tipValue(val, st);
    const src = document.createElement("div");
    src.className = "muted";
    src.textContent = state.filter && ser && ser.label ? ser.label : "";
    tip.append(title, body);
    if (src.textContent) tip.append(src);
    tip.classList.add("is-on");
    tipFrame = requestAnimationFrame(() => {
      if (tipTarget !== target || !target.isConnected) return;
      placeTip(target, tip);
    });
  }

  function placeTip(target, tip) {
    const r = target.getBoundingClientRect();
    const tw = tip.offsetWidth || 180;
    const th = tip.offsetHeight || 64;
    const margin = 8;
    let left = r.left + r.width / 2 - tw / 2;
    let top = r.top - th - 8;
    if (top < margin) top = r.bottom + 8;
    const maxLeft = Math.max(margin, window.innerWidth - tw - margin);
    const maxTop = Math.max(margin, window.innerHeight - th - margin);
    left = Math.min(Math.max(left, margin), maxLeft);
    top = Math.min(Math.max(top, margin), maxTop);
    tip.style.left = left + "px";
    tip.style.top = top + "px";
  }

  function scheduleTipHide(target) {
    if (hoverTarget === target || focusTarget === target) return;
    clearTimeout(tipTimer);
    tipTimer = setTimeout(() => {
      if (tipTarget !== target || hoverTarget === target || focusTarget === target) return;
      dismissTip();
    }, 160);
  }

  function dismissTip() {
    const tip = $("tip");
    clearTimeout(tipTimer);
    cancelAnimationFrame(tipFrame);
    tipTimer = 0;
    tipFrame = 0;
    tipTarget = null;
    hoverTarget = null;
    focusTarget = null;
    tip.classList.remove("is-on");
    tip.hidden = true;
  }

  $("coverage-chip").addEventListener("click", () => {
    const box = $("coverage");
    box.open = true;
    box.scrollIntoView({ block: "nearest" });
  });
  document.querySelectorAll("[data-theme-set]").forEach((b) => {
    b.addEventListener("click", () => applyTheme(b.getAttribute("data-theme-set")));
  });
  $("heat-scroll").addEventListener("scroll", () => { dismissTip(); updateMonthLabelVisibility(); }, { passive: true });
  window.addEventListener("resize", () => { dismissTip(); updateMonthLabelVisibility(); }, { passive: true });
  window.addEventListener("scroll", dismissTip, { passive: true, capture: true });
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
