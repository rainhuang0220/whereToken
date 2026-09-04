/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEMO?: string
  readonly VITE_HOSTED?: string
}

declare module 'virtual:wheretoken-themes.css'

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}
