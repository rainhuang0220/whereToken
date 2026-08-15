/// <reference types="vite/client" />

declare module 'virtual:wheretoken-themes.css'

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}
