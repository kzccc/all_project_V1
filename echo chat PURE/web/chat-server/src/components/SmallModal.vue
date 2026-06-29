<template>
  <!-- 该组件负责承载局部弹窗或导航交互，供多个页面复用。 -->

  <div class="modal-overlay" v-if="isVisible">
    <div class="modal-content">
      <slot name="header">
      </slot>
      <slot name="body">
      </slot>
      <slot name="footer">
      </slot>
    </div>
  </div>
</template>

<script>
// 组件脚本负责当前组件的状态管理、事件响应和对外通信。
export default {
  name: "Modal",
  props: {
    isVisible: false,
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;
  border-radius: 30px;
}

.modal-content {
  background: #fff;
  height: 300px;
  border-radius: 20px;
  width: 350px;
  box-shadow: 0 2px 15px rgb(195, 8, 8);
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  
}


</style>
