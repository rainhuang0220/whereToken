(function () {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const params = new URLSearchParams(location.search);
  const ABSOLUTE_TOKEN_CAP = 1_000_000_000;
  const HEAT_MID_POSITION = 0.25;
  const HEAT_HIGH_POSITION = 0.60;
  const WALL_PALETTES = {
    cobalt: {
      label: "Cobalt",
      stops: [
        [0, [0.88, 0.055, 260]],
        [HEAT_MID_POSITION, [0.72, 0.140, 260]],
        [HEAT_HIGH_POSITION, [0.54, 0.170, 260]],
        [1, [0.46, 0.180, 260]],
      ],
    },
    magenta: {
      label: "Magenta",
      stops: [
        [0, [0.88, 0.080, 340]],
        [HEAT_MID_POSITION, [0.72, 0.180, 340]],
        [HEAT_HIGH_POSITION, [0.54, 0.180, 340]],
        [1, [0.46, 0.190, 340]],
      ],
    },
    newsprint: {
      label: "Newsprint",
      texture: true,
      stops: [
        [0, [0.86, 0, 0]],
        [HEAT_MID_POSITION, [0.67, 0, 0]],
        [HEAT_HIGH_POSITION, [0.43, 0, 0]],
        [1, [0.20, 0, 0]],
      ],
    },
  };
  const state = {
    range: params.get("range") || "all",
    tab: params.get("tab") || "agents",
    filter: params.get("filter") || "",
    metric: params.get("metric") === "requests" ? "requests" : "tokens",
    palette: "newsprint",
    published: "newsprint",
    explicit: false,
    snap: null,
    freshnessSource: "fallback",
    hostedUpdatedAt: "",
    jobID: "",
  };
  const SESSION_KEY = "wt-profile-session";
  const LIVE_PROFILE = "/api/v1/public-profile/";
  const LIVE_AUTH = "/api/v1/auth/github";
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

  function publishedPalette(pres) {
    if (!pres || typeof pres !== "object" || pres.schema_version !== 1) return "newsprint";
    return WALL_PALETTES[pres.public_palette] ? pres.public_palette : "newsprint";
  }

  function visitorPalette() {
    try {
      const saved = localStorage.getItem("wt-visitor-palette");
      return WALL_PALETTES[saved] ? saved : "";
    } catch (_) {
      return "";
    }
  }

  function ownerHref(id) {
    return "http://127.0.0.1:8787/themes?public_palette=" + encodeURIComponent(id) + "&intent=publish#public-profile";
  }

  function syncOwnerLink() {
    const link = $("owner-set-link");
    if (!link) return;
    link.href = ownerHref(state.palette || "newsprint");
  }

  function bindOwnerLink() {
    const link = $("owner-set-link");
    const note = $("owner-set-note");
    if (!link || link.dataset.bound === "1") return;
    link.dataset.bound = "1";
    if (!window.matchMedia("(pointer: coarse)").matches) return;
    link.addEventListener("click", (event) => {
      event.preventDefault();
      const href = link.href;
      const done = "已复制。请在运行 wheretoken serve 的电脑上打开。手机上的 127.0.0.1 不是那台电脑。";
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(href).then(() => { note.textContent = done; }, () => { note.textContent = href; });
      } else {
        note.textContent = href;
      }
    });
  }

  function applyWallPalette(id, persist) {
    if (!WALL_PALETTES[id]) return;
    state.palette = id;
    document.documentElement.setAttribute("data-activity-palette", id);
    syncOwnerLink();
    if (!persist) return;
    state.explicit = true;
    try { localStorage.setItem("wt-visitor-palette", id); } catch (_) {}
  }

  function wallPalette() {
    return WALL_PALETTES[state.palette] || WALL_PALETTES.newsprint;
  }

  function presentationIntensity(value, level) {
    if (state.metric === "tokens") {
      return Math.sqrt(Math.min(Math.max(Number(value) || 0, 0) / ABSOLUTE_TOKEN_CAP, 1));
    }
    return level > 0 ? Math.min(level / 5, 1) : 0;
  }

  function heatColor(palette, intensity) {
    intensity = Math.min(Math.max(intensity, 0), 1);
    let from = palette.stops[0];
    let to = palette.stops[palette.stops.length - 1];
    for (let i = 1; i < palette.stops.length; i++) {
      if (intensity <= palette.stops[i][0]) {
        from = palette.stops[i - 1];
        to = palette.stops[i];
        break;
      }
    }
    const span = to[0] - from[0] || 1;
    const t = (intensity - from[0]) / span;
    const color = from[1].map((value, i) => value + (to[1][i] - value) * t);
    return oklchHex(color[0], color[1], color[2]);
  }

  function oklchHex(l, c, h) {
    const radians = h * Math.PI / 180;
    const a = c * Math.cos(radians);
    const b = c * Math.sin(radians);
    const lRoot = l + 0.3963377774 * a + 0.2158037573 * b;
    const mRoot = l - 0.1055613458 * a - 0.0638541728 * b;
    const sRoot = l - 0.0894841775 * a - 1.291485548 * b;
    const ll = lRoot ** 3;
    const mm = mRoot ** 3;
    const ss = sRoot ** 3;
    const rgb = [
      4.0767416621 * ll - 3.3077115913 * mm + 0.2309699292 * ss,
      -1.2684380046 * ll + 2.6097574011 * mm - 0.3413193965 * ss,
      -0.0041960863 * ll - 0.7034186147 * mm + 1.707614701 * ss,
    ];
    const channel = (linear) => {
      const encoded = linear <= 0.0031308 ? 12.92 * linear : 1.055 * linear ** (1 / 2.4) - 0.055;
      return Math.round(Math.min(Math.max(encoded, 0), 1) * 255).toString(16).padStart(2, "0");
    };
    return "#" + rgb.map(channel).join("");
  }

  function applyHeatPresentation(node, palette, intensity) {
    node.style.backgroundColor = heatColor(palette, intensity);
    node.classList.toggle("newsprint", Boolean(palette.texture));
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
    if (state.palette && state.palette !== state.published) q.set("palette", state.palette);
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
    const when = state.hostedUpdatedAt || snap.generated_at || snap.as_of_date || "";
    const pretty = relativeAge(when);
    if (demo) {
      $("freshness").textContent = "DEMO DATA · " + (pretty ? "updated " + pretty : "");
    } else if (state.freshnessSource === "hosted") {
      $("freshness").textContent = "updated " + pretty + " · hosted";
    } else if (state.freshnessSource === "offline") {
      $("freshness").textContent = "updated " + pretty + " · hosted unavailable · committed snapshot";
    } else {
      $("freshness").textContent = "updated " + pretty + " · committed snapshot";
    }
    updateOwnerChrome();
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
    renderSeg($("palettes"), Object.entries(WALL_PALETTES).map(([id, palette]) => ({ id, label: palette.label })), state.palette, (id) => {
      applyWallPalette(id, true);
      writeURL();
      render();
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
    items.forEach((item, index) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = item.label;
      b.setAttribute("role", "tab");
      b.setAttribute("aria-selected", selected === item.id ? "true" : "false");
      b.tabIndex = selected === item.id ? 0 : -1;
      b.addEventListener("click", () => onPick(item.id));
      b.addEventListener("keydown", (event) => {
        let next = index;
        if (event.key === "ArrowRight" || event.key === "ArrowDown") next = (index + 1) % items.length;
        else if (event.key === "ArrowLeft" || event.key === "ArrowUp") next = (index - 1 + items.length) % items.length;
        else if (event.key === "Home") next = 0;
        else if (event.key === "End") next = items.length - 1;
        else return;
        event.preventDefault();
        onPick(items[next].id);
        requestAnimationFrame(() => root.querySelectorAll('[role="tab"]')[next]?.focus());
      });
      root.append(b);
    });
  }

  function renderWall(snap, ser, row) {
    const wall = $("wall");
    const empty = $("wall-empty");
    const dates = snap.activity.dates || [];
    const palette = wallPalette();
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
      cell.className = "cell " + st + (i === peakIdx ? " peak" : "");
      if (st === "active" && val > 0) {
        applyHeatPresentation(cell, palette, presentationIntensity(val, lv));
      }
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
    const legendSteps = [null, 0, HEAT_MID_POSITION, HEAT_HIGH_POSITION, 1];
    legendSteps.forEach((intensity) => {
      const i = document.createElement("i");
      i.className = "cell";
      if (intensity != null) {
        applyHeatPresentation(i, palette, intensity);
      }
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
    placeTip(target, tip);
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
  bindOwnerLink();
  bindApply();
  syncOwnerLink();
  $("heat-scroll").addEventListener("scroll", () => { dismissTip(); updateMonthLabelVisibility(); }, { passive: true });
  window.addEventListener("resize", () => { dismissTip(); updateMonthLabelVisibility(); }, { passive: true });
  window.addEventListener("scroll", dismissTip, { passive: true, capture: true });
  const urlPalette = params.get("palette");
  if (WALL_PALETTES[urlPalette]) {
    state.palette = urlPalette;
    state.explicit = true;
  } else {
    const saved = visitorPalette();
    if (saved) {
      state.palette = saved;
      state.explicit = true;
    }
  }
  applyWallPalette(state.palette, false);
  try { applyTheme(localStorage.getItem("wt-theme") || "system"); } catch (_) { applyTheme("system"); }

  function relativeAge(iso) {
    if (!iso) return "";
    const t = Date.parse(iso);
    if (Number.isNaN(t)) return "";
    const mins = Math.max(0, Math.round((Date.now() - t) / 60000));
    if (mins < 1) return "just now";
    if (mins < 60) return mins + " min ago";
    const hours = Math.round(mins / 60);
    if (hours < 36) return hours + " hr ago";
    const days = Math.round(hours / 24);
    if (days < 14) return days + " day ago";
    return formatDay(String(iso).slice(0, 10));
  }

  function liveOrigin() {
    const hooked = window.__WT_LIVE_ORIGIN;
    if (typeof hooked === "string" && /^https:\/\/[A-Za-z0-9.-]+(?::\d+)?$/.test(hooked)) return hooked.replace(/\/$/, "");
    if (location.hostname === "rainhuang0220.github.io") return "https://wheretoken.plainlist.space";
    return "";
  }

  function liveOwner(snap) {
    const hooked = window.__WT_LIVE_OWNER;
    if (typeof hooked === "string" && /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$/.test(hooked)) return hooked;
    const login = snap && snap.owner && snap.owner.github_login;
    if (typeof login === "string" && login) return login;
    if (location.hostname === "rainhuang0220.github.io" && location.pathname.indexOf("/profile-demo/") < 0 && location.pathname.indexOf("/profile") >= 0) {
      return "rainhuang0220";
    }
    return "";
  }

  function readSession() {
    try {
      const raw = sessionStorage.getItem(SESSION_KEY);
      if (!raw) return null;
      const sess = JSON.parse(raw);
      if (!sess || typeof sess.token !== "string" || typeof sess.csrf !== "string" || typeof sess.login !== "string") return null;
      if (sess.expires_at && Date.parse(sess.expires_at) < Date.now()) return null;
      return sess;
    } catch (_) {
      return null;
    }
  }

  function writeSession(sess) {
    sessionStorage.setItem(SESSION_KEY, JSON.stringify({
      token: sess.token,
      csrf: sess.csrf,
      login: sess.login,
      expires_at: sess.expires_at || "",
    }));
  }

  function unsafeSnapshot(snap) {
    let raw = "";
    try { raw = JSON.stringify(snap); } catch (_) { return true; }
    const low = raw.toLowerCase();
    return low.indexOf("/users/") >= 0 || low.indexOf("/home/") >= 0 || low.indexOf("access_token") >= 0 ||
      low.indexOf("device_token") >= 0 || low.indexOf("begin rsa") >= 0 || Boolean(snap.privacy && snap.privacy.raw_events);
  }

  function hostedNewer(fallback, envelope) {
    if (!envelope || envelope.schema !== "wheretoken.public-profile-live" || envelope.schema_version !== 1) return false;
    const fresh = envelope.freshness || {};
    if (fresh.mode !== "near_real_time" || fresh.source !== "hosted" || !fresh.updated_at) return false;
    const snap = envelope.snapshot;
    if (!snap || snap.schema !== "wheretoken.public-profile") return false;
    const hostedAt = Date.parse(fresh.updated_at);
    const fallbackAt = Date.parse(fallback.generated_at || "");
    if (Number.isNaN(hostedAt)) return false;
    if (!Number.isNaN(fallbackAt) && hostedAt < fallbackAt) return false;
    return true;
  }

  function profileDemo() {
    return location.pathname.indexOf("/profile-demo/") >= 0;
  }

  function updateOwnerChrome() {
    const mode = $("palette-mode");
    const button = $("apply-github");
    if (!mode || !button) return;
    const owner = liveOwner(state.snap);
    const session = readSession();
    const label = (WALL_PALETTES[state.published] || WALL_PALETTES.newsprint).label;
    if (profileDemo() || !liveOrigin() || !owner) {
      button.hidden = true;
      mode.textContent = "仅预览";
      return;
    }
    if (session && session.login !== owner) {
      button.hidden = true;
      mode.textContent = "仅预览";
      return;
    }
    button.hidden = false;
    if (session && session.login === owner) {
      button.textContent = "应用到我的 GitHub 主页";
      mode.textContent = state.palette === state.published ? "已发布 · " + label : "预览 · 已发布 " + label;
      return;
    }
    button.textContent = "登录并应用到我的 GitHub 主页";
    mode.textContent = "预览主题";
  }

  function openPublish() {
    const dialog = $("publish-dialog");
    if (!dialog) return;
    const from = (WALL_PALETTES[state.published] || WALL_PALETTES.newsprint).label;
    const to = (WALL_PALETTES[state.palette] || WALL_PALETTES.newsprint).label;
    const owner = liveOwner(state.snap);
    $("publish-summary").textContent = from + " → " + to;
    $("publish-target").textContent = owner ? "https://github.com/" + owner : "";
    $("publish-progress").textContent = "确认后才会写入 GitHub。";
    $("publish-confirm").hidden = false;
    $("publish-confirm").disabled = false;
    $("publish-retry").hidden = true;
    dialog.hidden = false;
  }

  function closePublish() {
    const dialog = $("publish-dialog");
    if (dialog) dialog.hidden = true;
  }

  function showJob(job) {
    state.jobID = job && job.id || state.jobID;
    const progress = $("publish-progress");
    const phase = job && job.phase;
    if (phase === "published" || phase === "already_published") {
      progress.textContent = "已应用 ✓";
      state.published = state.palette;
      $("publish-confirm").hidden = true;
      $("publish-retry").hidden = true;
      updateOwnerChrome();
      return;
    }
    if (phase === "partial_failure" || phase === "conflict") {
      progress.textContent = (job.phase_label || "发布未完成") + (job.retry_readme ? "。可以只重试 README。" : "");
      $("publish-confirm").hidden = true;
      $("publish-retry").hidden = !job.retry_readme;
      return;
    }
    progress.textContent = (job && job.phase_label) || "正在发布";
  }

  function authHeaders() {
    const session = readSession();
    return {
      "Content-Type": "application/json",
      "Authorization": "Bearer " + (session ? session.token : ""),
      "X-CSRF-Token": session ? session.csrf : "",
    };
  }

  function pollJob(owner, id, left) {
    if (!id || left <= 0) return Promise.resolve();
    return fetch(liveOrigin() + LIVE_PROFILE + encodeURIComponent(owner) + "/jobs/" + encodeURIComponent(id), {
      headers: authHeaders(),
    }).then((r) => r.ok ? r.json() : Promise.reject(new Error("job"))).then((job) => {
      showJob(job);
      if (job.phase === "published" || job.phase === "already_published" || job.phase === "failed" || job.phase === "partial_failure" || job.phase === "conflict") {
        return job;
      }
      return new Promise((resolve) => setTimeout(resolve, 400)).then(() => pollJob(owner, id, left - 1));
    });
  }

  function confirmPublish() {
    const session = readSession();
    const owner = liveOwner(state.snap);
    if (!session || session.login !== owner) return;
    $("publish-progress").textContent = "正在发布…";
    $("publish-confirm").disabled = true;
    fetch(liveOrigin() + LIVE_PROFILE + encodeURIComponent(owner) + "/publish", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ palette: state.palette, confirm: true }),
    }).then((r) => r.json().then((job) => ({ ok: r.ok, job }))).then(({ job }) => {
      showJob(job);
      if (job && job.id && job.phase !== "published" && job.phase !== "already_published" && job.phase !== "failed" && job.phase !== "partial_failure" && job.phase !== "conflict") {
        return pollJob(owner, job.id, 8);
      }
      return job;
    }).catch(() => {
      $("publish-progress").textContent = "发布失败";
      $("publish-confirm").disabled = false;
    });
  }

  function retryPublish() {
    const owner = liveOwner(state.snap);
    if (!state.jobID || !readSession()) return;
    $("publish-progress").textContent = "正在重试 README";
    fetch(liveOrigin() + LIVE_PROFILE + encodeURIComponent(owner) + "/jobs/" + encodeURIComponent(state.jobID) + "/retry", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ confirm: true }),
    }).then((r) => r.json()).then((job) => showJob(job)).catch(() => {
      $("publish-progress").textContent = "发布失败";
    });
  }

  function bindApply() {
    const button = $("apply-github");
    if (!button || button.dataset.bound === "1") return;
    button.dataset.bound = "1";
    button.addEventListener("click", () => {
      const owner = liveOwner(state.snap);
      const session = readSession();
      if (!session || session.login !== owner) {
        const back = location.origin + location.pathname;
        location.assign(liveOrigin() + LIVE_AUTH + "?return_to=" + encodeURIComponent(back));
        return;
      }
      openPublish();
    });
    const cancel = $("publish-cancel");
    const confirm = $("publish-confirm");
    const retry = $("publish-retry");
    if (cancel) cancel.addEventListener("click", closePublish);
    if (confirm) confirm.addEventListener("click", confirmPublish);
    if (retry) retry.addEventListener("click", retryPublish);
  }

  function consumeExchange() {
    const origin = liveOrigin();
    if (!origin) return Promise.resolve();
    const match = location.hash.match(/(?:^#|&)wt_code=([^&]+)/);
    if (!match) return Promise.resolve();
    const code = decodeURIComponent(match[1]);
    history.replaceState(null, "", location.pathname + location.search);
    return fetch(origin + LIVE_PROFILE + "session", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code: code }),
    }).then((r) => { if (!r.ok) throw new Error("auth"); return r.json(); }).then((sess) => {
      writeSession(sess);
      updateOwnerChrome();
      if (sess.login && sess.login === liveOwner(state.snap)) openPublish();
    }).catch(() => {});
  }

  function refreshHosted(fallback) {
    const origin = liveOrigin();
    const owner = liveOwner(fallback);
    if (!origin || !owner) return Promise.resolve();
    if (fallback.provenance && fallback.provenance.kind === "synthetic_demo" && !window.__WT_LIVE_ORIGIN) return Promise.resolve();
    return fetch(origin + LIVE_PROFILE + encodeURIComponent(owner), { cache: "no-store" })
      .then((r) => { if (!r.ok) throw new Error("hosted status"); return r.json(); })
      .then((envelope) => {
        if (!hostedNewer(fallback, envelope)) throw new Error("hosted schema");
        const snap = envelope.snapshot;
        if (unsafeSnapshot(snap)) throw new Error("hosted privacy");
        validateSnapshot(snap);
        state.snap = snap;
        state.hostedUpdatedAt = (envelope.freshness && envelope.freshness.updated_at) || "";
        state.freshnessSource = "hosted";
        const published = publishedPalette(envelope.presentation);
        state.published = published;
        if (!state.explicit) applyWallPalette(published, false);
        render();
      })
      .catch(() => {
        state.freshnessSource = origin ? "offline" : "fallback";
        state.hostedUpdatedAt = "";
        render();
      });
  }

  Promise.all([
    fetch("./presentation.json", { cache: "no-store" }).then((r) => r.ok ? r.json() : null).catch(() => null),
    fetch("./profile.json", { cache: "no-store" }).then((r) => { if (!r.ok) throw new Error("snapshot missing"); return r.json(); }),
  ])
    .then(([pres, snap]) => {
      state.published = publishedPalette(pres);
      if (!state.explicit) applyWallPalette(state.published, false);
      validateSnapshot(snap);
      state.snap = snap;
      state.freshnessSource = "fallback";
      render();
      return refreshHosted(snap).then(() => consumeExchange());
    })
    .catch((err) => {
      $("status").hidden = false;
      $("status").textContent = "Could not load public snapshot: " + err.message;
    });
})();
