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
			Status:    pb.SessionStatus_SS_RUNNING,
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
		_, ok, err := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID)
		if err != nil || !ok {
			return nil, fmt.Errorf("会话不存在")
		}

		_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_RUNNING)
		log.Debugw(ctx, "session_loaded", "session_id", sessionID)
	}

	messages, err := s.loadHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	// 最外层记录增量保存的起点
	newMessageStart := len(messages)

	// 若之前有未处理中断，当用户直接发新提问时，自动作废旧中断并将取消消息 append 到内存消息数组中
	cancelMsgs, _ := s.cancelPendingInterruptsOnNewCompletion(ctx, sessionID)
	if len(cancelMsgs) > 0 {
		messages = append(messages, cancelMsgs...)
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role: "user", Content: req.Message,
	})

	tools := s.toolRegistry.BuildTools()

	start := time.Now()
	loopRes, err := s.runChatLoop(ctx, sessionID, messages, tools, req.Model, nil)
	duration := time.Since(start)
	if err != nil {
		log.Errorw(ctx, "completion_error", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "error", err)
		return nil, fmt.Errorf("对话失败: %v", err)
	}

	// 🎯 消息存储固定在最外层：统一拉取全量新产生的 messages 批量落盘
	if len(loopRes.Messages) > newMessageStart {
		if err := s.saveHistory(ctx, sessionID, loopRes.Messages[newMessageStart:]); err != nil {
			return nil, fmt.Errorf("保存对话历史失败: %v", err)
		}
	}

	if loopRes.Status == pb.SessionStatus_SS_IDLE {
		_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_IDLE)
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
	if session.Status != pb.SessionStatus_SS_INTERRUPTED {
		return nil, fmt.Errorf("会话当前不处于中断挂起状态")
	}

	interrupt, ok, err := s.allDb.Base.NocliInterruptRepo.GetByInterruptID(ctx, interruptID)
	if err != nil || !ok {
		return nil, fmt.Errorf("中断记录不存在")
	}
	if interrupt.Status != pb.InterruptStatus_IS_PENDING {
		return nil, fmt.Errorf("该中断已被处理或已失效")
	}

	messages, err := s.loadHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	// 最外层记录增量保存的起点
	newMessageStart := len(messages)

	// 1. 调用 executeResumeTool 执行单次中断决策并获取产生的 Tool 消息
	toolMsg, err := s.executeResumeTool(ctx, user.Openid, interrupt, req.Action, req.Reason)
	if err != nil {
		return nil, fmt.Errorf("执行中断恢复工具失败: %v", err)
	}
	messages = append(messages, toolMsg)

	// 2. 检查同会话中是否还有其他处于 IS_PENDING 状态的待审批中断
	remainingPendingModels, err := s.allDb.Base.NocliInterruptRepo.GetPendingBySessionID(ctx, sessionID)
	if err == nil && len(remainingPendingModels) > 0 {
		// 尚有待审批中断：在最外层落盘已产生的 Tool 消息，会话保持 SS_INTERRUPTED，不进入 runChatLoop
		_ = s.saveHistory(ctx, sessionID, messages[newMessageStart:])
		return &pb.CompletionResponse{
			Reply:            "当前操作已审批，尚有其他待授权确认的操作，请继续审批",
			SessionId:        sessionID,
			Status:           pb.SessionStatus_SS_INTERRUPTED,
			PendingToolCalls: buildPendingToolCalls(remainingPendingModels),
		}, nil
	}

	// 3. 所有 Pending 中断均已处理，恢复会话状态为 SS_RUNNING 并启动 Agent 循环
	_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_RUNNING)

	approvedTools := make(map[string]bool)
	if req.Action == pb.ResumeAction_RA_APPROVE && req.ApproveScope == pb.ApproveScope_AS_SESSION_TOOL {
		approvedTools[interrupt.ToolName] = true
	}

	tools := s.toolRegistry.BuildTools()
	start := time.Now()
	loopRes, err := s.runChatLoop(ctx, sessionID, messages, tools, req.Model, approvedTools)
	duration := time.Since(start)
	if err != nil {
		log.Errorw(ctx, "resume_error", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "error", err)
		return nil, fmt.Errorf("恢复对话后继续执行失败: %v", err)
	}

	// 🎯 消息存储固定在最外层：统一将 Resume 生成的 Tool 消息 + 后续 LLM 生成的新消息进行单次批量保存
	if len(loopRes.Messages) > newMessageStart {
		if err := s.saveHistory(ctx, sessionID, loopRes.Messages[newMessageStart:]); err != nil {
			return nil, fmt.Errorf("保存恢复后的对话历史失败: %v", err)
		}
	}

	if loopRes.Status == pb.SessionStatus_SS_IDLE {
		_ = s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_IDLE)
	}

	return &pb.CompletionResponse{
		Reply:            loopRes.Reply,
		SessionId:        sessionID,
		Status:           loopRes.Status,
		PendingToolCalls: loopRes.PendingToolCalls,
	}, nil
}

func (s *ChatBiz) runChatLoop(ctx context.Context, sessionID string, messages []openai.ChatCompletionMessage, tools []openai.Tool, model string, approvedTools map[string]bool) (*LoopResult, error) {
	if model == "" {
		model = s.cfg.Source.OpenAI.Model
	}
	if approvedTools == nil {
		approvedTools = make(map[string]bool)
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
				Status:   pb.SessionStatus_SS_IDLE,
			}, nil
		}

		// 调用独立的 processToolCalls 处理（不插入单独的 saveHistory）
		res, err := s.processToolCalls(ctx, sessionID, baseFields, msg.ToolCalls, approvedTools, &totalToolCalls)
		if err != nil {
			return nil, err
		}

		if len(res.ExecutedMsgs) > 0 {
			messages = append(messages, res.ExecutedMsgs...)
		}

		// 若发生了中断，直接返回中断结构，消息的批量落盘由最外层统一接管
		if res.HasInterrupt {
			return &LoopResult{
				Messages:         messages,
				Reply:            "包含需要授权确认的操作，请审批后恢复执行",
				Status:           pb.SessionStatus_SS_INTERRUPTED,
				PendingToolCalls: []*pb.PendingToolCall{res.PendingToolCall},
			}, nil
		}
	}
}

func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return text
}
