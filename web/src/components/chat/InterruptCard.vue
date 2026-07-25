<template>
  <div class="interrupt-card my-4 p-5 rounded-2xl border border-amber-500/30 bg-slate-950/90 shadow-2xl backdrop-blur-xl transition-all duration-300">
    <!-- 头部警告与提醒 -->
    <div class="flex items-start gap-3.5 mb-4 pb-3.5 border-b border-amber-500/20">
      <div class="p-2.5 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-400 flex items-center justify-center shrink-0 shadow-inner">
        <ShieldAlert :size="20" class="animate-pulse" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <h4 class="text-sm font-bold text-slate-100 tracking-wide">高风险敏感操作拦截</h4>
          <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40">
            等待人工审批
          </span>
        </div>
        <p class="text-xs text-slate-400 mt-1 leading-relaxed">
          Agent 触发了敏感系统指令或危险工具请求，需您安全审计与授权决策后方可继续执行。
        </p>
      </div>
    </div>

    <!-- 待审批的工具列表 -->
    <div class="space-y-3 mb-4">
      <div
        v-for="toolCall in pendingCalls"
        :key="toolCall.interrupt_id + toolCall.tool_call_id"
        class="p-3.5 rounded-xl bg-black/50 border border-slate-800 text-xs font-mono shadow-inner"
      >
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-2">
            <Terminal :size="14" class="text-amber-400" />
            <span class="text-slate-200 font-bold text-xs font-mono">工具: {{ toolCall.tool_name }}</span>
          </div>
          <span class="text-[10px] px-2 py-0.5 rounded-md bg-slate-800 text-slate-400 border border-slate-700 font-mono">
            ID: {{ toolCall.tool_call_id }}
          </span>
        </div>
        <div>
          <div class="text-[11px] text-slate-400 font-medium mb-1 flex items-center justify-between">
            <span>调用指令 / 参数:</span>
          </div>
          <pre class="p-3 bg-[#060812] rounded-lg border border-slate-800 text-indigo-300 overflow-x-auto text-[11.5px] font-mono leading-relaxed whitespace-pre-wrap word-break-all select-all">{{ formatArgs(toolCall.arguments) }}</pre>
        </div>
      </div>
    </div>

    <!-- 授权设置与操作区 -->
    <div class="space-y-4 pt-1">
      <!-- 授权范围 Pills 组 -->
      <div class="space-y-1.5">
        <span class="text-xs text-slate-400 font-medium">授权范围:</span>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <button
            type="button"
            @click="approveScope = ApproveScope.AS_SINGLE_CALL"
            class="scope-pill-btn"
            :class="{ active: approveScope === ApproveScope.AS_SINGLE_CALL }"
          >
            <div class="pill-radio-dot"></div>
            <div class="flex flex-col text-left">
              <span class="font-semibold text-xs">仅同意本次调用</span>
              <span class="text-[10px] opacity-75">仅放行当前这一条敏感指令</span>
            </div>
          </button>

          <button
            type="button"
            @click="approveScope = ApproveScope.AS_SESSION_TOOL"
            class="scope-pill-btn"
            :class="{ active: approveScope === ApproveScope.AS_SESSION_TOOL }"
          >
            <div class="pill-radio-dot"></div>
            <div class="flex flex-col text-left">
              <span class="font-semibold text-xs">本会话后续同名工具自动授权</span>
              <span class="text-[10px] opacity-75">此 Session 中后续同名工具不再拦截</span>
            </div>
          </button>
        </div>
      </div>

      <!-- 拒绝原因输入框 (当点击拒绝时展开) -->
      <div v-if="showRejectInput" class="transition-all duration-200">
        <input
          v-model="rejectReason"
          type="text"
          placeholder="请输入拒绝原因或改进指令 (例如: 请换用安全只读模式)..."
          class="w-full px-3.5 py-2 text-xs bg-slate-900/90 border border-red-500/40 rounded-xl text-slate-200 placeholder:text-slate-500 focus:outline-none focus:border-red-400 focus:ring-1 focus:ring-red-400/30 transition-all"
        />
      </div>

      <!-- 底部按钮操作栏 -->
      <div class="flex items-center justify-end gap-3 pt-1">
        <button
          v-if="!showRejectInput"
          @click="showRejectInput = true"
          class="px-4 py-2 text-xs font-semibold bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 text-red-400 rounded-xl transition-all flex items-center gap-1.5 active:scale-95"
        >
          <XCircle :size="14" />
          拒绝执行
        </button>

        <button
          v-else
          @click="handleReject"
          class="px-4 py-2 text-xs font-semibold bg-red-600 hover:bg-red-500 text-white rounded-xl shadow-lg shadow-red-600/20 transition-all flex items-center gap-1.5 active:scale-95"
        >
          <XCircle :size="14" />
          确认拒绝
        </button>

        <button
          @click="handleApprove"
          class="px-5 py-2 text-xs font-bold bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white rounded-xl shadow-lg shadow-emerald-500/25 transition-all flex items-center gap-1.5 active:scale-95"
        >
          <CheckCircle2 :size="15" />
          同意并恢复对话
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { ShieldAlert, Terminal, CheckCircle2, XCircle } from 'lucide-vue-next';
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
    const obj = JSON.parse(str);
    if (obj && typeof obj === 'object' && obj.command) {
      return obj.command;
    }
    return JSON.stringify(obj, null, 2);
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

<style scoped>
.scope-pill-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.scope-pill-btn:hover {
  border-color: rgba(245, 158, 11, 0.4);
  color: #f1f5f9;
}

.scope-pill-btn.active {
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.6);
  color: #fbbf24;
  box-shadow: 0 0 16px rgba(245, 158, 11, 0.15);
}

.pill-radio-dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid rgba(255, 255, 255, 0.2);
  shrink: 0;
  transition: all 0.2s ease;
}

.scope-pill-btn.active .pill-radio-dot {
  border-color: #fbbf24;
  background: #fbbf24;
  box-shadow: inset 0 0 0 3px #0f172a;
}
</style>

