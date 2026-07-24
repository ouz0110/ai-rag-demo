package nocli

import (
	"context"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/cache"
)

type ChatBiz struct {
	cache *cache.Cache
}

func NewChatBiz(
	cache *cache.Cache,
) *ChatBiz {
	return &ChatBiz{
		cache: cache,
	}
}

func (s *ChatBiz) Chat(ctx context.Context, req *pb.ChatRequest) (rsp *pb.ChatResponse, err error) {
	return &pb.ChatResponse{
		Reply:     "ok",
		SessionId: req.SessionId,
	}, nil
}
