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
} from 'lucide-vue-next';

const kbStore = useKBStore();
const isOpen = ref(false);
const dropdownRef = ref<HTMLElement | null>(null);

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
}

onMounted(() => {
  kbStore.fetchKBs();
  document.addEventListener('click', handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
});
</script>
