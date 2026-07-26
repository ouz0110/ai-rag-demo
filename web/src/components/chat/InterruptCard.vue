<template>
  <div
    class="interrupt-card group relative overflow-hidden rounded-2xl border border-amber-500/40 bg-[#0d1120]/95 p-6 sm:p-7 md:p-8 shadow-2xl backdrop-blur-2xl transition-all duration-300 mx-auto my-2 font-sans"
    :class="{ 'is-collapsed': isCollapsed }"
  >
    <!-- Ambient Top Glow Accent -->
    <div class="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-transparent via-amber-500/80 to-transparent"></div>

    <!-- 1. Header Bar (Spacious Title & Subtitle) -->
    <div class="flex items-start justify-between gap-4 select-none" :class="{ 'mb-6': !isCollapsed }">
      <div
        @click="isCollapsed = !isCollapsed"
        class="flex items-start gap-4 cursor-pointer group/title flex-1 min-w-0"
      >
        <div class="w-11 h-11 rounded-2xl bg-amber-500/15 border border-amber-500/40 text-amber-400 flex items-center justify-center shrink-0 shadow-lg shadow-amber-500/10">
          <ShieldAlert :size="22" class="animate-pulse" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-3 flex-wrap">
            <h3 class="text-base sm:text-lg font-extrabold text-slate-100 tracking-wide group-hover/title:text-amber-400 transition-colors">
              高风险敏感指令拦截
            </h3>
            <span class="px-3 py-0.5 rounded-full text-xs font-extrabold bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-sm">
              待人工决策
            </span>
          </div>
          <p v-if="!isCollapsed" class="text-xs sm:text-sm text-slate-400 mt-1.5 leading-relaxed">
            Agent 尝试在当前环境中运行安全敏感指令，请谨慎核对参数确认无误后授权执行
          </p>
        </div>
      </div>

      <div class="flex items-center gap-3 shrink-0 pt-0.5">
        <!-- Mini Summary when Collapsed -->
        <div
          v-if="isCollapsed && pendingCalls[0]"
          class="hidden sm:flex items-center gap-2 px-3.5 py-2 rounded-xl bg-slate-900/90 border border-slate-800 text-xs font-mono text-indigo-300 max-w-[280px] truncate"
        >
          <Terminal :size="14" class="text-amber-400 shrink-0" />
          <span class="truncate font-semibold">{{ formatArgs(getArguments(pendingCalls[0])) }}</span>
        </div>

        <!-- Collapse / Expand Toggle Button -->
        <button
          type="button"
          @click="isCollapsed = !isCollapsed"
          class="px-3.5 py-2 rounded-xl bg-slate-900/90 hover:bg-slate-800 text-slate-300 border border-slate-800 transition-all flex items-center gap-1.5 text-xs sm:text-sm font-semibold cursor-pointer shadow-sm"
          :title="isCollapsed ? '展开详情' : '折叠面板'"
        >
          <span>{{ isCollapsed ? '展开面板' : '折叠面板' }}</span>
          <ChevronUp v-if="!isCollapsed" :size="15" />
          <ChevronDown v-else :size="15" />
        </button>
      </div>
    </div>

    <!-- Expanded Body (Generous Padding & Clear Gaps Between Divisions) -->
    <div v-show="!isCollapsed" class="space-y-6">
      <!-- 2. Command Display Box -->
      <div v-for="(call, idx) in pendingCalls" :key="'cmd-' + (getToolCallId(call) || idx)" class="rounded-2xl border border-slate-800/90 bg-[#040612] overflow-hidden shadow-inner my-1">
        <!-- Terminal Header Bar -->
        <div class="flex items-center justify-between px-4.5 py-3 bg-slate-900/80 border-b border-slate-800/80 text-xs sm:text-sm font-mono">
          <div class="flex items-center gap-3 min-w-0">
            <div class="flex items-center gap-1.5 mr-1">
              <span class="w-2.5 h-2.5 rounded-full bg-red-500/80 inline-block"></span>
              <span class="w-2.5 h-2.5 rounded-full bg-amber-500/80 inline-block"></span>
              <span class="w-2.5 h-2.5 rounded-full bg-emerald-500/80 inline-block"></span>
            </div>
            <Terminal :size="15" class="text-amber-400 shrink-0" />
            <span class="text-slate-400 shrink-0 font-sans font-medium">调用工具:</span>
            <span class="px-2.5 py-0.5 rounded-md bg-amber-500/15 text-amber-300 font-bold border border-amber-500/30 text-xs shrink-0">
              {{ getToolName(call) }}
            </span>
            <span v-if="getToolCallId(call)" class="text-xs text-slate-500 truncate hidden sm:inline font-mono">
              (ID: {{ getToolCallId(call) }})
            </span>
          </div>

          <button
            @click="copyCommand(formatArgs(getArguments(call)))"
            class="px-3 py-1.5 rounded-lg bg-slate-800/90 hover:bg-slate-700 text-slate-200 border border-slate-700 transition-all text-xs flex items-center gap-1.5 cursor-pointer shrink-0 ml-2"
            :title="copiedCmd ? '已复制指令' : '复制指令'"
          >
            <Check v-if="copiedCmd" :size="14" class="text-emerald-400" />
            <Copy v-else :size="14" />
            <span>{{ copiedCmd ? '已复制' : '复制指令' }}</span>
          </button>
        </div>

        <!-- Terminal Code Content Area -->
        <div class="px-5 py-4 flex items-start gap-3.5 text-sm sm:text-base font-mono max-h-[220px] overflow-y-auto">
          <span class="text-amber-500 select-none font-bold shrink-0 text-base leading-relaxed">$</span>
          <code class="text-indigo-100 leading-relaxed font-mono whitespace-pre-wrap word-break-all flex-1 text-sm sm:text-base font-medium tracking-wide">{{ formatArgs(getArguments(call)) }}</code>
        </div>
      </div>

      <!-- 3. Scope Selection Control Bar -->
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 sm:p-5 rounded-2xl bg-slate-900/60 border border-slate-800/70 my-1">
        <span class="text-xs sm:text-sm font-bold text-slate-300 flex items-center gap-2 shrink-0">
          <ShieldAlert :size="16" class="text-amber-400" />
          授权范围选择:
        </span>
        <div class="inline-flex p-1.5 rounded-xl bg-slate-950 border border-slate-800 shrink-0">
          <button
            type="button"
            @click="approveScope = ApproveScope.AS_SINGLE_CALL"
            class="scope-pill-btn"
            :class="{ active: approveScope === ApproveScope.AS_SINGLE_CALL }"
          >
            <span class="pill-dot"></span>
            <span>仅放行本次指令</span>
          </button>

          <button
            type="button"
            @click="approveScope = ApproveScope.AS_SESSION_TOOL"
            class="scope-pill-btn"
            :class="{ active: approveScope === ApproveScope.AS_SESSION_TOOL }"
          >
            <span class="pill-dot"></span>
            <span>本会话同名工具免审 (自动放行)</span>
          </button>
        </div>
      </div>

      <!-- 4. Bottom Action Footer Bar -->
      <div class="pt-5 border-t border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <span class="text-xs sm:text-sm text-slate-400 hidden lg:inline truncate">
          💡 点击“同意并运行”后，Agent 将按照所选范围继续完成对应功能
        </span>

        <div class="flex items-center justify-end gap-4 w-full sm:w-auto">
          <button
            v-if="!showRejectInput"
            @click="showRejectInput = true"
            class="px-5 py-2.5 text-xs sm:text-sm font-bold bg-red-500/10 hover:bg-red-500/20 border border-red-500/30 text-red-400 rounded-xl transition-all flex items-center gap-2 active:scale-95 cursor-pointer"
          >
            <XCircle :size="16" />
            <span>拒绝执行</span>
          </button>

          <template v-else>
            <button
              @click="showRejectInput = false"
              class="px-3.5 py-2.5 text-xs sm:text-sm text-slate-400 hover:text-slate-200 transition-colors cursor-pointer"
            >
              取消
            </button>
            <button
              @click="handleReject"
              class="px-5 py-2.5 text-xs sm:text-sm font-bold bg-red-600 hover:bg-red-500 text-white rounded-xl shadow-lg shadow-red-600/20 transition-all flex items-center gap-2 active:scale-95 cursor-pointer"
            >
              <XCircle :size="16" />
              <span>确认拒绝</span>
            </button>
          </template>

          <button
            @click="handleApprove"
            style="margin-left: 15px;width: 121px;"
            class="px-7 py-2.5 text-sm sm:text-base font-extrabold bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-400 hover:to-teal-500 text-white rounded-xl shadow-2xl shadow-emerald-500/30 transition-all flex items-center gap-2.5 active:scale-95 cursor-pointer shrink-0"
          >
            <CheckCircle2 :size="18" style="margin-left: 5px;" />
            <span>同意并运行</span>
          </button>
        </div>
      </div>

      <!-- Optional Rejection Reason Drawer -->
      <div v-if="showRejectInput" class="pt-2 transition-all duration-200">
        <input
          v-model="rejectReason"
          type="text"
          placeholder="请输入拒绝原因 (可留空，默认反馈给 AI Agent)..."
          class="w-full px-4.5 py-3 text-xs sm:text-sm bg-slate-950 border border-red-500/40 rounded-xl text-slate-200 placeholder:text-slate-500 focus:outline-none focus:border-red-400 focus:ring-1 focus:ring-red-400/30 transition-all font-sans"
          @keyup.enter="handleReject"
        />
      </div>
    </div>

    <!-- Collapsed Quick Bar -->
    <div v-show="isCollapsed" class="flex items-center justify-between pt-2.5 mt-2.5 border-t border-slate-800/60">
      <span class="text-xs sm:text-sm text-slate-400 font-mono flex items-center gap-2">
        <span class="w-2.5 h-2.5 rounded-full bg-amber-400 animate-ping"></span>
        敏感操作挂起，等待授权...
      </span>

      <div class="flex items-center gap-3">
        <button
          @click="handleReject"
          class="px-4 py-2 text-xs sm:text-sm font-semibold bg-red-500/15 hover:bg-red-500/25 text-red-300 border border-red-500/30 rounded-xl transition-all active:scale-95 cursor-pointer"
        >
          拒绝
        </button>
        <button
          @click="handleApprove"
          class="px-5 py-2 text-xs sm:text-sm font-bold bg-emerald-600 hover:bg-emerald-500 text-white rounded-xl shadow-md shadow-emerald-600/20 transition-all active:scale-95 cursor-pointer"
        >
          同意并运行
        </button>
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
.scope-pill-btn {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 8px 16px;
  border-radius: 10px;
  font-size: 0.8125rem;
  font-weight: 600;
  color: #94a3b8;
  background: transparent;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
}

.scope-pill-btn:hover {
  color: #f1f5f9;
}

.scope-pill-btn .pill-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #64748b;
  transition: all 0.2s ease;
}

.scope-pill-btn.active {
  background: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.4);
  color: #fbbf24;
  box-shadow: 0 2px 10px rgba(245, 158, 11, 0.15);
}

.scope-pill-btn.active .pill-dot {
  background: #fbbf24;
  box-shadow: 0 0 6px #fbbf24;
}
</style>
