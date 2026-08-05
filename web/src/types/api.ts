// API 响应通用包装 (common.proto)
export interface CommonResponse<T = any> {
  code: number;
  message: string;
  cause?: string;
  request_id?: string;
  data: T;
  reason?: string;
}

// ---------------- Accounts 服务 ----------------
export interface Account {
  id: number | string;
  account: string;
  nickname: string;
  avatar: string;
  openid: string;
  status: number;
  createdAt: number;
  updatedAt: number;
  lastLoginTime?: number;
}

export interface RegisterRequest {
  account: string;
  password: string;
  nickname?: string;
  avatar?: string;
  openid?: string;
}

export interface RegisterResponse {
  token: string;
}

export interface LoginRequest {
  account: string;
  password: string;
}

export interface LoginResponse {
  token: string;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

// ---------------- Chat / Nocli 服务 ----------------

// 会话状态枚举
export const SessionStatus = {
  SS_UNSPECIFIED: 0,
  SS_IDLE: 1,        // 空闲/已完成
  SS_RUNNING: 2,     // 处理中
  SS_INTERRUPTED: 3, // 已中断，等待人工决策
  SS_PAUSED: 4,      // 用户主动停止/暂停 (可继续生成)
} as const;
export type SessionStatus = typeof SessionStatus[keyof typeof SessionStatus];

// 中断事件状态
export const InterruptStatus = {
  IS_UNSPECIFIED: 0,
  IS_PENDING: 1,  // 待审批
  IS_APPROVED: 2, // 已批准
  IS_REJECTED: 3, // 已拒绝
  IS_EXPIRED: 4,  // 已超时/取消
} as const;
export type InterruptStatus = typeof InterruptStatus[keyof typeof InterruptStatus];

// 恢复执行动作
export const ResumeAction = {
  RA_UNSPECIFIED: 0,
  RA_APPROVE: 1, // 批准执行
  RA_REJECT: 2,  // 拒绝执行
} as const;
export type ResumeAction = typeof ResumeAction[keyof typeof ResumeAction];

// 授权范围
export const ApproveScope = {
  AS_UNSPECIFIED: 0,
  AS_SINGLE_CALL: 1,  // 仅同意本次调用
  AS_SESSION_TOOL: 2, // 同意本轮/本会话后续所有同名工具调用
} as const;
export type ApproveScope = typeof ApproveScope[keyof typeof ApproveScope];

// SSE 流式事件类型
export const StreamEventType = {
  SET_UNSPECIFIED: 0,
  SET_TEXT_DELTA: 1,  // 文本回复增量 (打字机)
  SET_REASONING: 2,   // 思考/推理过程增量 (CoT)
  SET_TOOL_START: 3,  // 自动工具开始执行
  SET_TOOL_RESULT: 4, // 自动工具执行完成
  SET_INTERRUPT: 5,   // 触发中断挂起 (人工授权)
  SET_DONE: 6,        // 流传输正常完成
  SET_ERROR: 7,       // 异常错误
  SET_CONTEXT_COMPRESSED: 8, // 上下文压缩触发事件
} as const;
export type StreamEventType = typeof StreamEventType[keyof typeof StreamEventType];

// 待确认的工具调用信息
export interface PendingToolCall {
  interrupt_id: string;
  tool_call_id: string;
  tool_name: string;
  arguments: string;
  agent_name?: string;
}

// 流式工具执行信息
export interface StreamToolInfo {
  tool_call_id: string;
  tool_name: string;
  arguments: string;
  result_preview: string;
}

// 流式错误信息
export interface StreamError {
  code: number;
  message: string;
}

// 上下文压缩状态
export const CompressStatus = {
  CS_UNSPECIFIED: 0,
  CS_COMPRESSING: 1, // 压缩/提炼中
  CS_COMPLETED: 2,   // 压缩完成
} as const;
export type CompressStatus = typeof CompressStatus[keyof typeof CompressStatus];

// 上下文压缩事件详情
export interface CompressInfo {
  original_tokens?: number;
  compressed_tokens?: number;
  compressed_msg_count?: number;
  compress_count?: number;
  summary_preview?: string;
  is_max_limit_reached?: boolean;
  status?: CompressStatus;
}

// 统一数据 Chunk
export interface StreamChunk {
  event: StreamEventType | string | number;
  agent_name?: string;
  session_id?: string;
  status?: SessionStatus;
  text?: string;
  reasoning_text?: string;
  tool_info?: StreamToolInfo;
  pending_tool_calls?: PendingToolCall[];
  error?: StreamError;
  role?: string;
  compress_info?: CompressInfo;
}

// 补全请求
export interface CompletionRequest {
  message: string;
  session_id?: string;
  model?: string;
  kb_tenant_id?: string;
  kb_id?: string;
  enable_rag?: boolean;
  enable_skill?: boolean;
  enable_mcp?: boolean;
  enable_rerank?: boolean;
  agent_tool_options?: {
    pass_full_context_to_sub_agent?: boolean;
    return_full_context_to_parent?: boolean;
    stream_sub_agent_execution?: boolean;
  };
  is_continue?: boolean;
}

// 恢复执行请求
export interface ResumeRequest {
  session_id: string;
  interrupt_id: string;
  action: ResumeAction;
  approve_scope: ApproveScope;
  reason?: string;
  model?: string;
  kb_tenant_id?: string;
  kb_id?: string;
  enable_rag?: boolean;
  enable_skill?: boolean;
  enable_mcp?: boolean;
  enable_rerank?: boolean;
  agent_tool_options?: {
    pass_full_context_to_sub_agent?: boolean;
    return_full_context_to_parent?: boolean;
    stream_sub_agent_execution?: boolean;
  };
}

// 会话概览信息
export interface SessionInfo {
  session_id: string;
  name: string;
  status: SessionStatus;
  created_at: number;
  updated_at: number;
}

export interface Pagination {
  size?: number;
  number?: number;
  total?: number;
  sort?: string[];
}

// 会话列表请求与响应
export interface ListSessionsRequest {
  page?: Pagination;
}

export interface ListSessionsResponse {
  sessions: SessionInfo[];
  total?: number;
  page?: Pagination;
}

// 删除会话
export interface DeleteSessionRequest {
  session_id: string;
}

export interface DeleteSessionResponse {
  success: boolean;
  session_id: string;
}

// 获取会话历史
export interface GetSessionHistoryRequest {
  session_id: string;
  page?: Pagination;
}

export interface GetSessionHistoryResponse {
  session_id: string;
  status: SessionStatus;
  chunks: StreamChunk[];
  pending_tool_calls?: PendingToolCall[];
  total?: number;
  has_more?: boolean;
}
