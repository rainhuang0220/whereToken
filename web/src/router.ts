import { createRouter, createWebHistory } from 'vue-router'
import Home from './pages/Home.vue'
import Themes from './pages/Themes.vue'
import Login from './pages/Login.vue'
import Pair from './pages/Pair.vue'
import Devices from './pages/Devices.vue'
import Privacy from './pages/Privacy.vue'
import { isHosted } from './mode'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', component: Home },
    { path: '/app', component: Home },
    { path: '/login', component: Login },
    { path: '/pair/:code', component: Pair },
    { path: '/settings/devices', component: Devices },
    { path: '/settings/privacy', component: Privacy },
    { path: '/themes/:id?', component: Themes },
  ],
})

router.beforeEach((to) => {
  if (!isHosted()) return
  if (to.path === '/') return { path: '/app' }
})
