package nocli

import (
	"context"
	"io"
	"net/http"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli"
	"ai-rag-demo/internal/pkg/sse"

	"google.golang.org/protobuf/encoding/protojson"
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

// Completion 非流式一问一答 RPC
func (s *ChatService) Completion(ctx context.Context, req *pb.CompletionRequest) (*pb.StreamChunk, error) {
	return s.chatBiz.Completion(ctx, req)
}

// Resume 非流式恢复执行 RPC
func (s *ChatService) Resume(ctx context.Context, req *pb.ResumeRequest) (*pb.StreamChunk, error) {
	return s.chatBiz.Resume(ctx, req)
}

// StreamCompletion 原生 gRPC 流式服务端接口
func (s *ChatService) StreamCompletion(req *pb.CompletionRequest, stream pb.NocliChat_StreamCompletionServer) error {
	ctx := stream.Context()
	emitter := func(chunk *pb.StreamChunk) {
		_ = stream.Send(chunk)
	}

	return s.chatBiz.StreamCompletion(ctx, req, emitter)
}

// StreamResume 原生 gRPC 流式恢复服务端接口
func (s *ChatService) StreamResume(req *pb.ResumeRequest, stream pb.NocliChat_StreamResumeServer) error {
	ctx := stream.Context()
	emitter := func(chunk *pb.StreamChunk) {
		_ = stream.Send(chunk)
	}

	return s.chatBiz.StreamResume(ctx, req, emitter)
}

var protoUnmarshaler = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

// StreamCompletionHTTP 专门提供给 HTTP/REST 框架 (如 Kratos/Gin/net.http) 的 SSE 响应接口
func (s *ChatService) StreamCompletionHTTP(w http.ResponseWriter, r *http.Request) {
	emitter, err := sse.NewStreamEmitter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	var req pb.CompletionRequest
	if err := protoUnmarshaler.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	_ = s.chatBiz.StreamCompletion(r.Context(), &req, emitter)
}

// StreamResumeHTTP 专门提供给 HTTP/REST 框架 (如 Kratos/Gin/net.http) 的 SSE 恢复响应接口
func (s *ChatService) StreamResumeHTTP(w http.ResponseWriter, r *http.Request) {
	emitter, err := sse.NewStreamEmitter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	var req pb.ResumeRequest
	if err := protoUnmarshaler.Unmarshal(bodyBytes, &req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	_ = s.chatBiz.StreamResume(r.Context(), &req, emitter)
}
