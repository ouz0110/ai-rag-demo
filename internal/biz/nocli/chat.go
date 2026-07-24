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
	}

	messages, err := s.loadHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role: "user", Content: req.Message,
	})

	userMsg, _ := json.Marshal(openai.ChatCompletionMessage{
		Role: "user", Content: req.Message,
	})
	if err := s.allDb.Base.NocliMessageRepo.Create(ctx, &dataBase.NocliMessageModel{
		SessionID: sessionID,
		Msg:       string(userMsg),
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		return nil, fmt.Errorf("保存用户消息失败: %v", err)
	}

	tools := s.toolRegistry.BuildTools()

	reply, err := s.runChatLoop(ctx, messages, tools, req.Model)
	if err != nil {
		return nil, fmt.Errorf("对话失败: %v", err)
	}

	assistantMsg, _ := json.Marshal(openai.ChatCompletionMessage{
		Role: "assistant", Content: reply,
	})
	if err := s.allDb.Base.NocliMessageRepo.Create(ctx, &dataBase.NocliMessageModel{
		SessionID: sessionID,
		Msg:       string(assistantMsg),
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		return nil, fmt.Errorf("保存助手消息失败: %v", err)
	}

	s.allDb.Base.NocliSessionRepo.UpdateUpdatedAt(ctx, sessionID, time.Now().Unix())

	return &pb.CompletionResponse{
		Reply:     reply,
		SessionId: sessionID,
	}, nil
}

func (s *ChatBiz) runChatLoop(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool, model string) (string, error) {
	if model == "" {
		model = s.cfg.Source.OpenAI.Model
	}

	for {
		req := openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
			Tools:    tools,
		}

		client := s.openaiChatModel.GetOpenAI(ctx)
		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", fmt.Errorf("OpenAI 调用失败: %v", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("OpenAI 返回空响应")
		}

		choice := resp.Choices[0]

		if choice.Message.Content == "" {
			continue
		}

		msg := choice.Message
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		})

		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		})

		for _, tc := range msg.ToolCalls {
			result, err := s.toolRegistry.Call(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("工具执行失败: %v", err)
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
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

	if err := s.allDb.Base.NocliMessageRepo.DeleteBySessionID(ctx, sessionID); err != nil {
		return err
	}

	now := time.Now().Unix()
	models := make([]*dataBase.NocliMessageModel, 0, len(messages))
	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		models = append(models, &dataBase.NocliMessageModel{
			SessionID: sessionID,
			Msg:       string(data),
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
