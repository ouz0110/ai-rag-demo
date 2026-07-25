<template>
  <div class="chat-layout">
    <!-- 左侧 Sidebar -->
    <Sidebar />

    <!-- 右侧内容主面板 -->
    <main class="chat-main">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import Sidebar from '../components/layout/Sidebar.vue';
</script>

<style scoped>
.chat-layout {
  width: 100vw;
  height: 100vh;
  min-height: 100vh;
  display: flex;
  background-color: #060810;
  color: #f8fafc;
  overflow: hidden;
  user-select: none;
  font-family: 'Inter', system-ui, -apple-system, sans-serif;
}

.chat-main {
  flex: 1;
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
  background-color: #080a13;
}
</style>

