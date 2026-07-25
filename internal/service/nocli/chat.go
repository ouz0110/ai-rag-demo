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

func (s *ChatService) Completion(ctx context.Context, req *pb.CompletionRequest) (*pb.CompletionResponse, error) {
	return s.chatBiz.Completion(ctx, req)
}

func (s *ChatService) Resume(ctx context.Context, req *pb.ResumeRequest) (*pb.CompletionResponse, error) {
	return s.chatBiz.Resume(ctx, req)
}
