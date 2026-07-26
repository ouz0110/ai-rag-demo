<template>
  <div
    class="interrupt-card group relative overflow-hidden rounded-2xl border border-amber-500/35 bg-[#0a0d18]/95 shadow-2xl backdrop-blur-2xl transition-all duration-300"
    :class="{ 'is-collapsed': isCollapsed }"
  >
    <!-- Top Glowing Gradient Accent Bar -->
    <div class="h-1 w-full bg-gradient-to-r from-amber-500 via-orange-500 to-red-500"></div>

    <div class="p-4.5 sm:p-5.5 space-y-4">
      <!-- 1. Header Bar (Always Visible, Clickable to Fold/Unfold) -->
      <div class="flex items-center justify-between gap-3 select-none">
        <div
          @click="isCollapsed = !isCollapsed"
          class="flex items-center gap-3 cursor-pointer group/title flex-1"
        >
          <div class="p-2 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 flex items-center justify-center shrink-0 shadow-inner">
            <ShieldAlert :size="18" class="animate-pulse" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h4 class="text-sm font-bold text-slate-100 tracking-wide group-hover/title:text-amber-400 transition-colors">
                高风险敏感指令拦截
              </h4>
              <span class="px-2.5 py-0.5 rounded-full text-[11px] font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40">
                待授权审批
              </span>
            </div>
            <p v-if="!isCollapsed" class="text-xs text-slate-400 mt-0.5">
              Agent 尝试运行高风险系统指令，请进行安全决策后恢复对话
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <!-- Mini Collapsed Summary Tag -->
          <div
            v-if="isCollapsed && pendingCalls[0]"
            class="hidden sm:flex items-center gap-2 px-3 py-1 rounded-lg bg-slate-900/90 border border-slate-800 text-xs font-mono text-indigo-300 max-w-[280px] truncate"
          >
            <Terminal :size="13" class="text-amber-400 shrink-0" />
            <span class="truncate font-semibold">{{ formatArgs(getArguments(pendingCalls[0])) }}</span>
          </div>

          <!-- Collapse / Expand Toggle Button -->
          <button
            type="button"
            @click="isCollapsed = !isCollapsed"
            class="p-1.5 rounded-xl bg-slate-900/80 hover:bg-slate-800 text-slate-400 hover:text-slate-200 border border-slate-800 transition-all flex items-center gap-1 text-xs font-medium cursor-pointer"
            :title="isCollapsed ? '展开详情' : '折叠面板'"
          >
            <span class="text-[11px] px-1 hidden sm:inline">{{ isCollapsed ? '展开' : '折叠' }}</span>
            <ChevronUp v-if="!isCollapsed" :size="16" />
            <ChevronDown v-else :size="16" />
          </button>
        </div>
      </div>

      <!-- Expanded Content -->
      <div v-show="!isCollapsed" class="space-y-4 pt-1 transition-all duration-300">
        <!-- 2. Command Display Window -->
        <div v-for="(call, idx) in pendingCalls" :key="'cmd-' + (getToolCallId(call) || idx)" class="relative group/cmd">
          <div class="flex items-center justify-between text-xs text-slate-400 mb-1.5 font-mono px-1">
            <span class="flex items-center gap-1.5 font-semibold text-slate-300">
              <Terminal :size="13" class="text-amber-400" />
              工具: <span class="text-amber-300 font-bold">{{ getToolName(call) }}</span>
            </span>
            <span v-if="getToolCallId(call)" class="text-[10px] text-slate-500 font-mono">ID: {{ getToolCallId(call) }}</span>
          </div>

          <div class="p-3.5 bg-[#050711] rounded-xl border border-slate-800 flex items-start gap-2.5 text-xs font-mono shadow-inner overflow-x-auto">
            <span class="text-amber-500 select-none font-bold shrink-0 text-sm">$</span>
            <code class="text-indigo-200 leading-relaxed font-mono whitespace-pre-wrap word-break-all flex-1 text-[12.5px]">{{ formatArgs(getArguments(call)) }}</code>
            <button
              @click="copyCommand(formatArgs(getArguments(call)))"
              class="shrink-0 px-2 py-1 rounded-lg bg-slate-800/90 hover:bg-slate-700 text-slate-300 border border-slate-700 transition-all text-[11px] flex items-center gap-1 cursor-pointer"
              :title="copiedCmd ? '已复制' : '复制指令'"
            >
              <Check v-if="copiedCmd" :size="13" class="text-emerald-400" />
              <Copy v-else :size="13" />
              <span>{{ copiedCmd ? '已复制' : '复制' }}</span>
            </button>
          </div>
        </div>

        <!-- 3. Scope Cards Selection Group -->
        <div class="space-y-2 pt-1">
          <span class="text-xs font-bold text-slate-300 tracking-wide flex items-center gap-1.5">
            <span>授权范围设置:</span>
          </span>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div
              @click="approveScope = ApproveScope.AS_SINGLE_CALL"
              class="scope-option-card"
              :class="{ active: approveScope === ApproveScope.AS_SINGLE_CALL }"
            >
              <div class="option-radio-dot"></div>
              <div class="flex flex-col text-left">
                <span class="font-bold text-xs">仅同意本次调用</span>
                <span class="text-[11px] opacity-75 mt-0.5">仅放行当前这一条敏感指令</span>
              </div>
            </div>

            <div
              @click="approveScope = ApproveScope.AS_SESSION_TOOL"
              class="scope-option-card"
              :class="{ active: approveScope === ApproveScope.AS_SESSION_TOOL }"
            >
              <div class="option-radio-dot"></div>
              <div class="flex flex-col text-left">
                <span class="font-bold text-xs">本会话同名工具免审</span>
                <span class="text-[11px] opacity-75 mt-0.5">此 Session 中后续同名工具不再拦截</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Optional Rejection Reason Drawer -->
        <div v-if="showRejectInput" class="pt-1 transition-all duration-200">
          <input
            v-model="rejectReason"
            type="text"
            placeholder="请输入拒绝原因 (可留空，默认反馈给 AI)..."
            class="w-full px-4 py-2 text-xs bg-slate-900/90 border border-red-500/40 rounded-xl text-slate-200 placeholder:text-slate-500 focus:outline-none focus:border-red-400 focus:ring-1 focus:ring-red-400/30 transition-all font-sans"
            @keyup.enter="handleReject"
          />
        </div>

        <!-- 4. Action Buttons -->
        <div class="flex items-center justify-end gap-3 pt-2 border-t border-slate-800/80">
          <button
            v-if="!showRejectInput"
            @click="showRejectInput = true"
            class="px-4 py-2 text-xs font-bold bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 text-red-400 rounded-xl transition-all flex items-center gap-1.5 active:scale-95 cursor-pointer"
          >
            <XCircle :size="15" />
            <span>拒绝执行</span>
          </button>

          <button
            v-else
            @click="handleReject"
            class="px-4 py-2 text-xs font-bold bg-red-600 hover:bg-red-500 text-white rounded-xl shadow-lg shadow-red-600/20 transition-all flex items-center gap-1.5 active:scale-95 cursor-pointer"
          >
            <XCircle :size="15" />
            <span>确认拒绝</span>
          </button>

          <button
            @click="handleApprove"
            class="px-5 py-2 text-xs font-extrabold bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white rounded-xl shadow-xl shadow-emerald-500/25 transition-all flex items-center gap-1.5 active:scale-95 cursor-pointer shrink-0"
          >
            <CheckCircle2 :size="16" />
            <span>同意并运行</span>
          </button>
        </div>
      </div>

      <!-- Collapsed Quick Action Bar -->
      <div v-show="isCollapsed" class="flex items-center justify-between pt-2 border-t border-slate-800/60">
        <span class="text-xs text-slate-400 font-mono flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-amber-400 animate-ping"></span>
          操作挂起中，请决策...
        </span>

        <div class="flex items-center gap-2.5">
          <button
            @click="handleReject"
            class="px-3.5 py-1.5 text-xs font-semibold bg-red-500/15 hover:bg-red-500/25 text-red-300 border border-red-500/30 rounded-xl transition-all active:scale-95 cursor-pointer"
          >
            拒绝
          </button>
          <button
            @click="handleApprove"
            class="px-4 py-1.5 text-xs font-bold bg-emerald-600 hover:bg-emerald-500 text-white rounded-xl shadow-md shadow-emerald-600/20 transition-all active:scale-95 cursor-pointer"
          >
            同意并运行
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { ShieldAlert, Terminal, CheckCircle2, XCircle, Copy, Check, ChevronDown, ChevronUp } from 'lucide-vue-next';
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

const isCollapsed = ref(false);
const approveScope = ref<ApproveScope>(ApproveScope.AS_SINGLE_CALL);
const showRejectInput = ref(false);
const rejectReason = ref('');
const copiedCmd = ref(false);

function getToolName(call: any): string {
  return call?.tool_name || call?.toolName || call?.ToolName || 'terminal';
}

function getToolCallId(call: any): string {
  const id = call?.tool_call_id || call?.toolCallId || call?.ToolCallId || '';
  return id ? (id.length > 14 ? `${id.slice(0, 14)}...` : id) : '';
}

function getInterruptId(call: any): string {
  return call?.interrupt_id || call?.interruptId || call?.InterruptId || '';
}

function getArguments(call: any): string {
  return call?.arguments || call?.Arguments || '';
}

function formatArgs(str: string) {
  if (!str) return '';
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
  const firstCall = props.pendingCalls[0];
  const interruptId = getInterruptId(firstCall);
  emit('respond', {
    interruptId,
    action: ResumeAction.RA_APPROVE,
    approveScope: approveScope.value,
  });
}

function handleReject() {
  const firstCall = props.pendingCalls[0];
  const interruptId = getInterruptId(firstCall);
  emit('respond', {
    interruptId,
    action: ResumeAction.RA_REJECT,
    approveScope: approveScope.value,
    reason: rejectReason.value || '用户拒绝执行此敏感工具',
  });
}
</script>

<style scoped>
.scope-option-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.scope-option-card:hover {
  border-color: rgba(245, 158, 11, 0.4);
  color: #f1f5f9;
  background: rgba(15, 23, 42, 0.9);
}

.scope-option-card.active {
  background: rgba(245, 158, 11, 0.12);
  border-color: rgba(245, 158, 11, 0.6);
  color: #fbbf24;
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.15);
}

.option-radio-dot {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid rgba(255, 255, 255, 0.25);
  flex-shrink: 0;
  transition: all 0.2s ease;
}

.scope-option-card.active .option-radio-dot {
  border-color: #fbbf24;
  background: #fbbf24;
  box-shadow: inset 0 0 0 3.5px #0a0d18;
}
</style>
