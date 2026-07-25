package nocli

import (
	"context"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	tool "ai-rag-demo/internal/biz/nocli/openai/tool"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	"ai-rag-demo/internal/pkg/log"

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

func (s *ChatBiz) Completion(ctx context.Context, req *pb.CompletionRequest) (*pb.StreamChunk, error) {
	sessionID, err := s.initOrCreateSession(ctx, req.SessionId, req.Message)
	if err != nil {
		return nil, err
	}

	messages, newMessageStart, err := s.prepareMessagesForCompletion(ctx, sessionID, req.Message)
	if err != nil {
		return nil, err
	}

	tools := s.toolRegistry.BuildTools()
	model := s.resolveModel(req.Model)

	start := time.Now()
	loopRes, err := s.runChatLoop(ctx, sessionID, messages, tools, model, nil, nil)
	duration := time.Since(start)
	if err != nil {
		log.Errorw(ctx, "completion_error", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "error", err)
		return nil, fmt.Errorf("对话失败: %v", err)
	}

	if err := s.finalizeSessionTurn(ctx, sessionID, loopRes.Messages[newMessageStart:], loopRes.Status); err != nil {
		return nil, err
	}

	log.Debugw(ctx, "completion_end", "session_id", sessionID, "duration_ms", duration.Milliseconds(), "reply_len", len(loopRes.Reply))

	event := pb.StreamEventType_SET_DONE
	if loopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
		event = pb.StreamEventType_SET_INTERRUPT
	}

	return &pb.StreamChunk{
		Event:            event,
		SessionId:        sessionID,
		Status:           loopRes.Status,
		Text:             loopRes.Reply,
		PendingToolCalls: loopRes.PendingToolCalls,
	}, nil
}

func (s *ChatBiz) Resume(ctx context.Context, req *pb.ResumeRequest) (*pb.StreamChunk, error) {
	approvedTools, rejectedTools, err := s.validateAndPrepareResume(ctx, req)
	if err != nil {
		return nil, err
	}

	messages, err := s.loadHistory(ctx, req.SessionId)
	if err != nil {
		return nil, fmt.Errorf("加载对话历史失败: %v", err)
	}

	newMessageStart := len(messages)
	tools := s.toolRegistry.BuildTools()
	model := s.resolveModel(req.Model)
	start := time.Now()

	loopRes, err := s.runChatLoop(ctx, req.SessionId, messages, tools, model, approvedTools, rejectedTools)
	duration := time.Since(start)
	if err != nil {
		log.Errorw(ctx, "resume_error", "session_id", req.SessionId, "duration_ms", duration.Milliseconds(), "error", err)
		return nil, fmt.Errorf("恢复对话后继续执行失败: %v", err)
	}

	if err := s.finalizeSessionTurn(ctx, req.SessionId, loopRes.Messages[newMessageStart:], loopRes.Status); err != nil {
		return nil, err
	}

	event := pb.StreamEventType_SET_DONE
	if loopRes.Status == pb.SessionStatus_SS_INTERRUPTED {
		event = pb.StreamEventType_SET_INTERRUPT
	}

	return &pb.StreamChunk{
		Event:            event,
		SessionId:        req.SessionId,
		Status:           loopRes.Status,
		Text:             loopRes.Reply,
		PendingToolCalls: loopRes.PendingToolCalls,
	}, nil
}

func (s *ChatBiz) runChatLoop(
	ctx context.Context,
	sessionID string,
	messages []openai.ChatCompletionMessage,
	tools []openai.Tool,
	model string,
	approvedTools map[string]bool,
	rejectedTools map[string]string,
) (*LoopResult, error) {
	// 非流式 fetcher 闭包
	fetcher := GetChatFetcher(s.openaiChatModel)

	// 🎯 非流式模式：传入 NoopStreamEmitter 作为事件推送闭包
	return s.runAgentLoop(ctx, sessionID, messages, tools, model, approvedTools, rejectedTools, NoopStreamEmitter, fetcher)
}

func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return text
}
