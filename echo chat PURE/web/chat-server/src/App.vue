<template>
  <!-- 该组件模板负责渲染当前页面或组件的可视区域。 -->
  <router-view />
</template>

<script>
// 组件脚本负责当前组件的状态管理、事件响应和对外通信。
import { onMounted } from "vue";
import { useStore } from "vuex";
import { useRouter } from "vue-router";
import axios from "axios";
import { ElMessage } from "element-plus";
import {
  connectSocket,
  logoutSession,
  normalizeUserInfo,
} from "@/utils/session";
export default {
  name: "App",
  setup() {
    const store = useStore();
    const router = useRouter();
    const getUserInfo = async () => {
      try {
        const req = {
          uuid: store.state.userInfo.uuid,
        };
        const rsp = await axios.post(
          store.state.backendUrl + "/user/getUserInfo",
          req
        );
        if (rsp.data.code == 200) {
          const userInfo = normalizeUserInfo(store, rsp.data.data);
          store.commit("setUserInfo", userInfo);
          return userInfo;
        } else {
          console.error(rsp.data.message);
        }
      } catch (error) {
        console.log(error);
      }
      return null;
    };
    const logout = async () => {
      await logoutSession(store);
      router.push("/login");
      ElMessage.error("账号已被禁用，请联系管理员。");
    };
    onMounted(async () => {
      if (store.state.accessToken && store.state.refreshToken && store.state.userInfo.uuid) {
        const userInfo = await getUserInfo();
        if (!userInfo) {
          return;
        }
        if (userInfo.status == 1) {
          await logout();
          return;
        }
        connectSocket(store);
      }
    });
    return {};
  },
};
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box; /* 推荐使用，以确保布局计算的一致性 */
}
</style>
