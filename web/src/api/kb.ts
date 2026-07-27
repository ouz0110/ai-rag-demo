import { http } from './http';
import type {
  KnowledgeBase,
  KnowledgeDocument,
  CreateKBRequest,
  UploadFileResponse,
} from '../types/kb';

// 1. 获取全量知识库列表 (GET /nocli/v1/kb/list)
export async function listKnowledgeBases(): Promise<{ kbs: KnowledgeBase[] }> {
  return http.get('/nocli/v1/kb/list');
}

// 2. 创建自定义知识库 (POST /nocli/v1/kb/create)
export async function createKnowledgeBase(
  data: CreateKBRequest
): Promise<{ kb: KnowledgeBase }> {
  return http.post('/nocli/v1/kb/create', data);
}

// 3. 删除自定义知识库 (POST /nocli/v1/kb/delete)
export async function deleteKnowledgeBase(
  kb_id: string
): Promise<{ success: boolean }> {
  return http.post('/nocli/v1/kb/delete', { kb_id });
}

// 4. 上传文档二进制并触发增量解析 (POST /nocli/v1/rag/upload - multipart/form-data)
export async function uploadDocumentFile(
  file: File,
  kb_id?: string
): Promise<UploadFileResponse> {
  const formData = new FormData();
  formData.append('file', file);
  if (kb_id) {
    formData.append('kb_id', kb_id);
  }

  return http.post('/nocli/v1/rag/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });
}

// 5. 获取指定知识库下的文档列表 (GET /nocli/v1/kb/doc/list)
export async function listDocuments(
  kb_id?: string
): Promise<{ docs: KnowledgeDocument[] }> {
  return http.get('/nocli/v1/kb/doc/list', { params: { kb_id } });
}

// 6. 删除文档及其关联切片 (POST /nocli/v1/kb/doc/delete)
export async function deleteDocument(
  doc_id: string
): Promise<{ success: boolean }> {
  return http.post('/nocli/v1/kb/doc/delete', { doc_id });
}
