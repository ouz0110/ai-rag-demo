package nocli

import (
	"context"
	"errors"
	"fmt"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	openaierr "ai-rag-demo/internal/biz/nocli/openai/error"
	"ai-rag-demo/internal/common"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"

	openai "github.com/sashabaranov/go-openai"
)

// ProcessToolCallsResult 工具处理执行结果
type ProcessToolCallsResult struct {
	HasInterrupt     bool
	PendingInterrupt *dataBase.NocliInterruptModel
	PendingToolCall  *pb.PendingToolCall
	ExecutedMsgs     []openai.ChatCompletionMessage
}

// processToolCalls 统一工具处理管道：使用 emitter 闭包（流式推送 SSE，非流式空操作），
// 严格顺序执行工具、处理拒绝工具、自动调用 emitter 发送事件并拦截高危中断
func (s *ChatBiz) processToolCalls(
	ctx context.Context,
	sessionID string,
	baseFields []interface{},
	toolCalls []openai.ToolCall,
	approvedTools map[string]bool,
	rejectedTools map[string]string,
	totalToolCalls *int,
	emitter StreamEmitter,
) (*ProcessToolCallsResult, error) {
	if emitter == nil {
		emitter = NoopStreamEmitter
	}

	result := &ProcessToolCallsResult{
		ExecutedMsgs: make([]openai.ChatCompletionMessage, 0, len(toolCalls)),
	}

	for _, tc := range toolCalls {
		toolName := tc.Function.Name
		toolID := tc.ID

		// 1. 若在 Resume 阶段被拒绝
		if reason, isRejected := rejectedTools[toolID]; isRejected {
			toolMsg := openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    fmt.Sprintf("操作被用户拒绝: %s", reason),
				ToolCallID: toolID,
			}
			result.ExecutedMsgs = append(result.ExecutedMsgs, toolMsg)
			log.Debugw(ctx, "tool_execution_rejected", append(baseFields, "tool_id", toolID, "tool_name", toolName, "reason", reason)...)
			continue
		}

		// 2. 检查是否需要审批且未在授权放行名单中
		if s.toolRegistry.RequiresApproval(toolName, tc.Function.Arguments) && !approvedTools[toolName] && !approvedTools[toolID] {
			// 🎯 严格顺序控制：一旦遇到第一个需要审批的高危工具，立即中断生成挂起事项并切出！
			intRes := s.buildInterruptResult(ctx, sessionID, toolID, toolName, tc.Function.Arguments, baseFields)
			intRes.ExecutedMsgs = result.ExecutedMsgs
			result = intRes
			break // 停止遍历后续任何 ToolCall
		}

		// 3. 安全工具或已授权放行工具：按顺序立即执行
		*totalToolCalls++
		log.Debugw(ctx, "tool_call", append(baseFields, "tool_index", *totalToolCalls, "tool_id", toolID, "tool_name", toolName, "args_len", len(tc.Function.Arguments))...)

		// 通过 emitter 闭包发送 SET_TOOL_START 事件
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_TOOL_START,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_RUNNING,
			ToolInfo: &pb.StreamToolInfo{
				ToolCallId: toolID,
				ToolName:   toolName,
				Arguments:  tc.Function.Arguments,
			},
		})

		toolResult, err := s.toolRegistry.Call(ctx, toolName, tc.Function.Arguments)
		if err != nil {
			var interruptErr openaierr.InterruptErr
			if errors.As(err, &interruptErr) && !approvedTools[toolName] && !approvedTools[toolID] {
				intRes := s.buildInterruptResult(ctx, sessionID, toolID, toolName, tc.Function.Arguments, baseFields)
				intRes.ExecutedMsgs = result.ExecutedMsgs
				result = intRes
				break
			}
			toolResult = fmt.Sprintf("工具执行失败: %v", err)
			log.Debugw(ctx, "tool_result_error", append(baseFields, "tool_id", toolID, "tool_name", toolName, "error", err)...)
		} else {
			log.Debugw(ctx, "tool_result_success", append(baseFields, "tool_id", toolID, "tool_name", toolName, "result_len", len(toolResult))...)
		}

		// 通过 emitter 闭包发送 SET_TOOL_RESULT 事件
		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_TOOL_RESULT,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_RUNNING,
			ToolInfo: &pb.StreamToolInfo{
				ToolCallId:    toolID,
				ToolName:      toolName,
				ResultPreview: truncateText(toolResult, 200),
			},
		})

		toolMsg := openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    toolResult,
			ToolCallID: toolID,
		}
		result.ExecutedMsgs = append(result.ExecutedMsgs, toolMsg)
	}

	// 如果发生了中断，仅落盘中断记录与更新 Session 状态，消息统一由最外层保存
	if result.HasInterrupt {
		if err := s.allDb.Base.NocliInterruptRepo.CreateBatch(ctx, []*dataBase.NocliInterruptModel{result.PendingInterrupt}); err != nil {
			return nil, fmt.Errorf("创建中断记录失败: %v", err)
		}

		if err := s.allDb.Base.NocliSessionRepo.UpdateStatus(ctx, sessionID, pb.SessionStatus_SS_INTERRUPTED); err != nil {
			return nil, fmt.Errorf("更新会话中断状态失败: %v", err)
		}
	}

	return result, nil
}

// buildInterruptResult 封装生成中断挂起事项结构体的通用辅助函数
func (s *ChatBiz) buildInterruptResult(
	ctx context.Context,
	sessionID, toolID, toolName, arguments string,
	baseFields []interface{},
) *ProcessToolCallsResult {
	interruptID := utils.NewUUID()
	log.Debugw(ctx, "tool_approval_interrupted", append(baseFields, "tool_name", toolName, "tool_id", toolID)...)

	return &ProcessToolCallsResult{
		HasInterrupt: true,
		PendingInterrupt: &dataBase.NocliInterruptModel{
			InterruptID: interruptID,
			SessionID:   sessionID,
			Status:      pb.InterruptStatus_IS_PENDING,
			ToolCallID:  toolID,
			ToolName:    toolName,
			Arguments:   arguments,
			CreatedAt:   time.Now().Unix(),
		},
		PendingToolCall: &pb.PendingToolCall{
			InterruptId: interruptID,
			ToolCallId:  toolID,
			ToolName:    toolName,
			Arguments:   arguments,
		},
	}
}

// cancelPendingInterruptsOnNewCompletion 当用户发送新问题时，自动作废旧的待处理中断，并返回产生的取消消息
func (s *ChatBiz) cancelPendingInterruptsOnNewCompletion(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	pendingModels, err := s.allDb.Base.NocliInterruptRepo.GetPendingBySessionID(ctx, sessionID)
	if err != nil || len(pendingModels) == 0 {
		return nil, nil
	}

	_, user := common.UserFromContext(ctx)
	now := time.Now().Unix()
	cancelMsgs := make([]openai.ChatCompletionMessage, 0, len(pendingModels))

	for _, pm := range pendingModels {
		_ = s.allDb.Base.NocliInterruptRepo.UpdateStatus(
			ctx,
			pm.InterruptID,
			pb.InterruptStatus_IS_EXPIRED,
			pm.ApproveScope,
			now,
			user.Openid,
			"用户发送了新问题，自动取消旧中断",
		)
		cancelMsgs = append(cancelMsgs, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    "操作已取消：用户提交了新的提问，放弃了此授权操作",
			ToolCallID: pm.ToolCallID,
		})
	}

	return cancelMsgs, nil
}

// buildPendingToolCalls 构建返回给前端的待确认工具列表
func buildPendingToolCalls(models []dataBase.NocliInterruptModel) []*pb.PendingToolCall {
	list := make([]*pb.PendingToolCall, 0, len(models))
	for _, m := range models {
		list = append(list, &pb.PendingToolCall{
			InterruptId: m.InterruptID,
			ToolCallId:  m.ToolCallID,
			ToolName:    m.ToolName,
			Arguments:   m.Arguments,
		})
	}
	return list
}
