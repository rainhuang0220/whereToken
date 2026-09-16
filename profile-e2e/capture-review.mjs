import { chromium } from "@playwright/test";
import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const staticBundle = path.join(repo, "internal", "profilewebembed", "static");
const productionSnapshot = path.join(repo, "public-profile", "profile.json");
const output = path.resolve(process.argv[2] || path.join(repo, "tmp", "profile-review"));
const mount = "/whereToken/profile/";

const server = http.createServer(async (req, res) => {
  const requestURL = new URL(req.url, "http://127.0.0.1");
  if (!requestURL.pathname.startsWith(mount)) return res.writeHead(404).end("not found");
  const relative = requestURL.pathname.slice(mount.length) || "index.html";
  if (relative === "profile.json") {
    res.setHeader("content-type", "application/json");
    return res.end(await fs.readFile(productionSnapshot));
  }
  const safePath = path.resolve(staticBundle, relative);
  if (!safePath.startsWith(staticBundle + path.sep)) return res.writeHead(403).end("forbidden");
  try {
    const body = await fs.readFile(safePath);
    res.setHeader("content-type", relative.endsWith(".css") ? "text/css" : relative.endsWith(".js") ? "text/javascript" : "text/html");
    res.end(body);
  } catch {
    res.writeHead(404).end("not found");
  }
});

await fs.mkdir(output, { recursive: true });
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const baseURL = `http://127.0.0.1:${server.address().port}${mount}`;
const browser = await chromium.launch({ headless: true });

try {
  for (const [name, width, height] of [
    ["desktop", 1440, 900],
    ["mobile", 390, 844],
  ]) {
    for (const palette of ["Cobalt", "Magenta", "Newsprint"]) {
      const page = await browser.newPage({ viewport: { width, height } });
      await page.goto(baseURL, { waitUntil: "networkidle" });
      await page.getByRole("tab", { name: palette, exact: true }).click();
      await page.screenshot({ path: path.join(output, `live-${name}-${palette.toLowerCase()}.png`) });
      await page.close();
    }
  }
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
