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

// ListDocuments 获取文档列表 Protobuf RPC / HTTP
func (s *KBService) ListDocuments(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.ListDocumentsResponse, error) {
	docs, err := s.kbBiz.ListDocuments(ctx, req.KbId)
	if err != nil {
		return nil, err
	}

	list := make([]*pb.DocumentInfo, len(docs))
	for i, d := range docs {
		list[i] = &pb.DocumentInfo{
			DocId:       d.DocID,
			KbId:        d.KBID,
			Title:       d.Title,
			SourceType:  d.SourceType,
			DocVersion:  d.DocVersion,
			Category:    d.Category,
			Tags:        d.Tags,
			IsActive:    d.IsActive,
			SourceUrl:   d.SourceURL,
			FilePath:    d.FilePath,
			FileHash:    d.FileHash,
			Status:      d.Status,
			TotalChunks: d.TotalChunks,
			ErrMsg:      d.ErrMsg,
			CreatedAt:   d.CreatedAt.Unix(),
			UpdatedAt:   d.UpdatedAt.Unix(),
		}
	}

	return &pb.ListDocumentsResponse{
		Docs: list,
	}, nil
}

// DeleteDocument 删除文档及其关联向量与切片 Protobuf RPC / HTTP
func (s *KBService) DeleteDocument(ctx context.Context, req *pb.DeleteDocumentRequest) (*pb.DeleteDocumentResponse, error) {
	if err := s.kbBiz.DeleteDocument(ctx, req.DocId); err != nil {
		return nil, err
	}

	return &pb.DeleteDocumentResponse{
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
		return err
	}

	return ctx.Result(http.StatusOK, &pb.UploadFileResponse{
		DocId:       docModel.DocID,
		KbId:        docModel.KBID,
		Title:       docModel.Title,
		SourceType:  docModel.SourceType,
		TotalChunks: docModel.TotalChunks,
		Status:      docModel.Status,
	})
}
