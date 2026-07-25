<template>
  <div class="chat-layout w-screen h-screen flex bg-[#131419] overflow-hidden">
    <!-- 左侧 Sidebar -->
    <Sidebar />

    <!-- 右侧内容主面板 -->
    <main class="flex-1 h-full flex flex-col relative overflow-hidden bg-[#131419]">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import Sidebar from '../components/layout/Sidebar.vue';
</script>
