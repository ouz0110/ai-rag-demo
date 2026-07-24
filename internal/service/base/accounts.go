package base

import (
	"context"

	pb "ai-rag-demo/api/base/v1"
	"ai-rag-demo/internal/biz/base"
)

type AccountService struct {
	pb.UnimplementedAccountsServer
	accountBiz *base.AccountBiz
}

func NewAccountService(
	accountBiz *base.AccountBiz,
) *AccountService {
	return &AccountService{
		accountBiz: accountBiz,
	}
}

func (s *AccountService) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	return s.accountBiz.ChangePassword(ctx, req)
}
func (s *AccountService) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.Account, error) {
	return s.accountBiz.GetUserInfo(ctx, req)
}
func (s *AccountService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return s.accountBiz.Login(ctx, req)
}
func (s *AccountService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return s.accountBiz.Logout(ctx, req)
}
func (s *AccountService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return s.accountBiz.Register(ctx, req)
}
