package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestObservabilityCompositeAndLogObserver(t *testing.T) {
	logObs := NewLogObserver()
	otelObs := NewOTelObserver(nil)
	composite := NewCompositeObserver(logObs, otelObs)

	ctx := WithRequestID(context.Background(), "req-test-123")
	ctx = WithObserver(ctx, composite)

	obs := GetObserver(ctx)

	// 1. Test Agent Observer Hook
	agentCtx, endAgent := obs.OnAgentStart(ctx, &AgentRunInfo{
		AgentName:     "main",
		SessionID:     "sess-test-456",
		Model:         "deepseek-v3.2",
		MaxIterations: 20,
		Timeout:       5 * time.Minute,
	})
	if agentCtx == nil {
		t.Fatal("expected non-nil context from OnAgentStart")
	}

	// 2. Test LLM Observer Hook
	llmCtx, endLLM := obs.OnLLMStart(agentCtx, &LLMCallInfo{
		AgentName:     "main",
		SessionID:     "sess-test-456",
		Model:         "deepseek-v3.2",
		MessagesCount: 5,
		ToolsCount:    3,
		Iteration:     1,
	})
	if llmCtx == nil {
		t.Fatal("expected non-nil context from OnLLMStart")
	}
	endLLM(&openai.ChatCompletionMessage{Role: "assistant", Content: "hello"}, nil)

	// 3. Test Tool Observer Hook
	toolCtx, endTool := obs.OnToolStart(agentCtx, &ToolCallInfo{
		AgentName: "main",
		SessionID: "sess-test-456",
		ToolName:  "read_files",
		ArgsJSON:  `{"files":["main.go"]}`,
	})
	if toolCtx == nil {
		t.Fatal("expected non-nil context from OnToolStart")
	}
	endTool("file contents", nil)

	// 4. Test Compress Observer Hook
	compCtx, endCompress := obs.OnCompressStart(agentCtx, &CompressInfo{
		AgentName:      "main",
		SessionID:      "sess-test-456",
		OriginalTokens: 16384,
		CompressCount:  1,
	})
	if compCtx == nil {
		t.Fatal("expected non-nil context from OnCompressStart")
	}
	endCompress(5734, false, "compressed summary checkpoint text", nil)

	// Finish Agent
	endAgent("final response", nil)

	// 5. Test error case
	_, endErr := obs.OnToolStart(agentCtx, &ToolCallInfo{
		AgentName: "main",
		SessionID: "sess-test-456",
		ToolName:  "terminal",
		ArgsJSON:  `{"cmd":"ls"}`,
	})
	endErr("", errors.New("command failed"))
}
