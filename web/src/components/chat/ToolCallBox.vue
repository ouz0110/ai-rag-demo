<template>
  <div class="tools-group-box">
    <!-- 头部汇总栏 (点击折叠/展开全部工具调用) -->
    <div class="tools-group-header" @click="isExpanded = !isExpanded">
      <div class="header-left">
        <div class="status-icon-badge" :class="isAllCompleted ? 'completed' : 'running'">
          <Wrench v-if="isAllCompleted" :size="13" />
          <Loader2 v-else :size="13" class="animate-spin" />
        </div>

        <div class="title-info">
          <span class="main-title">
            工具调用 ({{ tools.length }})
          </span>
          <div class="tool-tags-inline">
            <span v-for="name in uniqueToolNames" :key="name" class="tool-name-tag">
              {{ name }}
            </span>
          </div>
        </div>
      </div>

      <div class="header-right">
        <span class="status-badge" :class="isAllCompleted ? 'completed' : 'running'">
          {{ isAllCompleted ? '执行完成' : `执行中 (${completedCount}/${tools.length})...` }}
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
      >
        <div class="step-header" @click="toggleStep(index)">
          <div class="step-title-box">
            <span class="step-index">#{{ index + 1 }}</span>
            <span class="tool-name">{{ t.tool_name }}</span>
            <span
              class="step-status-tag"
              :class="t.status === 'completed' ? 'success' : 'running'"
            >
              {{ t.status === 'completed' ? '成功' : '运行中' }}
            </span>
          </div>
          <ChevronDown :size="12" class="chevron-icon" :class="{ 'rotate-180': openSteps.has(index) }" />
        </div>

        <div v-show="openSteps.has(index)" class="step-detail-box">
          <div class="detail-section">
            <span class="section-label">调用参数:</span>
            <pre class="code-pre">{{ formatArgs(t.arguments) }}</pre>
          </div>
          <div v-if="t.result_preview" class="detail-section mt-2">
            <span class="section-label text-emerald-400">执行结果预览:</span>
            <pre class="code-pre result-pre">{{ t.result_preview }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { Wrench, Loader2, ChevronDown } from 'lucide-vue-next';
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
</script>

<style scoped>
.tools-group-box {
  margin-bottom: 0.875rem;
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
  padding: 8px 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  background: rgba(0, 0, 0, 0.4);
}

.section-label {
  font-size: 0.7rem;
  color: #94a3b8;
  display: block;
  margin-bottom: 4px;
}

.code-pre {
  margin: 0;
  padding: 8px 10px;
  background: #060812;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  color: #cbd5e1;
  font-size: 0.75rem;
  font-family: 'Fira Code', monospace;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.result-pre {
  color: #6ee7b7;
}
</style>
