export interface KnowledgeBase {
  kb_id: string;
  name: string;
  description: string;
  is_default: boolean;
  tenant_id: string;
  created_at: number;
  updated_at: number;
}

export interface KnowledgeDocument {
  doc_id: string;
  kb_id: string;
  title: string;
  source_type: string;
  doc_version?: string;
  category?: string;
  tags?: string;
  is_active?: number;
  source_url?: string;
  total_chunks: number;
  embedding_cost?: number;
  status: number; // 0:待处理, 1:解析中, 2:已向量化, 3:失败
  err_msg?: string;
  file_path?: string;
  file_hash?: string;
  created_at?: string | number;
  updated_at?: string | number;
}

export interface CreateKBRequest {
  name: string;
  description: string;
}

export interface UploadFileResponse {
  docId: string;
  kbId: string;
  title: string;
  sourceType: string;
  totalChunks: number;
  status: number;
}

export interface RetrievalResult {
  chunk_id: string;
  doc_id: string;
  parent_id: string;
  score: number;
  content: string;
  full_context: string;
}
