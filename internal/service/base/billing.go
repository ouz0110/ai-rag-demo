package base

import (
	"context"

	pb "ai-rag-demo/api/base/v1"
	bizBase "ai-rag-demo/internal/biz/base"
	"ai-rag-demo/internal/common"
)

type BillingService struct {
	pb.UnimplementedBillingServer
	billingBiz *bizBase.BillingBiz
}

func NewBillingService(bBiz *bizBase.BillingBiz) *BillingService {
	return &BillingService{billingBiz: bBiz}
}

func (s *BillingService) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		if ok, user := common.UserFromContext(ctx); ok && user.Openid != "" {
			userID = user.Openid
		}
	}
	if userID == "" {
		userID = "default_user"
	}

	bal, err := s.billingBiz.GetUserBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &pb.GetBalanceResponse{
		UserId:        bal.UserID,
		Balance:       bal.Balance,
		GiftBalance:   bal.GiftBalance,
		TotalConsumed: bal.TotalConsumed,
	}, nil
}

func (s *BillingService) Recharge(ctx context.Context, req *pb.RechargeRequest) (*pb.RechargeResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		if ok, user := common.UserFromContext(ctx); ok && user.Openid != "" {
			userID = user.Openid
		}
	}
	if userID == "" {
		userID = "default_user"
	}

	bal, err := s.billingBiz.RechargeUserBalance(ctx, userID, req.GetAmount())
	if err != nil {
		return nil, err
	}

	return &pb.RechargeResponse{
		UserId:        bal.UserID,
		Balance:       bal.Balance,
		GiftBalance:   bal.GiftBalance,
		TotalConsumed: bal.TotalConsumed,
	}, nil
}

func (s *BillingService) ListLogs(ctx context.Context, req *pb.ListLogsRequest) (*pb.ListLogsResponse, error) {
	userID := req.GetUserId()
	if userID == "" {
		if ok, user := common.UserFromContext(ctx); ok && user.Openid != "" {
			userID = user.Openid
		}
	}
	if userID == "" {
		userID = "default_user"
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	page := int(req.GetPage())
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	logs, total, err := s.billingBiz.ListUsageLogs(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	pbLogs := make([]*pb.UsageLogItem, 0, len(logs))
	for _, l := range logs {
		var createdAt int64
		if !l.CreatedAt.IsZero() {
			createdAt = l.CreatedAt.UnixMilli()
		}
		pbLogs = append(pbLogs, &pb.UsageLogItem{
			RequestId:        l.RequestID,
			UserId:           l.UserID,
			ServiceType:      string(l.ServiceType),
			Provider:         l.Provider,
			ModelName:        l.ModelName,
			PromptTokens:     l.PromptTokens,
			CompletionTokens: l.CompletionTokens,
			TotalTokens:      l.TotalTokens,
			DocCount:         l.DocCount,
			TotalCost:        l.TotalCost,
			Status:           l.Status,
			CreatedAt:        createdAt,
		})
	}

	return &pb.ListLogsResponse{
		Total: total,
		Page:  int32(page),
		Limit: int32(limit),
		Logs:  pbLogs,
	}, nil
}

func (s *BillingService) GetPricing(ctx context.Context, req *pb.GetPricingRequest) (*pb.GetPricingResponse, error) {
	pricesMap := s.billingBiz.GetPricingMap()
	resMap := make(map[string]*pb.ModelPricing, len(pricesMap))
	for k, v := range pricesMap {
		if v != nil {
			resMap[k] = &pb.ModelPricing{
				InputUnitPrice:  v.InputUnitPrice,
				OutputUnitPrice: v.OutputUnitPrice,
				UnitSize:        int32(v.UnitSize),
			}
		}
	}
	return &pb.GetPricingResponse{
		Prices: resMap,
	}, nil
}
