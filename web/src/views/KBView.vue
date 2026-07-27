<template>
  <div class="h-full flex flex-col bg-[#060810] text-gray-100 overflow-hidden font-sans select-none">
    <!-- 1. 顶部 Header 导航栏 (Breadcrumb & 全局 Action) -->
    <header class="h-16 border-b border-white/10 bg-[#080c16]/80 backdrop-blur-xl px-6 flex items-center justify-between shrink-0 z-20">
      <!-- 面包屑与选定 KB 标识 -->
      <div class="flex items-center gap-3">
        <div class="p-2 rounded-xl bg-gradient-to-tr from-cyan-500/20 via-blue-600/20 to-indigo-600/20 text-cyan-400 border border-cyan-500/30 shadow-lg shadow-cyan-500/10">
          <Database class="w-5 h-5" />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-sm font-semibold text-gray-400">知识库工作台</span>
          <ChevronRight class="w-4 h-4 text-gray-600" />
          <h1 class="text-base font-bold tracking-wide bg-gradient-to-r from-white via-gray-100 to-gray-300 bg-clip-text text-transparent flex items-center gap-2">
            {{ activeKBName }}
          </h1>
          <span
            :class="[
              'text-[10px] font-mono px-2 py-0.5 rounded-full border font-semibold ml-1',
              kbStore.activeKB?.is_default
                ? 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30'
                : 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30'
            ]"
          >
            {{ kbStore.activeKB?.is_default ? 'SYSTEM PUBLIC' : 'CUSTOM BASE' }}
          </span>
        </div>
      </div>

      <!-- 右侧按钮组 -->
      <div class="flex items-center gap-3">
        <button
          @click="kbStore.fetchKBs()"
          class="p-2 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 text-gray-300 transition-all flex items-center gap-1.5 text-xs cursor-pointer active:scale-95"
          title="刷新全量知识库与文档"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': kbStore.loading || kbStore.docLoading }" />
          <span>刷新</span>
        </button>

        <button
          @click="showUploadModal = true"
          class="px-3.5 py-2 rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-300 hover:bg-cyan-500/20 text-xs font-semibold transition-all flex items-center gap-1.5 cursor-pointer shadow-sm shadow-cyan-500/10"
        >
          <UploadCloud class="w-4 h-4 text-cyan-400" />
          <span>上传增量文档</span>
        </button>

        <button
          @click="showCreateModal = true"
          class="px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 via-blue-600 to-indigo-600 hover:from-cyan-400 hover:to-indigo-500 text-white font-semibold text-xs transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 cursor-pointer active:scale-95"
        >
          <Plus class="w-4 h-4" />
          <span>新建知识库</span>
        </button>
      </div>
    </header>

    <!-- 2. 主体左右 2 栏工作区 -->
    <div class="flex-1 flex overflow-hidden">
      <!-- 2.1 左侧知识库导航侧边栏 (280px) -->
      <aside class="w-72 border-r border-white/10 bg-[#070a13] flex flex-col shrink-0 overflow-hidden">
        <!-- 搜索与卡片标头 -->
        <div class="p-3.5 border-b border-white/10 space-y-2.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-gray-400 uppercase tracking-wider flex items-center gap-1.5">
              <Layers class="w-3.5 h-3.5 text-cyan-400" />
              知识库空间 ({{ kbStore.kbs.length }})
            </span>
          </div>

          <div class="relative">
            <Search class="w-3.5 h-3.5 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              v-model="kbSearchQuery"
              type="text"
              placeholder="搜索知识库..."
              class="w-full bg-white/5 border border-white/10 rounded-xl pl-8 pr-3 py-1.5 text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 transition-colors"
            />
          </div>
        </div>

        <!-- 知识库卡片列表 -->
        <div class="flex-1 overflow-y-auto p-3 space-y-3">
          <!-- 默认系统公共库 -->
          <div v-if="kbStore.defaultKB && matchesKBSearch(kbStore.defaultKB)">
            <div class="text-[11px] font-semibold text-cyan-400 mb-1.5 px-2 flex items-center gap-1.5">
              <ShieldCheck class="w-3.5 h-3.5" />
              系统公共向量库
            </div>
            <div
              @click="kbStore.selectKB(kbStore.defaultKB.kb_id)"
              :class="[
                'p-3.5 rounded-xl border transition-all cursor-pointer relative overflow-hidden group',
                kbStore.activeKbId === kbStore.defaultKB.kb_id
                  ? 'bg-gradient-to-r from-cyan-500/20 via-blue-600/15 to-transparent border-cyan-500/60 shadow-lg shadow-cyan-500/10'
                  : 'bg-white/[0.03] border-white/10 hover:border-white/20 hover:bg-white/[0.06]'
              ]"
            >
              <div class="flex items-start justify-between">
                <div class="flex-1 min-w-0 pr-2">
                  <div class="flex items-center gap-2">
                    <h3 class="font-bold text-xs text-gray-100 truncate">{{ kbStore.defaultKB.name }}</h3>
                  </div>
                  <p class="text-[11px] text-gray-400 mt-1 line-clamp-2">{{ kbStore.defaultKB.description || '系统公共默认向量知识库' }}</p>
                </div>
                <BookOpen class="w-4 h-4 text-cyan-400 shrink-0 mt-0.5" />
              </div>

              <div class="mt-2.5 pt-2 border-t border-white/5 flex items-center justify-between text-[10px] text-gray-500 font-mono">
                <span>ID: {{ kbStore.defaultKB.kb_id.slice(0, 14) }}...</span>
                <span class="text-cyan-400 bg-cyan-500/10 px-1.5 py-0.5 rounded border border-cyan-500/20">SYSTEM</span>
              </div>
            </div>
          </div>

          <!-- 自定义知识库列表 -->
          <div>
            <div class="text-[11px] font-semibold text-gray-400 mb-1.5 px-2 flex items-center justify-between">
              <span>自定义向量库 ({{ customKBsFiltered.length }})</span>
            </div>

            <div v-if="customKBsFiltered.length === 0" class="p-4 text-center border border-dashed border-white/10 rounded-xl bg-white/[0.01]">
              <FolderOpen class="w-6 h-6 mx-auto text-gray-600 mb-1" />
              <p class="text-[11px] text-gray-500">无自定义知识库</p>
            </div>

            <div v-else class="space-y-2">
              <div
                v-for="kb in customKBsFiltered"
                :key="kb.kb_id"
                @click="kbStore.selectKB(kb.kb_id)"
                :class="[
                  'p-3.5 rounded-xl border transition-all cursor-pointer group relative overflow-hidden',
                  kbStore.activeKbId === kb.kb_id
                    ? 'bg-gradient-to-r from-cyan-500/20 via-blue-600/15 to-transparent border-cyan-500/60 shadow-lg shadow-cyan-500/10'
                    : 'bg-white/[0.03] border-white/10 hover:border-white/20 hover:bg-white/[0.06]'
                ]"
              >
                <div class="flex items-start justify-between">
                  <div class="flex-1 min-w-0 pr-2">
                    <h3 class="font-bold text-xs text-gray-100 truncate">{{ kb.name }}</h3>
                    <p class="text-[11px] text-gray-400 mt-1 line-clamp-1">{{ kb.description || '无描述' }}</p>
                  </div>

                  <button
                    @click.stop="promptDeleteKB(kb.kb_id, kb.name)"
                    class="opacity-0 group-hover:opacity-100 p-1.5 hover:bg-rose-500/20 text-gray-400 hover:text-rose-400 rounded-lg transition-all cursor-pointer shrink-0"
                    title="删除知识库"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>

                <div class="mt-2.5 pt-2 border-t border-white/5 flex items-center justify-between text-[10px] text-gray-500 font-mono">
                  <span>ID: {{ kb.kb_id.slice(0, 14) }}...</span>
                  <span class="text-indigo-300 bg-indigo-500/10 px-1.5 py-0.5 rounded border border-indigo-500/20">CUSTOM</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 底部系统引擎状态底栏 -->
        <div class="p-3 border-t border-white/10 bg-white/[0.01] flex items-center justify-between text-[11px] text-gray-400">
          <div class="flex items-center gap-1.5">
            <Cpu class="w-3.5 h-3.5 text-cyan-400" />
            <span>Milvus 2.3+ Dense</span>
          </div>
          <span class="text-emerald-400 font-mono flex items-center gap-1">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-ping"></span>
            1536D
          </span>
        </div>
      </aside>

      <!-- 2.2 右侧知识库工作台与文档大盘 -->
      <main class="flex-1 flex flex-col overflow-hidden bg-[#080c16]">
        <!-- 2.2.1 4 KPI 指标统计栏 -->
        <div class="p-5 border-b border-white/10 bg-white/[0.015] grid grid-cols-1 md:grid-cols-4 gap-4 shrink-0">
          <!-- KPI 1: 文档数 -->
          <div class="p-4 rounded-2xl bg-white/[0.03] border border-white/10 flex items-center justify-between relative overflow-hidden group hover:border-cyan-500/30 transition-all">
            <div>
              <div class="text-xs text-gray-400 font-medium">包含文档总数</div>
              <div class="text-xl font-bold font-mono text-gray-100 mt-1">{{ kbStore.documents.length }} <span class="text-xs font-normal text-gray-400">篇</span></div>
            </div>
            <div class="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
              <FileText class="w-5 h-5" />
            </div>
          </div>

          <!-- KPI 2: 向量切片数 -->
          <div class="p-4 rounded-2xl bg-white/[0.03] border border-white/10 flex items-center justify-between relative overflow-hidden group hover:border-emerald-500/30 transition-all">
            <div>
              <div class="text-xs text-gray-400 font-medium">向量切片数 (Chunks)</div>
              <div class="text-xl font-bold font-mono text-emerald-400 mt-1">{{ totalChunksCount }} <span class="text-xs font-normal text-gray-400">chunks</span></div>
            </div>
            <div class="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <Sparkles class="w-5 h-5" />
            </div>
          </div>

          <!-- KPI 3: 分块策略 -->
          <div class="p-4 rounded-2xl bg-white/[0.03] border border-white/10 flex items-center justify-between relative overflow-hidden group hover:border-indigo-500/30 transition-all">
            <div>
              <div class="text-xs text-gray-400 font-medium">切片向量策略</div>
              <div class="text-xs font-semibold font-mono text-indigo-300 mt-1.5">Parent-Child 两阶段</div>
              <div class="text-[10px] text-gray-500">512B Child / 2KB Parent</div>
            </div>
            <div class="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
              <Layers class="w-5 h-5" />
            </div>
          </div>

          <!-- KPI 4: 当前 KB 空间 ID -->
          <div class="p-4 rounded-2xl bg-white/[0.03] border border-white/10 flex items-center justify-between relative overflow-hidden group hover:border-blue-500/30 transition-all">
            <div class="min-w-0 flex-1 pr-2">
              <div class="text-xs text-gray-400 font-medium">当前空间 UUID</div>
              <div class="text-xs font-mono text-cyan-300 mt-1.5 truncate" :title="kbStore.activeKbId">
                {{ kbStore.activeKbId }}
              </div>
              <div class="text-[10px] text-gray-500 truncate">{{ activeKBDesc }}</div>
            </div>
            <div class="w-10 h-10 rounded-xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-400 shrink-0">
              <HardDrive class="w-5 h-5" />
            </div>
          </div>
        </div>

        <!-- 2.2.2 消息 Alert 提示框 -->
        <div v-if="uploadSuccessMsg" class="mx-6 mt-4 p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center justify-between shadow-lg">
          <div class="flex items-center gap-2">
            <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
            <span>{{ uploadSuccessMsg }}</span>
          </div>
          <button @click="uploadSuccessMsg = ''" class="text-emerald-400 hover:text-emerald-200 cursor-pointer text-sm font-bold">×</button>
        </div>

        <div v-if="kbStore.error" class="mx-6 mt-4 p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center justify-between shadow-lg">
          <div class="flex items-center gap-2">
            <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
            <span>{{ kbStore.error }}</span>
          </div>
          <button @click="kbStore.error = null" class="text-rose-400 hover:text-rose-200 cursor-pointer text-sm font-bold">×</button>
        </div>

        <!-- 2.2.3 文档管理主工作区域 -->
        <div class="flex-1 p-6 overflow-y-auto space-y-4">
          <!-- 搜索与筛选工具栏 -->
          <div class="flex flex-wrap items-center justify-between gap-4 bg-white/[0.02] p-4 rounded-2xl border border-white/10 shadow-lg">
            <!-- 搜索词 -->
            <div class="relative flex-1 min-w-[240px]">
              <Search class="w-4 h-4 text-gray-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <input
                v-model="searchQuery"
                type="text"
                placeholder="搜索文档名称、文件格式、Doc ID..."
                class="w-full bg-white/5 border border-white/10 rounded-xl pl-10 pr-4 py-2 text-xs text-gray-200 placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 transition-colors"
              />
            </div>

            <!-- 格式 Pill 筛选 -->
            <div class="flex items-center gap-1.5 overflow-x-auto py-0.5">
              <button
                v-for="fmt in formatFilterOptions"
                :key="fmt.value"
                @click="selectedFormat = fmt.value"
                :class="[
                  'px-3 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer shrink-0',
                  selectedFormat === fmt.value
                    ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-sm'
                    : 'bg-white/5 border border-white/10 text-gray-400 hover:text-gray-200 hover:bg-white/10'
                ]"
              >
                {{ fmt.label }}
              </button>
            </div>

            <!-- 状态筛选 -->
            <select
              v-model="statusFilter"
              class="bg-white/5 border border-white/10 rounded-xl px-3 py-2 text-xs text-gray-300 focus:outline-none focus:border-cyan-500/50 transition-colors cursor-pointer"
            >
              <option value="all" class="bg-[#121827] text-gray-200">全部解析状态</option>
              <option value="2" class="bg-[#121827] text-emerald-300">已向量化 (Status 2)</option>
              <option value="1" class="bg-[#121827] text-blue-300">解析向量化中 (Status 1)</option>
              <option value="3" class="bg-[#121827] text-rose-300">解析失败 (Status 3)</option>
            </select>
          </div>

          <!-- 文档展示列表表格 (Modern Data Table) -->
          <div class="bg-white/[0.02] border border-white/10 rounded-2xl overflow-hidden shadow-2xl">
            <div class="overflow-x-auto">
              <table class="w-full text-left text-xs text-gray-300">
                <thead class="bg-white/[0.04] text-gray-400 uppercase tracking-wider font-semibold border-b border-white/10 text-[11px]">
                  <tr>
                    <th class="py-3.5 px-5">文档名称与类型</th>
                    <th class="py-3.5 px-4">解析状态</th>
                    <th class="py-3.5 px-4">切片数 (Chunks)</th>
                    <th class="py-3.5 px-4">SHA256 哈希</th>
                    <th class="py-3.5 px-4">入库时间</th>
                    <th class="py-3.5 px-5 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-white/5">
                  <!-- 加载状态 -->
                  <tr v-if="kbStore.docLoading">
                    <td colspan="6" class="py-16 text-center text-gray-400">
                      <RefreshCw class="w-7 h-7 animate-spin mx-auto text-cyan-400 mb-2" />
                      <p class="text-xs">加载文档数据中...</p>
                    </td>
                  </tr>

                  <!-- 空状态 -->
                  <tr v-else-if="filteredDocuments.length === 0">
                    <td colspan="6" class="py-16 text-center">
                      <div class="w-12 h-12 mx-auto rounded-2xl bg-white/5 border border-white/10 flex items-center justify-center text-gray-500 mb-3">
                        <FileCode class="w-6 h-6" />
                      </div>
                      <p class="text-xs text-gray-400 font-medium">当前知识库下无匹配文档</p>
                      <button
                        @click="showUploadModal = true"
                        class="mt-3 text-xs text-cyan-400 hover:text-cyan-300 underline underline-offset-4 cursor-pointer font-medium"
                      >
                        + 立即上传第一份文档
                      </button>
                    </td>
                  </tr>

                  <!-- 文档列表行 -->
                  <tr
                    v-for="doc in filteredDocuments"
                    :key="doc.doc_id"
                    class="hover:bg-white/[0.04] transition-colors group cursor-pointer"
                    @click="openDocDetail(doc)"
                  >
                    <!-- 名称与格式 -->
                    <td class="py-4 px-5">
                      <div class="flex items-center gap-3">
                        <div
                          :class="[
                            'w-9 h-9 rounded-xl border flex items-center justify-center text-xs font-mono font-bold uppercase shrink-0 shadow-sm',
                            getFileFormatBadgeClass(doc.source_type)
                          ]"
                        >
                          {{ doc.source_type }}
                        </div>
                        <div class="min-w-0">
                          <div class="font-bold text-gray-100 group-hover:text-cyan-300 transition-colors truncate max-w-sm" :title="doc.title">
                            {{ doc.title }}
                          </div>
                          <div class="text-[10px] text-gray-500 font-mono truncate max-w-xs mt-0.5">
                            DocID: {{ doc.doc_id }}
                          </div>
                        </div>
                      </div>
                    </td>

                    <!-- 解析状态 -->
                    <td class="py-4 px-4">
                      <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border text-[11px] font-semibold" :class="getStatusStyle(doc.status).bg">
                        <span
                          v-if="getStatusStyle(doc.status).dot"
                          class="w-1.5 h-1.5 rounded-full"
                          :class="getStatusStyle(doc.status).dot"
                        ></span>
                        <RefreshCw
                          v-if="getStatusStyle(doc.status).spin"
                          class="w-3 h-3 animate-spin text-blue-400"
                        />
                        <span>{{ getStatusStyle(doc.status).text }}</span>
                      </div>
                      <div v-if="doc.err_msg" class="text-[10px] text-rose-400 mt-1 line-clamp-1 max-w-[160px]" :title="doc.err_msg">
                        {{ doc.err_msg }}
                      </div>
                    </td>

                    <!-- 向量切片数 -->
                    <td class="py-4 px-4 font-mono">
                      <span class="px-2.5 py-1 rounded-lg bg-cyan-500/10 border border-cyan-500/20 text-cyan-300 text-xs font-bold">
                        {{ doc.total_chunks }} chunks
                      </span>
                    </td>

                    <!-- SHA256 哈希 -->
                    <td class="py-4 px-4 font-mono text-[11px] text-gray-400">
                      <span v-if="doc.file_hash" class="bg-white/5 px-2 py-1 rounded-md border border-white/10 text-gray-300" :title="doc.file_hash">
                        {{ doc.file_hash.substring(0, 12) }}...
                      </span>
                      <span v-else class="text-gray-600">-</span>
                    </td>

                    <!-- 入库时间 -->
                    <td class="py-4 px-4 text-xs text-gray-400 font-mono">
                      {{ formatDate(doc.created_at) }}
                    </td>

                    <!-- 操作栏 -->
                    <td class="py-4 px-5 text-right" @click.stop>
                      <div class="flex items-center justify-end gap-2">
                        <button
                          @click="openDocDetail(doc)"
                          class="p-1.5 hover:bg-white/10 text-gray-400 hover:text-cyan-300 rounded-lg transition-all cursor-pointer"
                          title="查看文档元数据详情"
                        >
                          <Eye class="w-4 h-4" />
                        </button>

                        <button
                          @click="promptDeleteDoc(doc.doc_id, doc.title)"
                          class="p-1.5 hover:bg-rose-500/20 text-gray-400 hover:text-rose-400 rounded-lg transition-all cursor-pointer"
                          title="删除文档与向量切片"
                        >
                          <Trash2 class="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </main>
    </div>

    <!-- 3. 上传增量文档 Modal 弹窗 -->
    <div v-if="showUploadModal" class="fixed inset-0 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 z-50">
      <div class="bg-[#111625] border border-white/15 rounded-2xl w-full max-w-lg p-6 shadow-2xl space-y-4 relative overflow-hidden">
        <div class="flex items-center justify-between border-b border-white/10 pb-3">
          <div class="flex items-center gap-2 text-gray-100 font-semibold text-base">
            <UploadCloud class="w-5 h-5 text-cyan-400" />
            <span>上传增量文档至 [{{ activeKBName }}]</span>
          </div>
          <button @click="showUploadModal = false" class="text-gray-400 hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- 拖拽上传区域 -->
        <div
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="handleDrop"
          :class="[
            'border-2 border-dashed rounded-2xl p-8 text-center transition-all cursor-pointer relative overflow-hidden group',
            isDragging
              ? 'border-cyan-400 bg-cyan-500/10 scale-[1.002]'
              : 'border-white/15 bg-white/[0.03] hover:border-cyan-500/50 hover:bg-white/[0.06]'
          ]"
          @click="triggerFileInput"
        >
          <input
            type="file"
            ref="fileInputRef"
            class="hidden"
            accept=".md,.txt,.pdf,.docx,.json,.csv,.tsv"
            @change="handleFileSelected"
          />

          <div class="max-w-md mx-auto pointer-events-none">
            <div class="w-14 h-14 mx-auto rounded-2xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-400 mb-3 shadow-inner group-hover:scale-105 transition-transform">
              <UploadCloud class="w-7 h-7" />
            </div>
            <h3 class="text-sm font-semibold text-gray-200 mb-1">
              点击选择或拖拽文件到此处
            </h3>
            <p class="text-xs text-gray-400 mb-3">
              支持格式：<span class="text-cyan-300 font-mono">.md, .txt, .pdf, .docx, .json, .csv</span>
            </p>
            <div class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-cyan-500/10 border border-cyan-500/30 text-[11px] text-cyan-300">
              <Sparkles class="w-3.5 h-3.5" />
              自动 SHA256 去重 · 两阶段 Parent-Child 向量分块
            </div>
          </div>

          <!-- Loader 遮罩 -->
          <div v-if="kbStore.uploading" class="absolute inset-0 bg-black/80 backdrop-blur-sm flex flex-col items-center justify-center gap-3 z-10">
            <RefreshCw class="w-8 h-8 text-cyan-400 animate-spin" />
            <p class="text-sm font-semibold text-cyan-300">正在解析文件并写入 Milvus 向量库...</p>
          </div>
        </div>

        <div class="flex justify-end pt-2">
          <button
            @click="showUploadModal = false"
            class="px-4 py-2 rounded-xl border border-white/10 text-gray-300 hover:bg-white/5 text-xs transition-colors cursor-pointer"
          >
            关闭
          </button>
        </div>
      </div>
    </div>

    <!-- 4. 新建自定义知识库 Modal 弹窗 -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 z-50">
      <div class="bg-[#111625] border border-white/15 rounded-2xl w-full max-w-md p-6 shadow-2xl space-y-4">
        <div class="flex items-center justify-between border-b border-white/10 pb-3">
          <h3 class="text-base font-bold text-gray-100 flex items-center gap-2">
            <Plus class="w-4 h-4 text-cyan-400" />
            新建自定义知识库
          </h3>
          <button @click="showCreateModal = false" class="text-gray-400 hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <form @submit.prevent="handleCreateKBSubmit" class="space-y-4">
          <div>
            <label class="block text-xs font-medium text-gray-300 mb-1">知识库名称 *</label>
            <input
              v-model="newKbName"
              type="text"
              required
              placeholder="例如：产品规格说明书 2026"
              class="w-full bg-white/5 border border-white/10 rounded-xl px-3.5 py-2.5 text-sm text-gray-100 placeholder-gray-500 focus:outline-none focus:border-cyan-500 transition-colors"
            />
          </div>

          <div>
            <label class="block text-xs font-medium text-gray-300 mb-1">知识库描述</label>
            <textarea
              v-model="newKbDesc"
              rows="3"
              placeholder="说明该知识库适用的业务场景及文档范围..."
              class="w-full bg-white/5 border border-white/10 rounded-xl px-3.5 py-2.5 text-sm text-gray-100 placeholder-gray-500 focus:outline-none focus:border-cyan-500 transition-colors"
            ></textarea>
          </div>

          <div class="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              @click="showCreateModal = false"
              class="px-4 py-2 rounded-xl border border-white/10 text-gray-300 hover:bg-white/5 text-xs transition-colors cursor-pointer"
            >
              取消
            </button>
            <button
              type="submit"
              :disabled="kbStore.loading"
              class="px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 to-indigo-600 hover:from-cyan-400 hover:to-indigo-500 text-white font-semibold text-xs transition-all shadow-lg shadow-cyan-500/20 cursor-pointer flex items-center gap-2"
            >
              <RefreshCw v-if="kbStore.loading" class="w-3.5 h-3.5 animate-spin" />
              确定创建
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- 5. 文档元数据详情 Drawer/Modal 弹窗 -->
    <div v-if="detailDocModal.show && detailDocModal.doc" class="fixed inset-0 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 z-50">
      <div class="bg-[#111625] border border-cyan-500/30 rounded-2xl w-full max-w-lg p-6 shadow-2xl space-y-4">
        <div class="flex items-center justify-between border-b border-white/10 pb-3">
          <div class="flex items-center gap-3">
            <div :class="['w-8 h-8 rounded-lg border flex items-center justify-center text-xs font-mono font-bold uppercase', getFileFormatBadgeClass(detailDocModal.doc.source_type)]">
              {{ detailDocModal.doc.source_type }}
            </div>
            <div>
              <h3 class="text-sm font-bold text-gray-100 max-w-xs truncate" :title="detailDocModal.doc.title">{{ detailDocModal.doc.title }}</h3>
              <p class="text-[10px] text-gray-400 font-mono">KnowledgeDocumentModel Details</p>
            </div>
          </div>
          <button @click="detailDocModal.show = false" class="text-gray-400 hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="space-y-2.5 text-xs text-gray-300 font-mono">
          <div class="bg-white/5 p-3 rounded-xl border border-white/10 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-gray-400">业务文档 UUID:</span>
              <span class="text-cyan-300 select-all">{{ detailDocModal.doc.doc_id }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-gray-400">关联 KB ID:</span>
              <span class="text-gray-200 select-all">{{ detailDocModal.doc.kb_id }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-gray-400">解析状态:</span>
              <span class="font-sans font-semibold" :class="getStatusStyle(detailDocModal.doc.status).bg">
                {{ getStatusStyle(detailDocModal.doc.status).text }}
              </span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-gray-400">向量切片总数:</span>
              <span class="text-emerald-400 font-bold">{{ detailDocModal.doc.total_chunks }} chunks</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-gray-400">SHA256 内容哈希:</span>
              <span class="text-gray-300 select-all truncate max-w-[240px]" :title="detailDocModal.doc.file_hash">{{ detailDocModal.doc.file_hash || '-' }}</span>
            </div>
          </div>

          <div class="bg-white/5 p-3 rounded-xl border border-white/10 space-y-2 text-[11px]">
            <div>
              <span class="text-gray-400 block mb-1">物理磁盘储存路径:</span>
              <span class="text-gray-300 bg-black/40 p-2 rounded block break-all font-mono">{{ detailDocModal.doc.file_path || '-' }}</span>
            </div>

            <div v-if="detailDocModal.doc.err_msg" class="pt-2 border-t border-white/5">
              <span class="text-rose-400 block mb-1">失败异常日志:</span>
              <span class="text-rose-300 bg-rose-500/10 p-2 rounded block break-all font-sans">{{ detailDocModal.doc.err_msg }}</span>
            </div>
          </div>

          <div class="flex items-center justify-between text-[10px] text-gray-500 pt-1">
            <span>创建时间: {{ formatDate(detailDocModal.doc.created_at) }}</span>
            <span>更新时间: {{ formatDate(detailDocModal.doc.updated_at) }}</span>
          </div>
        </div>

        <div class="flex justify-end pt-2">
          <button
            @click="detailDocModal.show = false"
            class="px-4 py-2 rounded-xl bg-white/10 hover:bg-white/15 text-gray-200 text-xs transition-colors cursor-pointer"
          >
            关 闭
          </button>
        </div>
      </div>
    </div>

    <!-- 6. 自定义确认删除 Modal 弹窗 -->
    <div v-if="confirmDeleteModal.show" class="fixed inset-0 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 z-50">
      <div class="bg-[#111625] border border-rose-500/40 rounded-2xl w-full max-w-sm p-6 shadow-2xl space-y-4">
        <div class="flex items-center gap-3 text-rose-400">
          <div class="p-2 rounded-xl bg-rose-500/10 border border-rose-500/30">
            <AlertTriangle class="w-6 h-6" />
          </div>
          <div>
            <h3 class="text-base font-bold text-gray-100">确认要执行删除？</h3>
            <p class="text-xs text-gray-400">该操作无法撤销</p>
          </div>
        </div>

        <p class="text-xs text-gray-300 leading-relaxed bg-white/5 p-3 rounded-xl border border-white/10">
          {{ confirmDeleteModal.message }}
        </p>

        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            @click="confirmDeleteModal.show = false"
            class="px-4 py-2 rounded-xl border border-white/10 text-gray-300 hover:bg-white/5 text-xs transition-colors cursor-pointer"
          >
            取消
          </button>
          <button
            @click="executeDelete"
            class="px-4 py-2 rounded-xl bg-gradient-to-r from-rose-500 to-red-600 hover:from-rose-400 hover:to-red-500 text-white font-semibold text-xs transition-all shadow-lg shadow-rose-500/20 cursor-pointer"
          >
            确认删除
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useKBStore } from '../stores/kb';
import type { KnowledgeDocument, KnowledgeBase } from '../types/kb';
import {
  Database,
  Plus,
  RefreshCw,
  ShieldCheck,
  BookOpen,
  Trash2,
  FileText,
  UploadCloud,
  Sparkles,
  CheckCircle2,
  AlertCircle,
  Layers,
  Search,
  X,
  FolderOpen,
  AlertTriangle,
  HardDrive,
  Cpu,
  Eye,
  ChevronRight,
  FileCode,
} from 'lucide-vue-next';

const kbStore = useKBStore();

const showCreateModal = ref(false);
const showUploadModal = ref(false);
const newKbName = ref('');
const newKbDesc = ref('');

const kbSearchQuery = ref('');
const searchQuery = ref('');
const selectedFormat = ref('all');
const statusFilter = ref<'all' | '0' | '1' | '2' | '3'>('all');

const isDragging = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);
const uploadSuccessMsg = ref('');

// 详情 Drawer/Modal 控制
const detailDocModal = ref<{ show: boolean; doc: KnowledgeDocument | null }>({
  show: false,
  doc: null,
});

// 确认删除 Modal 控制
const confirmDeleteModal = ref({
  show: false,
  type: '' as 'kb' | 'doc',
  id: '',
  message: '',
});

const formatFilterOptions = [
  { label: '全部格式', value: 'all' },
  { label: 'MD', value: 'md' },
  { label: 'PDF', value: 'pdf' },
  { label: 'TXT', value: 'txt' },
  { label: 'DOCX', value: 'docx' },
  { label: 'JSON/CSV', value: 'csv' },
];

onMounted(() => {
  kbStore.fetchKBs();
});

const activeKBName = computed(() => {
  return kbStore.activeKB?.name || (kbStore.kbs.length > 0 ? kbStore.kbs[0]?.name : '系统公共知识库');
});

const activeKBDesc = computed(() => {
  return kbStore.activeKB?.description || (kbStore.kbs.length > 0 ? kbStore.kbs[0]?.description : '系统默认向量知识库');
});

const customKBsFiltered = computed(() => {
  return kbStore.customKBs.filter((kb) => matchesKBSearch(kb));
});

function matchesKBSearch(kb: KnowledgeBase) {
  const q = kbSearchQuery.value.trim().toLowerCase();
  if (!q) return true;
  return kb.name.toLowerCase().includes(q) || (kb.description || '').toLowerCase().includes(q) || kb.kb_id.toLowerCase().includes(q);
}

const totalChunksCount = computed(() => {
  return kbStore.documents.reduce((acc, d) => acc + (d.total_chunks || 0), 0);
});

// 过滤文档列表
const filteredDocuments = computed(() => {
  return kbStore.documents.filter((doc) => {
    // 1. 搜索词
    const q = searchQuery.value.trim().toLowerCase();
    const matchesSearch =
      !q ||
      doc.title.toLowerCase().includes(q) ||
      doc.source_type.toLowerCase().includes(q) ||
      doc.doc_id.toLowerCase().includes(q) ||
      (doc.file_hash && doc.file_hash.toLowerCase().includes(q));

    // 2. 格式过滤
    const fmt = selectedFormat.value;
    let matchesFormat = true;
    if (fmt !== 'all') {
      if (fmt === 'csv') {
        matchesFormat = doc.source_type.toLowerCase() === 'csv' || doc.source_type.toLowerCase() === 'json';
      } else {
        matchesFormat = doc.source_type.toLowerCase() === fmt;
      }
    }

    // 3. 状态过滤
    const matchesStatus =
      statusFilter.value === 'all' ||
      String(doc.status) === statusFilter.value;

    return matchesSearch && matchesFormat && matchesStatus;
  });
});

async function handleCreateKBSubmit() {
  if (!newKbName.value.trim()) return;
  try {
    await kbStore.createKB(newKbName.value.trim(), newKbDesc.value.trim());
    newKbName.value = '';
    newKbDesc.value = '';
    showCreateModal.value = false;
  } catch (e) {}
}

function promptDeleteKB(kb_id: string, name: string) {
  confirmDeleteModal.value = {
    show: true,
    type: 'kb',
    id: kb_id,
    message: `确定要删除自定义知识库 [${name}] 吗？对应底层的所有向量索引与文件记录将被永久清除。`,
  };
}

function promptDeleteDoc(doc_id: string, title: string) {
  confirmDeleteModal.value = {
    show: true,
    type: 'doc',
    id: doc_id,
    message: `确定要删除文档 [${title}] 吗？关联的 MySQL Parent-Child 切片与 Milvus 向量将被同步清除。`,
  };
}

async function executeDelete() {
  const { type, id } = confirmDeleteModal.value;
  confirmDeleteModal.value.show = false;
  try {
    if (type === 'kb') {
      await kbStore.removeKB(id);
    } else if (type === 'doc') {
      await kbStore.removeDocument(id);
    }
  } catch (e) {}
}

function openDocDetail(doc: KnowledgeDocument) {
  detailDocModal.value = {
    show: true,
    doc,
  };
}

function triggerFileInput() {
  fileInputRef.value?.click();
}

function handleDrop(e: DragEvent) {
  isDragging.value = false;
  const files = e.dataTransfer?.files;
  if (files && files.length > 0) {
    uploadFile(files[0]);
  }
}

function handleFileSelected(e: Event) {
  const target = e.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    uploadFile(target.files[0]);
  }
}

async function uploadFile(file: File) {
  uploadSuccessMsg.value = '';
  try {
    const res = await kbStore.uploadFile(file);
    uploadSuccessMsg.value = `文档 [${res.title}] 增量解析成功！分割生成 ${res.total_chunks} 个 Parent-Child 向量切片。`;
    showUploadModal.value = false;
  } catch (err) {}
}

function getFileFormatBadgeClass(ext: string) {
  const format = (ext || '').toLowerCase();
  switch (format) {
    case 'pdf':
      return 'bg-rose-500/20 border-rose-500/40 text-rose-300';
    case 'md':
    case 'markdown':
      return 'bg-cyan-500/20 border-cyan-500/40 text-cyan-300';
    case 'txt':
      return 'bg-blue-500/20 border-blue-500/40 text-blue-300';
    case 'docx':
    case 'doc':
      return 'bg-indigo-500/20 border-indigo-500/40 text-indigo-300';
    case 'csv':
    case 'json':
      return 'bg-amber-500/20 border-amber-500/40 text-amber-300';
    default:
      return 'bg-gray-500/20 border-gray-500/40 text-gray-300';
  }
}

function getStatusStyle(status: number) {
  switch (status) {
    case 2:
      return {
        text: '已向量化',
        bg: 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300 px-2 py-0.5 rounded-full',
        dot: 'bg-emerald-400 animate-pulse',
      };
    case 1:
      return {
        text: '解析向量化中',
        bg: 'bg-blue-500/10 border-blue-500/30 text-blue-300 px-2 py-0.5 rounded-full',
        spin: true,
      };
    case 0:
      return {
        text: '排队待处理',
        bg: 'bg-amber-500/10 border-amber-500/30 text-amber-300 px-2 py-0.5 rounded-full',
        dot: 'bg-amber-400',
      };
    case 3:
    default:
      return {
        text: '解析失败',
        bg: 'bg-rose-500/10 border-rose-500/30 text-rose-300 px-2 py-0.5 rounded-full',
        dot: 'bg-rose-400',
      };
  }
}

function formatDate(timestamp?: string | number) {
  if (!timestamp) return '-';
  let date: Date;
  if (typeof timestamp === 'number') {
    date = new Date(timestamp > 10000000000 ? timestamp : timestamp * 1000);
  } else {
    date = new Date(timestamp);
  }
  if (isNaN(date.getTime())) return '-';
  const pad = (n: number) => (n < 10 ? '0' + n : n);
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`;
}
</script>
