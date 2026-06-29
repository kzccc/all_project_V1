// Vuex 仓库负责保存全局用户信息、后端地址和 WebSocket 实例。
import { createStore } from 'vuex'

const protocol = window.location.protocol === 'https:' ? 'https' : 'http'
const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
const originHost = window.location.host

export default createStore({
  state: {
    // backendUrl 和 wsUrl 跟随当前页面协议和域名生成，便于本地与线上共用一套前端代码。
    backendUrl: `${protocol}://${originHost}`,
    wsUrl: `${wsProtocol}://${originHost}`,
    // accessToken 用于访问受保护资源和建立 WebSocket。
    accessToken: sessionStorage.getItem('accessToken') || '',
    // refreshToken 用于在 access token 过期后刷新会话。
    refreshToken: sessionStorage.getItem('refreshToken') || '',
    // userInfo 持久化在 sessionStorage 中，页面刷新后仍能恢复登录态。
    userInfo: (sessionStorage.getItem('userInfo') && JSON.parse(sessionStorage.getItem('userInfo'))) || {},
    // socket 保存全局唯一的 WebSocket 连接实例，供多个页面共享。
    socket: null,
  },
  getters: {
  },
  mutations: {
    setAccessToken(state, accessToken) {
      state.accessToken = accessToken || '';
      if (state.accessToken) {
        sessionStorage.setItem('accessToken', state.accessToken);
      } else {
        sessionStorage.removeItem('accessToken');
      }
    },
    setRefreshToken(state, refreshToken) {
      state.refreshToken = refreshToken || '';
      if (state.refreshToken) {
        sessionStorage.setItem('refreshToken', state.refreshToken);
      } else {
        sessionStorage.removeItem('refreshToken');
      }
    },
    // setUserInfo 在更新内存态的同时同步写入 sessionStorage。
    setUserInfo(state, userInfo) {
      state.userInfo = userInfo;
      sessionStorage.setItem('userInfo', JSON.stringify(userInfo));
    },
    setLoginSession(state, payload) {
      state.accessToken = payload.accessToken || '';
      state.refreshToken = payload.refreshToken || '';
      state.userInfo = payload.userInfo || {};
      if (state.accessToken) {
        sessionStorage.setItem('accessToken', state.accessToken);
      } else {
        sessionStorage.removeItem('accessToken');
      }
      if (state.refreshToken) {
        sessionStorage.setItem('refreshToken', state.refreshToken);
      } else {
        sessionStorage.removeItem('refreshToken');
      }
      sessionStorage.setItem('userInfo', JSON.stringify(state.userInfo));
    },
    setRefreshedTokens(state, payload) {
      state.accessToken = payload.accessToken || '';
      state.refreshToken = payload.refreshToken || '';
      if (state.accessToken) {
        sessionStorage.setItem('accessToken', state.accessToken);
      } else {
        sessionStorage.removeItem('accessToken');
      }
      if (state.refreshToken) {
        sessionStorage.setItem('refreshToken', state.refreshToken);
      } else {
        sessionStorage.removeItem('refreshToken');
      }
    },
    setSocket(state, socket) {
      state.socket = socket || null;
    },
    // cleanUserInfo 用于退出登录后的全局状态清理。
    cleanUserInfo(state) {
      state.userInfo = {};
      sessionStorage.removeItem('userInfo');
      state.accessToken = '';
      state.refreshToken = '';
      sessionStorage.removeItem('accessToken');
      sessionStorage.removeItem('refreshToken');
    },
    cleanAuth(state) {
      state.userInfo = {};
      state.socket = null;
      state.accessToken = '';
      state.refreshToken = '';
      sessionStorage.removeItem('userInfo');
      sessionStorage.removeItem('accessToken');
      sessionStorage.removeItem('refreshToken');
    }
  },
  actions: {
  },
  modules: {
  }
})
