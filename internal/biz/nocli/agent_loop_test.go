package nocli

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestFindUnexecutedToolCalls(t *testing.T) {
	// Case 1: 无 Assistant 消息
	msgs1 := []openai.ChatCompletionMessage{
		{Role: "user", Content: "hello"},
	}
	_, unexec1 := findUnexecutedToolCalls(msgs1)
	if len(unexec1) != 0 {
		t.Errorf("Case 1 期望 0 个未执行，实际: %d", len(unexec1))
	}

	// Case 2: 包含 2 个 ToolCalls，第 1 个已执行 (role: tool)，第 2 个未执行
	msgs2 := []openai.ChatCompletionMessage{
		{Role: "user", Content: "run tools"},
		{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{
				{ID: "call_1", Function: openai.FunctionCall{Name: "list_files"}},
				{ID: "call_2", Function: openai.FunctionCall{Name: "terminal"}},
			},
		},
		{Role: "tool", ToolCallID: "call_1", Content: "file list ok"},
	}

	idx2, unexec2 := findUnexecutedToolCalls(msgs2)
	if idx2 != 1 {
		t.Errorf("Case 2 期望 assistant 索引 1，实际: %d", idx2)
	}
	if len(unexec2) != 1 || unexec2[0].ID != "call_2" {
		t.Fatalf("Case 2 期望剩余未执行 [call_2]，实际: %+v", unexec2)
	}

	// Case 3: 2 个 ToolCalls 均已执行
	msgs3 := []openai.ChatCompletionMessage{
		{Role: "user", Content: "run tools"},
		{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{
				{ID: "call_1", Function: openai.FunctionCall{Name: "list_files"}},
				{ID: "call_2", Function: openai.FunctionCall{Name: "terminal"}},
			},
		},
		{Role: "tool", ToolCallID: "call_1", Content: "ok"},
		{Role: "tool", ToolCallID: "call_2", Content: "ok"},
	}

	_, unexec3 := findUnexecutedToolCalls(msgs3)
	if len(unexec3) != 0 {
		t.Errorf("Case 3 期望 0 个未执行，实际: %d", len(unexec3))
	}

	// Case 4: 复杂场景——多轮历史对话 + 3 个 ToolCalls 混合状态 + 无 ToolCalls 的中间 Assistant
	msgs4 := []openai.ChatCompletionMessage{
		{Role: "system", Content: "you are an assistant"},
		{Role: "user", Content: "第一轮提问: 扫描目录"},
		{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{
				{ID: "old_call_1", Function: openai.FunctionCall{Name: "list_files"}},
			},
		},
		{Role: "tool", ToolCallID: "old_call_1", Content: "list_files result"},
		{Role: "assistant", Content: "第一轮回答完成：分析完毕。"},
		{Role: "user", Content: "第二轮提问: 混合执行3个工具"},
		{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{
				{ID: "call_a", Function: openai.FunctionCall{Name: "list_files"}},
				{ID: "call_b", Function: openai.FunctionCall{Name: "read_files"}},
				{ID: "call_c", Function: openai.FunctionCall{Name: "terminal"}},
			},
		},
		{Role: "tool", ToolCallID: "call_a", Content: "call_a list_files ok"},
		{Role: "tool", ToolCallID: "call_b", Content: "call_b read_files ok"},
		// 注意: call_c (terminal) 尚无对应的 tool 消息，被高危拦截中断！
	}

	idx4, unexec4 := findUnexecutedToolCalls(msgs4)
	if idx4 != 6 {
		t.Errorf("Case 4 期望最新带 ToolCalls 的 assistant 索引 6，实际: %d", idx4)
	}
	if len(unexec4) != 1 {
		t.Fatalf("Case 4 期望精准抓取 1 个未执行的 ToolCall，实际抓取到: %d", len(unexec4))
	}
	if unexec4[0].ID != "call_c" || unexec4[0].Function.Name != "terminal" {
		t.Errorf("Case 4 期望未执行工具为 call_c (terminal)，实际: %+v", unexec4[0])
	}

	// Case 5: 历史中最新的 Assistant 消息是纯文本回答（无 ToolCalls），前轮的中断不应干扰
	msgs5 := []openai.ChatCompletionMessage{
		{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{
				{ID: "call_x", Function: openai.FunctionCall{Name: "list_files"}},
			},
		},
		{Role: "tool", ToolCallID: "call_x", Content: "ok"},
		{Role: "assistant", Content: "纯文本回答，没有发起新的 ToolCalls"},
	}

	_, unexec5 := findUnexecutedToolCalls(msgs5)
	if len(unexec5) != 0 {
		t.Errorf("Case 5 期望 0 个未执行，实际: %d", len(unexec5))
	}
}
