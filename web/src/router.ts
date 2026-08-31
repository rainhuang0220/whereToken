import { createRouter, createWebHistory } from 'vue-router'
import Home from './pages/Home.vue'
import Themes from './pages/Themes.vue'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', component: Home },
    { path: '/themes/:id?', component: Themes },
  ],
})
