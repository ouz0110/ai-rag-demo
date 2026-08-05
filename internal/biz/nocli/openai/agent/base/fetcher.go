package base

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	pb "ai-rag-demo/api/nocli/v1"
	bizCommon "ai-rag-demo/internal/biz/common"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/common"
	dataBase "ai-rag-demo/internal/data/base"

	openai "github.com/sashabaranov/go-openai"
)

// GetStreamFetcher 构造流式 MessageFetcher 闭包
func (b *BaseAgent) GetStreamFetcher(sessionID string, chatModel *chatmodel.ChatModel, emitter StreamEmitter) MessageFetcher {
	return func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error) {
		req.StreamOptions = &openai.StreamOptions{
			IncludeUsage: true,
		}

		stream, err := chatModel.GetOpenAI(ctx).CreateChatCompletionStream(ctx, req)
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}
		defer stream.Close()

		var textBuilder strings.Builder
		var combinedMessage openai.ChatCompletionMessage
		var finalUsage openai.Usage
		toolCallsMap := make(map[int]*openai.ToolCall)

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return openai.ChatCompletionMessage{}, err
			}

			if response.Usage != nil {
				finalUsage = *response.Usage
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
		combinedMessage.Name = b.Name()

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

		// 【扣费与 Token 统计落盘】
		if recorder := chatModel.GetUsageRecorder(); recorder != nil {
			if finalUsage.TotalTokens == 0 {
				var promptBuilder strings.Builder
				for _, msg := range req.Messages {
					promptBuilder.WriteString(msg.Content)
				}
				pTokens := bizCommon.CalculateTextTokens(promptBuilder.String())
				cTokens := bizCommon.CalculateTextTokens(textBuilder.String())
				finalUsage = openai.Usage{
					PromptTokens:     int(pTokens),
					CompletionTokens: int(cTokens),
					TotalTokens:      int(pTokens + cTokens),
				}
			}

			userID := "default_user"
			if ok, u := common.UserFromContext(ctx); ok && u.Openid != "" {
				userID = u.Openid
			}

			modelName := req.Model
			if modelName == "" && chatModel.GetConfig() != nil && chatModel.GetConfig().Source.OpenAI != nil {
				modelName = chatModel.GetConfig().Source.OpenAI.Model
			}

			reqID := fmt.Sprintf("stream_%s_%d", sessionID, time.Now().UnixNano())
			_, _ = recorder.RecordOpenAIUsage(
				ctx,
				reqID,
				userID,
				dataBase.ServiceTypeOpenAI,
				"openai",
				modelName,
				finalUsage,
				0,
			)
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

		// 【扣费与 Token 统计落盘】
		if recorder := chatModel.GetUsageRecorder(); recorder != nil {
			userID := "default_user"
			if ok, u := common.UserFromContext(ctx); ok && u.Openid != "" {
				userID = u.Openid
			}

			modelName := req.Model
			if modelName == "" && chatModel.GetConfig() != nil && chatModel.GetConfig().Source.OpenAI != nil {
				modelName = chatModel.GetConfig().Source.OpenAI.Model
			}

			reqID := resp.ID
			if reqID == "" {
				reqID = fmt.Sprintf("req_%d", time.Now().UnixNano())
			}

			_, _ = recorder.RecordOpenAIUsage(
				ctx,
				reqID,
				userID,
				dataBase.ServiceTypeOpenAI,
				"openai",
				modelName,
				resp.Usage,
				0,
			)
		}

		return resp.Choices[0].Message, nil
	}
}
