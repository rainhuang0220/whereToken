import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "profile.spec.js",
  fullyParallel: false,
  workers: 1,
  reporter: "line",
  projects: [
    { name: "chromium", use: { browserName: "chromium", headless: true } },
    { name: "firefox", use: { browserName: "firefox", headless: true } },
    { name: "webkit", use: { browserName: "webkit", headless: true } },
  ],
});
