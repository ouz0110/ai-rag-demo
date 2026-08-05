<template>
  <div class="tools-group-box" :class="{ 'has-agent-tool': hasAgentTools }">
    <!-- 头部汇总栏 (点击折叠/展开全部工具调用) -->
    <div class="tools-group-header" @click="isExpanded = !isExpanded">
      <div class="header-left">
        <div class="status-icon-badge" :class="[isAllCompleted ? 'completed' : 'running', hasAgentTools ? 'agent-tool-badge' : '']">
          <Bot v-if="hasAgentTools" :size="13" class="text-amber-400" />
          <Wrench v-else-if="isAllCompleted" :size="13" />
          <Loader2 v-else :size="13" class="animate-spin" />
        </div>

        <div class="title-info">
          <span class="main-title">
            {{ hasAgentTools ? 'Agent 调度与工具执行' : '工具调用' }} ({{ tools.length }})
          </span>
          <div class="tool-tags-inline">
            <span
              v-for="name in uniqueToolNames"
              :key="name"
              class="tool-name-tag"
              :class="isAgentTool(name) ? 'agent-tag' : ''"
            >
              {{ isAgentTool(name) ? `⚡ @${getSubAgentName(name)}` : name }}
            </span>
          </div>
        </div>
      </div>

      <div class="header-right">
        <span class="status-badge" :class="isAllCompleted ? 'completed' : 'running'">
          {{ isAllCompleted ? `执行完成${totalDurationText ? ` (${totalDurationText})` : ''}` : `执行中 (${completedCount}/${tools.length})...` }}
        </span>
        <ChevronDown :size="14" class="chevron-icon" :class="{ 'rotate-180': isExpanded }" />
      </div>
    </div>

    <!-- 展开后的工具调用步骤列表 -->
    <div v-show="isExpanded" class="tools-list-body">
      <div
        v-for="(t, index) in tools"
        :key="t.tool_call_id || index"
        class="tool-step-item"
        :class="isAgentTool(t.tool_name) ? 'agent-step-item' : ''"
      >
        <div class="step-header" @click="toggleStep(index)">
          <div class="step-title-box">
            <span class="step-index">#{{ index + 1 }}</span>

            <!-- 如果是 AgentTool (委派子 Agent)，渲染为专有 Sub-Agent Badge -->
            <span v-if="isAgentTool(t.tool_name)" class="agent-delegation-title flex items-center gap-1.5 font-semibold text-amber-300">
              <Bot :size="14" class="text-amber-400" />
              <span>⚡ 委派子 Agent (@{{ getSubAgentName(t.tool_name) }})</span>
            </span>

            <span v-else class="tool-name">{{ t.tool_name }}</span>

            <span
              class="step-status-tag"
              :class="t.status === 'completed' ? 'success' : 'running'"
            >
              {{ t.status === 'completed' ? (formatDuration(t.duration_ms) ? `成功 (${formatDuration(t.duration_ms)})` : '成功') : '运行中' }}
            </span>
          </div>
          <ChevronDown :size="12" class="chevron-icon" :class="{ 'rotate-180': openSteps.has(index) }" />
        </div>

        <div v-show="openSteps.has(index)" class="step-detail-box">
          <!-- 专有 AgentTool 委派渲染: 清晰展示 Task 任务与总结，而非裸 JSON -->
          <template v-if="isAgentTool(t.tool_name)">
            <div class="detail-section p-2.5 rounded-lg bg-amber-500/10 border border-amber-500/20">
              <div class="flex items-center gap-1.5 text-xs font-semibold text-amber-300 mb-1">
                <span>📋 委派任务:</span>
                <span class="text-amber-200/90 font-normal">{{ extractQueryArg(t.arguments) }}</span>
              </div>
              <div v-if="t.result_preview" class="detail-section mt-2 border-t border-amber-500/15 pt-2">
                <span class="section-label text-amber-400 text-xs font-semibold">执行总结:</span>
                <div class="text-xs text-slate-200 leading-relaxed mt-1 whitespace-pre-wrap max-h-60 overflow-y-auto custom-scrollbar p-2 rounded bg-black/30 border border-amber-500/10">
                  {{ t.result_preview }}
                </div>
              </div>
            </div>
          </template>

          <!-- 普通物理工具渲染 -->
          <template v-else>
            <div class="detail-section">
              <span class="section-label">调用参数:</span>
              <pre class="code-pre">{{ formatArgs(t.arguments) }}</pre>
            </div>
            <div v-if="t.result_preview" class="detail-section mt-2">
              <span class="section-label text-emerald-400">执行结果预览:</span>
              <pre class="code-pre result-pre">{{ t.result_preview }}</pre>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { Wrench, Loader2, ChevronDown, Bot } from 'lucide-vue-next';
import type { UIStreamTool } from '../../stores/chat';

const props = defineProps<{
  tools: UIStreamTool[];
}>();

const isExpanded = ref(false);
const openSteps = ref<Set<number>>(new Set([0])); // 默认展开第1步

const uniqueToolNames = computed(() => {
  const names = new Set(props.tools.map((t) => t.tool_name));
  return Array.from(names);
});

const completedCount = computed(() => {
  return props.tools.filter((t) => t.status === 'completed').length;
});

const isAllCompleted = computed(() => {
  return completedCount.value === props.tools.length;
});

const hasAgentTools = computed(() => {
  return props.tools.some((t) => isAgentTool(t.tool_name));
});

function isAgentTool(toolName: string): boolean {
  return toolName.startsWith('delegate_to_') || toolName.includes('delegate_to_');
}

function getSubAgentName(toolName: string): string {
  if (toolName.startsWith('delegate_to_')) {
    return toolName.replace('delegate_to_', '');
  }
  return toolName;
}

function extractQueryArg(argsStr: string): string {
  try {
    const obj = JSON.parse(argsStr);
    if (obj && typeof obj === 'object') {
      if (obj.query) return obj.query;
      if (obj.task) return obj.task;
    }
  } catch (e) {}
  return argsStr;
}

function toggleStep(index: number) {
  if (openSteps.value.has(index)) {
    openSteps.value.delete(index);
  } else {
    openSteps.value.add(index);
  }
}

function formatArgs(jsonStr: string) {
  try {
    const obj = JSON.parse(jsonStr);
    if (obj && typeof obj === 'object' && obj.command) {
      return obj.command;
    }
    return JSON.stringify(obj, null, 2);
  } catch (e) {
    return jsonStr;
  }
}

const totalDurationText = computed(() => {
  let sum = 0;
  for (const t of props.tools) {
    if (t.duration_ms && Number(t.duration_ms) > 0) {
      sum += Number(t.duration_ms);
    }
  }
  return formatDuration(sum);
});

function formatDuration(ms?: any): string {
  if (ms === undefined || ms === null || ms === '' || ms === 0) return '';
  const num = typeof ms === 'number' ? ms : Number(ms);
  if (isNaN(num) || num <= 0) return '';
  if (num < 1000) return `${Math.round(num)}ms`;
  return `${(num / 1000).toFixed(2)}s`;
}
</script>

<style scoped>
.tools-group-box {
  margin-top: 0.75rem;
  margin-bottom: 0.25rem;
  border-radius: 12px;
  background: rgba(10, 14, 26, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.08);
  overflow: hidden;
  backdrop-filter: blur(8px);
}

.tools-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  background: rgba(255, 255, 255, 0.03);
  cursor: pointer;
  user-select: none;
  transition: background 0.2s ease;
}

.tools-group-header:hover {
  background: rgba(255, 255, 255, 0.06);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-icon-badge {
  width: 26px;
  height: 26px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-icon-badge.completed {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.status-icon-badge.running {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
}

.title-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.main-title {
  font-size: 0.8125rem;
  font-weight: 700;
  color: #f1f5f9;
}

.tool-tags-inline {
  display: flex;
  align-items: center;
  gap: 4px;
}

.tool-name-tag {
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(99, 102, 241, 0.15);
  border: 1px solid rgba(99, 102, 241, 0.25);
  color: #818cf8;
  font-size: 0.6875rem;
  font-family: monospace;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-badge {
  font-size: 0.725rem;
  padding: 2px 8px;
  border-radius: 6px;
  font-weight: 600;
}

.status-badge.completed {
  background: rgba(16, 185, 129, 0.1);
  color: #34d399;
}

.status-badge.running {
  background: rgba(56, 189, 248, 0.1);
  color: #38bdf8;
}

.chevron-icon {
  color: #94a3b8;
  transition: transform 0.2s ease;
}

.tools-list-body {
  padding: 8px 12px 12px 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tool-step-item {
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.05);
  overflow: hidden;
}

.step-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
}

.step-title-box {
  display: flex;
  align-items: center;
  gap: 6px;
}

.step-index {
  font-size: 0.7rem;
  font-weight: 700;
  color: #64748b;
  font-family: monospace;
}

.tool-name {
  font-size: 0.78125rem;
  font-weight: 600;
  color: #e2e8f0;
  font-family: monospace;
}

.step-status-tag {
  font-size: 0.65rem;
  padding: 1px 5px;
  border-radius: 4px;
}

.step-status-tag.success {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
}

.step-status-tag.running {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
}

.step-detail-box {
  padding: 8px 10px 10px 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.04);
  background: rgba(0, 0, 0, 0.2);
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.section-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: #94a3b8;
}

.code-pre {
  margin: 0;
  padding: 6px 8px;
  background: rgba(15, 23, 42, 0.6);
  border-radius: 6px;
  font-size: 0.725rem;
  color: #cbd5e1;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.result-pre {
  color: #67e8f9;
}

/* 🎯 AgentTool 专有统一视觉样式 */
.tools-group-box.has-agent-tool {
  border-color: rgba(245, 158, 11, 0.25);
  background: rgba(245, 158, 11, 0.04);
}

.status-icon-badge.agent-tool-badge {
  background: rgba(245, 158, 11, 0.15);
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.tool-name-tag.agent-tag {
  background: rgba(245, 158, 11, 0.15);
  color: #fcd34d;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.agent-step-item {
  border-color: rgba(245, 158, 11, 0.2) !important;
  background: rgba(245, 158, 11, 0.03) !important;
}
</style>
