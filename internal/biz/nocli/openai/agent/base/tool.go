package base

import (
	pb "ai-rag-demo/api/nocli/v1"
	openaierr "ai-rag-demo/internal/biz/nocli/openai/error"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"
	"context"
	"errors"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// ProcessToolCalls 成员方法：调度工具执行管道
func (b *BaseAgent) ProcessToolCalls(
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

		if reason, isRejected := rejectedTools[toolID]; isRejected {
			rejText := fmt.Sprintf("操作被用户拒绝: %s", reason)
			emitter(&pb.StreamChunk{
				Event:     pb.StreamEventType_SET_TOOL_RESULT,
				SessionId: sessionID,
				Status:    pb.SessionStatus_SS_RUNNING,
				ToolInfo: &pb.StreamToolInfo{
					ToolCallId:    toolID,
					ToolName:      toolName,
					ResultPreview: rejText,
				},
			})
			toolMsg := openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    rejText,
				ToolCallID: toolID,
			}
			result.ExecutedMsgs = append(result.ExecutedMsgs, toolMsg)
			log.Debugw(ctx, "tool_execution_rejected", append(baseFields, "tool_id", toolID, "tool_name", toolName, "reason", reason)...)
			continue
		}

		requiresApproval := b.toolRegistry.RequiresApproval(toolName, tc.Function.Arguments)
		isApproved := approvedTools[toolID] || approvedTools[toolName]

		// 🛡️ 动态安全防护: terminal 工具如果是危险/修改性指令 (RequiresApproval 为 true),
		// 必须仅凭单次审批 (approvedTools[toolID]) 放行，不能直接套用工具级别的全局 session 放行，防止误穿透高危命令！
		if toolName == "terminal" && requiresApproval && !approvedTools[toolID] {
			isApproved = false
		}

		if requiresApproval && !isApproved {
			intRes := b.BuildInterruptResult(ctx, sessionID, toolID, toolName, tc.Function.Arguments, baseFields)
			intRes.ExecutedMsgs = result.ExecutedMsgs
			result = intRes
			break
		}

		*totalToolCalls++
		log.Debugw(ctx, "tool_call", append(baseFields, "tool_index", *totalToolCalls, "tool_id", toolID, "tool_name", toolName, "args_len", len(tc.Function.Arguments))...)

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

		toolResult, err := b.toolRegistry.Call(ctx, toolName, tc.Function.Arguments)
		if err != nil {
			var interruptErr openaierr.InterruptErr
			if errors.As(err, &interruptErr) {
				isAppr := approvedTools[toolID] || approvedTools[toolName]
				if toolName == "terminal" && !approvedTools[toolID] {
					isAppr = false
				}
				if !isAppr {
					intRes := b.BuildInterruptResult(ctx, sessionID, toolID, toolName, tc.Function.Arguments, baseFields)
					intRes.ExecutedMsgs = result.ExecutedMsgs
					result = intRes
					break
				}
			}
			toolResult = fmt.Sprintf("工具执行失败: %v", err)
			log.Debugw(ctx, "tool_result_error", append(baseFields, "tool_id", toolID, "tool_name", toolName, "error", err)...)
		} else {
			log.Debugw(ctx, "tool_result_success", append(baseFields, "tool_id", toolID, "tool_name", toolName, "result_len", len(toolResult))...)
		}

		emitter(&pb.StreamChunk{
			Event:     pb.StreamEventType_SET_TOOL_RESULT,
			SessionId: sessionID,
			Status:    pb.SessionStatus_SS_RUNNING,
			ToolInfo: &pb.StreamToolInfo{
				ToolCallId:    toolID,
				ToolName:      toolName,
				ResultPreview: TruncateText(toolResult, 200),
			},
		})

		toolMsg := openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    toolResult,
			ToolCallID: toolID,
		}
		result.ExecutedMsgs = append(result.ExecutedMsgs, toolMsg)
	}

	return result, nil
}

func (b *BaseAgent) BuildInterruptResult(
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
