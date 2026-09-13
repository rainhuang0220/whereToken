import { test, expect } from "@playwright/test";
import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const bundle = path.join(repo, "docs", "media", "public-profile-demo");
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
    const safePath = path.resolve(bundle, relative);
    if (!safePath.startsWith(bundle + path.sep)) {
      res.writeHead(403).end("forbidden");
      return;
    }
    try {
      const body = await fs.readFile(safePath);
      const type = relative.endsWith(".css") ? "text/css" : relative.endsWith(".js") ? "text/javascript" : "text/html";
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
  await expect(page.locator("#kpis div").first().locator("dd")).toHaveText(snapshot.periods.all.totals.total.display);
  await expect(page.locator("#freshness")).toContainText(snapshot.generated_at);
  await expect(page.locator("#freshness")).toContainText("DEMO DATA");

  snapshot.provenance = { kind: "local_sanitized_snapshot", refresh_mode: "manual_publish", live_sync: false };
  await page.reload();
  await expect(page.locator("#freshness")).not.toContainText("DEMO DATA");
  await expect(page.locator("#freshness")).toContainText("Public snapshot");
});

test("switches every range and optional breakdown tab", async ({ page }) => {
  await page.goto(baseURL);
  for (const [label, id] of [["Today", "today"], ["7d", "7d"], ["30d", "30d"], ["53w", "53w"], ["All", "all"]]) {
    await page.getByRole("tab", { name: label, exact: true }).click();
    await expect(page.locator("#kpis div").nth(1).locator("dd")).toHaveText(snapshot.periods[id].totals.total.display);
  }
  await expect(page.getByRole("tab", { name: "Agents" })).toBeVisible();
  await page.getByRole("tab", { name: "Providers" }).click();
  await expect(page.getByRole("tab", { name: "Providers" })).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tab", { name: "Models" })).toHaveCount(0);

  for (const period of Object.values(snapshot.periods)) period.by_model = structuredClone(period.by_agent);
  const modelSeries = structuredClone(snapshot.activity.series.find((series) => series.dimension === "agent"));
  modelSeries.dimension = "model";
  snapshot.activity.series.push(modelSeries);
  await page.reload();
  await expect(page.getByRole("tab", { name: "Models" })).toBeVisible();
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
  await expect(page.locator("#tip")).toContainText(await row.locator("span").first().innerText());
  await active.focus();
  await expect(page.locator("#tip")).toContainText("tokens");
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
  await expect(page.locator("#status")).toHaveText("Partial data");
});

test("keeps the page fixed at 390px and starts the wall at recent weeks", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(baseURL);
	await expect(page.locator("#wall .cell")).toHaveCount(371);
	await expect.poll(() => page.locator("#wall").evaluate((node) => node.scrollLeft)).toBeGreaterThan(0);
  const dimensions = await page.evaluate(() => ({
    page: document.documentElement.scrollWidth,
    viewport: innerWidth,
    wallLeft: document.getElementById("wall").scrollLeft,
    wallWidth: document.getElementById("wall").scrollWidth,
  }));
  expect(dimensions.page).toBeLessThanOrEqual(dimensions.viewport);
  expect(dimensions.wallWidth).toBeGreaterThan(dimensions.viewport);
  expect(dimensions.wallLeft).toBeGreaterThan(0);
  await expect(page.locator("#wall-hint")).toContainText("scroll");
});

test("makes no external runtime request", async ({ page }) => {
  await page.goto(baseURL);
  await expect(page.locator("#kpis")).not.toBeEmpty();
  expect(requests.sort()).toEqual([
    "/whereToken/profile/",
    "/whereToken/profile/assets/profile.css",
    "/whereToken/profile/assets/profile.js",
    "/whereToken/profile/profile.json",
  ].sort());
});
