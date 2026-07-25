package base

import (
	pb "ai-rag-demo/api/nocli/v1"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// GetStreamFetcher 构造流式 MessageFetcher 闭包
func (b *BaseAgent) GetStreamFetcher(sessionID string, chatModel *chatmodel.ChatModel, emitter StreamEmitter) MessageFetcher {
	return func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error) {
		stream, err := chatModel.GetOpenAI(ctx).CreateChatCompletionStream(ctx, req)
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}
		defer stream.Close()

		var textBuilder strings.Builder
		var combinedMessage openai.ChatCompletionMessage
		toolCallsMap := make(map[int]*openai.ToolCall)

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return openai.ChatCompletionMessage{}, err
			}

			if len(response.Choices) == 0 {
				continue
			}

			delta := response.Choices[0].Delta

			if delta.Content != "" {
				textBuilder.WriteString(delta.Content)
				emitter(&pb.StreamChunk{
					Event:     pb.StreamEventType_SET_TEXT_DELTA,
					SessionId: sessionID,
					Status:    pb.SessionStatus_SS_RUNNING,
					Text:      delta.Content,
				})
			}

			if len(delta.ToolCalls) > 0 {
				for _, tc := range delta.ToolCalls {
					index := 0
					if tc.Index != nil {
						index = *tc.Index
					}

					existing, ok := toolCallsMap[index]
					if !ok {
						toolCallsMap[index] = &openai.ToolCall{
							Index: tc.Index,
							ID:    tc.ID,
							Type:  tc.Type,
							Function: openai.FunctionCall{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						}
					} else {
						if tc.ID != "" {
							existing.ID = tc.ID
						}
						if tc.Function.Name != "" {
							existing.Function.Name += tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							existing.Function.Arguments += tc.Function.Arguments
						}
					}
				}
			}
		}

		combinedMessage.Role = openai.ChatMessageRoleAssistant
		combinedMessage.Content = textBuilder.String()

		if len(toolCallsMap) > 0 {
			toolCalls := make([]openai.ToolCall, 0, len(toolCallsMap))
			for i := 0; i < len(toolCallsMap); i++ {
				if tc, ok := toolCallsMap[i]; ok {
					toolCalls = append(toolCalls, *tc)
				}
			}
			if len(toolCalls) == 0 {
				for _, tc := range toolCallsMap {
					toolCalls = append(toolCalls, *tc)
				}
			}
			combinedMessage.ToolCalls = toolCalls
		}

		return combinedMessage, nil
	}
}

// GetSyncFetcher 构造同步 MessageFetcher 闭包
func (b *BaseAgent) GetSyncFetcher(chatModel *chatmodel.ChatModel) MessageFetcher {
	return func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error) {
		resp, err := chatModel.GetOpenAI(ctx).CreateChatCompletion(ctx, req)
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}

		if len(resp.Choices) == 0 {
			return openai.ChatCompletionMessage{}, fmt.Errorf("LLM 返回空的 Choices")
		}

		return resp.Choices[0].Message, nil
	}
}
