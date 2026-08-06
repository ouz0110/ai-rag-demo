package base

import (
	"context"
	"strings"
	"testing"
	"time"

	"ai-rag-demo/internal/biz/nocli/openai/tool"
	"ai-rag-demo/internal/conf"

	openai "github.com/sashabaranov/go-openai"
)

type SlowMockTool struct {
	sleepDuration time.Duration
}

func (s *SlowMockTool) Definition() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "slow_mock_tool",
			Description: "A mock tool that sleeps for testing context timeout",
		},
	}
}

func (s *SlowMockTool) Run(ctx context.Context, argsJSON string) (string, error) {
	select {
	case <-time.After(s.sleepDuration):
		return "slow tool completed successfully", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *SlowMockTool) RequiresApproval(ctx context.Context, argsJSON string) bool {
	return false
}

func TestProcessToolCalls_TimeoutInterruption(t *testing.T) {
	cfg := &conf.Config{}
	cfg.Source.Nocli = &conf.NocliConfig{
		DefaultToolTimeout: conf.DurationWrapper{Duration: 50 * time.Millisecond},
	}

	registry := tool.NewRegistry(cfg, nil, nil)
	registry.Register(&SlowMockTool{sleepDuration: 300 * time.Millisecond})

	agent := &BaseAgent{
		cfg:          cfg,
		toolRegistry: registry,
	}

	toolCalls := []openai.ToolCall{
		{
			ID: "call_slow_1",
			Function: openai.FunctionCall{
				Name:      "slow_mock_tool",
				Arguments: "{}",
			},
		},
	}

	total := 0
	res, err := agent.ProcessToolCalls(
		context.Background(),
		"sess_test",
		nil,
		toolCalls,
		nil,
		nil,
		&total,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.ExecutedMsgs) != 1 {
		t.Fatalf("expected 1 executed msg, got %d", len(res.ExecutedMsgs))
	}

	toolMsg := res.ExecutedMsgs[0]
	if !strings.Contains(toolMsg.Content, "工具执行超时中断") {
		t.Fatalf("expected timeout warning in tool result, got: %s", toolMsg.Content)
	}
	t.Logf("Captured timeout result preview: %s", toolMsg.Content)
}
