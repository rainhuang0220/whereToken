import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'
import 'virtual:wheretoken-themes.css'
import './styles.css'
import { STORAGE_KEY, applyTheme, resolveThemeId } from './themes'

try {
  applyTheme(resolveThemeId(localStorage.getItem(STORAGE_KEY)))
} catch {
  applyTheme('kiln')
}

createApp(App).use(createPinia()).mount('#app')
