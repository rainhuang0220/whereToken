import { test, expect } from "@playwright/test";
import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const bundle = path.join(repo, "docs", "media", "public-profile-demo");
const staticBundle = path.join(repo, "internal", "profilewebembed", "static");
const mount = "/whereToken/profile/";
let server;
let baseURL;
let baseline;
let snapshot;
let requests;

test.beforeAll(async () => {
  const raw = await fs.readFile(path.join(bundle, "profile.json"), "utf8");
  baseline = JSON.parse(raw);
  snapshot = structuredClone(baseline);
  server = http.createServer(async (req, res) => {
    requests.push(req.url);
    const requestURL = new URL(req.url, "http://127.0.0.1");
    if (!requestURL.pathname.startsWith(mount)) {
      res.writeHead(404).end("not found");
      return;
    }
    const relative = requestURL.pathname.slice(mount.length) || "index.html";
    if (relative === "profile.json") {
      res.setHeader("content-type", "application/json");
      res.end(JSON.stringify(snapshot));
      return;
    }
    const safePath = path.resolve(staticBundle, relative);
    if (!safePath.startsWith(staticBundle + path.sep)) {
      res.writeHead(403).end("forbidden");
      return;
    }
    try {
      const body = await fs.readFile(safePath);
      const type = relative.endsWith(".css") ? "text/css" : relative.endsWith(".js") ? "text/javascript" : relative.endsWith(".svg") ? "image/svg+xml" : relative.endsWith(".png") ? "image/png" : relative.endsWith(".jpg") ? "image/jpeg" : "text/html";
      res.setHeader("content-type", type);
      res.end(body);
    } catch {
      res.writeHead(404).end("not found");
    }
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  baseURL = `http://127.0.0.1:${server.address().port}${mount}`;
});

test.afterAll(async () => {
  await new Promise((resolve) => server.close(resolve));
});

test.beforeEach(() => {
  snapshot = structuredClone(baseline);
  requests = [];
});

test("loads the static project subpath with truthful snapshot provenance", async ({ page }) => {
  await page.goto(baseURL);
  await expect(page.locator('[aria-label="Time range"] [aria-selected="true"]')).toHaveText("All");
  await expect(page.locator("#hero-value")).toHaveText(snapshot.periods.all.totals.total.display);
  await expect(page.locator("#freshness")).toContainText("DEMO DATA");
  await expect(page.locator("#freshness")).toContainText("updated");

  snapshot.provenance = { kind: "local_sanitized_snapshot", refresh_mode: "manual_publish", live_sync: false };
  await page.reload();
  await expect(page.locator("#freshness")).not.toContainText("DEMO DATA");
  await expect(page.locator("#freshness")).not.toContainText("Public snapshot");
  await expect(page.locator("#freshness")).toContainText("updated");
});

test("switches every range and optional breakdown tab", async ({ page }) => {
  await page.goto(baseURL);
  for (const [label, id] of [["Today", "today"], ["7d", "7d"], ["30d", "30d"], ["53w", "53w"], ["All", "all"]]) {
    await page.getByRole("tab", { name: label, exact: true }).click();
    await expect(page.locator("#hero-value")).toHaveText(snapshot.periods[id].totals.total.display);
  }
  await expect(page.getByRole("tab", { name: "Agents" })).toBeVisible();
  await page.getByRole("tab", { name: "Providers" }).click();
  await expect(page.getByRole("tab", { name: "Providers" })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tab", { name: "Models" })).toHaveCount(0);

  for (const period of Object.values(snapshot.periods)) period.by_model = structuredClone(period.by_agent);
  const modelSeries = structuredClone(snapshot.activity.series.find((series) => series.dimension === "agent" && (series.metric || "tokens") === "tokens"));
  modelSeries.dimension = "model";
  snapshot.activity.series.push(modelSeries);
  await page.reload();
  await expect(page.getByRole("tab", { name: "Models" })).toBeVisible();
});

test("explains mixed source coverage next to the hero and with exact dates", async ({ page }) => {
  const cursor = snapshot.periods.all.by_agent.find((row) => row.id === "cursor") || snapshot.periods.all.by_agent[0];
  cursor.coverage = {
    tokens: "available",
    requests: "available",
    token_source: "account_api",
    token_window: { from: "2025-09-08", to: "2026-09-14", label: "53w account usage" },
  };
  await page.goto(baseURL);
  await expect(page.locator("#hero-coverage")).toContainText("Coverage varies by source");
  await expect(page.locator("#coverage-table")).toContainText("Sep 8, 2025 – Sep 14, 2026");
});

test("uses the selected range for the trend window", async ({ page }) => {
  const series = snapshot.activity.series.find((item) => item.dimension === "all" && (item.metric || "tokens") === "tokens");
  series.values.fill(0);
  series.levels.fill(0);
  series.states.fill("empty");
  let end = snapshot.activity.dates.findIndex((date) => date > snapshot.activity.to);
  if (end < 0) end = snapshot.activity.dates.length;
  for (let i = end - 7; i < end; i++) {
    series.values[i] = 1;
    series.levels[i] = 1;
    series.states[i] = "active";
  }
  series.values[end - 30] = 100;
  series.levels[end - 30] = 5;
  series.states[end - 30] = "active";

  await page.goto(baseURL);
  await page.getByRole("tab", { name: "Today", exact: true }).click();
  await expect(page.locator("#trend-label")).toHaveText("Today");
  await expect(page.locator("#trend-value")).toHaveText("1 tokens");
  await page.getByRole("tab", { name: "7d", exact: true }).click();
  await expect(page.locator("#trend-label")).toHaveText("Last 7 days");
  await expect(page.locator("#trend-value")).toHaveText("7 tokens");
  await page.getByRole("tab", { name: "30d", exact: true }).click();
  await expect(page.locator("#trend-label")).toHaveText("Last 30 days");
  await expect(page.locator("#trend-value")).toHaveText("107 tokens");
});

test("announces async status and keeps the trend accessible as range changes", async ({ page }) => {
  await page.goto(baseURL);
  await expect(page.locator("#status")).toHaveAttribute("aria-live", "polite");
  await expect(page.locator("#trend")).toHaveAttribute("aria-label", "Available 53-week activity");
  await page.getByRole("tab", { name: "Today", exact: true }).click();
  await expect(page.locator("#trend")).toHaveAttribute("aria-label", "Today");
  await expect(page.locator('meta[name="theme-color"]')).toHaveCount(2);
});

test("persists filter URL state and exposes mouse and keyboard tooltips", async ({ page }) => {
  await page.goto(baseURL);
  const row = page.locator("#rows button").first();
  await row.click();
  await expect(row).toHaveAttribute("aria-pressed", "true");
  await expect(page).toHaveURL(/filter=/);
  await page.reload();
  await expect(page.locator("#rows button").first()).toHaveAttribute("aria-pressed", "true");

  const active = page.locator("#wall .cell.active").first();
  await active.hover();
  await expect(page.locator("#tip")).toContainText("tokens");
  await expect(page.locator("#tip")).toContainText(await row.locator(".rank-name").innerText());
  await active.focus();
  await expect(page.locator("#tip")).toContainText("tokens");
  await expect(active).toHaveAttribute("aria-describedby", "tip");
});

test("keeps the newest tooltip after rapid A to B to C movement", async ({ page }) => {
  await page.goto(baseURL);
  const cells = page.locator("#wall .cell.active");
  const a = cells.nth(0);
  const b = cells.nth(1);
  const c = cells.nth(2);
  const cDate = await c.getAttribute("data-date");

  await a.dispatchEvent("mouseenter");
  await a.dispatchEvent("mouseleave");
  await b.dispatchEvent("mouseenter");
  await b.dispatchEvent("mouseleave");
  await c.dispatchEvent("mouseenter");

  await expect(page.locator("#tip")).toContainText(formatExpectedDay(cDate));
  await page.waitForTimeout(220);
  await expect(page.locator("#tip")).toBeVisible();
  await expect(page.locator("#tip")).toContainText(formatExpectedDay(cDate));
});

test("keeps tooltip ownership across rapid keyboard focus and mixed hover", async ({ page }) => {
  await page.goto(baseURL);
  const cells = page.locator("#wall .cell.active");
  const a = cells.nth(0);
  const b = cells.nth(1);
  const bDate = await b.getAttribute("data-date");

  await a.focus();
  await b.focus();
  await page.waitForTimeout(220);
  await expect(page.locator("#tip")).toBeVisible();
  await expect(page.locator("#tip")).toContainText(formatExpectedDay(bDate));

  await b.dispatchEvent("mouseenter");
  await b.dispatchEvent("mouseleave");
  await page.waitForTimeout(220);
  await expect(page.locator("#tip")).toBeVisible();
});

test("supports pointer lifecycle and hides after the active trigger leaves", async ({ page }) => {
  await page.goto(baseURL);
  const active = page.locator("#wall .cell.active").first();
  await active.dispatchEvent("pointerenter", { pointerType: "pen" });
  await expect(page.locator("#tip")).toBeVisible();
  await expect(page.locator("#tip > *")).toHaveCount(2);
  await expect(page.locator("#tip")).not.toContainText("Peak day");
  await active.dispatchEvent("pointerleave", { pointerType: "pen" });
  await page.waitForTimeout(220);
  await expect(page.locator("#tip")).toBeHidden();

  await active.dispatchEvent("mouseenter");
  await active.dispatchEvent("mouseleave");
  await page.waitForTimeout(220);
  await expect(page.locator("#tip")).toBeHidden();
});

test("clears stale tooltip state before range metric and filter rerenders", async ({ page }) => {
  await page.goto(baseURL);
  const tip = page.locator("#tip");

  await page.locator("#wall .cell.active").first().dispatchEvent("mouseenter");
  await expect(tip).toBeVisible();
  await page.getByRole("tab", { name: "7d", exact: true }).click();
  await expect(tip).toBeHidden();

  await page.locator("#wall .cell.active").first().dispatchEvent("mouseenter");
  await page.getByRole("tab", { name: "Requests", exact: true }).click();
  await expect(tip).toBeHidden();

  await page.locator("#wall .cell.active").first().dispatchEvent("mouseenter");
  await page.locator("#rows button").first().click();
  await expect(tip).toBeHidden();
});

test("clamps tooltips at viewport edges and closes them on resize", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(baseURL);
  const tip = page.locator("#tip");
  const edgeCases = [
    [page.locator("#wall .cell").first(), "start"],
    [page.locator("#wall .cell").last(), "end"],
  ];
  for (const [cell, edge] of edgeCases) {
    await page.locator("#heat-scroll").evaluate((node, side) => {
      node.scrollLeft = side === "start" ? 0 : node.scrollWidth;
    }, edge);
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await cell.dispatchEvent("mouseenter");
    await expect(tip).toBeVisible();
    const box = await tip.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(8);
    expect(box.y).toBeGreaterThanOrEqual(8);
    expect(box.x + box.width).toBeLessThanOrEqual(382);
    expect(box.y + box.height).toBeLessThanOrEqual(836);
  }
  await page.locator("#heat-scroll").evaluate((node) => { node.scrollLeft -= 20; });
  await expect(tip).toBeHidden();
  await page.locator("#wall .cell").last().dispatchEvent("mouseenter");
  await expect(tip).toBeVisible();
  await page.evaluate(() => window.dispatchEvent(new Event("resize")));
  await expect(tip).toBeHidden();
});

test("persists theme and honors reduced motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(baseURL);
  await page.getByRole("button", { name: "Light" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await page.getByRole("button", { name: "Dark" }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const transition = await page.locator("#wall").evaluate((node) => getComputedStyle(node).transitionDuration);
  expect(transition).toBe("0s");

  const cells = page.locator("#wall .cell.active");
  await cells.nth(0).dispatchEvent("mouseenter");
  await cells.nth(0).dispatchEvent("mouseleave");
  await cells.nth(1).dispatchEvent("mouseenter");
  await page.waitForTimeout(220);
  await expect(page.locator("#tip")).toBeVisible();
});

test("keeps structural ink separate from Activity data accents", async ({ page }) => {
  await page.goto(baseURL);
  const visualTokens = () => page.locator("html").evaluate(() => {
    const root = getComputedStyle(document.documentElement);
    const body = getComputedStyle(document.body);
    return {
      page: root.getPropertyValue("--page").trim(),
      surface: root.getPropertyValue("--surface").trim(),
      border: root.getPropertyValue("--border").trim(),
      uiAccent: root.getPropertyValue("--ui-accent").trim(),
      dataAccent: root.getPropertyValue("--data-accent").trim(),
      paper: body.backgroundImage,
    };
  });

  await page.getByRole("tab", { name: "Cobalt", exact: true }).click();
  await expect.poll(visualTokens).toEqual({
    page: "#ffffff",
    surface: "#ffffff",
    border: "#1f2328",
    uiAccent: "#1f2328",
    dataAccent: "#ffd700",
    paper: "none",
  });
  await expect(page.locator(".top")).toHaveCSS("border-bottom-color", "rgb(31, 35, 40)");
  await expect(page.locator(".trend .line")).toHaveCSS("stroke", "rgb(255, 215, 0)");
  await expect(page.locator(".rank-bar i").first()).toHaveCSS("background-color", "rgb(255, 215, 0)");

  await page.getByRole("tab", { name: "Magenta", exact: true }).click();
  await expect.poll(visualTokens).toEqual({
    page: "#ffffff",
    surface: "#ffffff",
    border: "#1f2328",
    uiAccent: "#1f2328",
    dataAccent: "#c2185b",
    paper: "none",
  });
  await expect(page.locator(".top")).toHaveCSS("border-bottom-color", "rgb(31, 35, 40)");
  await expect(page.locator(".trend .line")).toHaveCSS("stroke", "rgb(194, 24, 91)");
  await expect(page.locator(".rank-bar i").first()).toHaveCSS("background-color", "rgb(194, 24, 91)");

  await page.getByRole("tab", { name: "Newsprint", exact: true }).click();
  const newsprint = await visualTokens();
  expect(newsprint.dataAccent).toBe("#1f2328");
  expect(newsprint.page).toBe("#f7f6f1");
  expect(newsprint.paper).toContain("newsprint-surface.jpg");
  expect(newsprint.paper).not.toContain("newsprint-fiber.svg");
  expect(newsprint.paper).not.toContain("newsprint-folds.svg");
  const paperSize = await page.locator("body").evaluate((node) => getComputedStyle(node).backgroundSize);
  expect(paperSize).toContain("560px");
  expect(paperSize).not.toContain("100%");
  expect(await page.locator("body").evaluate((node) => getComputedStyle(node, "::before").content)).toBe("none");
});

test("keeps an explicit dark display preference when the Activity palette changes", async ({ page }) => {
  await page.goto(baseURL);
  await page.getByRole("button", { name: "Dark" }).click();
  await page.getByRole("tab", { name: "Cobalt", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect.poll(() => page.locator("body").evaluate((node) => getComputedStyle(node).backgroundColor)).toBe("rgb(17, 16, 15)");
});

test("operates the Activity color tabs with arrows and persists the selected palette", async ({ page }) => {
  await page.goto(baseURL);
  const cobalt = page.getByRole("tab", { name: "Cobalt", exact: true });
  const magenta = page.getByRole("tab", { name: "Magenta", exact: true });
  const newsprint = page.getByRole("tab", { name: "Newsprint", exact: true });

  await expect(newsprint).toHaveAttribute("aria-selected", "true");
  await expect(page.locator("#wall .cell.active").first()).toHaveClass(/newsprint/);
  await cobalt.focus();
  await cobalt.press("ArrowRight");
  await expect(magenta).toBeFocused();
  await expect(magenta).toHaveAttribute("aria-selected", "true");
  await magenta.press("End");
  await expect(newsprint).toBeFocused();
  await expect(newsprint).toHaveAttribute("aria-selected", "true");
  await expect(page.locator("#wall .cell.active").first()).toHaveClass(/newsprint/);

  await page.reload();
  await expect(newsprint).toHaveAttribute("aria-selected", "true");
  await expect(page.locator("#wall .cell.active").first()).toHaveClass(/newsprint/);
});

test("shows explicit invalid, empty, and partial states", async ({ page }) => {
  snapshot.schema_version = 999;
  await page.goto(baseURL);
  await expect(page.locator("#status")).toContainText("Could not load public snapshot");

  snapshot = structuredClone(baseline);
  snapshot.data_status = "unavailable";
  for (const period of Object.values(snapshot.periods)) {
    for (const component of Object.values(period.totals)) Object.assign(component, { value: null, display: "—", status: "unavailable" });
  }
  for (const series of snapshot.activity.series) {
    series.values.fill(0);
    series.levels.fill(0);
    series.states.fill("unknown");
  }
  await page.reload();
  await expect(page.locator("#status")).toContainText("No public activity");
  await expect(page.locator("#wall .cell.unknown")).toHaveCount(371);

  snapshot = structuredClone(baseline);
  snapshot.data_status = "partial";
  await page.reload();
  await expect(page.locator("#hero-coverage")).toContainText("Partial coverage");
  await expect(page.locator("#coverage-chip")).toBeVisible();
});

test("keeps the page fixed at 390px and starts the wall at recent weeks", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(baseURL);
  await expect(page.locator("#wall .cell")).toHaveCount(371);
  await expect.poll(() => page.locator("#heat-scroll").evaluate((node) => node.scrollLeft)).toBeGreaterThan(0);
  const dimensions = await page.evaluate(() => ({
    page: document.documentElement.scrollWidth,
    viewport: innerWidth,
    wallLeft: document.getElementById("heat-scroll").scrollLeft,
    wallWidth: document.getElementById("heat-scroll").scrollWidth,
  }));
  expect(dimensions.page).toBeLessThanOrEqual(dimensions.viewport);
  expect(dimensions.wallWidth).toBeGreaterThan(dimensions.viewport);
  expect(dimensions.wallLeft).toBeGreaterThan(0);
  await expect(page.locator("#wall-hint")).toContainText("scroll");
});

test("keeps mobile month labels and wrapped metadata rules clean", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(baseURL);
  await page.locator("#heat-scroll").evaluate((node) => {
    const label = [...node.querySelectorAll("#months span")].find((item) => item.textContent && item.offsetLeft > node.clientWidth);
    node.scrollLeft = label.offsetLeft - node.offsetLeft + 5;
    node.dispatchEvent(new Event("scroll"));
  });
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(resolve)));
  const layout = await page.evaluate(() => {
    const scroller = document.getElementById("heat-scroll").getBoundingClientRect();
    const monthLabels = [...document.querySelectorAll("#months span")]
      .filter((node) => node.textContent && getComputedStyle(node).visibility !== "hidden")
      .map((node) => node.getBoundingClientRect())
      .filter((rect) => rect.right > scroller.left && rect.left < scroller.right);
    const meta = [...document.querySelectorAll("#hero-meta li")];
    return {
      monthLefts: monthLabels.map((rect) => rect.left),
      scrollerLeft: scroller.left,
      firstMetaLeft: meta[0].getBoundingClientRect().left,
      thirdMetaLeft: meta[2].getBoundingClientRect().left,
      thirdMetaBorder: getComputedStyle(meta[2]).borderLeftWidth,
    };
  });
  expect(Math.min(...layout.monthLefts)).toBeGreaterThanOrEqual(layout.scrollerLeft);
  expect(layout.thirdMetaLeft).toBeCloseTo(layout.firstMetaLeft, 0);
  expect(layout.thirdMetaBorder).toBe("0px");
});

test("makes no external runtime request", async ({ page }) => {
  await page.goto(baseURL);
  await expect(page.locator("#hero-value")).not.toBeEmpty();
  expect(requests.sort()).toEqual([
    "/whereToken/profile/",
    "/whereToken/profile/assets/profile.css",
    "/whereToken/profile/assets/profile.js",
    "/whereToken/profile/assets/newsprint-surface.jpg",
    "/whereToken/profile/profile.json",
  ].sort());
});

test("does not fall back to All when selected token series is missing", async ({ page }) => {
  const agent = snapshot.periods.all.by_agent[0];
  snapshot.activity.series = snapshot.activity.series.filter((series) => !(
    series.dimension === "agent" && series.id === agent.id && (series.metric || "tokens") === "tokens"
  ));
  const hasRequests = snapshot.activity.series.some((series) => series.dimension === "agent" && series.id === agent.id && series.metric === "requests");
  if (!hasRequests) {
    const template = structuredClone(snapshot.activity.series.find((series) => series.dimension === "all"));
    template.dimension = "agent";
    template.id = agent.id;
    template.label = agent.label;
    template.metric = "requests";
    snapshot.activity.series.push(template);
  }
  agent.quality = "degraded";
  agent.totals.total = { value: null, display: "—", status: "unavailable" };
  agent.share = "—";
  agent.coverage = { tokens: "unavailable", requests: "available", token_source: "local", token_window: null, reason: "account_api_skipped" };

  await page.goto(baseURL);
  await page.locator("#rows button").first().click();
  await expect(page.locator("#wall-empty")).toContainText("Token activity unavailable");
  await expect(page.locator("#wall .cell")).toHaveCount(0);
  await page.getByRole("button", { name: "View request activity" }).click();
  await expect(page).toHaveURL(/metric=requests/);
  await expect(page.locator("#wall .cell")).toHaveCount(371);
  await expect(page.locator("#tip")).toBeHidden();
  const active = page.locator("#wall .cell.active").first();
  if (await active.count()) {
    await active.hover();
    await expect(page.locator("#tip")).toContainText("requests");
  }
});

test("switches Tokens and Requests without treating counts as tokens", async ({ page }) => {
  await page.goto(baseURL);
  await expect(page.getByRole("tab", { name: "Tokens", exact: true })).toHaveAttribute("aria-selected", "true");
  await page.getByRole("tab", { name: "Requests", exact: true }).click();
  await expect(page).toHaveURL(/metric=requests/);
  await expect(page.locator("#hero-label")).toHaveText("requests");
  await expect(page.locator("#hero-value")).toHaveText(snapshot.periods.all.requests.display);
  const active = page.locator("#wall .cell.active").first();
  await active.hover();
  await expect(page.locator("#tip")).toContainText("requests");
  await expect(page.locator("#tip")).not.toContainText("tokens");
});

function formatExpectedDay(iso) {
  return new Date(iso + "T00:00:00").toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}
