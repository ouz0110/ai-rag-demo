package chatmodel

import (
	"ai-rag-demo/internal/conf"
	"context"

	openai "github.com/sashabaranov/go-openai"
)

type ChatModel struct {
	client *openai.Client
	cfg    *conf.Config
}

func NewChatModel(c *conf.Config) *ChatModel {
	return &ChatModel{
		cfg: c,
	}
}

func (s *ChatModel) GetOpenAI(ctx context.Context) *openai.Client {
	if s.client == nil {
		cfg := openai.DefaultConfig(s.cfg.Source.OpenAI.APIKey)
		cfg.BaseURL = s.cfg.Source.OpenAI.BaseURL
		s.client = openai.NewClientWithConfig(cfg)
	}
	return s.client
}
