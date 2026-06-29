import axios from "axios";

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
  const socket = store.state.socket;
  if (socket) {
    try {
      socket.close();
    } catch (error) {
      console.error(error);
    }
  }
  store.commit("setSocket", null);
};

export const connectSocket = (store, handlers = {}) => {
  if (!store.state.accessToken) {
    return null;
  }
  if (
    store.state.socket &&
    (store.state.socket.readyState === WebSocket.OPEN ||
      store.state.socket.readyState === WebSocket.CONNECTING)
  ) {
    return store.state.socket;
  }
  disconnectSocket(store);
  const wsUrl =
    store.state.wsUrl +
    "/wss?token=" +
    encodeURIComponent(store.state.accessToken);
  const socket = new WebSocket(wsUrl);
  socket.onopen =
    handlers.onOpen ||
    (() => {
      console.log("WebSocket连接已打开");
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
      store.commit("setSocket", null);
    });
  socket.onerror =
    handlers.onError ||
    ((error) => {
      console.log("WebSocket连接发生错误", error);
    });
  store.commit("setSocket", socket);
  return socket;
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
