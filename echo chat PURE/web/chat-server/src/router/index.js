// 路由文件负责声明页面映射关系，并在进入受保护页面前校验登录态。
import { createRouter, createWebHistory } from 'vue-router'
import store from '../store/index.js'

// 路由表集中声明登录页、聊天页和后台页的懒加载入口。
const routes = [
  {
    path: '/',
    redirect: { name: 'Login' }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/access/Login.vue')
  },
  {
    path: '/smsLogin',
    name: 'smsLogin',
    component: () => import('../views/access/SmsLogin.vue')
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('../views/access/Register.vue')
  },
  {
    path: '/chat/owninfo',
    name: 'OwnInfo',
    component: () => import('../views/chat/user/OwnInfo.vue')
  },
  {
    path: '/chat/contactlist',
    name: 'ContactList',
    component: () => import('../views/chat/contact/ContactList.vue')
  },
  {
    path: '/chat/:id',
    name: 'ContactChat',
    component: () => import('../views/chat/contact/ContactChat.vue')
  },
  {
    path: '/chat/sessionList',
    name: 'SessionList',
    component: () => import('../views/chat/session/SessionList.vue')
  },
  {
    path: '/manager',
    name: 'Manager',
    component: () => import('../views/manager/Manager.vue')
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
});

router.beforeEach((to, from, next) => {
  const hasAuth = !!store.state.accessToken && !!store.state.refreshToken && !!store.state.userInfo.uuid
  // 未登录时只允许访问登录、注册和验证码登录页，其余页面统一跳回登录页。
  if (!hasAuth) {
    if (to.path === '/login' || to.path === '/register' || to.path === '/smsLogin') {
      next()
      return
    }
    next('/login')
    return
  }
  if (to.path === '/manager' && store.state.userInfo.is_admin !== 1) {
    next('/chat/sessionlist')
    return
  }
  next()
})

export default router
