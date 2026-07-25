package nocli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	codeanalyzer "ai-rag-demo/internal/biz/nocli/openai/prompt/code_analyzer"
	tool "ai-rag-demo/internal/biz/nocli/openai/tool"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	openai "github.com/sashabaranov/go-openai"
)

type ChatBiz struct {
	cache           *cache.Cache
	openaiChatModel *chatmodel.ChatModel
	toolRegistry    *tool.Registry
	cfg             *conf.Config
	allDb           *data.DB
}

type LoopResult struct {
	Messages         []openai.ChatCompletionMessage
	Reply            string
	Status           pb.SessionStatus
	PendingToolCalls []*pb.PendingToolCall
}

func NewChatBiz(
	cache *cache.Cache,
	openaiChatModel *chatmodel.ChatModel,
	cfg *conf.Config,
	allDb *data.DB,
) *ChatBiz {
	return &ChatBiz{
		cache:           cache,
		openaiChatModel: openaiChatModel,
		toolRegistry:    tool.NewRegistry(cfg),
		cfg:             cfg,
		allDb:           allDb,
	}
}

func (s *ChatBiz) Completion(ctx context.Context, req *pb.CompletionRequest) (rsp *pb.CompletionResponse, err error) {
	sessionID := req.SessionId
	workDir := s.cfg.Source.Nocli.WorkDir
	if workDir == "" {
		workDir = "."
	}

	systemPrompt := codeanalyzer.SystemPrompt(workDir)
	_, user := common.UserFromContext(ctx)
	now := time.Now().Unix()

	log.Debugw(ctx, "completion_start", "has_session", sessionID != "", "model", req.Model, "message_len", len(req.Message))

	if sessionID == "" {
		sessionID = utils.NewUUID()
		session := &dataBase.NocliSessionModel{
			SessionID: sessionID,
			Openid:    user.Openid,
			Name:      truncateText(req.Message, 30),
			Status:    pb.SessionStatus_SESSION_STATUS_RUNNING,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.allDb.Base.NocliSessionRepo.Create(ctx, session); err != nil {
			return nil, fmt.Errorf("创建会话失败: %v", err)
		}
		log.Debugw(ctx, "session_created", "session_id", sessionID)

		sysMsg, _ := json.Marshal(openai.ChatCompletionMessage{
			Role: "system", Content: systemPrompt,
		})
		if err := s.allDb.Base.NocliMessageRepo.Create(ctx, &dataBase.NocliMessageModel{
			SessionID: sessionID,
			Msg:       string(sysMsg),
			CreatedAt: now,
		}); err != nil {
			return nil, fmt.Errorf("保存系统消息失败: %v", err)
		}
	} else {
		session, ok, err := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
		if err != nil || !ok {
			return nil, fmt.Errorf("会话不存在")
		}

		// 若之前有尚未确认的中断，当用户直接发送新提问时，自动作废/取消未决的中断
		if session.Status == pb.SessionStatus_SESSION_STATUS_INTERRUPTED {
			pendingModels, err := s.allDb.Base.NocliInterruptRepo.GetPendingBySessionID(ctx, sessionID)
			if err == nil && len(pendingModels) > 0 {
				cancelMsgs := make([]openai.ChatCompletionMessage, 0, len(pendingModels))
				for _, pm := range pendingModels {
					_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(ctx, pm.InterruptID, pb.InterruptStatus_INTERRUPT_STATUS_EXPIRED, now, user.Openid, "用户发送了新问题，自动取消旧中断")
					cancelMsgs = append(cancelMsgs, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    "操作已取消：用户提交了新的提问，放弃了此授权操作",
						ToolCallID: pm.ToolCallID,
					})
				}
				_ = s.saveHistory(ctx, sessionID, cancelMsgs)
			}
			_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SESSION_STATUS_RUNNING)
		} else {
			_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SESSION_STATUS_RUNNING)
		}
		log.Debugw(ctx, "session_loaded", "session_id", sessionID)
	}

	messages, err := s.loadHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	// 记录新消息的起始位置，用于后续保存
	newMessageStart := len(messages)

	messages = append(messages, openai.ChatCompletionMessage{
		Role: "user", Content: req.Message,
	})

	tools := s.toolRegistry.BuildTools()

	start := time.Now()
	loopRes, err := s.runChatLoop(ctx, sessionID, messages, tools, req.Model)
	duration := time.Since(start)
	if err != nil {
		log.Errorw(ctx, "completion_error", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "error", err)
		return nil, fmt.Errorf("对话失败: %v", err)
	}

	if err := s.saveHistory(ctx, sessionID, loopRes.Messages[newMessageStart:]); err != nil {
		return nil, fmt.Errorf("保存对话历史失败: %v", err)
	}

	if loopRes.Status == pb.SessionStatus_SESSION_STATUS_IDLE {
		_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SESSION_STATUS_IDLE)
	}

	log.Debugw(ctx, "completion_end", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "reply_len", len(loopRes.Reply))

	return &pb.CompletionResponse{
		Reply:            loopRes.Reply,
		SessionId:        sessionID,
		Status:           loopRes.Status,
		PendingToolCalls: loopRes.PendingToolCalls,
	}, nil
}

func (s *ChatBiz) Resume(ctx context.Context, req *pb.ResumeRequest) (*pb.CompletionResponse, error) {
	_, user := common.UserFromContext(ctx)
	sessionID := req.SessionId
	interruptID := req.InterruptId

	if sessionID == "" || interruptID == "" {
		return nil, fmt.Errorf("session_id 与 interrupt_id 不能为空")
	}

	session, ok, err := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
	if err != nil || !ok {
		return nil, fmt.Errorf("会话不存在")
	}
	if session.Status != pb.SessionStatus_SESSION_STATUS_INTERRUPTED {
		return nil, fmt.Errorf("会话当前不处于中断挂起状态")
	}

	interrupt, ok, err := s.allDb.Base.NocliInterruptRepo.GetByInterruptID(ctx, interruptID)
	if err != nil || !ok {
		return nil, fmt.Errorf("中断记录不存在")
	}
	if interrupt.Status != pb.InterruptStatus_INTERRUPT_STATUS_PENDING {
		return nil, fmt.Errorf("该中断已被处理或已失效")
	}

	now := time.Now().Unix()
	var toolResult string

	if req.Action == pb.ResumeAction_RESUME_ACTION_APPROVE {
		result, err := s.toolRegistry.Call(ctx, interrupt.ToolName, interrupt.Arguments)
		if err != nil {
			toolResult = fmt.Sprintf("工具执行失败: %v", err)
		} else {
			toolResult = result
		}
		_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(ctx, interruptID, pb.InterruptStatus_INTERRUPT_STATUS_APPROVED, now, user.Openid, req.Reason)
	} else {
		reason := req.Reason
		if reason == "" {
			reason = "用户拒绝执行该操作"
		}
		toolResult = fmt.Sprintf("操作被用户拒绝: %s", reason)
		_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(ctx, interruptID, pb.InterruptStatus_INTERRUPT_STATUS_REJECTED, now, user.Openid, reason)
	}

	toolMsg := openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    toolResult,
		ToolCallID: interrupt.ToolCallID,
	}

	if err := s.saveHistory(ctx, sessionID, []openai.ChatCompletionMessage{toolMsg}); err != nil {
		return nil, fmt.Errorf("保存工具操作结果失败: %v", err)
	}

	_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SESSION_STATUS_RUNNING)

	messages, err := s.loadHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	tools := s.toolRegistry.BuildTools()
	start := time.Now()
	loopRes, err := s.runChatLoop(ctx, sessionID, messages, tools, req.Model)
	duration := time.Since(start)
	if err != nil {
		log.Errorw(ctx, "resume_error", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "error", err)
		return nil, fmt.Errorf("恢复对话后继续执行失败: %v", err)
	}

	newMessageStart := len(messages)
	if len(loopRes.Messages) > newMessageStart {
		if err := s.saveHistory(ctx, sessionID, loopRes.Messages[newMessageStart:]); err != nil {
			return nil, fmt.Errorf("保存恢复后的对话历史失败: %v", err)
		}
	}

	if loopRes.Status == pb.SessionStatus_SESSION_STATUS_IDLE {
		_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SESSION_STATUS_IDLE)
	}

	return &pb.CompletionResponse{
		Reply:            loopRes.Reply,
		SessionId:        sessionID,
		Status:           loopRes.Status,
		PendingToolCalls: loopRes.PendingToolCalls,
	}, nil
}

func (s *ChatBiz) runChatLoop(ctx context.Context, sessionID string, messages []openai.ChatCompletionMessage, tools []openai.Tool, model string) (*LoopResult, error) {
	if model == "" {
		model = s.cfg.Source.OpenAI.Model
	}

	baseFields := []interface{}{
		"session_id", sessionID,
		"model", model,
	}

	log.Debugw(ctx, "chat_loop_start", append(baseFields, "messages_count", len(messages), "tools_count", len(tools))...)
	loopStart := time.Now()
	iteration := 0
	totalToolCalls := 0

	for {
		iteration++

		log.Debugw(ctx, "llm_call_start", append(baseFields, "iteration", iteration, "messages_count", len(messages), "tools_count", len(tools))...)

		req := openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
			Tools:    tools,
		}

		client := s.openaiChatModel.GetOpenAI(ctx)
		callStart := time.Now()
		resp, err := client.CreateChatCompletion(ctx, req)
		callDuration := time.Since(callStart)
		if err != nil {
			log.Errorw(ctx, "llm_call_error", append(baseFields, "iteration", iteration, "duration_ms", callDuration.Milliseconds(), "error", err)...)
			return nil, fmt.Errorf("OpenAI 调用失败: %v", err)
		}

		choice := resp.Choices[0]
		msg := choice.Message

		log.Debugw(ctx, "llm_call_end", append(baseFields, "iteration", iteration, "duration_ms", callDuration.Milliseconds(), "finish_reason", choice.FinishReason)...)

		if msg.ReasoningContent != "" {
			log.Debugw(ctx, "llm_reasoning", append(baseFields, "iteration", iteration, "content_len", len(msg.ReasoningContent), "content", truncateText(msg.ReasoningContent, 500))...)
		}

		if msg.Content == "" && len(msg.ToolCalls) == 0 {
			continue
		}

		if msg.Content != "" {
			log.Debugw(ctx, "llm_content", append(baseFields, "iteration", iteration, "content_len", len(msg.Content), "content", truncateText(msg.Content, 500))...)
		}

		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			log.Debugw(ctx, "chat_loop_end", append(baseFields, "total_iterations", iteration, "total_tool_calls", totalToolCalls, "loop_duration_ms", time.Since(loopStart).Milliseconds())...)
			return &LoopResult{
				Messages: messages,
				Reply:    msg.Content,
				Status:   pb.SessionStatus_SESSION_STATUS_IDLE,
			}, nil
		}

		// 检查是否有任何工具需要人工审批确认
		var pendingInterrupts []*dataBase.NocliInterruptModel
		var pendingToolCalls []*pb.PendingToolCall

		for _, tc := range msg.ToolCalls {
			if s.toolRegistry.RequiresApproval(tc.Function.Name) {
				interruptID := utils.NewUUID()
				pendingInterrupts = append(pendingInterrupts, &dataBase.NocliInterruptModel{
					InterruptID:   interruptID,
					SessionID:     sessionID,
					Status:        pb.InterruptStatus_INTERRUPT_STATUS_PENDING,
					ToolCallID:    tc.ID,
					ToolName:      tc.Function.Name,
					Arguments:     tc.Function.Arguments,
					CreatedAt:     time.Now().Unix(),
				})
				pendingToolCalls = append(pendingToolCalls, &pb.PendingToolCall{
					InterruptId: interruptID,
					ToolCallId:  tc.ID,
					ToolName:    tc.Function.Name,
					Arguments:   tc.Function.Arguments,
				})
			}
		}

		if len(pendingInterrupts) > 0 {
			if err := s.allDb.Base.NocliInterruptRepo.CreateBatch(ctx, pendingInterrupts); err != nil {
				return nil, fmt.Errorf("创建中断记录失败: %v", err)
			}
			_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SESSION_STATUS_INTERRUPTED)

			log.Debugw(ctx, "chat_loop_interrupted", append(baseFields, "pending_count", len(pendingInterrupts))...)

			return &LoopResult{
				Messages:         messages,
				Reply:            "包含需要授权确认的操作，请审批后恢复执行",
				Status:           pb.SessionStatus_SESSION_STATUS_INTERRUPTED,
				PendingToolCalls: pendingToolCalls,
			}, nil
		}

		for _, tc := range msg.ToolCalls {
			totalToolCalls++
			log.Debugw(ctx, "tool_call", append(baseFields, "iteration", iteration, "tool_index", totalToolCalls, "tool_id", tc.ID, "tool_name", tc.Function.Name, "args_len", len(tc.Function.Arguments))...)

			result, err := s.toolRegistry.Call(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("工具执行失败: %v", err)
				log.Debugw(ctx, "tool_result", append(baseFields, "iteration", iteration, "tool_id", tc.ID, "tool_name", tc.Function.Name, "result_len", len(result), "error", err)...)
			} else {
				log.Debugw(ctx, "tool_result", append(baseFields, "iteration", iteration, "tool_id", tc.ID, "tool_name", tc.Function.Name, "result_len", len(result))...)
			}

			toolMsg := openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolMsg)
		}
	}
}

func (s *ChatBiz) loadHistory(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	models, err := s.allDb.Base.NocliMessageRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	messages := make([]openai.ChatCompletionMessage, 0, len(models))
	for _, m := range models {
		var msg openai.ChatCompletionMessage
		if err := json.Unmarshal([]byte(m.Msg), &msg); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	// 容错修复：确保历史消息中悬空的 tool_calls 一定有对应的 tool 响应，防止调用 OpenAI 报 400 错
	messages = sanitizeUnclosedToolCalls(messages)

	return messages, nil
}

func sanitizeUnclosedToolCalls(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	respondedToolIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleTool && msg.ToolCallID != "" {
			respondedToolIDs[msg.ToolCallID] = true
		}
	}

	sanitized := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		sanitized = append(sanitized, msg)
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if !respondedToolIDs[tc.ID] {
					sanitized = append(sanitized, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    "操作已取消：未获得用户明确授权响应",
						ToolCallID: tc.ID,
					})
					respondedToolIDs[tc.ID] = true
				}
			}
		}
	}

	return sanitized
}

func (s *ChatBiz) saveHistory(ctx context.Context, sessionID string, messages []openai.ChatCompletionMessage) error {
	if len(messages) == 0 {
		return nil
	}

	_ = s.allDb.Base.NocliSessionRepo.UpdateUpdatedAt(ctx, sessionID, time.Now().Unix())

	now := time.Now().Unix()
	models := make([]*dataBase.NocliMessageModel, 0, len(messages))
	for _, msg := range messages {
		str, err := msg.MarshalJSON()
		if err != nil {
			return err
		}
		models = append(models, &dataBase.NocliMessageModel{
			SessionID: sessionID,
			Msg:       string(str),
			CreatedAt: now,
		})
	}

	return s.allDb.Base.NocliMessageRepo.CreateBatch(ctx, models)
}

func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return text
}
