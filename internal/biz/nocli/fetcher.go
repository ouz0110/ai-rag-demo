package nocli

import (
	pb "ai-rag-demo/api/nocli/v1"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

func GetChatFetcher(chatModel *chatmodel.ChatModel) MessageFetcher {
	return func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error) {
		client := chatModel.GetOpenAI(ctx)
		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}
		return resp.Choices[0].Message, nil
	}
}

func GetStreamFetcher(sessionID string, chatModel *chatmodel.ChatModel, emitter StreamEmitter) MessageFetcher {
	return func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error) {
		req.Stream = true
		client := chatModel.GetOpenAI(ctx)
		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}
		defer stream.Close()

		var fullContent string
		var fullReasoning string
		toolCallMap := make(map[int]*openai.ToolCall)

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return openai.ChatCompletionMessage{}, err
			}

			if len(resp.Choices) == 0 {
				continue
			}

			choice := resp.Choices[0]
			delta := choice.Delta

			// 1. 实时推送思考过程 (CoT)
			if delta.ReasoningContent != "" {
				fullReasoning += delta.ReasoningContent
				emitter(&pb.StreamChunk{
					Event:         pb.StreamEventType_SET_REASONING,
					SessionId:     sessionID,
					Status:        pb.SessionStatus_SS_RUNNING,
					ReasoningText: delta.ReasoningContent,
				})
			}

			// 2. 实时推送回复文本打字机片段
			if delta.Content != "" {
				fullContent += delta.Content
				emitter(&pb.StreamChunk{
					Event:     pb.StreamEventType_SET_TEXT_DELTA,
					SessionId: sessionID,
					Status:    pb.SessionStatus_SS_RUNNING,
					Text:      delta.Content,
				})
			}

			// 3. 工具调用片段增量组合
			if len(delta.ToolCalls) > 0 {
				for _, tcChunk := range delta.ToolCalls {
					idx := 0
					if tcChunk.Index != nil {
						idx = *tcChunk.Index
					}

					existing, ok := toolCallMap[idx]
					if !ok {
						existing = &openai.ToolCall{
							Index: tcChunk.Index,
							ID:    tcChunk.ID,
							Type:  tcChunk.Type,
							Function: openai.FunctionCall{
								Name:      tcChunk.Function.Name,
								Arguments: tcChunk.Function.Arguments,
							},
						}
						toolCallMap[idx] = existing
					} else {
						if tcChunk.ID != "" {
							existing.ID = tcChunk.ID
						}
						if tcChunk.Function.Name != "" {
							existing.Function.Name = tcChunk.Function.Name
						}
						existing.Function.Arguments += tcChunk.Function.Arguments
					}
				}
			}
		}

		assistantMsg := openai.ChatCompletionMessage{
			Role:             openai.ChatMessageRoleAssistant,
			Content:          fullContent,
			ReasoningContent: fullReasoning,
		}

		if len(toolCallMap) > 0 {
			toolCalls := make([]openai.ToolCall, 0, len(toolCallMap))
			for i := 0; i < len(toolCallMap); i++ {
				if tc, ok := toolCallMap[i]; ok {
					toolCalls = append(toolCalls, *tc)
				}
			}
			assistantMsg.ToolCalls = toolCalls
		}

		return assistantMsg, nil
	}
}
