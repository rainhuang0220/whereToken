import { chromium } from "@playwright/test";
import assert from "node:assert/strict";
import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const staticBundle = path.join(repo, "internal", "profilewebembed", "static");
const snapshot = path.join(repo, "public-profile", "profile.json");
const output = path.resolve(process.argv[2] || path.join(repo, "review"));
const mount = "/whereToken/profile/";
const manifest = {};

const server = http.createServer(async (req, res) => {
  const requestURL = new URL(req.url, "http://127.0.0.1");
  if (!requestURL.pathname.startsWith(mount)) return res.writeHead(404).end("not found");
  const relative = requestURL.pathname.slice(mount.length) || "index.html";
  if (relative === "profile.json") {
    res.setHeader("content-type", "application/json");
    return res.end(await fs.readFile(snapshot));
  }
  const safePath = path.resolve(staticBundle, relative);
  if (!safePath.startsWith(staticBundle + path.sep)) return res.writeHead(403).end("forbidden");
  try {
    const body = await fs.readFile(safePath);
    const type = relative.endsWith(".css") ? "text/css" : relative.endsWith(".js") ? "text/javascript" : relative.endsWith(".svg") ? "image/svg+xml" : relative.endsWith(".png") ? "image/png" : relative.endsWith(".jpg") ? "image/jpeg" : relative.endsWith(".json") ? "application/json" : "text/html";
    res.setHeader("content-type", type);
    res.end(body);
  } catch {
    res.writeHead(404).end("not found");
  }
});

await fs.mkdir(output, { recursive: true });
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const baseURL = `http://127.0.0.1:${server.address().port}${mount}`;
const browser = await chromium.launch({ headless: process.env.WT_CAPTURE_HEADFUL !== '1' });

async function ready(page) {
  await page.waitForFunction(() => {
    const paper = window.__WT_PAPER3D__?.getDiagnostics();
    return paper?.snapshotReady && paper.sheetCssHeight === document.documentElement.scrollHeight;
  }, null, { timeout: 45000 });
  await page.waitForFunction(() => document.documentElement.classList.contains("wt-physical-print"));
}

async function openPage(width, height, palette = "newsprint", dpr = 1) {
  const context = await browser.newContext({
    viewport: { width, height }, deviceScaleFactor: dpr,
    colorScheme: "light", reducedMotion: "no-preference",
  });
  const page = await context.newPage();
  await page.goto(`${baseURL}?palette=${palette}`, { waitUntil: "networkidle" });
  await page.waitForFunction(() => document.getElementById("hero-value")?.textContent !== "—");
  if (palette === "newsprint") await ready(page);
  return { context, page };
}

async function neutral(page, width) {
  await page.mouse.move(20, 400);
  await page.waitForTimeout(350);
  assert.equal(await page.locator("html").evaluate((node) => node.classList.contains("wt-paper-ui")), false);
}

async function capture(page, name) {
  await page.screenshot({ path: path.join(output, `${name}.png`), animations: "disabled" });
}

async function diagnostics(page) {
  return page.evaluate(() => window.__WT_PAPER3D__?.getDiagnostics());
}

async function rects(page) {
  return page.evaluate(() => Object.fromEntries(
    ["main", "hero-value", "wall", "trend-card", "rows"].map((id) => {
      const rect = document.getElementById(id).getBoundingClientRect();
      return [id, [rect.x, rect.y, rect.width, rect.height]];
    }),
  ));
}

async function gradual(page, from, to, steps = 28) {
  for (let step = 1; step <= steps; step++) {
    await page.mouse.move(from.x + (to.x - from.x) * step / steps, from.y + (to.y - from.y) * step / steps);
    await page.waitForTimeout(14);
  }
}

async function fold(page, start, end, name) {
  await page.mouse.move(start.x, start.y);
  await page.mouse.down();
  await gradual(page, start, end);
  try {
    await page.waitForFunction(() => window.__WT_PAPER3D__.getDiagnostics().angleDegrees > 150, null, { timeout: 8000 });
  } catch (error) {
    console.error(name, await diagnostics(page));
    throw error;
  }
  await page.mouse.up();
  await page.waitForFunction(() => window.__WT_PAPER3D__.getDiagnostics().phase === "latched");
  manifest[name] = await diagnostics(page);
  await capture(page, name);
}

try {
  if (process.env.WT_CAPTURE_RESIDUAL_ONLY === "1") {
    const { context, page } = await openPage(1440, 900);
    await neutral(page, 1440);
    manifest.normal = await diagnostics(page);
    await capture(page, "newsprint-1440x900");
    await fold(page, { x: 1428, y: 12 }, { x: 1070, y: 292 }, "newsprint-folded-upper-corner");
    const latched = await diagnostics(page);
    const tip = latched.flapPoints.find((point) => point.u > 0.5 && point.v > 0.5);
    assert.ok(tip);
    await page.mouse.move(tip.x, tip.y);
    await page.mouse.down();
    await gradual(page, tip, { x: tip.x + 260, y: tip.y - 190 }, 24);
    await page.mouse.up();
    await page.waitForFunction(() => window.__WT_PAPER3D__.getDiagnostics().phase === "resting");
    await neutral(page, 1440);
    manifest.residual = await diagnostics(page);
    assert.ok(manifest.residual.plasticCreases > 0);
    await capture(page, "newsprint-residual-crease");
    await context.close();
  } else {
  {
    const { context, page } = await openPage(1440, 900, "cobalt");
    const cobalt = await rects(page);
    await capture(page, "cobalt-1440x900");
    await page.getByRole("tab", { name: "Newsprint", exact: true }).click();
    await ready(page);
    await neutral(page, 1440);
    const newsprint = await rects(page);
    const deltas = {};
    for (const id of Object.keys(cobalt)) {
      deltas[id] = cobalt[id].map((value, axis) => Math.abs(value - newsprint[id][axis]));
      for (const delta of deltas[id]) assert.ok(delta < 0.5, `${id} layout changed by ${delta}px`);
    }
    manifest.layoutDeltas = deltas;
    manifest.normal = await diagnostics(page);
    assert.equal(manifest.normal.sheetCssWidth, 1440);
    assert.equal(manifest.normal.sheetCssHeight, await page.evaluate(() => document.documentElement.scrollHeight));
    assert.equal(manifest.normal.printedInkResident, true);
    await capture(page, "newsprint-1440x900");
    await context.close();
  }

  {
    const { context, page } = await openPage(1920, 1080);
    await neutral(page, 1920);
    manifest.wide = await diagnostics(page);
    await capture(page, "newsprint-1920x1080");
    await context.close();
  }

  {
    const { context, page } = await openPage(1440, 900);
    await page.getByRole("button", { name: "Dark", exact: true }).click();
    await ready(page);
    await neutral(page, 1440);
    manifest.dark = await diagnostics(page);
    await capture(page, "newsprint-dark-1440x900");
    await context.close();
  }

  {
    const { context, page } = await openPage(1440, 900);
    await page.locator("#coverage summary").click();
    await ready(page);
    await page.evaluate(() => window.scrollTo(0, 800));
    await page.waitForFunction(() => window.scrollY === 800);
    await neutral(page, 1440);
    manifest.scrolled = await diagnostics(page);
    assert.equal(manifest.scrolled.cameraScrollY, 800);
    await capture(page, "newsprint-scrolled-800");
    await context.close();
  }

  {
    const { context, page } = await openPage(1440, 900);
    await neutral(page, 1440);
    await fold(page, { x: 1428, y: 12 }, { x: 1070, y: 292 }, "newsprint-folded-upper-corner");
    const latched = await diagnostics(page);
    const tip = latched.flapPoints.find((point) => point.u > 0.5 && point.v > 0.5);
    assert.ok(tip);
    await page.mouse.move(tip.x, tip.y);
    await page.mouse.down();
    await gradual(page, tip, { x: tip.x + 260, y: tip.y - 190 }, 24);
    await page.mouse.up();
    await page.waitForFunction(() => window.__WT_PAPER3D__.getDiagnostics().phase === "resting");
    await neutral(page, 1440);
    manifest.residual = await diagnostics(page);
    assert.ok(manifest.residual.plasticCreases > 0);
    assert.equal(manifest.residual.inkVisibleAtRest, true);
    await capture(page, "newsprint-residual-crease");
    await context.close();
  }

  {
    const { context, page } = await openPage(1440, 900);
    await page.locator("#coverage summary").click();
    await ready(page);
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    await page.waitForTimeout(250);
    await fold(page, { x: 12, y: 888 }, { x: 370, y: 608 }, "newsprint-folded-lower-corner");
    await context.close();
  }

  {
    const { context, page } = await openPage(1440, 900, "newsprint", 2);
    await neutral(page, 1440);
    const hero = await page.locator("#hero-value").boundingBox();
    await page.screenshot({
      path: path.join(output, "newsprint-ink-closeup.png"),
      clip: { x: Math.max(0, hero.x - 24), y: Math.max(0, hero.y - 18), width: 520, height: 190 },
      animations: "disabled",
    });
    manifest.closeup = await diagnostics(page);
    assert.ok(manifest.closeup.effectiveInkScale >= 1.5);
    const wall = await page.locator("#wall").boundingBox();
    const viewport = page.viewportSize();
    // The 53-week grid is horizontally scrollable: its bounding box may be
    // wider than the viewport and have a negative x when scrolled to today.
    const left = Math.min(viewport.width - 1, Math.max(0, wall.x - 20));
    const top = Math.min(viewport.height - 1, Math.max(0, wall.y - 34));
    await page.screenshot({
      path: path.join(output, "newsprint-heatmap-closeup.png"),
      clip: { x: left, y: top,
        width: Math.max(1, Math.min(1120, wall.width + 40, viewport.width - left)),
        height: Math.max(1, Math.min(wall.height + 72, viewport.height - top)) },
      animations: "disabled",
    });
    await context.close();
  }

  }
  await fs.writeFile(path.join(output, "capture-manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
  console.log(`Captured ${Object.keys(manifest).length} verified states in ${output}`);
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
