package base

import (
	"context"

	pb "ai-rag-demo/api/nocli/v1"
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/openai/compressor"
	dataBase "ai-rag-demo/internal/data/base"

	openai "github.com/sashabaranov/go-openai"
)

type LoopResult struct {
	AgentName        string
	Messages         []openai.ChatCompletionMessage
	Reply            string
	Status           pb.SessionStatus
	PendingToolCalls []*pb.PendingToolCall
	NewCheckpointMsg *openai.ChatCompletionMessage
}

type ProcessToolCallsResult struct {
	HasInterrupt     bool
	HasReject        bool
	PendingInterrupt *dataBase.NocliInterruptModel
	PendingToolCall  *pb.PendingToolCall
	ExecutedMsgs     []openai.ChatCompletionMessage
}

// MessageFetcher 获取 LLM Assistant 消息的策略闭包
type MessageFetcher func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionMessage, error)

// StreamEmitter 事件推送闭包
type StreamEmitter func(chunk *pb.StreamChunk)

// NoopStreamEmitter 空操作闭包
var NoopStreamEmitter StreamEmitter = func(chunk *pb.StreamChunk) {}

type RunOptions struct {
	SessionID     string
	Messages      []openai.ChatCompletionMessage
	ApprovedTools map[string]bool
	RejectedTools map[string]string
	Emitter       StreamEmitter
	Fetcher       MessageFetcher
	SyncFetcher   MessageFetcher
	Compressor    compressor.ICompressor
	CompressCount int32
}

type IAgent interface {
	Name() string
	Description() string
	SystemPrompt(workDir string) string
	MaxIterations() int
	Model() string
	Tools() []openai.Tool
	Run(ctx context.Context, opts *RunOptions) (*LoopResult, error)
	GetStreamFetcher(sessionID string, chatModel *chatmodel.ChatModel, emitter StreamEmitter) MessageFetcher
	GetSyncFetcher(chatModel *chatmodel.ChatModel) MessageFetcher
}
