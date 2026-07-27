package nocli

import (
	"context"
	"net/http"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/biz/nocli"

	transHttp "github.com/go-kratos/kratos/v2/transport/http"
)

type KBService struct {
	pb.UnimplementedKnowledgeBaseServer
	kbBiz *nocli.KBBiz
}

func NewKBService(kbBiz *nocli.KBBiz) *KBService {
	return &KBService{
		kbBiz: kbBiz,
	}
}

// CreateKnowledgeBase 创建新知识库 Protobuf RPC / HTTP
func (s *KBService) CreateKnowledgeBase(ctx context.Context, req *pb.CreateKnowledgeBaseRequest) (*pb.CreateKnowledgeBaseResponse, error) {
	kbModel, err := s.kbBiz.CreateKnowledgeBase(ctx, req.Name, req.Description)
	if err != nil {
		return nil, err
	}

	return &pb.CreateKnowledgeBaseResponse{
		Kb: &pb.KnowledgeBaseInfo{
			KbId:        kbModel.KBID,
			Name:        kbModel.Name,
			Description: kbModel.Description,
			IsDefault:   kbModel.IsDefault,
			CreatedAt:   kbModel.CreatedAt.Unix(),
			UpdatedAt:   kbModel.UpdatedAt.Unix(),
		},
	}, nil
}

// ListKnowledgeBases 获取知识库列表 Protobuf RPC / HTTP
func (s *KBService) ListKnowledgeBases(ctx context.Context, req *pb.ListKnowledgeBasesRequest) (*pb.ListKnowledgeBasesResponse, error) {
	kbs, err := s.kbBiz.ListKnowledgeBases(ctx)
	if err != nil {
		return nil, err
	}

	list := make([]*pb.KnowledgeBaseInfo, len(kbs))
	for i, k := range kbs {
		list[i] = &pb.KnowledgeBaseInfo{
			KbId:        k.KBID,
			Name:        k.Name,
			Description: k.Description,
			IsDefault:   k.IsDefault,
			CreatedAt:   k.CreatedAt.Unix(),
			UpdatedAt:   k.UpdatedAt.Unix(),
		}
	}

	return &pb.ListKnowledgeBasesResponse{
		Kbs: list,
	}, nil
}

// DeleteKnowledgeBase 删除自定义知识库 Protobuf RPC / HTTP
func (s *KBService) DeleteKnowledgeBase(ctx context.Context, req *pb.DeleteKnowledgeBaseRequest) (*pb.DeleteKnowledgeBaseResponse, error) {
	if err := s.kbBiz.DeleteKnowledgeBase(ctx, req.KbId); err != nil {
		return nil, err
	}

	return &pb.DeleteKnowledgeBaseResponse{
		Success: true,
	}, nil
}

// UploadFileHTTP 自定义 HTTP multipart 文件上传 Handler (保持独立处理)
func (s *KBService) UploadFileHTTP(ctx transHttp.Context) error {
	req := ctx.Request()

	// 限制 64MB 单文件上传大小
	if err := req.ParseMultipartForm(64 << 20); err != nil {
		return ctx.Result(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "parse multipart form error: " + err.Error(),
		})
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		return ctx.Result(http.StatusBadRequest, map[string]interface{}{
			"code":    400,
			"message": "form file field 'file' is required: " + err.Error(),
		})
	}
	defer file.Close()

	kbID := req.FormValue("kb_id")

	// 执行上传与解析向量化
	docModel, err := s.kbBiz.UploadAndIngestFile(req.Context(), kbID, header.Filename, file)
	if err != nil {
		return ctx.Result(http.StatusInternalServerError, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
	}

	return ctx.Result(http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "file uploaded and ingested successfully",
		"data": map[string]interface{}{
			"doc_id":       docModel.DocID,
			"kb_id":        docModel.KBID,
			"title":        docModel.Title,
			"source_type":  docModel.SourceType,
			"total_chunks": docModel.TotalChunks,
			"status":       docModel.Status,
		},
	})
}
