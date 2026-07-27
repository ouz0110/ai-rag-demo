import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import type { KnowledgeBase, KnowledgeDocument, UploadFileResponse } from '../types/kb';
import {
  listKnowledgeBases,
  createKnowledgeBase,
  deleteKnowledgeBase,
  uploadDocumentFile,
  listDocuments,
  deleteDocument,
} from '../api/kb';

// 辅助标准化数据转化，容错处理后端 ProtoJSON (camelCase / snake_case 转换)
function normalizeDoc(doc: any): KnowledgeDocument {
  if (!doc) return doc;
  const docId = doc.doc_id || doc.docId || '';
  const kbId = doc.kb_id || doc.kbId || '';
  const fileHash = doc.file_hash || doc.fileHash || '';
  const filePath = doc.file_path || doc.filePath || '';
  const sourceType = doc.source_type || doc.sourceType || 'md';
  const totalChunks = typeof doc.total_chunks !== 'undefined' ? doc.total_chunks : (doc.totalChunks || 0);
  const createdAt = doc.created_at || doc.createdAt || 0;
  const updatedAt = doc.updated_at || doc.updatedAt || 0;
  const errMsg = doc.err_msg || doc.errMsg || '';
  const docVersion = doc.doc_version || doc.docVersion || '';
  const sourceUrl = doc.source_url || doc.sourceUrl || '';
  const isActive = typeof doc.is_active !== 'undefined' ? doc.is_active : doc.isActive;

  return {
    ...doc,
    doc_id: docId,
    kb_id: kbId,
    file_hash: fileHash,
    file_path: filePath,
    source_type: sourceType,
    total_chunks: totalChunks,
    created_at: createdAt,
    updated_at: updatedAt,
    err_msg: errMsg,
    doc_version: docVersion,
    source_url: sourceUrl,
    is_active: isActive,
  };
}

function normalizeKB(kb: any): KnowledgeBase {
  if (!kb) return kb;
  const kbId = kb.kb_id || kb.kbId || '';
  const isDefault = typeof kb.is_default !== 'undefined' ? kb.is_default : kb.isDefault;
  const tenantId = kb.tenant_id || kb.tenantId || 'default_tenant';
  const createdAt = kb.created_at || kb.createdAt || 0;
  const updatedAt = kb.updated_at || kb.updatedAt || 0;

  return {
    ...kb,
    kb_id: kbId,
    is_default: isDefault,
    tenant_id: tenantId,
    created_at: createdAt,
    updated_at: updatedAt,
  };
}

export const useKBStore = defineStore('kb', () => {
  const kbs = ref<KnowledgeBase[]>([]);
  const documents = ref<KnowledgeDocument[]>([]);
  const activeKbId = ref<string>('kb_default_system');
  const activeKbTenantId = ref<string>('default_tenant');
  const enableRAG = ref<boolean>(false); // 默认关闭 RAG 开关！只有用户明确开启才允许触发 RAG 检索
  const loading = ref<boolean>(false);
  const docLoading = ref<boolean>(false);
  const uploading = ref<boolean>(false);
  const error = ref<string | null>(null);

  function toggleRAG() {
    enableRAG.value = !enableRAG.value;
  }

  // 获取默认知识库
  const defaultKB = computed(() => {
    return kbs.value.find((k) => k.is_default || (k as any).isDefault) || null;
  });

  // 获取自定义知识库列表
  const customKBs = computed(() => {
    return kbs.value.filter((k) => !k.is_default && !(k as any).isDefault);
  });

  // 当前选中知识库的详细 Model
  const activeKB = computed(() => {
    return kbs.value.find((k) => k.kb_id === activeKbId.value || (k as any).kbId === activeKbId.value) || null;
  });

  // 加载全量知识库列表
  async function fetchKBs() {
    loading.value = true;
    error.value = null;
    try {
      const res = await listKnowledgeBases();
      if (res && res.kbs) {
        kbs.value = res.kbs.map(normalizeKB);
        // 自动校验并设置 activeKbId，确保始终对应真实存在的 KB
        const exists = kbs.value.some((k) => k.kb_id === activeKbId.value);
        if (!exists && kbs.value.length > 0) {
          const def = kbs.value.find((k) => k.is_default);
          const target = def || kbs.value[0];
          activeKbId.value = target.kb_id;
          activeKbTenantId.value = target.tenant_id || 'default_tenant';
        }
      }
      // 加载选定 KB 的文档列表
      await fetchDocuments(activeKbId.value);
    } catch (err: any) {
      error.value = err.message || '获取知识库列表失败';
    } finally {
      loading.value = false;
    }
  }

  // 获取文档列表
  async function fetchDocuments(targetKbId?: string) {
    const kbIdToQuery = targetKbId !== undefined ? targetKbId : activeKbId.value;
    docLoading.value = true;
    try {
      const res = await listDocuments(kbIdToQuery);
      if (res && res.docs) {
        documents.value = res.docs.map(normalizeDoc);
      } else {
        documents.value = [];
      }
    } catch (err: any) {
      console.error('Fetch documents error:', err);
    } finally {
      docLoading.value = false;
    }
  }

  // 创建知识库
  async function createKB(name: string, description: string) {
    loading.value = true;
    error.value = null;
    try {
      const res = await createKnowledgeBase({ name, description });
      if (res && res.kb) {
        const normalized = normalizeKB(res.kb);
        kbs.value.unshift(normalized);
        activeKbId.value = normalized.kb_id;
        activeKbTenantId.value = normalized.tenant_id || 'default_tenant';
        await fetchDocuments(normalized.kb_id);
        return normalized;
      }
    } catch (err: any) {
      error.value = err.message || '创建知识库失败';
      throw err;
    } finally {
      loading.value = false;
    }
  }

  // 删除知识库
  async function removeKB(kb_id: string) {
    loading.value = true;
    error.value = null;
    try {
      await deleteKnowledgeBase(kb_id);
      kbs.value = kbs.value.filter((k) => k.kb_id !== kb_id && (k as any).kbId !== kb_id);
      if (activeKbId.value === kb_id) {
        const def = defaultKB.value;
        if (def) {
          activeKbId.value = def.kb_id;
          activeKbTenantId.value = def.tenant_id || 'default_tenant';
        } else {
          activeKbId.value = 'kb_default_system';
          activeKbTenantId.value = 'default_tenant';
        }
        await fetchDocuments(activeKbId.value);
      }
    } catch (err: any) {
      error.value = err.message || '删除知识库失败';
      throw err;
    } finally {
      loading.value = false;
    }
  }

  // 删除指定文档及其向量切片
  async function removeDocument(doc_id: string) {
    docLoading.value = true;
    error.value = null;
    try {
      await deleteDocument(doc_id);
      documents.value = documents.value.filter((d) => d.doc_id !== doc_id && (d as any).docId !== doc_id);
    } catch (err: any) {
      error.value = err.message || '删除文档失败';
      throw err;
    } finally {
      docLoading.value = false;
    }
  }

  // 切换当前选中的知识库
  async function selectKB(kb_id: string) {
    const target = kbs.value.find((k) => k.kb_id === kb_id);
    if (target) {
      activeKbId.value = target.kb_id;
      activeKbTenantId.value = target.tenant_id || 'default_tenant';
    } else {
      activeKbId.value = kb_id;
    }
    await fetchDocuments(activeKbId.value);
  }

  // 上传文件并解析向量化
  async function uploadFile(file: File, targetKbId?: string): Promise<UploadFileResponse> {
    uploading.value = true;
    error.value = null;
    try {
      const kbIdToUse = targetKbId || activeKbId.value;
      const res = await uploadDocumentFile(file, kbIdToUse);
      await fetchDocuments(kbIdToUse);
      return res;
    } catch (err: any) {
      error.value = err.message || '文档上传或解析失败';
      throw err;
    } finally {
      uploading.value = false;
    }
  }

  return {
    kbs,
    documents,
    activeKbId,
    activeKbTenantId,
    enableRAG,
    loading,
    docLoading,
    uploading,
    error,
    defaultKB,
    customKBs,
    activeKB,
    fetchKBs,
    fetchDocuments,
    createKB,
    removeKB,
    removeDocument,
    selectKB,
    uploadFile,
    toggleRAG,
  };
});
