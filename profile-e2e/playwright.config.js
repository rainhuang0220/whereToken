import { defineConfig } from "@playwright/test";

// Firefox needs a display-backed desktop pointer on the Linux CI runner.
const firefoxHeadless = process.env.WT_LINUX_DESKTOP !== "1";

export default defineConfig({
  testDir: ".",
  testMatch: "profile.spec.js",
  fullyParallel: false,
  workers: 1,
  reporter: "line",
  projects: [
    { name: "chromium", use: { browserName: "chromium", headless: true, reducedMotion: "reduce" } },
    { name: "firefox", use: { browserName: "firefox", headless: firefoxHeadless, reducedMotion: "reduce" } },
    { name: "webkit", use: { browserName: "webkit", headless: true, reducedMotion: "reduce" } },
  ],
});
