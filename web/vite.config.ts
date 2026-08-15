/// <reference types="vitest/config" />
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { themeStylesheet } from './src/themes'

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
  plugins: [vue(), themeCss()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8787',
    },
  },
  test: {
    environment: 'node',
  },
})
