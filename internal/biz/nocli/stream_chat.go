package nocli

import (
	"context"
	"fmt"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

// StreamCompletion 流式处理新提问 (由 Service 层直接传入 StreamEmitter 闭包，实现零延迟实时分发)
func (s *ChatBiz) StreamCompletion(ctx context.Context, req *pb.CompletionRequest, emitter StreamEmitter) error {
	if emitter == nil {
		emitter = NoopStreamEmitter
	}

	sessionID, err := s.initOrCreateSession(ctx, req.SessionId, req.Message)
	if err != nil {
		return err
	}

	messages, newMessageStart, err := s.prepareMessagesForCompletion(ctx, sessionID, req.Message)
	if err != nil {
		return err
	}

	tools := s.toolRegistry.BuildTools()
	model := s.resolveModel(req.Model)
	approvedTools := s.loadSessionApprovedTools(ctx, sessionID)

	log.Debugw(ctx, "stream_completion_start", "session_id", sessionID, "model", model)

	loopRes, err := s.runStreamChatLoop(ctx, sessionID, messages, tools, model, approvedTools, nil, emitter)
	if err != nil {
		log.Errorw(ctx, "stream_completion_error", "session_id", sessionID, "error", err)
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_ERROR,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_IDLE,
			Error:     &pb.StreamError{Code: 500, Message: err.Error()},
		})
		return err
	}

	// 🎯 消息存储固定在最外层：增量批量落盘
	if err := s.finalizeSessionTurn(ctx, sessionID, loopRes.Messages[newMessageStart:], loopRes.Status); err != nil {
		return err
	}

	return nil
}

// StreamResume 流式恢复执行 (由 Service 层直接传入 StreamEmitter 闭包)
func (s *ChatBiz) StreamResume(ctx context.Context, req *pb.ResumeRequest, emitter StreamEmitter) error {
	if emitter == nil {
		emitter = NoopStreamEmitter
	}

	approvedTools, rejectedTools, err := s.validateAndPrepareResume(ctx, req)
	if err != nil {
		return err
	}

	messages, err := s.loadHistory(ctx, req.SessionId)
	if err != nil {
		return fmt.Errorf("加载对话历史失败: %v", err)
	}

	newMessageStart := len(messages)
	tools := s.toolRegistry.BuildTools()
	model := s.resolveModel(req.Model)

	log.Debugw(ctx, "stream_resume_start", "session_id", req.SessionId, "model", model)

	loopRes, err := s.runStreamChatLoop(ctx, req.SessionId, messages, tools, model, approvedTools, rejectedTools, emitter)
	if err != nil {
		log.Errorw(ctx, "stream_resume_error", "session_id", req.SessionId, "error", err)
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_ERROR,
			SessionId: req.SessionId,
			Status:    pb.SessionStatus_SS_IDLE,
			Error:     &pb.StreamError{Code: 500, Message: err.Error()},
		})
		return err
	}

	if err := s.finalizeSessionTurn(ctx, req.SessionId, loopRes.Messages[newMessageStart:], loopRes.Status); err != nil {
		return err
	}

	return nil
}

func (s *ChatBiz) runStreamChatLoop(
	ctx context.Context,
	sessionID string,
	messages []openai.ChatCompletionMessage,
	tools []openai.Tool,
	model string,
	approvedTools map[string]bool,
	rejectedTools map[string]string,
	emitter StreamEmitter,
) (*LoopResult, error) {
	// 流式 fetcher 闭包：直接使用上层注入的 emitter 实时推送 Token 帧
	fetcher := GetStreamFetcher(sessionID, s.openaiChatModel, emitter)
	return s.runAgentLoop(ctx, sessionID, messages, tools, model, approvedTools, rejectedTools, emitter, fetcher)
}
