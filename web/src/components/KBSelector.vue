<template>
  <div class="flex items-center gap-2">
    <!-- 1. RAG 显式开关控制按钮 (默认关闭，只有明确点击开启才启用 RAG) -->
    <button
      @click="kbStore.toggleRAG"
      type="button"
      :class="[
        'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-medium transition-all cursor-pointer shadow-sm select-none',
        kbStore.enableRAG
          ? 'bg-gradient-to-r from-cyan-500/20 to-blue-600/20 border-cyan-500/50 text-cyan-300 shadow-cyan-500/10'
          : 'bg-white/5 border-white/10 text-gray-400 hover:text-gray-200 hover:bg-white/10'
      ]"
      :title="kbStore.enableRAG ? 'RAG 检索已开启 (点击关闭)' : 'RAG 检索已禁用 (点击显式开启)'"
    >
      <Sparkles class="w-3.5 h-3.5" :class="kbStore.enableRAG ? 'text-cyan-400 animate-pulse' : 'text-gray-500'" />
      <span>RAG: {{ kbStore.enableRAG ? '已启用' : '已禁用' }}</span>
    </button>

    <!-- 2. Skill 扩展技能显式开关按钮 -->
    <button
      @click="kbStore.toggleSkill"
      type="button"
      :class="[
        'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-medium transition-all cursor-pointer shadow-sm select-none',
        kbStore.enableSkill
          ? 'bg-gradient-to-r from-purple-500/20 to-indigo-600/20 border-purple-500/50 text-purple-300 shadow-purple-500/10'
          : 'bg-white/5 border-white/10 text-gray-400 hover:text-gray-200 hover:bg-white/10'
      ]"
      :title="kbStore.enableSkill ? 'Skill 技能扩展已开启 (点击禁用)' : 'Skill 技能扩展已禁用 (点击显式开启)'"
    >
      <Zap class="w-3.5 h-3.5" :class="kbStore.enableSkill ? 'text-purple-400 animate-pulse' : 'text-gray-500'" />
      <span>Skill: {{ kbStore.enableSkill ? '已启用' : '已禁用' }}</span>
    </button>

    <!-- 3. MCP 扩展工具显式开关按钮 -->
    <button
      @click="kbStore.toggleMCP"
      type="button"
      :class="[
        'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-medium transition-all cursor-pointer shadow-sm select-none',
        kbStore.enableMCP
          ? 'bg-gradient-to-r from-emerald-500/20 to-teal-600/20 border-emerald-500/50 text-emerald-300 shadow-emerald-500/10'
          : 'bg-white/5 border-white/10 text-gray-400 hover:text-gray-200 hover:bg-white/10'
      ]"
      :title="kbStore.enableMCP ? 'MCP 工具扩展已开启 (点击禁用)' : 'MCP 工具扩展已禁用 (点击显式开启)'"
    >
      <Cpu class="w-3.5 h-3.5" :class="kbStore.enableMCP ? 'text-emerald-400 animate-pulse' : 'text-gray-500'" />
      <span>MCP: {{ kbStore.enableMCP ? '已启用' : '已禁用' }}</span>
    </button>

    <!-- 4. Rerank 结果重排显式开关按钮 (当 RAG 开启时或独立展示) -->
    <button
      v-if="kbStore.enableRAG"
      @click="kbStore.toggleRerank"
      type="button"
      :class="[
        'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-medium transition-all cursor-pointer shadow-sm select-none',
        kbStore.enableRerank
          ? 'bg-gradient-to-r from-amber-500/20 to-orange-600/20 border-amber-500/50 text-amber-300 shadow-amber-500/10'
          : 'bg-white/5 border-white/10 text-gray-400 hover:text-gray-200 hover:bg-white/10'
      ]"
      :title="kbStore.enableRerank ? 'Rerank 重排精排已开启 (点击禁用)' : 'Rerank 重排精排已禁用 (点击显式开启)'"
    >
      <Layers class="w-3.5 h-3.5" :class="kbStore.enableRerank ? 'text-amber-400 animate-pulse' : 'text-gray-500'" />
      <span>Rerank: {{ kbStore.enableRerank ? '已启用' : '已禁用' }}</span>
    </button>

    <!-- 2. 关联知识库下拉选择器 (仅当用户开启 RAG 时展示/选择) -->
    <div v-if="kbStore.enableRAG" class="relative inline-block text-left" ref="dropdownRef">
      <button
        @click="isOpen = !isOpen"
        type="button"
        class="inline-flex items-center gap-2 px-3 py-1.5 rounded-xl bg-white/5 border border-white/10 hover:border-cyan-500/40 hover:bg-white/10 text-xs font-medium text-gray-200 transition-all cursor-pointer shadow-sm"
      >
        <Database class="w-3.5 h-3.5 text-cyan-400" />
        <span class="truncate max-w-[140px]">{{ activeKBName }}</span>
        <ChevronDown class="w-3.5 h-3.5 text-gray-400 transition-transform" :class="{ 'rotate-180': isOpen }" />
      </button>

      <!-- 下拉选项菜单 -->
      <div
        v-if="isOpen"
        class="absolute left-0 bottom-full mb-2 w-64 rounded-2xl bg-[#121827] border border-white/15 shadow-2xl z-50 p-2 space-y-1 backdrop-blur-xl animate-in fade-in slide-in-from-bottom-2 duration-150"
      >
        <div class="px-2 py-1 text-[10px] font-semibold text-gray-400 uppercase tracking-wider">
          关联对话知识库
        </div>

        <!-- 系统默认库 -->
        <button
          v-if="kbStore.defaultKB"
          @click="select(kbStore.defaultKB.kb_id)"
          :class="[
            'w-full text-left px-3 py-2 rounded-xl text-xs flex items-center justify-between transition-colors cursor-pointer',
            kbStore.activeKbId === kbStore.defaultKB.kb_id
              ? 'bg-cyan-500/20 text-cyan-300 font-medium'
              : 'text-gray-300 hover:bg-white/5'
          ]"
        >
          <div class="flex items-center gap-2 truncate">
            <ShieldCheck class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
            <span class="truncate">{{ kbStore.defaultKB.name }}</span>
          </div>
          <Check v-if="kbStore.activeKbId === kbStore.defaultKB.kb_id" class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
        </button>

        <!-- 自定义库分界线 -->
        <div v-if="kbStore.customKBs.length > 0" class="border-t border-white/10 my-1"></div>

        <!-- 自定义知识库列表 -->
        <div v-for="kb in kbStore.customKBs" :key="kb.kb_id">
          <button
            @click="select(kb.kb_id)"
            :class="[
              'w-full text-left px-3 py-2 rounded-xl text-xs flex items-center justify-between transition-colors cursor-pointer',
              kbStore.activeKbId === kb.kb_id
                ? 'bg-cyan-500/20 text-cyan-300 font-medium'
                : 'text-gray-300 hover:bg-white/5'
            ]"
          >
            <div class="flex items-center gap-2 truncate">
              <BookOpen class="w-3.5 h-3.5 text-blue-400 shrink-0" />
              <span class="truncate">{{ kb.name }}</span>
            </div>
            <Check v-if="kbStore.activeKbId === kb.kb_id" class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
          </button>
        </div>

        <div class="border-t border-white/10 pt-1">
          <router-link
            to="/kb"
            @click="isOpen = false"
            class="w-full text-left px-3 py-2 rounded-xl text-xs text-cyan-400 hover:bg-cyan-500/10 flex items-center justify-between transition-colors"
          >
            <span class="flex items-center gap-1.5">
              <Plus class="w-3.5 h-3.5" />
              管理与新建知识库
            </span>
            <ArrowRight class="w-3 h-3" />
          </router-link>
        </div>
      </div>
    </div>
    <!-- 5. Agent 子工具调度高阶选项下拉菜单 -->
    <div class="relative inline-block text-left" ref="agentOptsRef">
      <button
        @click="isAgentOptsOpen = !isAgentOptsOpen"
        type="button"
        :class="[
          'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-medium transition-all cursor-pointer shadow-sm select-none',
          kbStore.agentToolOptions.return_full_context_to_parent || kbStore.agentToolOptions.pass_full_context_to_sub_agent
            ? 'bg-gradient-to-r from-blue-500/20 to-indigo-600/20 border-blue-500/50 text-blue-300 shadow-blue-500/10'
            : 'bg-white/5 border-white/10 text-gray-400 hover:text-gray-200 hover:bg-white/10'
        ]"
        title="配置父子 Agent 上下文透传与流式推送行为"
      >
        <Bot class="w-3.5 h-3.5 text-indigo-400" />
        <span>Agent 选项</span>
        <ChevronDown class="w-3 h-3 text-gray-400 transition-transform" :class="{ 'rotate-180': isAgentOptsOpen }" />
      </button>

      <!-- 下拉面板 -->
      <div
        v-if="isAgentOptsOpen"
        class="absolute right-0 bottom-full mb-2 w-72 rounded-2xl bg-[#121827] border border-white/15 shadow-2xl z-50 p-3 space-y-2 backdrop-blur-xl animate-in fade-in slide-in-from-bottom-2 duration-150"
      >
        <div class="px-1 text-[10px] font-semibold text-gray-400 uppercase tracking-wider flex items-center justify-between">
          <span>子 Agent 调度规则设置</span>
          <Sliders class="w-3 h-3 text-indigo-400" />
        </div>

        <div class="space-y-2 text-xs">
          <!-- 1. PassFullContextToSubAgent -->
          <label class="flex items-center justify-between p-2 rounded-xl bg-white/5 hover:bg-white/10 cursor-pointer transition-colors">
            <div class="flex flex-col">
              <span class="font-medium text-gray-200">父 ➔ 子 上下文透传</span>
              <span class="text-[10px] text-gray-400">将父 Agent 完整历史透传给子 Agent</span>
            </div>
            <input
              type="checkbox"
              v-model="kbStore.agentToolOptions.pass_full_context_to_sub_agent"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-indigo-500 focus:ring-indigo-400 focus:ring-offset-gray-900 cursor-pointer"
            />
          </label>

          <!-- 2. ReturnFullContextToParent -->
          <label class="flex items-center justify-between p-2 rounded-xl bg-white/5 hover:bg-white/10 cursor-pointer transition-colors">
            <div class="flex flex-col">
              <span class="font-medium text-gray-200">子 ➔ 父 上下文追加</span>
              <span class="text-[10px] text-gray-400">将子 Agent 多轮执行过程追加回父历史</span>
            </div>
            <input
              type="checkbox"
              v-model="kbStore.agentToolOptions.return_full_context_to_parent"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-indigo-500 focus:ring-indigo-400 focus:ring-offset-gray-900 cursor-pointer"
            />
          </label>

          <!-- 3. StreamSubAgentExecution -->
          <label class="flex items-center justify-between p-2 rounded-xl bg-white/5 hover:bg-white/10 cursor-pointer transition-colors">
            <div class="flex flex-col">
              <span class="font-medium text-gray-200">子 Agent 过程流式推送</span>
              <span class="text-[10px] text-gray-400">实时推送子 Agent 中间推导/工具调用</span>
            </div>
            <input
              type="checkbox"
              v-model="kbStore.agentToolOptions.stream_sub_agent_execution"
              class="w-4 h-4 rounded border-gray-600 bg-gray-700 text-indigo-500 focus:ring-indigo-400 focus:ring-offset-gray-900 cursor-pointer"
            />
          </label>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useKBStore } from '../stores/kb';
import {
  Database,
  ChevronDown,
  ShieldCheck,
  BookOpen,
  Check,
  Plus,
  ArrowRight,
  Sparkles,
  Zap,
  Cpu,
  Layers,
  Bot,
  Sliders,
} from 'lucide-vue-next';

const kbStore = useKBStore();
const isOpen = ref(false);
const isAgentOptsOpen = ref(false);
const dropdownRef = ref<HTMLElement | null>(null);
const agentOptsRef = ref<HTMLElement | null>(null);

const activeKBName = computed(() => {
  const current = kbStore.kbs.find((k) => k.kb_id === kbStore.activeKbId);
  return current ? current.name : '公共知识库';
});

function select(kb_id: string) {
  kbStore.selectKB(kb_id);
  isOpen.value = false;
}

function handleClickOutside(e: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
    isOpen.value = false;
  }
  if (agentOptsRef.value && !agentOptsRef.value.contains(e.target as Node)) {
    isAgentOptsOpen.value = false;
  }
}

onMounted(() => {
  kbStore.fetchKBs();
  document.addEventListener('click', handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
});
</script>
