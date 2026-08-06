package compressor

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"ai-rag-demo/internal/conf"

	openai "github.com/sashabaranov/go-openai"
)

func TestEstimateTokens(t *testing.T) {
	cfg := &conf.OpenAIContextCompressConfig{
		Enable:           true,
		MaxContextTokens: 1000,
		CompressRatio:    0.75,
	}
	comp := NewContextCompressor(cfg)

	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
		{Role: openai.ChatMessageRoleUser, Content: "你好，请帮我分析一下这个 Go 语言项目和 Docker 配置。"},
		{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{
				{
					Function: openai.FunctionCall{
						Name:      "get_project_structure",
						Arguments: `{"path": "/app/src"}`,
					},
				},
			},
		},
	}

	tokens := comp.EstimateTokens(msgs)
	if tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", tokens)
	}
	t.Logf("Estimated tokens for mixed English/Chinese messages: %d", tokens)
}

func TestDistillToolOutputs_NoPanicOnCJK(t *testing.T) {
	cfg := &conf.OpenAIContextCompressConfig{Enable: true}
	comp := NewContextCompressor(cfg)

	// Create a Chinese string where byte length > 1200, but rune length is around 450 (< 600)
	// In the old code, len(m.Content) > 1200 would trigger runes[:600], causing panic.
	var sb strings.Builder
	for i := 0; i < 450; i++ {
		sb.WriteString("中") // 3 bytes per rune, total 1350 bytes, 450 runes
	}
	longChineseToolOutput := sb.String()

	msgs := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleTool,
			Content: longChineseToolOutput,
		},
	}

	// Should not panic!
	distilled := comp.distillToolOutputs(msgs)
	if len(distilled) != 1 {
		t.Fatalf("expected 1 distilled message, got %d", len(distilled))
	}
	if distilled[0].Content != longChineseToolOutput {
		t.Fatalf("expected content unchanged since runes <= 600")
	}

	// Now test with > 600 runes
	sb.Reset()
	for i := 0; i < 700; i++ {
		sb.WriteString("汉") // 700 runes, 2100 bytes
	}
	veryLongContent := sb.String()
	msgs[0].Content = veryLongContent

	distilled = comp.distillToolOutputs(msgs)
	if !strings.Contains(distilled[0].Content, "[已蒸馏长工具输出: 原始长度 700 字符]") {
		t.Fatalf("expected distilled marker, got: %s", distilled[0].Content)
	}
}

func TestCheckpointDeduplication(t *testing.T) {
	cfg := &conf.OpenAIContextCompressConfig{
		Enable:              true,
		MaxContextTokens:    500,
		CompressRatio:       0.5,
		KeepRecentMessages:  2,
		MinUncompressedMsgs: 2,
		MaxCompressCount:    5,
	}
	comp := NewContextCompressor(cfg)

	// Messages simulating context that already has a Checkpoint from prior compression
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "System Base Prompt"},
		{Role: openai.ChatMessageRoleSystem, Content: CheckpointPrefix + ":\nPrevious summary of turn 1-10 " + strings.Repeat("history_details ", 200)},
		{Role: openai.ChatMessageRoleUser, Content: "User message 11 " + strings.Repeat("detail ", 300)},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant reply 11 " + strings.Repeat("answer ", 300)},
		{Role: openai.ChatMessageRoleUser, Content: "User message 12 " + strings.Repeat("detail ", 300)},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant reply 12 " + strings.Repeat("answer ", 300)},
		{Role: openai.ChatMessageRoleUser, Content: "User message 13 " + strings.Repeat("detail ", 300)},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant reply 13 " + strings.Repeat("answer ", 300)},
	}

	mockSummarizer := func(ctx context.Context, toSum []openai.ChatCompletionMessage) (string, error) {
		// Verify that the old checkpoint IS INCLUDED in toSum for rolling summarization!
		hasOldCheckpoint := false
		for _, m := range toSum {
			if strings.Contains(m.Content, "Previous summary") {
				hasOldCheckpoint = true
				break
			}
		}
		if !hasOldCheckpoint {
			return "", fmt.Errorf("expected old checkpoint in toSum for rolling summarization")
		}
		return "Updated Rolling Summary of turn 1-13", nil
	}

	res, err := comp.Compress(context.Background(), 1, msgs, mockSummarizer, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsCompressed {
		t.Fatalf("expected compression to trigger")
	}

	// Verify that CompressedMessages has EXACTLY 1 Base System prompt + 1 Checkpoint + keep messages
	sysCount := 0
	checkpointCount := 0
	for _, m := range res.CompressedMessages {
		if m.Role == openai.ChatMessageRoleSystem {
			sysCount++
			if isCheckpointMessage(&m) {
				checkpointCount++
			}
		}
	}

	if checkpointCount != 1 {
		t.Fatalf("expected exactly 1 Checkpoint message in result, got %d", checkpointCount)
	}
	if sysCount != 2 { // 1 Base System + 1 Checkpoint
		t.Fatalf("expected 2 System messages total (1 Base + 1 Checkpoint), got %d", sysCount)
	}
}

func TestFindSafeToolBoundary(t *testing.T) {
	cfg := &conf.OpenAIContextCompressConfig{}
	comp := NewContextCompressor(cfg)

	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "User 1"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant 1"},
		{Role: openai.ChatMessageRoleUser, Content: "User 2"},
		{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{
				{ID: "call_1", Function: openai.FunctionCall{Name: "tool_1"}},
			},
		},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call_1", Content: "Tool output 1"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant 2 after tool"},
		{Role: openai.ChatMessageRoleUser, Content: "User 3"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant 3"},
	}

	// Candidate index at Tool output (idx 4) -> should back up safely to User 2 (idx 2)
	safeIdx := comp.findSafeToolBoundary(msgs, 4)
	if safeIdx != 2 {
		t.Fatalf("expected safe tool boundary index 2 (User 2), got %d", safeIdx)
	}

	// Candidate index at User 3 (idx 6) -> safe at 6
	safeIdx = comp.findSafeToolBoundary(msgs, 6)
	if safeIdx != 6 {
		t.Fatalf("expected safe index 6, got %d", safeIdx)
	}
}

func TestCompressMaxLimitReached(t *testing.T) {
	cfg := &conf.OpenAIContextCompressConfig{
		Enable:              true,
		MaxContextTokens:    300,
		CompressRatio:       0.5,
		MaxCompressCount:    3,
		KeepRecentMessages:  2,
		MinUncompressedMsgs: 2,
	}
	comp := NewContextCompressor(cfg)

	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "Base prompt"},
		{Role: openai.ChatMessageRoleUser, Content: "User msg 1 " + strings.Repeat("x ", 500)},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant msg 1 " + strings.Repeat("y ", 500)},
		{Role: openai.ChatMessageRoleUser, Content: "User msg 2 " + strings.Repeat("x ", 500)},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant msg 2 " + strings.Repeat("y ", 500)},
		{Role: openai.ChatMessageRoleUser, Content: "User msg 3 " + strings.Repeat("x ", 500)},
		{Role: openai.ChatMessageRoleAssistant, Content: "Assistant msg 3 " + strings.Repeat("y ", 500)},
	}

	// currentCompressCount = 3, equal to MaxCompressCount (3) -> triggers max limit fallback FIFO
	res, err := comp.Compress(context.Background(), 3, msgs, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.IsCompressed || !res.IsMaxLimitReached {
		t.Fatalf("expected IsMaxLimitReached to be true, got IsCompressed=%v, IsMaxLimitReached=%v", res.IsCompressed, res.IsMaxLimitReached)
	}
	if res.NewCheckpointMsg != nil {
		t.Fatalf("expected NewCheckpointMsg to be nil on max limit fuse")
	}
}

func TestCompressionRatio60To70(t *testing.T) {
	cfg := &conf.OpenAIContextCompressConfig{
		Enable:              true,
		MaxContextTokens:    8098,
		CompressRatio:       0.75,
		KeepRecentMessages:  4,
		MinUncompressedMsgs: 2,
		MaxCompressCount:    5,
	}
	comp := NewContextCompressor(cfg)

	var msgs []openai.ChatCompletionMessage
	msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "System prompt instructions"})

	for i := 1; i <= 15; i++ {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("User question turn %d: %s", i, strings.Repeat("关于这个项目的数据库架构与接口设计的详细讨论 ", 10)),
		})
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: fmt.Sprintf("Assistant answer turn %d: %s", i, strings.Repeat("针对您的问题，我们需要在 internal/biz 层进行事务编排，并在 data 层实现 CRUD 接口 ", 10)),
		})
	}

	mockSummarizer := func(ctx context.Context, toSum []openai.ChatCompletionMessage) (string, error) {
		return "Checkpoint Summary: 讨论了数据库架构与接口设计细节，要求在 biz 层开启事务，在 data 层编写 SQL。", nil
	}

	res, err := comp.Compress(context.Background(), 0, msgs, mockSummarizer, nil)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	if !res.IsCompressed {
		t.Fatalf("expected isCompressed to be true")
	}

	savedTokens := res.OriginalTokens - res.CompressedTokens
	savedRatio := float64(savedTokens) / float64(res.OriginalTokens) * 100.0

	t.Logf("Original Tokens: %d, Compressed Tokens: %d, Saved Tokens: %d (%.2f%% saved)",
		res.OriginalTokens, res.CompressedTokens, savedTokens, savedRatio)

	if savedRatio < 60.0 {
		t.Fatalf("expected saved token ratio to be >= 60%%, got %.2f%%", savedRatio)
	}
}

func TestBuildFallbackExtractionSummary(t *testing.T) {
	cfg := &conf.OpenAIContextCompressConfig{}
	comp := NewContextCompressor(cfg)

	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "帮我查询 MySQL 慢日志与内存占用"},
		{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{
				{Function: openai.FunctionCall{Name: "query_mysql_slow_log"}},
			},
		},
		{Role: openai.ChatMessageRoleTool, Content: "slow log data..."},
		{Role: openai.ChatMessageRoleAssistant, Content: "根据日志分析，主要是索引失效导致了慢查询。"},
	}

	summary := comp.buildFallbackExtractionSummary(msgs)
	if !strings.Contains(summary, "早期探讨核心主题") || !strings.Contains(summary, "帮我查询 MySQL 慢日志") {
		t.Fatalf("expected user topic in fallback summary, got: %s", summary)
	}
	if !strings.Contains(summary, "query_mysql_slow_log") {
		t.Fatalf("expected tool name in fallback summary, got: %s", summary)
	}
}
