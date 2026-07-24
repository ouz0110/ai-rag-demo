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
		if _, ok, err := s.allDb.Base.NocliSessionRepo.GetBySessionID(ctx, sessionID); err != nil || !ok {
			return nil, fmt.Errorf("会话不存在")
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
	messages, reply, err := s.runChatLoop(ctx, sessionID, messages, tools, req.Model)
	duration := time.Since(start)
	if err != nil {
		log.Errorw(ctx, "completion_error", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "error", err)
		return nil, fmt.Errorf("对话失败: %v", err)
	}

	if err := s.saveHistory(ctx, sessionID, messages[newMessageStart:]); err != nil {
		return nil, fmt.Errorf("保存对话历史失败: %v", err)
	}

	log.Debugw(ctx, "completion_end", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "reply_len", len(reply))

	return &pb.CompletionResponse{
		Reply:     reply,
		SessionId: sessionID,
	}, nil
}

func (s *ChatBiz) runChatLoop(ctx context.Context, sessionID string, messages []openai.ChatCompletionMessage, tools []openai.Tool, model string) ([]openai.ChatCompletionMessage, string, error) {
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
			return nil, "", fmt.Errorf("OpenAI 调用失败: %v", err)
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
			return messages, msg.Content, nil
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
	return messages, nil
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
