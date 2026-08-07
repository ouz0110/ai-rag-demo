package observability

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

// CompositeObserver 组合模式观察者容器
// 支持将多个 Observer (如 OTelObserver + LogObserver + CustomPrometheusObserver) 聚合为一个统一切面
type CompositeObserver struct {
	observers []Observer
}

func NewCompositeObserver(observers ...Observer) *CompositeObserver {
	valid := make([]Observer, 0, len(observers))
	for _, obs := range observers {
		if obs != nil {
			valid = append(valid, obs)
		}
	}
	return &CompositeObserver{observers: valid}
}

func (c *CompositeObserver) OnAgentStart(ctx context.Context, info *AgentRunInfo) (context.Context, EndAgentFunc) {
	ends := make([]EndAgentFunc, 0, len(c.observers))
	currentCtx := ctx
	for _, obs := range c.observers {
		newCtx, end := obs.OnAgentStart(currentCtx, info)
		if newCtx != nil {
			currentCtx = newCtx
		}
		if end != nil {
			ends = append(ends, end)
		}
	}
	return currentCtx, func(reply string, err error) {
		for _, end := range ends {
			end(reply, err)
		}
	}
}

func (c *CompositeObserver) OnLLMStart(ctx context.Context, info *LLMCallInfo) (context.Context, EndLLMFunc) {
	ends := make([]EndLLMFunc, 0, len(c.observers))
	currentCtx := ctx
	for _, obs := range c.observers {
		newCtx, end := obs.OnLLMStart(currentCtx, info)
		if newCtx != nil {
			currentCtx = newCtx
		}
		if end != nil {
			ends = append(ends, end)
		}
	}
	return currentCtx, func(msg *openai.ChatCompletionMessage, err error) {
		for _, end := range ends {
			end(msg, err)
		}
	}
}

func (c *CompositeObserver) OnToolStart(ctx context.Context, info *ToolCallInfo) (context.Context, EndToolFunc) {
	ends := make([]EndToolFunc, 0, len(c.observers))
	currentCtx := ctx
	for _, obs := range c.observers {
		newCtx, end := obs.OnToolStart(currentCtx, info)
		if newCtx != nil {
			currentCtx = newCtx
		}
		if end != nil {
			ends = append(ends, end)
		}
	}
	return currentCtx, func(result string, err error) {
		for _, end := range ends {
			end(result, err)
		}
	}
}

func (c *CompositeObserver) OnCompressStart(ctx context.Context, info *CompressInfo) (context.Context, EndCompressFunc) {
	ends := make([]EndCompressFunc, 0, len(c.observers))
	currentCtx := ctx
	for _, obs := range c.observers {
		newCtx, end := obs.OnCompressStart(currentCtx, info)
		if newCtx != nil {
			currentCtx = newCtx
		}
		if end != nil {
			ends = append(ends, end)
		}
	}
	return currentCtx, func(compressedTokens int, isMaxLimit bool, summaryText string, err error) {
		for _, end := range ends {
			end(compressedTokens, isMaxLimit, summaryText, err)
		}
	}
}

// Ensure interface compliance
var _ Observer = (*CompositeObserver)(nil)
