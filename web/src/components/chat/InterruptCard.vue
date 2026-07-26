<template>
  <div class="interrupt-card my-3.5 group relative overflow-hidden rounded-2xl border border-amber-500/30 bg-[#0b0f19]/90 shadow-2xl backdrop-blur-xl transition-all duration-300">
    <!-- Top Glowing Accent Line -->
    <div class="h-0.5 w-full bg-gradient-to-r from-amber-500 via-orange-500 to-red-500"></div>

    <div class="p-4 sm:p-4.5 space-y-3">
      <!-- 1. Header Bar -->
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2.5">
          <div class="p-1.5 rounded-lg bg-amber-500/15 border border-amber-500/30 text-amber-400 flex items-center justify-center shrink-0">
            <ShieldAlert :size="16" class="animate-pulse" />
          </div>
          <div class="flex items-center gap-2">
            <h4 class="text-xs font-bold text-slate-100 tracking-wide">高风险指令拦截</h4>
            <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/30">
              待授权
            </span>
          </div>
        </div>

        <div v-for="call in pendingCalls" :key="call.tool_call_id" class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-900/80 border border-slate-800 text-[11px] font-mono text-slate-400">
          <Terminal :size="12" class="text-amber-400" />
          <span class="text-slate-200 font-semibold">{{ call.tool_name }}</span>
          <span class="text-[10px] text-slate-500">({{ call.tool_call_id.slice(0, 10) }}...)</span>
        </div>
      </div>

      <!-- 2. Command Display Box -->
      <div v-for="call in pendingCalls" :key="'cmd-' + call.tool_call_id" class="relative group/cmd">
        <div class="p-3 bg-[#050711] rounded-xl border border-slate-800/90 flex items-start gap-2 text-xs font-mono shadow-inner overflow-x-auto">
          <span class="text-amber-500 select-none font-bold shrink-0">$</span>
          <code class="text-indigo-200 leading-relaxed font-mono whitespace-pre-wrap word-break-all flex-1">{{ formatArgs(call.arguments) }}</code>
          <button
            @click="copyCommand(formatArgs(call.arguments))"
            class="shrink-0 p-1 rounded-md bg-slate-800/80 hover:bg-slate-700 text-slate-400 hover:text-slate-200 border border-slate-700/60 transition-all opacity-0 group-hover/cmd:opacity-100"
            :title="copiedCmd ? '已复制指令' : '复制指令'"
          >
            <Check v-if="copiedCmd" :size="12" class="text-emerald-400" />
            <Copy v-else :size="12" />
          </button>
        </div>
      </div>

      <!-- 3. Integrated Scope & Action Bar -->
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-1 border-t border-slate-800/60">
        <!-- Scope Segmented Control -->
        <div class="inline-flex p-0.5 rounded-xl bg-slate-900/90 border border-slate-800 shrink-0">
          <button
            type="button"
            @click="approveScope = ApproveScope.AS_SINGLE_CALL"
            class="segmented-pill"
            :class="{ active: approveScope === ApproveScope.AS_SINGLE_CALL }"
          >
            <span class="dot"></span>
            <span>仅本次放行</span>
          </button>

          <button
            type="button"
            @click="approveScope = ApproveScope.AS_SESSION_TOOL"
            class="segmented-pill"
            :class="{ active: approveScope === ApproveScope.AS_SESSION_TOOL }"
          >
            <span class="dot"></span>
            <span>本会话同名免审</span>
          </button>
        </div>

        <!-- Action Buttons -->
        <div class="flex items-center justify-end gap-2.5">
          <button
            v-if="!showRejectInput"
            @click="showRejectInput = true"
            class="px-3.5 py-1.5 text-xs font-semibold bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 text-red-400 rounded-xl transition-all flex items-center gap-1.5 active:scale-95"
          >
            <XCircle :size="13" />
            <span>拒绝</span>
          </button>

          <button
            v-else
            @click="handleReject"
            class="px-3.5 py-1.5 text-xs font-semibold bg-red-600 hover:bg-red-500 text-white rounded-xl shadow-lg shadow-red-600/20 transition-all flex items-center gap-1.5 active:scale-95"
          >
            <XCircle :size="13" />
            <span>确认拒绝</span>
          </button>

          <button
            @click="handleApprove"
            class="px-4 py-1.5 text-xs font-bold bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white rounded-xl shadow-lg shadow-emerald-500/25 transition-all flex items-center gap-1.5 active:scale-95 shrink-0"
          >
            <CheckCircle2 :size="14" />
            <span>同意并运行</span>
          </button>
        </div>
      </div>

      <!-- Optional Rejection Reason Drawer -->
      <div v-if="showRejectInput" class="pt-1 transition-all duration-200">
        <input
          v-model="rejectReason"
          type="text"
          placeholder="请输入拒绝原因 (可留空，默认反馈给 AI)..."
          class="w-full px-3.5 py-1.5 text-xs bg-slate-900/90 border border-red-500/40 rounded-xl text-slate-200 placeholder:text-slate-500 focus:outline-none focus:border-red-400 focus:ring-1 focus:ring-red-400/30 transition-all font-sans"
          @keyup.enter="handleReject"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { ShieldAlert, Terminal, CheckCircle2, XCircle, Copy, Check } from 'lucide-vue-next';
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
const copiedCmd = ref(false);

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

function copyCommand(cmdText: string) {
  if (!cmdText) return;
  navigator.clipboard.writeText(cmdText);
  copiedCmd.value = true;
  setTimeout(() => {
    copiedCmd.value = false;
  }, 2000);
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
.segmented-pill {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 11px;
  border-radius: 10px;
  font-size: 0.725rem;
  font-weight: 600;
  color: #94a3b8;
  background: transparent;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.segmented-pill:hover {
  color: #f1f5f9;
}

.segmented-pill .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #64748b;
  transition: all 0.2s ease;
}

.segmented-pill.active {
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.35);
  color: #fbbf24;
  box-shadow: 0 2px 8px rgba(245, 158, 11, 0.12);
}

.segmented-pill.active .dot {
  background: #fbbf24;
  box-shadow: 0 0 6px #fbbf24;
}
</style>
