package base

import (
	"context"
	"time"

	pb "ai-rag-demo/api/base/v1"
	cmpb "ai-rag-demo/api/common/v1"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	dataBase "ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/external"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/utils"
)

type AccountBiz struct {
	allDb    *data.DB
	cfg      *conf.Config
	rpcProxy *external.RPCProxy
	cache    *cache.Cache
}

func NewAccountBiz(
	allDb *data.DB,
	cfg *conf.Config,
	rpcProxy *external.RPCProxy,
	cache *cache.Cache,
) *AccountBiz {
	return &AccountBiz{
		allDb:    allDb,
		cfg:      cfg,
		rpcProxy: rpcProxy,
		cache:    cache,
	}
}

// GetUserInfo 获取用户信息
func (s *AccountBiz) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (rsp *pb.Account, err error) {
	ok, wrap := common.UserFromContext(ctx)
	if !ok {
		return nil, cmpb.ErrorUnauthorized("")
	}
	account, exist, err := s.allDb.Base.AccountRepo.GetByOpenid(ctx, wrap.Openid)
	if err != nil {
		log.Errorw(ctx, "get base account by openid", "openid", wrap.Openid, "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}
	if !exist {
		return nil, nil
	}
	return account.DTO(), nil
}

// Register 注册
func (s *AccountBiz) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if _, exist, err := s.allDb.Base.AccountRepo.GetByAccount(ctx, req.Account); err != nil {
		log.Errorw(ctx, "check account exists", "account", req.Account, "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	} else if exist {
		return nil, cmpb.ErrorBadRequest("account already exists")
	}

	openid := utils.GenOpenID()
	salt := utils.GenOpenID()
	passwordHash, err := utils.HashPassword(req.Password, salt, openid)
	if err != nil {
		log.Errorw(ctx, "hash password", "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	now := time.Now().Unix()
	account := &dataBase.AccountsModel{
		Openid:        openid,
		Account:       req.Account,
		Password:      passwordHash,
		PasswordSalt:  salt,
		Nickname:      req.Nickname,
		Avatar:        req.Avatar,
		Status:        1,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastLoginTime: now,
	}
	if err := s.allDb.Base.AccountRepo.CreateAccount(ctx, account); err != nil {
		log.Errorw(ctx, "create account", "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	token := utils.GenerateJWT(openid)
	if err := s.cache.Base.SetToken(ctx, openid, token); err != nil {
		log.Errorw(ctx, "store jwttoken cache", "key", utils.JWTTokenCacheKey(openid), "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	return &pb.RegisterResponse{Token: token}, nil
}

// Login 登录
func (s *AccountBiz) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	account, exist, err := s.allDb.Base.AccountRepo.GetByAccount(ctx, req.Account)
	if err != nil {
		log.Errorw(ctx, "get account by account", "account", req.Account, "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}
	if !exist {
		return nil, cmpb.ErrorBadRequest("account or password incorrect")
	}

	passwordHash, err := utils.HashPassword(req.Password, account.PasswordSalt, account.Openid)
	if err != nil {
		log.Errorw(ctx, "hash password", "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}
	if string(passwordHash) != string(account.Password) {
		return nil, cmpb.ErrorBadRequest("account or password incorrect")
	}

	now := time.Now().Unix()
	account.UpdatedAt = now
	account.LastLoginTime = now
	if _, err := s.allDb.Base.AccountRepo.Update(ctx, account); err != nil {
		log.Errorw(ctx, "update account last login time", "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	token := utils.GenerateJWT(account.Openid)
	if err := s.cache.Base.SetToken(ctx, account.Openid, token); err != nil {
		log.Errorw(ctx, "store jwttoken cache", "key", utils.JWTTokenCacheKey(account.Openid), "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	return &pb.LoginResponse{Token: token}, nil
}

// Logout 登出
func (s *AccountBiz) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	ok, wrap := common.UserFromContext(ctx)
	if !ok {
		return nil, cmpb.ErrorUnauthorized("")
	}
	if err := s.cache.Base.GetCli().Del(ctx, utils.JWTTokenCacheKey(wrap.Openid)).Err(); err != nil {
		log.Errorw(ctx, "delete jwttoken cache", "key", utils.JWTTokenCacheKey(wrap.Openid), "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}
	return &pb.LogoutResponse{}, nil
}

// ChangePassword 修改密码
func (s *AccountBiz) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	ok, wrap := common.UserFromContext(ctx)
	if !ok {
		return nil, cmpb.ErrorUnauthorized("")
	}
	account, exist, err := s.allDb.Base.AccountRepo.GetByOpenid(ctx, wrap.Openid)
	if err != nil {
		log.Errorw(ctx, "get account by openid", "openid", wrap.Openid, "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}
	if !exist {
		return nil, cmpb.ErrorBadRequest("account not found")
	}

	oldPasswordHash, err := utils.HashPassword(req.OldPassword, account.PasswordSalt, account.Openid)
	if err != nil {
		log.Errorw(ctx, "hash old password", "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}
	if string(oldPasswordHash) != string(account.Password) {
		return nil, cmpb.ErrorBadRequest("old password incorrect")
	}

	newSalt := utils.GenOpenID()
	newPasswordHash, err := utils.HashPassword(req.NewPassword, newSalt, account.Openid)
	if err != nil {
		log.Errorw(ctx, "hash new password", "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	account.Password = newPasswordHash
	account.PasswordSalt = newSalt
	account.UpdatedAt = time.Now().Unix()
	if _, err := s.allDb.Base.AccountRepo.Update(ctx, account); err != nil {
		log.Errorw(ctx, "update account password", "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	if err := s.cache.Base.GetCli().Del(ctx, utils.JWTTokenCacheKey(account.Openid)).Err(); err != nil {
		log.Errorw(ctx, "delete jwttoken cache after password change", "key", utils.JWTTokenCacheKey(account.Openid), "err", err)
		return nil, cmpb.ErrorInternalServerError("")
	}

	return &pb.ChangePasswordResponse{}, nil
}
