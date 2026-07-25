package session

import (
	"encoding/json"
	"testing"

	pb "ai-rag-demo/api/nocli/v1"
	dataBase "ai-rag-demo/internal/data/base"

	openai "github.com/sashabaranov/go-openai"
)

func TestMapMessageModelToStreamChunks(t *testing.T) {
	sessionID := "ffad38ed-786c-47d2-87b6-bf28fb6b4458"

	// 1. 用户提问消息
	userMsgObj := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "https://agentskills.io/specification 这个网页有什么信息",
	}
	userMsgBytes, _ := json.Marshal(userMsgObj)
	userModel := dataBase.NocliMessageModel{
		ID:        134,
		SessionID: sessionID,
		Msg:       string(userMsgBytes),
	}

	userChunks := MapMessageModelToStreamChunks(sessionID, userModel)
	if len(userChunks) != 1 {
		t.Fatalf("Expected 1 chunk for user msg, got %d", len(userChunks))
	}
	if userChunks[0].Role != "user" || userChunks[0].Event != pb.StreamEventType_SET_DONE {
		t.Errorf("User chunk mismatch: role=%s, event=%s", userChunks[0].Role, userChunks[0].Event)
	}

	// 2. 助手文本+工具调用消息 (ID: 135)
	asstMsgObj := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: "我来帮您抓取这个网页的信息。首先，我需要读取网页抓取技能的SOP文档，然后执行抓取任务。\n\n",
		ToolCalls: []openai.ToolCall{
			{
				ID:   "call_df15c2eefd15475db0eddbf7",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "read_files",
					Arguments: `{"files": ["skills/web-fetcher/SKILL.md"]}`,
				},
			},
		},
	}
	asstMsgBytes, _ := json.Marshal(asstMsgObj)
	asstModel := dataBase.NocliMessageModel{
		ID:        135,
		SessionID: sessionID,
		Msg:       string(asstMsgBytes),
	}

	asstChunks := MapMessageModelToStreamChunks(sessionID, asstModel)
	if len(asstChunks) != 2 {
		t.Fatalf("Expected 2 chunks for assistant msg with content and tool_call, got %d", len(asstChunks))
	}

	// 校验 Chunk 1: 文本 Delta
	if asstChunks[0].Role != "assistant" || asstChunks[0].Event != pb.StreamEventType_SET_TEXT_DELTA {
		t.Errorf("Assistant text chunk mismatch: event=%s", asstChunks[0].Event)
	}
	if asstChunks[0].Text != asstMsgObj.Content {
		t.Errorf("Assistant text content lost: got %q, want %q", asstChunks[0].Text, asstMsgObj.Content)
	}

	// 校验 Chunk 2: 工具启动
	if asstChunks[1].Role != "assistant" || asstChunks[1].Event != pb.StreamEventType_SET_TOOL_START {
		t.Errorf("Assistant tool start chunk mismatch: event=%s", asstChunks[1].Event)
	}
	if asstChunks[1].ToolInfo == nil || asstChunks[1].ToolInfo.ToolName != "read_files" {
		t.Errorf("Assistant tool info mismatch: %+v", asstChunks[1].ToolInfo)
	}

	// 3. 工具结果响应消息 (ID: 136)
	toolMsgObj := openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    "--- FILE: skills\\web-fetcher\\SKILL.md ---",
		ToolCallID: "call_df15c2eefd15475db0eddbf7",
	}
	toolMsgBytes, _ := json.Marshal(toolMsgObj)
	toolModel := dataBase.NocliMessageModel{
		ID:        136,
		SessionID: sessionID,
		Msg:       string(toolMsgBytes),
	}

	toolChunks := MapMessageModelToStreamChunks(sessionID, toolModel)
	if len(toolChunks) != 1 {
		t.Fatalf("Expected 1 chunk for tool msg, got %d", len(toolChunks))
	}
	if toolChunks[0].Role != "tool" || toolChunks[0].Event != pb.StreamEventType_SET_TOOL_RESULT {
		t.Errorf("Tool result chunk mismatch: role=%s, event=%s", toolChunks[0].Role, toolChunks[0].Event)
	}
	if toolChunks[0].ToolInfo.ToolCallId != "call_df15c2eefd15475db0eddbf7" {
		t.Errorf("Tool call id mismatch: %s", toolChunks[0].ToolInfo.ToolCallId)
	}
}
