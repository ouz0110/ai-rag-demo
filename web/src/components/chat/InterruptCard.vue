<template>
  <div class="interrupt-card my-4 p-4 border border-amber-500/40 bg-amber-950/20 rounded-xl shadow-lg backdrop-blur-md">
    <!-- 头部警告与提醒 -->
    <div class="flex items-center gap-2 mb-3 pb-2.5 border-b border-amber-500/20">
      <AlertTriangle :size="18" class="text-amber-400 animate-bounce" />
      <div>
        <h4 class="text-sm font-semibold text-amber-200">高风险操作拦截 (需人工干预审批)</h4>
        <p class="text-xs text-amber-300/70">Agent 触发敏感工具执行请求，等待您的决策与授权确认。</p>
      </div>
    </div>

    <!-- 待审批的工具列表 -->
    <div class="space-y-2 mb-4">
      <div
        v-for="toolCall in pendingCalls"
        :key="toolCall.interrupt_id + toolCall.tool_call_id"
        class="p-3 bg-black/40 rounded-lg border border-amber-500/20 text-xs font-mono"
      >
        <div class="flex justify-between items-center mb-1">
          <span class="text-amber-300 font-bold">🛠 工具: {{ toolCall.tool_name }}</span>
          <span class="text-[10px] px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300">ID: {{ toolCall.tool_call_id.slice(0, 8) }}</span>
        </div>
        <div class="text-slate-300 text-[11px]">
          <span class="text-slate-400">调用参数:</span>
          <pre class="mt-1 p-2 bg-slate-900/80 rounded border border-slate-800 text-amber-200/90 overflow-x-auto whitespace-pre-wrap">{{ formatArgs(toolCall.arguments) }}</pre>
        </div>
      </div>
    </div>

    <!-- 授权设置区 -->
    <div class="space-y-3 pt-1 border-t border-amber-500/15">
      <!-- 授权范围选择 -->
      <div class="flex items-center gap-4 text-xs text-slate-300">
        <span class="text-slate-400 font-medium">授权范围:</span>
        <label class="flex items-center gap-1.5 cursor-pointer">
          <input
            type="radio"
            v-model="approveScope"
            :value="ApproveScope.AS_SINGLE_CALL"
            class="accent-amber-500"
          />
          <span>仅同意本次调用</span>
        </label>
        <label class="flex items-center gap-1.5 cursor-pointer">
          <input
            type="radio"
            v-model="approveScope"
            :value="ApproveScope.AS_SESSION_TOOL"
            class="accent-amber-500"
          />
          <span>本会话后续同名工具自动授权</span>
        </label>
      </div>

      <!-- 拒绝原因输入框 (当点击拒绝时展开) -->
      <div v-if="showRejectInput" class="transition-all">
        <input
          v-model="rejectReason"
          type="text"
          placeholder="请输入拒绝原因或用户修正意见 (可选)..."
          class="w-full px-3 py-1.5 text-xs bg-black/50 border border-red-500/40 rounded-lg text-slate-200 placeholder:text-slate-500 focus:outline-none focus:border-red-400"
        />
      </div>

      <!-- 按钮操作栏 -->
      <div class="flex items-center justify-end gap-3 pt-1">
        <button
          v-if="!showRejectInput"
          @click="showRejectInput = true"
          class="px-3.5 py-1.5 text-xs font-medium bg-red-950/60 hover:bg-red-900 border border-red-800/60 text-red-300 rounded-lg transition-all"
        >
          拒绝执行
        </button>

        <button
          v-else
          @click="handleReject"
          class="px-3.5 py-1.5 text-xs font-medium bg-red-600 hover:bg-red-500 text-white rounded-lg transition-all"
        >
          确认拒绝
        </button>

        <button
          @click="handleApprove"
          class="px-4 py-1.5 text-xs font-semibold bg-gradient-to-r from-amber-500 to-emerald-500 hover:from-amber-400 hover:to-emerald-400 text-black rounded-lg shadow-md transition-all flex items-center gap-1"
        >
          <CheckCircle2 :size="14" />
          同意并恢复对话
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { AlertTriangle, CheckCircle2 } from 'lucide-vue-next';
import { ResumeAction, ApproveScope, type PendingToolCall } from '../../types/api';

const props = defineProps<{
  pendingCalls: PendingToolCall[];
}>();

const emit = defineEmits<{
  (
    e: 'respond',
    payload: {
      interruptId: string;
      action: ResumeAction;
      approveScope: ApproveScope;
      reason?: string;
    }
  ): void;
}>();

const approveScope = ref<ApproveScope>(ApproveScope.AS_SINGLE_CALL);
const showRejectInput = ref(false);
const rejectReason = ref('');

function formatArgs(str: string) {
  try {
    return JSON.stringify(JSON.parse(str), null, 2);
  } catch (e) {
    return str;
  }
}

function handleApprove() {
  const interruptId = props.pendingCalls[0]?.interrupt_id || '';
  emit('respond', {
    interruptId,
    action: ResumeAction.RA_APPROVE,
    approveScope: approveScope.value,
  });
}

function handleReject() {
  const interruptId = props.pendingCalls[0]?.interrupt_id || '';
  emit('respond', {
    interruptId,
    action: ResumeAction.RA_REJECT,
    approveScope: approveScope.value,
    reason: rejectReason.value || '用户拒绝执行此敏感工具',
  });
}
</script>
