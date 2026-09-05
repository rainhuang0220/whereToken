/// <reference types="vitest/config" />
import { rmSync } from 'node:fs'
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { themeStylesheet } from './src/themes'

function omitSampleOnHosted(): Plugin {
  return {
    name: 'omit-sample-on-hosted',
    closeBundle() {
      if (process.env.VITE_HOSTED === '1') {
        rmSync('dist/sample', { recursive: true, force: true })
      }
    },
  }
}

function themeCss(): Plugin {
  const id = 'virtual:wheretoken-themes.css'
  const resolved = '\0' + id
  return {
    name: 'wheretoken-themes-css',
    resolveId(source) {
      if (source === id) return resolved
    },
    load(source) {
      if (source === resolved) return themeStylesheet()
    },
  }
}

export default defineConfig({
  // The embedded dashboard is served at root by `wheretoken serve`; the
  // public demo build (VITE_DEMO=1) lives at /whereToken/demo/ on the
  // GitHub Pages project site.
  base: process.env.VITE_DEMO === '1' ? '/whereToken/demo/' : '/',
  plugins: [vue(), themeCss(), omitSampleOnHosted()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8787',
    },
  },
  test: {
    environment: 'node',
  },
})
