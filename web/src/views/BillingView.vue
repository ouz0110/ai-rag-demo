<template>
  <div class="billing-workspace-root">
    <!-- 1. 顶部 Header 导航栏 (完全对齐 KBView) -->
    <header class="billing-header">
      <div class="breadcrumb-box">
        <div class="brand-icon-box">
          <Wallet class="w-5 h-5" />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs font-semibold text-gray-400">系统资产中心</span>
          <ChevronRight class="w-4 h-4 text-gray-600" />
          <h1 class="text-sm font-bold tracking-wide text-gray-100 flex items-center gap-2">
            AI 资源计费与资产引擎
          </h1>
          <span class="text-[10px] font-mono px-2 py-0.5 rounded-full border font-semibold ml-1.5 bg-cyan-500/15 text-cyan-300 border-cyan-500/30">
            BILLING V2.0 ENGINE
          </span>
        </div>
      </div>

      <!-- 右侧全局操作按钮组 -->
      <div class="header-actions">
        <button
          @click="fetchData"
          :disabled="isLoading"
          class="px-3 py-2 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 text-gray-300 transition-all flex items-center gap-1.5 text-xs font-medium cursor-pointer"
          title="刷新计费与日志数据"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': isLoading }" />
          <span>刷新</span>
        </button>

        <button
          @click="showRechargeModal = true"
          class="px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 via-blue-600 to-indigo-600 hover:from-cyan-400 hover:to-indigo-500 text-white font-semibold text-xs transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 cursor-pointer"
        >
          <Plus class="w-4 h-4" />
          <span>快捷测试充值</span>
        </button>
      </div>
    </header>

    <!-- 2. 主体工作区 -->
    <main class="billing-main-content">
      <div class="billing-content-scroll">
        <!-- 2.1 4 KPI 数据统计卡片 (100% 对齐 KBView kpi-grid & kpi-card) -->
        <div class="kpi-grid">
          <!-- 卡片 1: 可用总余额 -->
          <div class="kpi-card">
            <div>
              <div class="text-xs font-semibold text-gray-400">可用总余额 / 积分</div>
              <div class="text-2xl font-bold font-mono text-gray-100 mt-1">
                ￥ {{ formatMoney(balanceData.balance) }}
              </div>
              <div class="text-[10px] text-cyan-400 mt-1 flex items-center gap-1">
                <span class="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>
                含赠送 ￥{{ formatMoney(balanceData.gift_balance) }}
              </div>
            </div>
            <div class="kpi-icon-wrapper bg-cyan-500/10 border border-cyan-500/30 text-cyan-400">
              <Wallet class="w-5 h-5" />
            </div>
          </div>

          <!-- 卡片 2: 历史累计消耗 -->
          <div class="kpi-card">
            <div>
              <div class="text-xs font-semibold text-gray-400">历史累计消耗</div>
              <div class="text-2xl font-bold font-mono text-purple-300 mt-1">
                ￥ {{ formatMoney(balanceData.total_consumed) }}
              </div>
              <div class="text-[10px] text-purple-400 mt-1">
                调用流水总数: {{ logsTotalCount }} 次
              </div>
            </div>
            <div class="kpi-icon-wrapper bg-purple-500/10 border border-purple-500/30 text-purple-400">
              <TrendingUp class="w-5 h-5" />
            </div>
          </div>

          <!-- 卡片 3: 专享体验额度 -->
          <div class="kpi-card">
            <div>
              <div class="text-xs font-semibold text-gray-400">专享体验额度</div>
              <div class="text-2xl font-bold font-mono text-emerald-400 mt-1">
                ￥ {{ formatMoney(balanceData.gift_balance) }}
              </div>
              <div class="text-[10px] text-emerald-400 mt-1 flex items-center gap-1">
                <CheckCircle2 class="w-3 h-3" /> 优先扣除 / 无门槛
              </div>
            </div>
            <div class="kpi-icon-wrapper bg-emerald-500/10 border border-emerald-500/30 text-emerald-400">
              <Sparkles class="w-5 h-5" />
            </div>
          </div>

          <!-- 卡片 4: 计费结算机制 -->
          <div class="kpi-card">
            <div>
              <div class="text-xs font-semibold text-gray-400">计费结算机制</div>
              <div class="text-xl font-bold font-mono text-amber-400 mt-1">
                按 1K 单元
              </div>
              <div class="text-[10px] text-gray-400 mt-1">
                不足 1k 向上取整 / Prompt 区分
              </div>
            </div>
            <div class="kpi-icon-wrapper bg-amber-500/10 border border-amber-500/30 text-amber-400">
              <ShieldCheck class="w-5 h-5" />
            </div>
          </div>
        </div>

        <!-- 2.2 模型计费规则价格表 & 费用预估工具面板 -->
        <section class="card-panel">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-5 pb-4 border-b border-white/10">
            <div>
              <h2 class="text-sm font-bold text-gray-100 flex items-center gap-2">
                <Layers class="w-4 h-4 text-cyan-400" />
                AI 模型计费规则与费用预估
              </h2>
              <p class="text-xs text-gray-400 mt-1">查看系统支持的模型单价标准，或使用实时费用估算工具</p>
            </div>

            <!-- Tab 切换按钮 (对齐 KBView Pill 按钮) -->
            <div class="flex items-center gap-2">
              <button
                @click="activePricingTab = 'cards'"
                :class="['format-pill-btn', activePricingTab === 'cards' && 'active']"
              >
                <Layers class="w-3.5 h-3.5 inline mr-1" />
                模型单价表
              </button>
              <button
                @click="activePricingTab = 'calculator'"
                :class="['format-pill-btn', activePricingTab === 'calculator' && 'active']"
              >
                <Calculator class="w-3.5 h-3.5 inline mr-1" />
                费用估算器
              </button>
            </div>
          </div>

          <!-- Tab 1: 模型价格卡片网格 -->
          <div v-if="activePricingTab === 'cards'" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            <div
              v-for="(price, modelName) in combinedPricingMap"
              :key="modelName"
              class="model-card group"
            >
              <div class="space-y-2">
                <div class="flex items-center justify-between gap-2">
                  <span
                    class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider shrink-0"
                    :class="getModelCategoryClass(String(modelName))"
                  >
                    {{ getModelCategoryName(String(modelName)) }}
                  </span>
                  <span class="text-[10px] px-2 py-0.5 rounded bg-white/5 text-gray-400 border border-white/10 font-mono shrink-0">
                    {{ price.unit_size }} Units
                  </span>
                </div>

                <div
                  class="font-bold text-xs text-gray-100 font-mono group-hover:text-cyan-300 transition-colors break-all leading-snug"
                  :title="String(modelName)"
                >
                  {{ modelName }}
                </div>
              </div>

              <!-- 价格明细框 -->
              <div class="bg-black/30 p-2.5 rounded-xl border border-white/5 space-y-1 text-xs">
                <div class="flex justify-between items-center text-[11px]">
                  <span class="text-gray-400">输入 (Input):</span>
                  <span class="font-mono text-gray-200 font-semibold">￥ {{ price.input_unit_price }} / 1k</span>
                </div>
                <div class="flex justify-between items-center text-[11px]">
                  <span class="text-gray-400">输出 (Output):</span>
                  <span class="font-mono text-gray-200 font-semibold">
                    {{ price.output_unit_price > 0 ? `￥ ${price.output_unit_price} / 1k` : '不适用 / 包含' }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- Tab 2: 交互式费用预估计算器 -->
          <div v-else class="bg-black/30 rounded-2xl p-5 border border-white/5">
            <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-center">
              <div class="lg:col-span-7 space-y-4">
                <div>
                  <label class="block text-xs font-semibold text-gray-300 mb-1.5">选择评估 AI 模型</label>
                  <select
                    v-model="calcModel"
                    class="status-select w-full"
                  >
                    <option v-for="(_, name) in combinedPricingMap" :key="name" :value="name" class="bg-[#121827] text-gray-200">
                      {{ name }} ({{ getModelCategoryName(String(name)) }})
                    </option>
                  </select>
                </div>

                <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div>
                    <label class="block text-xs font-semibold text-gray-300 mb-1.5">
                      输入 (Prompt Tokens / 文本数)
                    </label>
                    <input
                      v-model.number="calcInputTokens"
                      type="number"
                      min="0"
                      step="100"
                      class="main-search-input !pl-3"
                      placeholder="例如: 1500"
                    />
                  </div>

                  <div>
                    <label class="block text-xs font-semibold text-gray-300 mb-1.5">
                      输出 (Completion Tokens / 文档数)
                    </label>
                    <input
                      v-model.number="calcOutputTokens"
                      type="number"
                      min="0"
                      step="100"
                      class="main-search-input !pl-3"
                      placeholder="例如: 800"
                    />
                  </div>
                </div>
              </div>

              <!-- 计算结果卡片 -->
              <div class="lg:col-span-5 bg-gradient-to-br from-cyan-950/30 via-black/40 to-black/40 p-5 rounded-2xl border border-cyan-500/30 flex flex-col justify-between">
                <div class="text-xs font-semibold text-cyan-300 uppercase tracking-wider mb-2 flex items-center justify-between">
                  <span>预计扣除金额估算</span>
                  <span class="text-[10px] text-gray-400 bg-white/10 px-2 py-0.5 rounded">不足 1k 按 1k 计算</span>
                </div>

                <div class="my-2">
                  <div class="text-3xl font-bold text-emerald-400 font-mono">
                    ￥ {{ calculatedCost.toFixed(6) }}
                  </div>
                  <div class="text-xs text-gray-400 mt-2 space-y-1 font-mono">
                    <div class="flex justify-between">
                      <span>输入计费 Units (取整 1K):</span>
                      <span>{{ calcInputUnits }} K units</span>
                    </div>
                    <div class="flex justify-between">
                      <span>输出计费 Units (取整 1K):</span>
                      <span>{{ calcOutputUnits }} K units</span>
                    </div>
                  </div>
                </div>

                <div class="text-[11px] text-gray-400 bg-black/40 p-2.5 rounded-xl border border-white/5 mt-2">
                  💡 公式: ceil(Prompt/1000) * input_price + ceil(Completion/1000) * output_price
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- 2.3 消费历史明细流水表格区 -->
        <section class="space-y-4">
          <!-- 筛选与搜索工具栏 (对齐 KBView .filter-toolbar-card) -->
          <div class="filter-toolbar-card">
            <div>
              <h2 class="text-sm font-bold text-gray-100 flex items-center gap-2">
                <FileText class="w-4 h-4 text-purple-400" />
                消费历史明细流水 (Usage Logs)
              </h2>
            </div>

            <div class="flex flex-wrap items-center gap-3">
              <!-- 服务类型 Filter Pills -->
              <div class="format-filter-group">
                <button
                  v-for="st in serviceTypeOptions"
                  :key="st.value"
                  @click="selectedServiceType = st.value"
                  :class="['format-pill-btn', selectedServiceType === st.value && 'active']"
                >
                  {{ st.label }}
                </button>
              </div>

              <!-- 搜索框 -->
              <div class="main-search-wrapper !min-w-[220px]">
                <Search class="main-search-icon" />
                <input
                  v-model="searchQuery"
                  type="text"
                  placeholder="搜索 Request ID / 模型..."
                  class="main-search-input"
                />
              </div>
            </div>
          </div>

          <!-- 表格主体 (100% 对齐 KBView .doc-table-card & .doc-table) -->
          <div class="doc-table-card">
            <table class="doc-table">
              <thead>
                <tr>
                  <th style="width: 18%;">Request ID</th>
                  <th style="width: 14%;">计费服务类型</th>
                  <th style="width: 18%;">调用模型</th>
                  <th style="width: 12%; text-align: right;">Prompt Tokens</th>
                  <th style="width: 12%; text-align: right;">Completion / Doc</th>
                  <th style="width: 12%; text-align: right;">扣除金额</th>
                  <th style="width: 8%; text-align: center;">状态</th>
                  <th style="width: 16%; text-align: center;">调用时间</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="log in filteredLogs"
                  :key="log.id || log.request_id"
                  class="doc-row group"
                >
                  <!-- Request ID -->
                  <td class="font-mono text-xs text-gray-400">
                    <div class="flex items-center gap-1.5 max-w-[150px]">
                      <span class="truncate" :title="log.request_id">{{ log.request_id || '-' }}</span>
                      <button
                        v-if="log.request_id"
                        @click="copyToClipboard(log.request_id)"
                        class="opacity-0 group-hover:opacity-100 text-gray-500 hover:text-cyan-300 transition-all p-1 cursor-pointer"
                        title="复制 Request ID"
                      >
                        <Copy class="w-3 h-3" />
                      </button>
                    </div>
                  </td>

                  <!-- 计费服务类型 -->
                  <td>
                    <span
                      class="px-2.5 py-0.5 rounded-full text-[10px] font-semibold inline-flex items-center gap-1.5"
                      :class="getServiceTypeBadgeClass(log.service_type)"
                    >
                      <span class="w-1.5 h-1.5 rounded-full bg-current"></span>
                      {{ getServiceTypeLabel(log.service_type) }}
                    </span>
                  </td>

                  <!-- 调用模型 -->
                  <td class="font-mono text-xs">
                    <span class="text-gray-200 font-semibold">{{ log.model_name || '-' }}</span>
                    <span v-if="log.provider" class="ml-1 text-[10px] text-gray-500">({{ log.provider }})</span>
                  </td>

                  <!-- Prompt Tokens -->
                  <td class="text-right font-mono text-xs text-gray-300">
                    {{ formatNumber(log.prompt_tokens) }}
                  </td>

                  <!-- Completion / Doc -->
                  <td class="text-right font-mono text-xs text-gray-300">
                    <span v-if="log.service_type === 'rerank'">
                      {{ formatNumber(log.doc_count) }} <span class="text-[10px] text-gray-500">docs</span>
                    </span>
                    <span v-else>
                      {{ formatNumber(log.completion_tokens) }}
                    </span>
                  </td>

                  <!-- 扣除金额 -->
                  <td class="text-right font-mono text-xs font-bold text-emerald-400">
                    -￥ {{ log.total_cost.toFixed(6) }}
                  </td>

                  <!-- 状态 -->
                  <td class="text-center">
                    <span
                      v-if="log.status === 1"
                      class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"
                    >
                      <CheckCircle2 class="w-3 h-3" /> 已扣费
                    </span>
                    <span
                      v-else-if="log.status === 2"
                      class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20"
                    >
                      <AlertCircle class="w-3 h-3" /> 已退款
                    </span>
                    <span v-else class="text-xs text-gray-500">-</span>
                  </td>

                  <!-- 调用时间 -->
                  <td class="text-center text-xs text-gray-400 font-mono">
                    {{ formatDate(log.created_at) }}
                  </td>
                </tr>

                <!-- 空状态 -->
                <tr v-if="filteredLogs.length === 0">
                  <td colspan="8" style="padding: 4rem 1rem; text-align: center;">
                    <div class="w-12 h-12 mx-auto rounded-2xl bg-white/5 border border-white/10 flex items-center justify-center text-gray-500 mb-3">
                      <FileText class="w-6 h-6" />
                    </div>
                    <p class="text-xs text-gray-400 font-medium">暂无相符的消耗流水明细记录</p>
                    <p class="text-[11px] text-gray-500 mt-1">可以在 Chat 或知识库检索功能中发起请求触发计费</p>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- 分页控制栏 -->
          <div class="flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-gray-400 px-1 pt-1">
            <div>
              共计 <span class="font-mono text-gray-200 font-bold">{{ logsTotalCount }}</span> 条消耗流水记录，当前显示第
              <span class="font-mono text-gray-200 font-bold">{{ currentPage }}</span> 页
            </div>

            <div class="flex items-center gap-3">
              <div class="flex items-center gap-1.5">
                <button
                  @click="changePage(currentPage - 1)"
                  :disabled="currentPage <= 1 || isLoading"
                  class="p-1.5 rounded-lg bg-white/5 border border-white/10 hover:bg-white/10 disabled:opacity-40 disabled:cursor-not-allowed transition-all cursor-pointer text-gray-300"
                >
                  <ChevronLeft class="w-4 h-4" />
                </button>
                <span class="px-3 py-1 font-mono text-gray-300 bg-white/5 rounded-lg border border-white/10">
                  {{ currentPage }} / {{ totalPages }}
                </span>
                <button
                  @click="changePage(currentPage + 1)"
                  :disabled="currentPage >= totalPages || isLoading"
                  class="p-1.5 rounded-lg bg-white/5 border border-white/10 hover:bg-white/10 disabled:opacity-40 disabled:cursor-not-allowed transition-all cursor-pointer text-gray-300"
                >
                  <ChevronRight class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </section>
      </div>
    </main>

    <!-- 3. 测试充值 Modal (对齐 KBView 弹窗样式) -->
    <Teleport to="body">
      <div
        v-if="showRechargeModal"
        class="fixed inset-0 bg-black/75 backdrop-blur-2xl flex items-center justify-center p-4 z-50 animate-in fade-in duration-200"
      >
        <div class="bg-[#090d16]/95 border border-cyan-500/30 rounded-3xl w-full max-w-md p-6 sm:p-7 shadow-[0_0_80px_rgba(6,182,212,0.18)] relative overflow-hidden backdrop-blur-3xl animate-in zoom-in-95 duration-200">
          <div class="absolute -top-24 -right-24 w-60 h-60 bg-cyan-500/15 rounded-full blur-3xl pointer-events-none"></div>

          <div class="flex items-center justify-between border-b border-white/10 pb-4 mb-5">
            <h3 class="text-base font-bold text-gray-100 flex items-center gap-2">
              <Wallet class="w-5 h-5 text-cyan-400" />
              快捷测试额度充值
            </h3>
            <button @click="showRechargeModal = false" class="w-8 h-8 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 text-gray-400 hover:text-white transition-all flex items-center justify-center cursor-pointer">
              <X class="w-4 h-4" />
            </button>
          </div>

          <!-- 预设快捷选项 -->
          <div class="mb-5">
            <label class="block text-xs font-semibold text-gray-300 mb-2">选择快捷金额</label>
            <div class="grid grid-cols-4 gap-2">
              <button
                v-for="amt in [10, 50, 100, 500]"
                :key="amt"
                @click="rechargeAmount = amt"
                :class="[
                  'py-2 rounded-xl text-xs font-mono font-bold border transition-all cursor-pointer',
                  rechargeAmount === amt
                    ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40 shadow-md shadow-cyan-500/10'
                    : 'bg-white/5 text-gray-300 border-white/10 hover:bg-white/10'
                ]"
              >
                +￥{{ amt }}
              </button>
            </div>
          </div>

          <!-- 自定义输入 -->
          <div class="mb-6">
            <label class="block text-xs font-semibold text-gray-300 mb-1.5">自定义充值金额 (元/积分)</label>
            <div class="relative">
              <span class="absolute left-3.5 top-2.5 text-gray-400 font-mono text-sm">￥</span>
              <input
                v-model.number="rechargeAmount"
                type="number"
                min="1"
                step="10"
                class="main-search-input !pl-8 text-base font-mono"
                placeholder="输入充值额度"
              />
            </div>
            <div class="mt-2 text-[11px] text-cyan-300 flex justify-between font-mono">
              <span>当前余额: ￥{{ balanceData.balance.toFixed(2) }}</span>
              <span>充值后预计: ￥{{ (balanceData.balance + (rechargeAmount || 0)).toFixed(2) }}</span>
            </div>
          </div>

          <!-- Modal 按钮 -->
          <div class="flex justify-end gap-3 pt-3 border-t border-white/10">
            <button
              @click="showRechargeModal = false"
              class="px-4 py-2 rounded-xl border border-white/10 text-gray-300 hover:bg-white/10 text-xs font-medium transition-all cursor-pointer"
            >
              取消
            </button>
            <button
              @click="handleRecharge"
              :disabled="isRecharging || rechargeAmount <= 0"
              class="px-5 py-2 rounded-xl bg-gradient-to-r from-cyan-500 via-blue-600 to-indigo-600 hover:from-cyan-400 hover:to-indigo-500 text-white font-semibold text-xs transition-all shadow-lg shadow-cyan-500/25 disabled:opacity-50 flex items-center gap-2 cursor-pointer"
            >
              <RefreshCw v-if="isRecharging" class="w-3.5 h-3.5 animate-spin" />
              <span>确认充值</span>
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Toast Notification -->
    <Teleport to="body">
      <div
        v-if="toastMsg"
        class="fixed bottom-6 right-6 z-50 px-4 py-3 rounded-2xl bg-[#090d16]/95 border border-cyan-500/40 text-gray-100 text-xs font-medium shadow-2xl flex items-center gap-2 backdrop-blur-xl"
      >
        <Sparkles class="w-4 h-4 text-cyan-400" />
        {{ toastMsg }}
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import {
  Wallet,
  TrendingUp,
  Sparkles,
  ShieldCheck,
  RefreshCw,
  Plus,
  Search,
  Copy,
  Calculator,
  ChevronLeft,
  ChevronRight,
  Layers,
  CheckCircle2,
  AlertCircle,
  FileText,
  X,
} from 'lucide-vue-next';
import { billingApi, type UserBalance, type BillingLog, type ModelPricing } from '../api/billing';

// 基础状态
const isLoading = ref(false);
const isRecharging = ref(false);
const showRechargeModal = ref(false);
const rechargeAmount = ref<number>(50);
const toastMsg = ref('');

const balanceData = ref<UserBalance>({
  user_id: '',
  balance: 0,
  gift_balance: 0,
  total_consumed: 0,
});

const pricingMap = ref<Record<string, ModelPricing>>({});
const logsList = ref<BillingLog[]>([]);
const logsTotalCount = ref<number>(0);
const currentPage = ref<number>(1);
const pageSize = ref<number>(15);

// 筛选与计算器状态
const selectedServiceType = ref<string>('all');
const searchQuery = ref<string>('');
const activePricingTab = ref<'cards' | 'calculator'>('cards');

const calcModel = ref<string>('gpt-4o');
const calcInputTokens = ref<number>(1500);
const calcOutputTokens = ref<number>(800);

const serviceTypeOptions = [
  { label: '全部服务', value: 'all' },
  { label: 'OpenAI (LLM)', value: 'openai' },
  { label: 'Embedding (向量)', value: 'embedding' },
  { label: 'Rerank (重排)', value: 'rerank' },
];

// 预设默认模型价格（避免数据为空时展示过于空旷）
const defaultPricing: Record<string, ModelPricing> = {
  'gpt-4o': { input_unit_price: 0.002, output_unit_price: 0.006, unit_size: 1000 },
  'gpt-3.5-turbo': { input_unit_price: 0.001, output_unit_price: 0.002, unit_size: 1000 },
  'text-embedding-3-small': { input_unit_price: 0.0001, output_unit_price: 0, unit_size: 1000 },
  'bge-reranker-large': { input_unit_price: 0.0005, output_unit_price: 0, unit_size: 1000 },
};

// 合并后端获取的模型单价 map
const combinedPricingMap = computed(() => {
  return { ...defaultPricing, ...pricingMap.value };
});

// 计算器 Units & 扣费预估
const calcInputUnits = computed(() => {
  const tokens = calcInputTokens.value || 0;
  if (tokens <= 0) return 0;
  return Math.ceil(tokens / 1000);
});

const calcOutputUnits = computed(() => {
  const tokens = calcOutputTokens.value || 0;
  if (tokens <= 0) return 0;
  return Math.ceil(tokens / 1000);
});

const calculatedCost = computed(() => {
  const modelInfo = combinedPricingMap.value[calcModel.value];
  if (!modelInfo) return 0;
  const inputCost = calcInputUnits.value * (modelInfo.input_unit_price || 0);
  const outputCost = calcOutputUnits.value * (modelInfo.output_unit_price || 0);
  return inputCost + outputCost;
});

// 筛选后的日志列表
const filteredLogs = computed(() => {
  return logsList.value.filter((log) => {
    // 1. 服务类型筛选
    if (selectedServiceType.value !== 'all' && log.service_type !== selectedServiceType.value) {
      return false;
    }
    // 2. 搜索框筛选
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.trim().toLowerCase();
      const reqId = (log.request_id || '').toLowerCase();
      const model = (log.model_name || '').toLowerCase();
      const provider = (log.provider || '').toLowerCase();
      return reqId.includes(q) || model.includes(q) || provider.includes(q);
    }
    return true;
  });
});

// 总页数
const totalPages = computed(() => {
  return Math.max(1, Math.ceil((logsTotalCount.value || filteredLogs.value.length) / pageSize.value));
});

// 格式化函数
const formatMoney = (val: number) => {
  return Number(val || 0).toFixed(4);
};

const formatNumber = (val: number) => {
  return Number(val || 0).toLocaleString();
};

const formatDate = (val: number | string) => {
  if (!val) return '-';
  let date: Date;
  if (typeof val === 'number') {
    date = new Date(val);
  } else {
    const num = Number(val);
    date = !isNaN(num) && num > 1000000000 ? new Date(num) : new Date(val);
  }
  if (isNaN(date.getTime())) return String(val);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
};

const showToast = (msg: string) => {
  toastMsg.value = msg;
  setTimeout(() => {
    toastMsg.value = '';
  }, 2500);
};

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text);
    showToast(`已成功复制 Request ID: ${text.slice(0, 8)}...`);
  } catch (err) {
    showToast('复制失败，请手动选择复制');
  }
};

// 分类样式 Helper
const getModelCategoryName = (modelName: string) => {
  const m = modelName.toLowerCase();
  if (m.includes('embed')) return 'Embedding';
  if (m.includes('rerank')) return 'Rerank';
  return 'LLM 模型';
};

const getModelCategoryClass = (modelName: string) => {
  const m = modelName.toLowerCase();
  if (m.includes('embed')) return 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30';
  if (m.includes('rerank')) return 'bg-purple-500/20 text-purple-300 border border-purple-500/30';
  return 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30';
};

const getServiceTypeBadgeClass = (type: string) => {
  switch (type) {
    case 'openai':
      return 'bg-blue-500/10 text-blue-400 border border-blue-500/20';
    case 'embedding':
      return 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20';
    case 'rerank':
      return 'bg-purple-500/10 text-purple-400 border border-purple-500/20';
    default:
      return 'bg-white/5 text-gray-300 border border-white/10';
  }
};

const getServiceTypeLabel = (type: string) => {
  switch (type) {
    case 'openai':
      return 'OpenAI';
    case 'embedding':
      return 'Embedding';
    case 'rerank':
      return 'Rerank';
    default:
      return type || '默认';
  }
};

// 数据加载与交互
const fetchData = async () => {
  isLoading.value = true;
  try {
    const [bal, prices, logsRes] = await Promise.allSettled([
      billingApi.getBalance(),
      billingApi.getPricing(),
      billingApi.listLogs(currentPage.value, pageSize.value),
    ]);

    if (bal.status === 'fulfilled' && bal.value) {
      balanceData.value = bal.value;
    }
    if (prices.status === 'fulfilled' && prices.value) {
      pricingMap.value = prices.value;
    }
    if (logsRes.status === 'fulfilled' && logsRes.value) {
      logsList.value = logsRes.value.logs || [];
      logsTotalCount.value = logsRes.value.total || logsList.value.length;
    }
    showToast('计费数据已成功同步刷新');
  } catch (err) {
    console.error('Fetch billing data error:', err);
    showToast('同步数据出现异常，已加载本地快照');
  } finally {
    isLoading.value = false;
  }
};

const changePage = (page: number) => {
  if (page < 1 || page > totalPages.value) return;
  currentPage.value = page;
  fetchData();
};

const handleRecharge = async () => {
  if (rechargeAmount.value <= 0) return;
  isRecharging.value = true;
  try {
    const updatedBal = await billingApi.recharge(rechargeAmount.value);
    if (updatedBal) {
      balanceData.value = updatedBal;
    }
    showToast(`成功充值 ￥${rechargeAmount.value} 体验额度！`);
    showRechargeModal.value = false;
    await fetchData();
  } catch (err) {
    showToast('充值操作失败: ' + (err as Error).message);
  } finally {
    isRecharging.value = false;
  }
};

onMounted(() => {
  fetchData();
});
</script>

<style scoped>
/* 完全复用 KBView.vue 的基础与模块样式 */
.billing-workspace-root {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: #060812;
  color: #f1f5f9;
  overflow: hidden;
  font-family: 'Inter', system-ui, -apple-system, BlinkMacSystemFont, sans-serif;
  user-select: none;
}

/* Header 样式 */
.billing-header {
  height: 64px;
  min-height: 64px;
  background: rgba(11, 15, 27, 0.85);
  backdrop-filter: blur(16px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding: 0 1.5rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  z-index: 20;
  flex-shrink: 0;
}

.breadcrumb-box {
  display: flex;
  align-items: center;
  gap: 0.875rem;
}

.brand-icon-box {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: linear-gradient(135deg, rgba(6, 182, 212, 0.2), rgba(99, 102, 241, 0.2));
  border: 1px solid rgba(6, 182, 212, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #22d3ee;
  box-shadow: 0 0 12px rgba(6, 182, 212, 0.15);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

/* 主体与右侧面板 */
.billing-main-content {
  flex: 1;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: #0b0e1a;
}

.billing-content-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 1.75rem 2rem;
  display: flex;
  flex-direction: column;
  gap: 1.75rem;
}

/* KPI 卡片网格 (对齐 KBView) */
.kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1.25rem;
}

@media (max-width: 1280px) {
  .kpi-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.kpi-card {
  padding: 1.25rem 1.5rem;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
  transition: all 0.2s ease;
}

.kpi-card:hover {
  border-color: rgba(6, 182, 212, 0.3);
  background: rgba(255, 255, 255, 0.05);
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
}

.kpi-icon-wrapper {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* 区块通用面板 */
.card-panel {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  padding: 1.25rem 1.5rem;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
}

/* 模型单价表微卡片 */
.model-card {
  padding: 1.125rem;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.2s ease;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 0.75rem;
}

.model-card:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(6, 182, 212, 0.3);
  transform: translateY(-1px);
}

/* Filter Toolbar Card (对齐 KBView) */
.filter-toolbar-card {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  padding: 1rem 1.25rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 1.25rem;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
}

.main-search-wrapper {
  position: relative;
  flex: 1;
  min-width: 240px;
}

.main-search-icon {
  position: absolute;
  left: 1rem;
  top: 50%;
  transform: translateY(-50%);
  color: #64748b;
  width: 16px;
  height: 16px;
  pointer-events: none;
}

.main-search-input {
  width: 100%;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 0.625rem 1rem 0.625rem 2.75rem;
  font-size: 0.8125rem;
  color: #f1f5f9;
  outline: none;
  transition: all 0.2s ease;
}

.main-search-input:focus {
  border-color: #06b6d4;
  box-shadow: 0 0 0 3px rgba(6, 182, 212, 0.2);
  background: rgba(255, 255, 255, 0.08);
}

.format-filter-group {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  overflow-x: auto;
  padding: 2px 0;
}

.format-pill-btn {
  padding: 0.4rem 0.875rem;
  border-radius: 10px;
  font-size: 0.75rem;
  font-weight: 600;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.04);
  color: #94a3b8;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.format-pill-btn:hover {
  color: #f1f5f9;
  background: rgba(255, 255, 255, 0.08);
}

.format-pill-btn.active {
  background: rgba(6, 182, 212, 0.2);
  color: #22d3ee;
  border-color: rgba(6, 182, 212, 0.4);
  box-shadow: 0 2px 8px rgba(6, 182, 212, 0.15);
}

.status-select {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 0.625rem 1rem;
  font-size: 0.75rem;
  color: #cbd5e1;
  outline: none;
  cursor: pointer;
  transition: all 0.2s ease;
}

/* Document Table Card (对齐 KBView) */
.doc-table-card {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
}

.doc-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 0.8125rem;
}

.doc-table th {
  background: rgba(255, 255, 255, 0.04);
  padding: 1.125rem 1.25rem;
  color: #94a3b8;
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.doc-table td {
  padding: 1.25rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  color: #cbd5e1;
  vertical-align: middle;
}

.doc-table tr:last-child td {
  border-bottom: none;
}

.doc-table tr.doc-row {
  transition: all 0.15s ease;
  cursor: pointer;
}

.doc-table tr.doc-row:hover {
  background: rgba(255, 255, 255, 0.05);
}
</style>
