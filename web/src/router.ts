import { createRouter, createWebHistory } from 'vue-router'
import Home from './pages/Home.vue'
import Themes from './pages/Themes.vue'
import Login from './pages/Login.vue'
import Pair from './pages/Pair.vue'
import Settings from './pages/Settings.vue'
import { isHosted } from './mode'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', component: Home },
    { path: '/app', component: Home },
    { path: '/login', component: Login },
    { path: '/pair/:code', component: Pair },
    { path: '/settings/:section?', component: Settings },
    { path: '/themes/:id?', component: Themes },
  ],
})

router.beforeEach((to) => {
  if (!isHosted()) {
    if (to.path.startsWith('/settings') || to.path.startsWith('/login') || to.path.startsWith('/pair')) {
      return { path: '/' }
    }
    return
  }
  if (to.path === '/') return { path: '/app' }
})
