<template>
  <div class="chat-index-page h-full flex flex-col justify-between overflow-hidden">
    <!-- 中央欢迎内容 -->
    <div class="flex-1 flex flex-col items-center justify-center p-6 text-center overflow-y-auto">
      <div class="w-16 h-16 rounded-3xl bg-gradient-to-tr from-indigo-500 via-blue-500 to-cyan-400 flex items-center justify-center font-black text-white text-2xl shadow-xl mb-4 animate-pulse">
        DS
      </div>
      <h1 class="text-3xl font-bold text-slate-100 mb-2">我是 DeepSeek，很高兴见到你</h1>
      <p class="text-xs text-slate-400 max-w-md leading-relaxed mb-8">
        拥有强大的 CoT 深度推理能力，支持复杂的代码分析、大模型 Agent 工具自动化调用与实时人工授权审批。
      </p>

      <!-- 推荐 Prompt 卡片推荐 -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3 max-w-2xl w-full">
        <div
          v-for="item in prompts"
          :key="item.title"
          @click="selectPrompt(item.content)"
          class="p-4 glass-panel rounded-2xl border border-slate-800 hover:border-indigo-500/50 cursor-pointer text-left transition-all duration-200 hover:-translate-y-0.5 group"
        >
          <div class="flex items-center gap-2 mb-1.5">
            <component :is="item.icon" :size="16" class="text-indigo-400 group-hover:text-cyan-400 transition-colors" />
            <h3 class="text-xs font-semibold text-slate-200">{{ item.title }}</h3>
          </div>
          <p class="text-[11px] text-slate-400 line-clamp-2 leading-relaxed">{{ item.content }}</p>
        </div>
      </div>
    </div>

    <!-- 底部输入框 -->
    <ChatInput />
  </div>
</template>

<script setup lang="ts">
import { Code2, Cpu, FileSearch, HelpCircle } from 'lucide-vue-next';
import ChatInput from '../components/chat/ChatInput.vue';
import { useChatStore } from '../stores/chat';

const chatStore = useChatStore();

const prompts = [
  {
    title: '复杂算法与代码重构',
    icon: Code2,
    content: '分析以下算法复杂度，并帮我使用 Go/Vue3 优雅地重构并优化其性能...',
  },
  {
    title: 'Agent 自动化工具与授权',
    icon: Cpu,
    content: '帮我查询系统中的设备状态，需要调用后台 IoT/RAG 工具并触发人工审批流...',
  },
  {
    title: '深度逻辑思考与 CoT 推理',
    icon: FileSearch,
    content: '一步步推导为什么分布式一致性协议 Raft 能防止 split-brain 脑裂问题？',
  },
  {
    title: '系统架构设计咨询',
    icon: HelpCircle,
    content: '为亿级实时流计算系统设计高吞吐、高可用的架构与持久化存储方案...',
  },
];

function selectPrompt(content: string) {
  chatStore.sendMessage(content);
}
</script>
