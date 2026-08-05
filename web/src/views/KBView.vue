<template>
  <div class="kb-workspace-root">
    <!-- 1. 顶部 Header 导航栏 -->
    <header class="kb-header">
      <div class="breadcrumb-box">
        <div class="brand-icon-box">
          <Database class="w-5 h-5" />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs font-semibold text-gray-400">知识库工作台</span>
          <ChevronRight class="w-4 h-4 text-gray-600" />
          <h1 class="text-sm font-bold tracking-wide text-gray-100 flex items-center gap-2">
            {{ activeKBName }}
          </h1>
          <span
            :class="[
              'text-[10px] font-mono px-2 py-0.5 rounded-full border font-semibold ml-1.5',
              kbStore.activeKB?.is_default
                ? 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30'
                : 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30'
            ]"
          >
            {{ kbStore.activeKB?.is_default ? 'SYSTEM PUBLIC' : 'CUSTOM BASE' }}
          </span>
        </div>
      </div>

      <!-- 右侧全局操作按钮组 -->
      <div class="header-actions">
        <button
          @click="kbStore.fetchKBs()"
          class="px-3 py-2 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 text-gray-300 transition-all flex items-center gap-1.5 text-xs font-medium cursor-pointer"
          title="刷新全量知识库与文档"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': kbStore.loading || kbStore.docLoading }" />
          <span>刷新</span>
        </button>

        <button
          @click="openUploadModal"
          class="px-3.5 py-2 rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-300 hover:bg-cyan-500/20 text-xs font-semibold transition-all flex items-center gap-1.5 cursor-pointer shadow-sm shadow-cyan-500/10"
        >
          <UploadCloud class="w-4 h-4 text-cyan-400" />
          <span>上传文档</span>
        </button>

        <button
          @click="showCreateModal = true"
          class="px-4 py-2 rounded-xl bg-gradient-to-r from-cyan-500 via-blue-600 to-indigo-600 hover:from-cyan-400 hover:to-indigo-500 text-white font-semibold text-xs transition-all shadow-lg shadow-cyan-500/20 flex items-center gap-2 cursor-pointer"
        >
          <Plus class="w-4 h-4" />
          <span>新建知识库</span>
        </button>
      </div>
    </header>

    <!-- 2. 主体工作区 -->
    <div class="kb-body-container">
      <!-- 2.1 左侧知识库选择侧边栏 (290px) -->
      <aside class="kb-sidebar">
        <div class="sidebar-header-box">
          <div class="sidebar-title-row">
            <span class="flex items-center gap-1.5">
              <Layers class="w-4 h-4 text-cyan-400" />
              知识库空间
            </span>
            <span class="px-2 py-0.5 rounded bg-white/5 text-[10px] font-mono text-gray-400 border border-white/10">
              {{ kbStore.kbs.length }} 个
            </span>
          </div>

          <div class="sidebar-search-wrapper">
            <Search class="sidebar-search-icon" />
            <input
              v-model="kbSearchQuery"
              type="text"
              placeholder="搜索知识库名称/ID..."
              class="sidebar-search-input"
            />
          </div>
        </div>

        <div class="sidebar-kb-list">
          <!-- 系统公共库 -->
          <div v-if="kbStore.defaultKB && matchesKBSearch(kbStore.defaultKB)">
            <div class="text-[10px] font-semibold text-cyan-400 mb-1.5 px-1 flex items-center gap-1.5 uppercase tracking-wider">
              <ShieldCheck class="w-3.5 h-3.5" />
              系统核心公共库
            </div>
            <div
              v-if="kbStore.defaultKB"
              @click="kbStore.defaultKB?.kb_id && kbStore.selectKB(kbStore.defaultKB.kb_id)"
              :class="['kb-card', kbStore.activeKbId === kbStore.defaultKB?.kb_id && 'active']"
            >
              <div class="flex items-start justify-between">
                <div class="flex-1 min-w-0">
                  <h3 class="font-bold text-xs text-gray-100 truncate">{{ kbStore.defaultKB.name }}</h3>
                  <p class="text-[11px] text-gray-400 mt-1 line-clamp-2 leading-relaxed" :title="kbStore.defaultKB.description || '系统默认公共知识库'">{{ kbStore.defaultKB.description || '系统默认公共知识库' }}</p>
                </div>
                <BookOpen class="w-4 h-4 text-cyan-400 shrink-0 ml-2 mt-0.5" />
              </div>
              <div class="mt-2 pt-2 border-t border-white/5 flex items-center justify-between text-[10px] text-gray-500 font-mono">
                <span>ID: {{ truncateString(kbStore.defaultKB?.kb_id, 14) }}</span>
                <span class="text-cyan-400 bg-cyan-500/10 px-1.5 py-0.5 rounded border border-cyan-500/20 font-sans">SYSTEM</span>
              </div>
            </div>
          </div>

          <!-- 自定义知识库 -->
          <div>
            <div class="text-[10px] font-semibold text-gray-400 mb-1.5 px-1 flex items-center justify-between uppercase tracking-wider">
              <span>自定义向量库 ({{ customKBsFiltered.length }})</span>
            </div>

            <div v-if="customKBsFiltered.length === 0" class="p-4 text-center border border-dashed border-white/10 rounded-xl bg-white/[0.01]">
              <FolderOpen class="w-6 h-6 mx-auto text-gray-600 mb-1" />
              <p class="text-[11px] text-gray-500">无匹配自定义知识库</p>
            </div>

            <div v-else class="space-y-2">
              <div
                v-for="kb in customKBsFiltered"
                :key="kb.kb_id"
                @click="kb?.kb_id && kbStore.selectKB(kb.kb_id)"
                :class="['kb-card', kbStore.activeKbId === kb?.kb_id && 'active']"
              >
                <div class="flex items-start justify-between">
                  <div class="flex-1 min-w-0">
                    <h3 class="font-bold text-xs text-gray-100 truncate">{{ kb.name }}</h3>
                    <p class="text-[11px] text-gray-400 mt-1 line-clamp-1 leading-relaxed" :title="kb.description || '无描述'">{{ kb.description || '无描述' }}</p>
                  </div>
                  <button
                    @click.stop="kb?.kb_id && promptDeleteKB(kb.kb_id, kb.name)"
                    class="opacity-0 hover:opacity-100 p-1.5 hover:bg-rose-500/20 text-gray-400 hover:text-rose-400 rounded-lg transition-all cursor-pointer ml-1 shrink-0"
                    title="删除知识库"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
                <div class="mt-2 pt-2 border-t border-white/5 flex items-center justify-between text-[10px] text-gray-500 font-mono">
                  <span>ID: {{ truncateString(kb?.kb_id, 14) }}</span>
                  <span class="text-indigo-300 bg-indigo-500/10 px-1.5 py-0.5 rounded border border-indigo-500/20 font-sans">CUSTOM</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="sidebar-footer-box">
          <div class="flex items-center gap-1.5">
            <Cpu class="w-3.5 h-3.5 text-cyan-400" />
            <span>Milvus Dense Vector</span>
          </div>
          <span class="text-emerald-400 font-mono font-semibold">1536D</span>
        </div>
      </aside>

      <!-- 2.2 右侧主展示大盘 -->
      <main class="kb-main-content">
        <div class="kb-content-scroll">
          <!-- 2.2.1 4 KPI 数据统计卡片 -->
          <div class="kpi-grid">
            <!-- 卡片 1: 包含文档数 -->
            <div class="kpi-card">
              <div>
                <div class="text-xs font-semibold text-gray-400">包含文档总数</div>
                <div class="text-2xl font-bold font-mono text-gray-100 mt-1">
                  {{ kbStore.documents.length }} <span class="text-xs font-normal text-gray-400">篇</span>
                </div>
              </div>
              <div class="kpi-icon-wrapper bg-cyan-500/10 border border-cyan-500/30 text-cyan-400">
                <FileText class="w-5 h-5" />
              </div>
            </div>

            <!-- 卡片 2: 向量切片总数 -->
            <div class="kpi-card">
              <div>
                <div class="text-xs font-semibold text-gray-400">向量切片数 (Chunks)</div>
                <div class="text-2xl font-bold font-mono text-emerald-400 mt-1">
                  {{ totalChunksCount }} <span class="text-xs font-normal text-gray-400">chunks</span>
                </div>
              </div>
              <div class="kpi-icon-wrapper bg-emerald-500/10 border border-emerald-500/30 text-emerald-400">
                <Sparkles class="w-5 h-5" />
              </div>
            </div>

            <!-- 卡片 3: 切片向量策略 -->
            <div class="kpi-card">
              <div>
                <div class="text-xs font-semibold text-gray-400">向量分块策略</div>
                <div class="text-xs font-bold font-mono text-indigo-300 mt-1.5">Parent-Child 两阶段</div>
                <div class="text-[10px] text-gray-500 mt-0.5">512B Child / 2KB Parent</div>
              </div>
              <div class="kpi-icon-wrapper bg-indigo-500/10 border border-indigo-500/30 text-indigo-400">
                <Layers class="w-5 h-5" />
              </div>
            </div>

            <!-- 卡片 4: 空间 UUID -->
            <div class="kpi-card">
              <div class="min-w-0 pr-2">
                <div class="text-xs font-semibold text-gray-400">当前空间 UUID</div>
                <div class="text-xs font-mono text-cyan-300 mt-1.5 truncate" :title="kbStore.activeKbId">
                  {{ kbStore.activeKbId }}
                </div>
                <div class="text-[10px] text-gray-500 line-clamp-3 leading-relaxed mt-0.5" :title="activeKBDesc">{{ activeKBDesc }}</div>
              </div>
              <div class="kpi-icon-wrapper bg-blue-500/10 border border-blue-500/30 text-blue-400">
                <HardDrive class="w-5 h-5" />
              </div>
            </div>
          </div>

          <!-- Alert 消息提示栏 -->
          <div v-if="uploadSuccessMsg" class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center justify-between shadow-lg">
            <div class="flex items-center gap-2">
              <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
              <span>{{ uploadSuccessMsg }}</span>
            </div>
            <button @click="uploadSuccessMsg = ''" class="text-emerald-400 hover:text-emerald-200 cursor-pointer text-sm font-bold">×</button>
          </div>

          <div v-if="kbStore.error" class="p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center justify-between shadow-lg">
            <div class="flex items-center gap-2">
              <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
              <span>{{ kbStore.error }}</span>
            </div>
            <button @click="kbStore.error = null" class="text-rose-400 hover:text-rose-200 cursor-pointer text-sm font-bold">×</button>
          </div>

          <!-- 2.2.2 搜索与筛选工具栏 -->
          <div class="filter-toolbar-card">
            <!-- 搜索框 -->
            <div class="main-search-wrapper">
              <Search class="main-search-icon" />
              <input
                v-model="searchQuery"
                type="text"
                placeholder="搜索文档名称、类型、Doc ID、SHA256 哈希..."
                class="main-search-input"
              />
            </div>

            <!-- 格式 Pill -->
            <div class="format-filter-group">
              <button
                v-for="fmt in formatFilterOptions"
                :key="fmt.value"
                @click="selectedFormat = fmt.value"
                :class="['format-pill-btn', selectedFormat === fmt.value && 'active']"
              >
                {{ fmt.label }}
              </button>
            </div>

            <!-- 状态下拉框 -->
            <select v-model="statusFilter" class="status-select">
              <option value="all" class="bg-[#121827] text-gray-200">全部解析状态</option>
              <option value="2" class="bg-[#121827] text-emerald-300">已向量化 (Status 2)</option>
              <option value="1" class="bg-[#121827] text-blue-300">解析向量化中 (Status 1)</option>
              <option value="3" class="bg-[#121827] text-rose-300">解析失败 (Status 3)</option>
            </select>
          </div>

          <!-- 2.2.3 文档展示列表表格 -->
          <div class="doc-table-card">
            <table class="doc-table">
              <thead>
                <tr>
                  <th style="width: 32%;">文档名称与类型</th>
                  <th style="width: 16%;">解析状态</th>
                  <th style="width: 15%;">切片数 (CHUNKS)</th>
                  <th style="width: 17%;">SHA256 哈希</th>
                  <th style="width: 12%;">入库时间</th>
                  <th style="width: 8%; text-align: right;">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="kbStore.docLoading">
                  <td colspan="6" style="padding: 4rem 1rem; text-align: center; color: #94a3b8;">
                    <RefreshCw class="w-7 h-7 animate-spin mx-auto text-cyan-400 mb-2" />
                    <p class="text-xs">加载文档数据中...</p>
                  </td>
                </tr>

                <tr v-else-if="filteredDocuments.length === 0">
                  <td colspan="6" style="padding: 4rem 1rem; text-align: center;">
                    <div class="w-12 h-12 mx-auto rounded-2xl bg-white/5 border border-white/10 flex items-center justify-center text-gray-500 mb-3">
                      <FileCode class="w-6 h-6" />
                    </div>
                    <p class="text-xs text-gray-400 font-medium">当前知识库下无匹配文档</p>
                    <button
                      @click="openUploadModal"
                      class="mt-3 text-xs text-cyan-400 hover:text-cyan-300 underline underline-offset-4 cursor-pointer font-semibold"
                    >
                      + 立即上传第一份文档
                    </button>
                  </td>
                </tr>

                <tr
                  v-for="doc in filteredDocuments"
                  :key="doc.doc_id"
                  class="doc-row"
                  @click="openDocDetail(doc)"
                >
                  <!-- 名称与格式 -->
                  <td>
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
                  <td>
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

                  <!-- 向量切片数与花费 -->
                  <td class="font-mono">
                    <div class="flex flex-col gap-1">
                      <span class="px-2.5 py-0.5 rounded-lg bg-cyan-500/10 border border-cyan-500/20 text-cyan-300 text-xs font-bold w-max">
                        {{ doc.total_chunks }} chunks
                      </span>
                      <span class="text-[11px] text-emerald-400 font-semibold">
                        花费: ￥{{ (doc.embedding_cost || 0).toFixed(6) }}
                      </span>
                    </div>
                  </td>

                  <!-- SHA256 哈希 -->
                  <td class="font-mono text-[11px] text-gray-400">
                    <span v-if="doc.file_hash" class="bg-white/5 px-2 py-1 rounded-md border border-white/10 text-gray-300" :title="doc.file_hash">
                      {{ truncateString(doc.file_hash, 12) }}
                    </span>
                    <span v-else class="text-gray-600">-</span>
                  </td>

                  <!-- 入库时间 -->
                  <td class="text-xs text-gray-400 font-mono">
                    {{ formatDate(doc.created_at) }}
                  </td>

                  <!-- 操作栏 -->
                  <td style="text-align: right;" @click.stop>
                    <div class="flex items-center justify-end gap-1.5">
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
      </main>
    </div>

    <!-- 3. 上传增量文档 Modal 弹窗 -->
    <div v-if="showUploadModal" class="fixed inset-0 bg-black/75 backdrop-blur-2xl flex items-center justify-center p-4 z-50 animate-in fade-in duration-200">
      <div class="bg-[#090d16]/95 border border-cyan-500/30 rounded-3xl w-full max-w-xl p-6 sm:p-7 shadow-[0_0_80px_rgba(6,182,212,0.18)] flex flex-col gap-5 relative overflow-hidden backdrop-blur-3xl animate-in zoom-in-95 duration-200">
        <!-- 背景渐变光晕效果 -->
        <div class="absolute -top-32 -right-32 w-72 h-72 bg-cyan-500/15 rounded-full blur-3xl pointer-events-none"></div>
        <div class="absolute -bottom-32 -left-32 w-72 h-72 bg-indigo-500/15 rounded-full blur-3xl pointer-events-none"></div>

        <!-- Modal Header -->
        <div class="flex items-center justify-between relative z-10 pb-4 border-b border-white/10">
          <div class="flex items-center gap-3.5">
            <div class="w-11 h-11 rounded-2xl bg-gradient-to-br from-cyan-500/20 via-blue-500/20 to-indigo-500/20 border border-cyan-500/40 flex items-center justify-center text-cyan-400 shadow-lg shadow-cyan-500/15 shrink-0">
              <UploadCloud class="w-5 fill-cyan-400/20 h-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-base font-bold text-gray-100 tracking-wide">上传增量文档</h2>
                <span class="text-[10px] font-mono px-2 py-0.5 rounded-full border bg-cyan-500/10 text-cyan-300 border-cyan-500/30 font-semibold tracking-wider">
                  RAG V2.0 PIPELINE
                </span>
              </div>
              <p class="text-xs text-gray-400 mt-0.5 flex items-center gap-2">
                <span>目标知识库:</span>
                <span class="text-cyan-300 font-semibold bg-cyan-500/10 px-2 py-0.5 rounded-lg border border-cyan-500/25 font-mono text-[11px] flex items-center gap-1">
                  <Database class="w-3 h-3 text-cyan-400" />
                  {{ activeKBName }}
                </span>
              </p>
            </div>
          </div>

          <button
            @click="closeUploadModal"
            class="w-8 h-8 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 hover:border-white/20 text-gray-400 hover:text-gray-100 transition-all flex items-center justify-center cursor-pointer shrink-0"
            title="关闭"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Modal Body -->
        <div class="relative z-10 flex flex-col gap-4">
          <!-- 默认系统知识库禁止添加警告 -->
          <div v-if="isDefaultKB" class="p-3.5 rounded-2xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs flex items-center gap-3 font-medium shadow-lg">
            <AlertCircle class="w-5 h-5 text-amber-400 shrink-0" />
            <div>
              <div class="font-bold text-amber-200 text-xs sm:text-sm">禁止通过接口添加文档</div>
              <div class="text-[11px] text-amber-300/90 mt-0.5">默认知识库 (kb_default_system) 目前只能项目初始化时处理。如需上传，请先在左侧新建或切换至自定义知识库。</div>
            </div>
          </div>

          <!-- 未选择文件时的 Dropzone -->
          <div
            v-if="!selectedFile && !kbStore.uploading"
            @dragover.prevent="!isDefaultKB && (isDragging = true)"
            @dragleave.prevent="isDragging = false"
            @drop.prevent="!isDefaultKB && handleDrop($event)"
            @click="!isDefaultKB && triggerFileInput()"
            :class="[
              'border-2 border-dashed rounded-2xl p-6 text-center transition-all relative overflow-hidden group',
              isDefaultKB
                ? 'border-amber-500/30 bg-amber-500/5 cursor-not-allowed opacity-60'
                : isDragging
                  ? 'border-cyan-400 bg-cyan-500/15 shadow-xl shadow-cyan-500/20 scale-[1.01] cursor-pointer'
                  : 'border-white/15 bg-white/[0.02] hover:border-cyan-500/50 hover:bg-cyan-500/[0.04] cursor-pointer'
            ]"
          >
            <input
              type="file"
              ref="fileInputRef"
              class="hidden"
              accept=".md,.txt,.pdf,.docx,.json,.csv,.tsv"
              @change="handleFileSelected"
            />

            <div class="max-w-md mx-auto pointer-events-none flex flex-col items-center">
              <div class="w-14 h-14 rounded-2xl bg-gradient-to-tr from-cyan-500/20 via-blue-500/15 to-indigo-500/20 border border-cyan-500/35 flex items-center justify-center text-cyan-400 mb-3 shadow-inner group-hover:scale-110 group-hover:border-cyan-400 transition-all duration-300">
                <UploadCloud class="w-7 h-7 group-hover:animate-bounce" />
              </div>

              <h3 class="text-sm font-bold text-gray-200 mb-1 flex items-center gap-1">
                <span>拖拽文档到此处，或</span>
                <span class="text-cyan-400 underline underline-offset-4 font-extrabold group-hover:text-cyan-300">点击浏览本地文件</span>
              </h3>

              <p class="text-xs text-gray-400 mb-3">
                支持单文件 &le; 50MB · 自动解析 Markdown、PDF 及结构化数据
              </p>

              <!-- Format Badge Pills -->
              <div class="flex flex-wrap items-center justify-center gap-1.5">
                <span class="px-2.5 py-1 rounded-lg bg-cyan-500/10 border border-cyan-500/30 text-[10px] font-mono text-cyan-300 font-semibold">.MD</span>
                <span class="px-2.5 py-1 rounded-lg bg-rose-500/10 border border-rose-500/30 text-[10px] font-mono text-rose-300 font-semibold">.PDF</span>
                <span class="px-2.5 py-1 rounded-lg bg-blue-500/10 border border-blue-500/30 text-[10px] font-mono text-blue-300 font-semibold">.TXT</span>
                <span class="px-2.5 py-1 rounded-lg bg-indigo-500/10 border border-indigo-500/30 text-[10px] font-mono text-indigo-300 font-semibold">.DOCX</span>
                <span class="px-2.5 py-1 rounded-lg bg-amber-500/10 border border-amber-500/30 text-[10px] font-mono text-amber-300 font-semibold">.JSON</span>
                <span class="px-2.5 py-1 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-[10px] font-mono text-emerald-300 font-semibold">.CSV</span>
              </div>
            </div>
          </div>

          <!-- 已选择待上传文件 Card -->
          <div v-else-if="selectedFile && !kbStore.uploading" class="bg-white/[0.03] border border-cyan-500/30 rounded-2xl p-4.5 space-y-3.5 shadow-lg">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3.5 min-w-0">
                <div :class="['w-11 h-11 rounded-xl border flex items-center justify-center text-xs font-mono font-bold uppercase shrink-0 shadow-md', getFileFormatBadgeClass(getFileExtension(selectedFile.name))]">
                  {{ getFileExtension(selectedFile.name) }}
                </div>
                <div class="min-w-0">
                  <h4 class="text-xs sm:text-sm font-bold text-gray-100 truncate" :title="selectedFile.name">{{ selectedFile.name }}</h4>
                  <p class="text-[11px] text-gray-400 font-mono mt-0.5 flex items-center gap-2">
                    <span>{{ formatFileSize(selectedFile.size) }}</span>
                    <span>•</span>
                    <span class="text-emerald-400 flex items-center gap-1 font-sans font-semibold">
                      <CheckCircle2 class="w-3.5 h-3.5" /> 准备就绪
                    </span>
                  </p>
                </div>
              </div>

              <button
                @click="clearSelectedFile"
                class="p-2 hover:bg-rose-500/20 text-gray-400 hover:text-rose-400 rounded-xl transition-all cursor-pointer shrink-0"
                title="更换文件"
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <!-- Vector Specs Breakdown -->
            <div class="bg-black/30 rounded-xl p-3 border border-white/5 text-[11px] text-gray-400 space-y-1.5 font-mono">
              <div class="flex justify-between items-center">
                <span class="text-gray-400">预估向量切片:</span>
                <span class="text-cyan-300 font-semibold bg-cyan-500/10 px-2 py-0.5 rounded border border-cyan-500/20">
                  约 {{ Math.max(1, Math.ceil(selectedFile.size / 1024)) }} chunks
                </span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-gray-400">向量引擎规格:</span>
                <span class="text-indigo-300 font-semibold bg-indigo-500/10 px-2 py-0.5 rounded border border-indigo-500/20">
                  1536D Dense Embedding
                </span>
              </div>
            </div>
          </div>

          <!-- 向量切片流水线进度条 (Uploading Visualizer) -->
          <div v-else-if="kbStore.uploading" class="bg-cyan-950/30 border border-cyan-500/40 rounded-2xl p-5 text-center space-y-4 relative overflow-hidden backdrop-blur-xl">
            <div class="flex items-center justify-center gap-3">
              <RefreshCw class="w-5 h-5 text-cyan-400 animate-spin" />
              <span class="text-xs sm:text-sm font-bold text-cyan-200 tracking-wide">AI 向量切片流水线构建中...</span>
            </div>

            <!-- Pipeline step cards -->
            <div class="grid grid-cols-4 gap-2 text-[10px] font-medium pt-1">
              <div class="p-2 rounded-xl bg-cyan-500/15 border border-cyan-500/35 text-cyan-300 flex flex-col items-center gap-1 shadow-sm">
                <FileText class="w-4 h-4 animate-pulse text-cyan-400" />
                <span>1. 读取文件</span>
              </div>
              <div class="p-2 rounded-xl bg-cyan-500/15 border border-cyan-500/35 text-cyan-300 flex flex-col items-center gap-1 shadow-sm">
                <ShieldCheck class="w-4 h-4 animate-pulse text-cyan-400" />
                <span>2. SHA256</span>
              </div>
              <div class="p-2 rounded-xl bg-indigo-500/20 border border-indigo-500/40 text-indigo-300 flex flex-col items-center gap-1 shadow-sm">
                <Layers class="w-4 h-4 animate-pulse text-indigo-400" />
                <span>3. ParentChild</span>
              </div>
              <div class="p-2 rounded-xl bg-emerald-500/20 border border-emerald-500/40 text-emerald-300 flex flex-col items-center gap-1 shadow-sm">
                <Sparkles class="w-4 h-4 animate-pulse text-emerald-400" />
                <span>4. Milvus写入</span>
              </div>
            </div>

            <div class="w-full bg-white/10 rounded-full h-1.5 overflow-hidden">
              <div class="bg-gradient-to-r from-cyan-400 via-blue-500 to-emerald-400 h-full w-full animate-pulse"></div>
            </div>

            <p class="text-[11px] text-gray-400 font-mono">文档构建包含文本清洗与向量编排，请稍候...</p>
          </div>

          <!-- Error Alert -->
          <div v-if="uploadErrorMsg" class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center justify-between">
            <div class="flex items-center gap-2">
              <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
              <span>{{ uploadErrorMsg }}</span>
            </div>
            <button @click="uploadErrorMsg = ''" class="text-rose-400 hover:text-rose-200 cursor-pointer font-bold">×</button>
          </div>

          <!-- 底部架构标签 -->
          <div class="px-3.5 py-2.5 rounded-xl bg-white/[0.02] border border-white/5 flex items-center justify-between text-xs text-gray-400">
            <div class="flex items-center gap-2">
              <Sparkles class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
              <span class="text-gray-300 text-[11px] font-medium">Parent-Child 两阶段向量架构</span>
            </div>
            <span class="text-[10px] font-mono text-gray-400 bg-white/5 px-2 py-0.5 rounded border border-white/10">512B Child / 2KB Parent</span>
          </div>
        </div>

        <!-- Modal Footer -->
        <div class="flex items-center justify-between pt-3.5 border-t border-white/10 relative z-10">
          <span class="text-[11px] text-gray-500 font-mono">UTF-8 编码格式与文本抽取</span>

          <div class="flex items-center gap-3">
            <button
              @click="closeUploadModal"
              :disabled="kbStore.uploading"
              class="px-4 py-2 rounded-xl border border-white/10 text-gray-300 hover:bg-white/10 disabled:opacity-50 text-xs font-medium transition-all cursor-pointer"
            >
              取消
            </button>

            <button
              v-if="selectedFile"
              @click="startUploadSelectedFile"
              :disabled="kbStore.uploading || isDefaultKB"
              class="px-5 py-2 rounded-xl bg-gradient-to-r from-cyan-500 via-blue-600 to-indigo-600 hover:from-cyan-400 hover:to-indigo-500 text-white font-semibold text-xs transition-all shadow-lg shadow-cyan-500/25 hover:shadow-cyan-500/40 cursor-pointer flex items-center gap-2 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <RefreshCw v-if="kbStore.uploading" class="w-3.5 h-3.5 animate-spin" />
              <span>开始向量化构建</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 4. 新建自定义知识库 Modal 弹窗 -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black/75 backdrop-blur-2xl flex items-center justify-center p-4 z-50 animate-in fade-in duration-200">
      <div class="bg-[#090d16]/95 border border-cyan-500/30 rounded-3xl w-full max-w-lg p-6 sm:p-7 shadow-[0_0_60px_rgba(6,182,212,0.18)] relative overflow-hidden backdrop-blur-3xl animate-in zoom-in-95 duration-200">
        <!-- 背景光影 -->
        <div class="absolute -top-28 -right-28 w-60 h-60 bg-cyan-500/15 rounded-full blur-3xl pointer-events-none"></div>

        <!-- Header -->
        <div class="flex items-center justify-between border-b border-white/10 pb-4 relative z-10">
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-600/20 border border-cyan-500/40 flex items-center justify-center text-cyan-400 shadow-md">
              <Plus class="w-5 h-5" />
            </div>
            <div>
              <h3 class="text-base font-bold text-gray-100 flex items-center gap-2">
                新建自定义知识库
              </h3>
              <p class="text-xs text-gray-400 mt-0.5">创建独立隔离的向量存储引擎库</p>
            </div>
          </div>
          <button @click="showCreateModal = false" class="w-8 h-8 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 text-gray-400 hover:text-white transition-all flex items-center justify-center cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Form -->
        <form @submit.prevent="handleCreateKBSubmit" class="space-y-4 pt-4 relative z-10">
          <div>
            <label class="block text-xs font-semibold text-gray-300 mb-1.5">知识库名称 <span class="text-cyan-400">*</span></label>
            <div class="relative">
              <div class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-gray-500">
                <BookOpen class="w-4 h-4" />
              </div>
              <input
                v-model="newKbName"
                type="text"
                required
                placeholder="例如：产品规格说明书 2026"
                class="w-full bg-white/[0.03] border border-white/15 rounded-xl pl-10 pr-3.5 py-2.5 text-sm text-gray-100 placeholder-gray-500 focus:outline-none focus:border-cyan-400 focus:ring-1 focus:ring-cyan-400/50 transition-all"
              />
            </div>
          </div>

          <div>
            <label class="block text-xs font-semibold text-gray-300 mb-1.5">知识库描述</label>
            <div class="relative">
              <div class="absolute top-3 left-0 pl-3.5 pointer-events-none text-gray-500">
                <AlignLeft class="w-4 h-4" />
              </div>
              <textarea
                v-model="newKbDesc"
                rows="3"
                placeholder="说明该知识库适用的业务场景及文档范围..."
                class="w-full bg-white/[0.03] border border-white/15 rounded-xl pl-10 pr-3.5 py-2.5 text-sm text-gray-100 placeholder-gray-500 focus:outline-none focus:border-cyan-400 focus:ring-1 focus:ring-cyan-400/50 transition-all resize-none"
              ></textarea>
            </div>
          </div>

          <div class="flex items-center justify-end gap-3 pt-3 border-t border-white/10">
            <button
              type="button"
              @click="showCreateModal = false"
              class="px-4 py-2 rounded-xl border border-white/10 text-gray-300 hover:bg-white/10 text-xs font-medium transition-all cursor-pointer"
            >
              取消
            </button>
            <button
              type="submit"
              :disabled="kbStore.loading"
              class="px-5 py-2 rounded-xl bg-gradient-to-r from-cyan-500 via-blue-600 to-indigo-600 hover:from-cyan-400 hover:to-indigo-500 text-white font-semibold text-xs transition-all shadow-lg shadow-cyan-500/25 hover:shadow-cyan-500/40 cursor-pointer flex items-center gap-2"
            >
              <RefreshCw v-if="kbStore.loading" class="w-3.5 h-3.5 animate-spin" />
              <span>确定创建知识库</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- 5. 文档元数据详情 Drawer/Modal 弹窗 -->
    <div v-if="detailDocModal.show && detailDocModal.doc" class="fixed inset-0 bg-black/75 backdrop-blur-2xl flex items-center justify-center p-4 z-50 animate-in fade-in duration-200">
      <div class="bg-[#090d16]/95 border border-cyan-500/30 rounded-3xl w-full max-w-xl p-6 sm:p-7 shadow-[0_0_60px_rgba(6,182,212,0.18)] relative overflow-hidden backdrop-blur-3xl animate-in zoom-in-95 duration-200 space-y-4">
        <!-- Glow orb -->
        <div class="absolute -top-28 -right-28 w-64 h-64 bg-cyan-500/10 rounded-full blur-3xl pointer-events-none"></div>

        <!-- Modal Header -->
        <div class="flex items-center justify-between border-b border-white/10 pb-4 relative z-10">
          <div class="flex items-center gap-3.5 min-w-0 pr-2">
            <div :class="['w-11 h-11 rounded-2xl border flex items-center justify-center text-xs font-mono font-bold uppercase shrink-0 shadow-lg', getFileFormatBadgeClass(detailDocModal.doc.source_type)]">
              {{ detailDocModal.doc.source_type }}
            </div>
            <div class="min-w-0">
              <h3 class="text-base font-bold text-gray-100 truncate" :title="detailDocModal.doc.title">{{ detailDocModal.doc.title }}</h3>
              <div class="flex items-center gap-2 mt-0.5">
                <span class="text-[10px] font-mono text-gray-400">DOC METADATA</span>
                <span class="font-sans font-semibold text-[10px] px-2 py-0.5 rounded-md" :class="getStatusStyle(detailDocModal.doc.status).bg">
                  {{ getStatusStyle(detailDocModal.doc.status).text }}
                </span>
              </div>
            </div>
          </div>
          <button @click="detailDocModal.show = false" class="w-8 h-8 rounded-xl bg-white/5 border border-white/10 hover:bg-white/10 text-gray-400 hover:text-white transition-all flex items-center justify-center cursor-pointer shrink-0">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- Content Grid Details -->
        <div class="space-y-3 relative z-10">
          <!-- Item 1: Doc ID & KB ID -->
          <div class="bg-white/[0.03] p-3.5 rounded-2xl border border-white/10 space-y-2.5 text-xs font-mono">
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-400 font-sans text-xs">文档 ID:</span>
              <div class="flex items-center gap-2 min-w-0">
                <span class="text-cyan-300 font-semibold bg-black/30 px-2 py-1 rounded-lg border border-cyan-500/20 truncate max-w-[260px] select-all">
                  {{ detailDocModal.doc.doc_id || (detailDocModal.doc as any).docId }}
                </span>
                <button
                  @click="copyToClipboard(detailDocModal.doc.doc_id || (detailDocModal.doc as any).docId, 'doc_id')"
                  class="p-1 hover:bg-white/10 text-gray-400 hover:text-cyan-300 rounded-md transition-colors cursor-pointer"
                  title="复制文档 ID"
                >
                  <Check v-if="copiedField === 'doc_id'" class="w-3.5 h-3.5 text-emerald-400" />
                  <Copy v-else class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-400 font-sans text-xs">关联知识库 ID:</span>
              <div class="flex items-center gap-2 min-w-0">
                <span class="text-gray-200 bg-black/30 px-2 py-1 rounded-lg border border-white/10 truncate max-w-[260px] select-all">
                  {{ detailDocModal.doc.kb_id || (detailDocModal.doc as any).kbId }}
                </span>
                <button
                  @click="copyToClipboard(detailDocModal.doc.kb_id || (detailDocModal.doc as any).kbId, 'kb_id')"
                  class="p-1 hover:bg-white/10 text-gray-400 hover:text-cyan-300 rounded-md transition-colors cursor-pointer"
                  title="复制知识库 ID"
                >
                  <Check v-if="copiedField === 'kb_id'" class="w-3.5 h-3.5 text-emerald-400" />
                  <Copy v-else class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div class="flex items-center justify-between border-t border-white/5 pt-2">
              <span class="text-gray-400 font-sans text-xs">向量切片数:</span>
              <span class="text-emerald-400 font-bold bg-emerald-500/10 px-2.5 py-0.5 rounded-lg border border-emerald-500/30">
                {{ detailDocModal.doc.total_chunks }} chunks
              </span>
            </div>

            <div v-if="detailDocModal.doc.file_hash" class="flex items-center justify-between gap-2">
              <span class="text-gray-400 font-sans text-xs">SHA256 哈希:</span>
              <div class="flex items-center gap-2 min-w-0">
                <span class="text-gray-300 bg-black/30 px-2 py-1 rounded-lg border border-white/10 truncate max-w-[260px] select-all text-[11px]" :title="detailDocModal.doc.file_hash">
                  {{ detailDocModal.doc.file_hash }}
                </span>
                <button
                  @click="copyToClipboard(detailDocModal.doc.file_hash!, 'file_hash')"
                  class="p-1 hover:bg-white/10 text-gray-400 hover:text-cyan-300 rounded-md transition-colors cursor-pointer"
                  title="复制文件哈希"
                >
                  <Check v-if="copiedField === 'file_hash'" class="w-3.5 h-3.5 text-emerald-400" />
                  <Copy v-else class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          </div>

          <!-- Item 2: File Path & Exception Log -->
          <div class="bg-white/[0.03] p-3.5 rounded-2xl border border-white/10 space-y-2 text-xs">
            <div>
              <span class="text-gray-400 block mb-1 font-sans">物理存储路径:</span>
              <div class="bg-black/50 p-2.5 rounded-xl border border-white/10 text-gray-300 break-all font-mono text-[11px] flex items-center justify-between gap-2">
                <span>{{ detailDocModal.doc.file_path || '-' }}</span>
                <button
                  v-if="detailDocModal.doc.file_path"
                  @click="copyToClipboard(detailDocModal.doc.file_path!, 'file_path')"
                  class="p-1 hover:bg-white/10 text-gray-400 hover:text-cyan-300 rounded-md transition-colors cursor-pointer shrink-0"
                  title="复制路径"
                >
                  <Check v-if="copiedField === 'file_path'" class="w-3.5 h-3.5 text-emerald-400" />
                  <Copy v-else class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <div v-if="detailDocModal.doc.err_msg" class="pt-2 border-t border-white/5">
              <span class="text-rose-400 font-semibold block mb-1">解析处理异常日志:</span>
              <span class="text-rose-300 bg-rose-500/10 p-2.5 rounded-xl border border-rose-500/20 block break-all font-sans text-xs leading-relaxed">
                {{ detailDocModal.doc.err_msg }}
              </span>
            </div>
          </div>

          <!-- Item 3: Footprint timestamps -->
          <div class="flex items-center justify-between text-[11px] text-gray-400 font-mono px-1">
            <span>创建时间: {{ formatDate(detailDocModal.doc.created_at) }}</span>
            <span>更新时间: {{ formatDate(detailDocModal.doc.updated_at) }}</span>
          </div>
        </div>

        <!-- Footer -->
        <div class="flex items-center justify-between pt-3 border-t border-white/10 relative z-10">
          <button
            @click="promptDeleteDoc(detailDocModal.doc.doc_id || (detailDocModal.doc as any).docId, detailDocModal.doc.title); detailDocModal.show = false;"
            class="px-3.5 py-1.5 rounded-xl bg-rose-500/15 border border-rose-500/30 text-rose-300 hover:bg-rose-500/25 hover:text-rose-200 text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5"
          >
            <Trash2 class="w-3.5 h-3.5" />
            <span>删除此文档</span>
          </button>

          <button
            @click="detailDocModal.show = false"
            class="px-5 py-2 rounded-xl bg-white/10 hover:bg-white/15 text-gray-200 text-xs font-semibold transition-all cursor-pointer"
          >
            关闭详情
          </button>
        </div>
      </div>
    </div>

    <!-- 6. 自定义确认删除 Modal 弹窗 -->
    <div v-if="confirmDeleteModal.show" class="fixed inset-0 bg-black/75 backdrop-blur-2xl flex items-center justify-center p-4 z-50 animate-in fade-in duration-200">
      <div class="bg-[#0c101d]/95 border border-rose-500/40 rounded-3xl w-full max-w-md p-6 sm:p-7 shadow-[0_0_70px_rgba(244,63,94,0.25)] relative overflow-hidden backdrop-blur-3xl animate-in zoom-in-95 duration-200 space-y-4">
        <!-- 红色危险渐变光晕 -->
        <div class="absolute -top-28 -right-28 w-60 h-60 bg-rose-500/20 rounded-full blur-3xl pointer-events-none"></div>

        <div class="flex items-center gap-3.5 text-rose-400 relative z-10">
          <div class="p-3 rounded-2xl bg-rose-500/15 border border-rose-500/40 shadow-lg shadow-rose-500/20">
            <ShieldAlert class="w-6 h-6 animate-pulse text-rose-400" />
          </div>
          <div>
            <h3 class="text-base font-bold text-gray-100">确认执行删除？</h3>
            <p class="text-xs text-rose-300/80 mt-0.5">此操作具有破坏性且不可撤销</p>
          </div>
        </div>

        <div class="bg-white/[0.03] p-4 rounded-2xl border border-white/10 relative z-10 space-y-2">
          <p class="text-xs text-gray-200 leading-relaxed font-sans">
            {{ confirmDeleteModal.message }}
          </p>
          <div class="p-2 rounded-xl bg-rose-500/10 border border-rose-500/20 text-[11px] text-rose-300 flex items-center gap-1.5 font-mono">
            <AlertCircle class="w-3.5 h-3.5 text-rose-400 shrink-0" />
            <span>底层 MySQL 物理记录与 Milvus 向量点阵将被直接干掉。</span>
          </div>
        </div>

        <div class="flex items-center justify-end gap-3 pt-2 relative z-10 border-t border-white/10">
          <button
            @click="confirmDeleteModal.show = false"
            class="px-4.5 py-2 rounded-xl border border-white/10 text-gray-300 hover:bg-white/10 text-xs font-semibold transition-all cursor-pointer"
          >
            取消
          </button>
          <button
            @click="executeDelete"
            class="px-5 py-2 rounded-xl bg-gradient-to-r from-rose-500 via-red-600 to-amber-600 hover:from-rose-400 hover:to-red-500 text-white font-bold text-xs transition-all shadow-lg shadow-rose-500/30 hover:shadow-rose-500/50 cursor-pointer flex items-center gap-1.5"
          >
            <Trash2 class="w-3.5 h-3.5" />
            <span>确认永久删除</span>
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
  HardDrive,
  Cpu,
  Eye,
  ChevronRight,
  FileCode,
  Copy,
  Check,
  AlignLeft,
  ShieldAlert,
} from 'lucide-vue-next';

const kbStore = useKBStore();

const isDefaultKB = computed(() => {
  return kbStore.activeKbId === 'kb_default_system' || !!kbStore.activeKB?.is_default;
});

const showCreateModal = ref(false);
const showUploadModal = ref(false);
const newKbName = ref('');
const newKbDesc = ref('');
const copiedField = ref<string | null>(null);

function copyToClipboard(text: string, fieldKey: string) {
  if (!text) return;
  navigator.clipboard.writeText(text).then(() => {
    copiedField.value = fieldKey;
    setTimeout(() => {
      if (copiedField.value === fieldKey) {
        copiedField.value = null;
      }
    }, 2000);
  }).catch(() => {});
}

const kbSearchQuery = ref('');
const searchQuery = ref('');
const selectedFormat = ref('all');
const statusFilter = ref<'all' | '0' | '1' | '2' | '3'>('all');

const isDragging = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);
const uploadSuccessMsg = ref('');
const selectedFile = ref<File | null>(null);
const uploadErrorMsg = ref('');

// 详情 Modal 控制
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
    const q = searchQuery.value.trim().toLowerCase();
    const matchesSearch =
      !q ||
      doc.title.toLowerCase().includes(q) ||
      doc.source_type.toLowerCase().includes(q) ||
      doc.doc_id.toLowerCase().includes(q) ||
      (doc.file_hash && doc.file_hash.toLowerCase().includes(q));

    const fmt = selectedFormat.value;
    let matchesFormat = true;
    if (fmt !== 'all') {
      if (fmt === 'csv') {
        matchesFormat = doc.source_type.toLowerCase() === 'csv' || doc.source_type.toLowerCase() === 'json';
      } else {
        matchesFormat = doc.source_type.toLowerCase() === fmt;
      }
    }

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
  const targetId = kb_id || (kbStore.activeKB?.kb_id || '');
  if (!targetId) return;
  confirmDeleteModal.value = {
    show: true,
    type: 'kb',
    id: targetId,
    message: `确定要删除自定义知识库 [${name || '未知知识库'}] 吗？对应底层的所有向量索引与文件记录将被永久清除。`,
  };
}

function promptDeleteDoc(doc_id: string, title: string) {
  const targetId = doc_id || (detailDocModal.value.doc?.doc_id || '');
  if (!targetId) return;
  confirmDeleteModal.value = {
    show: true,
    type: 'doc',
    id: targetId,
    message: `确定要删除文档 [${title || '未命名文档'}] 吗？关联的 MySQL Parent-Child 切片与 Milvus 向量将被同步清除。`,
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

function openUploadModal() {
  uploadErrorMsg.value = '';
  if (isDefaultKB.value) {
    uploadErrorMsg.value = '默认知识库 (kb_default_system) 目前只能项目初始化时处理，禁止通过接口添加';
  }
  showUploadModal.value = true;
}

function triggerFileInput() {
  if (isDefaultKB.value) {
    uploadErrorMsg.value = '默认知识库 (kb_default_system) 目前只能项目初始化时处理，禁止通过接口添加';
    return;
  }
  fileInputRef.value?.click();
}

function handleDrop(e: DragEvent) {
  isDragging.value = false;
  if (isDefaultKB.value) {
    uploadErrorMsg.value = '默认知识库 (kb_default_system) 目前只能项目初始化时处理，禁止通过接口添加';
    return;
  }
  const files = e.dataTransfer?.files;
  if (files && files.length > 0) {
    selectedFile.value = files[0];
    uploadErrorMsg.value = '';
  }
}

function handleFileSelected(e: Event) {
  if (isDefaultKB.value) {
    uploadErrorMsg.value = '默认知识库 (kb_default_system) 目前只能项目初始化时处理，禁止通过接口添加';
    return;
  }
  const target = e.target as HTMLInputElement;
  if (target.files && target.files.length > 0) {
    selectedFile.value = target.files[0];
    uploadErrorMsg.value = '';
  }
}

function clearSelectedFile() {
  selectedFile.value = null;
  if (fileInputRef.value) {
    fileInputRef.value.value = '';
  }
}

function closeUploadModal() {
  if (kbStore.uploading) return;
  showUploadModal.value = false;
  clearSelectedFile();
  uploadErrorMsg.value = '';
}

async function startUploadSelectedFile() {
  if (!selectedFile.value) return;
  if (isDefaultKB.value) {
    uploadErrorMsg.value = '默认知识库目前只能项目初始化时处理，禁止通过接口添加';
    return;
  }
  uploadErrorMsg.value = '';
  uploadSuccessMsg.value = '';
  try {
    const res = await kbStore.uploadFile(selectedFile.value);
    uploadSuccessMsg.value = `文档 [${res.title}] 增量解析成功！分割生成 ${res.totalChunks} 个 Parent-Child 向量切片。`;
    showUploadModal.value = false;
    clearSelectedFile();
  } catch (err: any) {
    uploadErrorMsg.value = err.message || '文档解析上传失败';
  }
}

function getFileExtension(filename: string): string {
  if (!filename) return 'TXT';
  const parts = filename.split('.');
  return parts.length > 1 ? parts.pop()!.toLowerCase() : 'TXT';
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
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
        bg: 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300 px-2.5 py-0.5 rounded-full',
        dot: 'bg-emerald-400 animate-pulse',
      };
    case 1:
      return {
        text: '解析向量化中',
        bg: 'bg-blue-500/10 border-blue-500/30 text-blue-300 px-2.5 py-0.5 rounded-full',
        spin: true,
      };
    case 0:
      return {
        text: '排队待处理',
        bg: 'bg-amber-500/10 border-amber-500/30 text-amber-300 px-2.5 py-0.5 rounded-full',
        dot: 'bg-amber-400',
      };
    case 3:
    default:
      return {
        text: '解析失败',
        bg: 'bg-rose-500/10 border-rose-500/30 text-rose-300 px-2.5 py-0.5 rounded-full',
        dot: 'bg-rose-400',
      };
  }
}

function truncateString(str?: string, length = 14) {
  if (!str) return '-';
  if (str.length <= length) return str;
  return str.slice(0, length) + '...';
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

<style scoped>
.kb-workspace-root {
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
.kb-header {
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

/* 主体容器 */
.kb-body-container {
  flex: 1;
  display: flex;
  overflow: hidden;
}

/* 左侧 Sidebar */
.kb-sidebar {
  width: 290px;
  min-width: 290px;
  background: #080b16;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex-shrink: 0;
}

.sidebar-header-box {
  padding: 1.25rem 1rem 0.875rem 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.sidebar-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.75rem;
  font-weight: 700;
  color: #94a3b8;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.sidebar-search-wrapper {
  position: relative;
  width: 100%;
}

.sidebar-search-icon {
  position: absolute;
  left: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: #64748b;
  width: 14px;
  height: 14px;
  pointer-events: none;
}

.sidebar-search-input {
  width: 100%;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  padding: 0.5rem 0.75rem 0.5rem 2.25rem;
  font-size: 0.75rem;
  color: #f1f5f9;
  outline: none;
  transition: all 0.2s ease;
}

.sidebar-search-input:focus {
  border-color: rgba(6, 182, 212, 0.5);
  background: rgba(255, 255, 255, 0.07);
}

.sidebar-kb-list {
  flex: 1;
  overflow-y: auto;
  padding: 0.875rem 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.kb-card {
  padding: 1rem;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.kb-card:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.18);
  transform: translateY(-1px);
}

.kb-card.active {
  background: linear-gradient(135deg, rgba(6, 182, 212, 0.15), rgba(99, 102, 241, 0.1));
  border-color: rgba(6, 182, 212, 0.5);
  box-shadow: 0 4px 16px rgba(6, 182, 212, 0.12);
}

.sidebar-footer-box {
  padding: 0.75rem 1rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(0, 0, 0, 0.2);
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.7rem;
  color: #64748b;
}

/* 右侧 Main Content */
.kb-main-content {
  flex: 1;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: #0b0e1a;
}

.kb-content-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 1.75rem 2rem;
  display: flex;
  flex-direction: column;
  gap: 1.75rem;
}

/* KPI 卡片网格 */
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

/* Filter Toolbar Card */
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
  min-width: 280px;
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

/* Document Table Card */
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
