import { createPinia } from 'pinia'
import { createApp } from 'vue'
import gsap from 'gsap'
import { Flip } from 'gsap/Flip'
import App from './App.vue'
import { router } from './router'
import 'virtual:wheretoken-themes.css'
import './styles.css'
import { STORAGE_KEY, applyTheme, resolveThemeId } from './themes'

gsap.registerPlugin(Flip)

try {
  applyTheme(resolveThemeId(localStorage.getItem(STORAGE_KEY)))
} catch {
  applyTheme('kiln')
}

createApp(App).use(createPinia()).use(router).mount('#app')
