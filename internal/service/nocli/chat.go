package nocli

import (
	"context"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli"
)

type ChatService struct {
	pb.UnimplementedNocliChatServer
	chatBiz *nocli.ChatBiz
}

func NewChatService(
	chatBiz *nocli.ChatBiz,
) *ChatService {
	return &ChatService{
		chatBiz: chatBiz,
	}
}

func (s *ChatService) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	return s.chatBiz.Chat(ctx, req)
}
