package nocli

import (
	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/common"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"context"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

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

// executeResumeTool 执行 Resume 中的中断决策，更新中断表状态，并返回生成的 Tool 消息（不操作消息持久化）
func (s *ChatBiz) executeResumeTool(
	ctx context.Context,
	userOpenid string,
	interrupt *dataBase.NocliInterruptModel,
	action pb.ResumeAction,
	reason string,
) (openai.ChatCompletionMessage, error) {
	now := time.Now().Unix()
	var toolResult string

	if action == pb.ResumeAction_RA_APPROVE {
		result, err := s.toolRegistry.Call(ctx, interrupt.ToolName, interrupt.Arguments)
		if err != nil {
			toolResult = fmt.Sprintf("工具执行失败: %v", err)
		} else {
			toolResult = result
		}
		_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(
			ctx,
			interrupt.InterruptID,
			pb.InterruptStatus_IS_APPROVED,
			now,
			userOpenid,
			reason,
		)
	} else {
		if reason == "" {
			reason = "用户拒绝执行该操作"
		}
		toolResult = fmt.Sprintf("操作被用户拒绝: %s", reason)
		_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(
			ctx,
			interrupt.InterruptID,
			pb.InterruptStatus_IS_REJECTED,
			now,
			userOpenid,
			reason,
		)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    toolResult,
		ToolCallID: interrupt.ToolCallID,
	}, nil
}
