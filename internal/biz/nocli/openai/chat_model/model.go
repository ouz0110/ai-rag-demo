package chatmodel

import (
	bizCommon "ai-rag-demo/internal/biz/common"
	"ai-rag-demo/internal/conf"
	"context"

	openai "github.com/sashabaranov/go-openai"
)

const DeepseekV32 = "deepseek-v3.2"

type ChatModel struct {
	client        *openai.Client
	cfg           *conf.Config
	usageRecorder *bizCommon.UsageRecorder
}

func NewChatModel(c *conf.Config, usageRecorder *bizCommon.UsageRecorder) *ChatModel {
	return &ChatModel{
		cfg:           c,
		usageRecorder: usageRecorder,
	}
}

func (s *ChatModel) GetUsageRecorder() *bizCommon.UsageRecorder {
	return s.usageRecorder
}

func (s *ChatModel) GetConfig() *conf.Config {
	return s.cfg
}

func (s *ChatModel) GetOpenAI(ctx context.Context) *openai.Client {
	if s.client == nil {
		cfg := openai.DefaultConfig(s.cfg.Source.OpenAI.APIKey)
		cfg.BaseURL = s.cfg.Source.OpenAI.BaseURL
		s.client = openai.NewClientWithConfig(cfg)
	}
	return s.client
}
