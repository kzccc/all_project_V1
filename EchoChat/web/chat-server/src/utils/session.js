import axios from "axios";

const reconnectState = {
  timer: null,
  retries: 0,
  active: false,
  maxWindowMs: 15 * 60 * 1000,
  startedAt: 0,
  schedule: [0, 1000, 2000, 5000, 10000],
  backoffMs: 30000,
  closingSocket: null,
};

export const isSocketAvailable = (socket) =>
  !!socket &&
  (socket.readyState === WebSocket.OPEN ||
    socket.readyState === WebSocket.CONNECTING);

export const shouldKeepReconnect = (store) => {
  return !!(
    store &&
    store.state &&
    store.state.accessToken &&
    store.state.refreshToken &&
    store.state.userInfo &&
    store.state.userInfo.uuid
  );
};

export const normalizeUserInfo = (store, userInfo) => {
  if (!userInfo) {
    return {};
  }
  const nextUserInfo = { ...userInfo };
  if (nextUserInfo.avatar && !nextUserInfo.avatar.startsWith("http")) {
    nextUserInfo.avatar = store.state.backendUrl + nextUserInfo.avatar;
  }
  return nextUserInfo;
};

const closeSocketOnly = (store) => {
  const socket = store.state.socket;
  if (socket) {
    reconnectState.closingSocket = socket;
    try {
      socket.close();
    } catch (error) {
      console.error(error);
    } finally {
      setTimeout(() => {
        if (reconnectState.closingSocket === socket) {
          reconnectState.closingSocket = null;
        }
      }, 0);
    }
  }
  store.commit("setSocket", null);
};

export const applyLoginSession = (store, payload) => {
  if (!payload || !payload.access_token || !payload.refresh_token) {
    return;
  }
  const {
    access_token: accessToken,
    refresh_token: refreshToken,
    ...userInfo
  } = payload;
  store.commit("setLoginSession", {
    accessToken,
    refreshToken,
    userInfo: normalizeUserInfo(store, userInfo),
  });
};

export const applyRefreshedTokens = (store, payload) => {
  if (!payload || !payload.access_token || !payload.refresh_token) {
    return;
  }
  store.commit("setRefreshedTokens", {
    accessToken: payload.access_token,
    refreshToken: payload.refresh_token,
  });
};

export const disconnectSocket = (store) => {
  stopReconnect(store);
  closeSocketOnly(store);
};

const clearReconnectTimer = () => {
  if (reconnectState.timer) {
    clearTimeout(reconnectState.timer);
    reconnectState.timer = null;
  }
};

export const stopReconnect = (store) => {
  clearReconnectTimer();
  reconnectState.active = false;
  reconnectState.retries = 0;
  reconnectState.startedAt = 0;
  if (store) {
    store.commit("setSocket", null);
  }
};

const scheduleReconnect = (store, handlers = {}) => {
  if (!reconnectState.active || !shouldKeepReconnect(store)) {
    stopReconnect(store);
    return;
  }
  const elapsed = Date.now() - reconnectState.startedAt;
  if (elapsed > reconnectState.maxWindowMs) {
    stopReconnect(store);
    return;
  }
  const index = reconnectState.retries;
  const delay =
    reconnectState.schedule[index] !== undefined
      ? reconnectState.schedule[index]
      : reconnectState.backoffMs;
  clearReconnectTimer();
  reconnectState.timer = setTimeout(() => {
    if (!reconnectState.active) {
      return;
    }
    const socket = connectSocket(store, handlers, true);
    if (!socket) {
      reconnectState.retries += 1;
      scheduleReconnect(store, handlers);
      return;
    }
  }, delay);
};

export const startReconnect = (store, handlers = {}) => {
  if (!shouldKeepReconnect(store)) {
    return;
  }
  if (reconnectState.active) {
    return;
  }
  reconnectState.active = true;
  reconnectState.startedAt = Date.now();
  reconnectState.retries = 0;
  scheduleReconnect(store, handlers);
};

export const connectSocket = (store, handlers = {}, force = false) => {
  if (!shouldKeepReconnect(store)) {
    return null;
  }
  if (!reconnectState.active) {
    reconnectState.active = true;
    reconnectState.startedAt = Date.now();
    reconnectState.retries = 0;
  }
  if (
    !force &&
    isSocketAvailable(store.state.socket)
  ) {
    return store.state.socket;
  }
  closeSocketOnly(store);
  const wsUrl =
    store.state.wsUrl +
    "/wss?token=" +
    encodeURIComponent(store.state.accessToken);
  const socket = new WebSocket(wsUrl);
  socket.onopen =
    handlers.onOpen ||
    (() => {
      console.log("WebSocket连接已打开");
      clearReconnectTimer();
      reconnectState.retries = 0;
      reconnectState.startedAt = Date.now();
    });
  socket.onmessage =
    handlers.onMessage ||
    ((message) => {
      console.log("收到消息：", message.data);
    });
  socket.onclose =
    handlers.onClose ||
    (() => {
      console.log("WebSocket连接已关闭");
      if (reconnectState.closingSocket === socket) {
        reconnectState.closingSocket = null;
        store.commit("setSocket", null);
        return;
      }
      store.commit("setSocket", null);
      if (reconnectState.active && shouldKeepReconnect(store)) {
        reconnectState.retries += 1;
        scheduleReconnect(store, handlers);
      }
    });
  socket.onerror =
    handlers.onError ||
    ((error) => {
      console.log("WebSocket连接发生错误", error);
      if (reconnectState.active && shouldKeepReconnect(store)) {
        reconnectState.retries += 1;
        scheduleReconnect(store, handlers);
      }
    });
  store.commit("setSocket", socket);
  return socket;
};

export const ensureSocketConnected = (store, handlers = {}) => {
  if (!shouldKeepReconnect(store)) {
    return null;
  }
  if (isSocketAvailable(store.state.socket)) {
    return store.state.socket;
  }
  return connectSocket(store, handlers);
};

export const logoutSession = async (store) => {
  const currentUserId = store.state.userInfo?.uuid;
  try {
    if (store.state.accessToken && currentUserId) {
      await axios.post(store.state.backendUrl + "/user/wsLogout", {
        owner_id: currentUserId,
      });
    }
  } catch (error) {
    console.error(error);
  } finally {
    stopReconnect(store);
    disconnectSocket(store);
    store.commit("cleanAuth");
  }
};

export const refreshAccessToken = async (store) => {
  if (!store.state.refreshToken) {
    throw new Error("refresh token missing");
  }
  const response = await axios.post(
    store.state.backendUrl + "/auth/refresh",
    {
      refresh_token: store.state.refreshToken,
    },
    {
      _skipAuthRefresh: true,
      _skipAccessToken: true,
    }
  );
  if (response?.data?.code !== 200 || !response?.data?.data) {
    throw new Error(response?.data?.message || "refresh failed");
  }
  applyRefreshedTokens(store, response.data.data);
  return response.data.data;
};
