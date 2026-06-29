// 前端入口文件负责创建应用、注册路由、状态管理和全局 UI 组件。
import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import store from './store'
import axios from 'axios'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
// 引入'https://webrtc.github.io/adapter/adapter-latest.js'
// import 'https://webrtc.github.io/adapter/adapter-latest.js'
// import '@/assets/css/font.css'
import '@/assets/css/chat.css'
import { disconnectSocket, refreshAccessToken } from './utils/session'

let refreshPromise = null

const handleAuthExpired = (message) => {
  disconnectSocket(store)
  store.commit('cleanAuth')
  if (router.currentRoute.value.path !== '/login') {
    router.push('/login')
  }
  ElMessage.error(message)
}

axios.interceptors.request.use((config) => {
  if (!config._skipAccessToken && store.state.accessToken) {
    config.headers = config.headers || {}
    if (!config.headers.Authorization) {
      config.headers.Authorization = `Bearer ${store.state.accessToken}`
    }
  }
  return config
})

axios.interceptors.response.use(
  async (response) => {
    const code = response?.data?.code
    const originalRequest = response?.config || {}
    const message = response?.data?.message || '登录已失效，请重新登录'
    if (
      code === 401 &&
      !originalRequest._retry &&
      !originalRequest._skipAuthRefresh &&
      store.state.refreshToken
    ) {
      originalRequest._retry = true
      try {
        if (!refreshPromise) {
          refreshPromise = refreshAccessToken(store).finally(() => {
            refreshPromise = null
          })
        }
        await refreshPromise
        originalRequest.headers = originalRequest.headers || {}
        originalRequest.headers.Authorization = `Bearer ${store.state.accessToken}`
        return axios(originalRequest)
      } catch (error) {
        handleAuthExpired(message)
        return response
      }
    }
    if (code === 401) {
      handleAuthExpired(message)
    }
    if (code === 403) {
      if (message.includes('禁用') || message.includes('登录已失效')) {
        handleAuthExpired(message)
      } else {
        ElMessage.error(message)
      }
    }
    return response
  },
  (error) => Promise.reject(error)
)

// 先创建根应用实例，再统一挂载图标、状态管理、路由和 UI 组件库。
const app = createApp(App)
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}
app.use(store).use(router).use(ElementPlus).mount('#app')
