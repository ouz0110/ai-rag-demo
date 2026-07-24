package base

import (
	"context"
	"errors"

	pb "ai-rag-demo/api/base/v1"
	"ai-rag-demo/internal/pkg/database"

	"gorm.io/gorm"
)

type AccountsModel struct {
	// 主键
	ID int64
	// 用户唯一标识
	Openid string
	// 登录账号（手机号/邮箱/用户名等）-用手机号吧
	Account string
	// 密码(哈希存储)
	Password []byte
	// 昵称
	Nickname string
	// 头像URL
	Avatar string
	// 状态 1-启用 2-禁用
	Status int32
	// 创建时间
	CreatedAt int64
	// 更新时间
	UpdatedAt int64
	// 删除时间
	DeletedAt int64
	// 最后登录时间
	LastLoginTime int64
	// 密码加密盐
	PasswordSalt string
}

func (AccountsModel) TableName() string {
	return "accounts"
}

func (m *AccountsModel) DTO() *pb.Account {
	return &pb.Account{
		Id:            m.ID,
		Openid:        m.Openid,
		Account:       m.Account,
		Nickname:      m.Nickname,
		Avatar:        m.Avatar,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
		DeletedAt:     m.DeletedAt,
		LastLoginTime: m.LastLoginTime,
	}
}

// AccountRepo 共创计划数据仓库
type AccountRepo struct {
	database.TableRepo[*AccountsModel]
}

func (s *AccountRepo) GetByOpenid(ctx context.Context, openid string) (*AccountsModel, bool, error) {
	var account AccountsModel
	if err := s.GormDB(ctx).Where("openid=?", openid).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &account, true, nil
}

func (s *AccountRepo) GetByAccount(ctx context.Context, account string) (*AccountsModel, bool, error) {
	var m AccountsModel
	if err := s.GormDB(ctx).Where("account=?", account).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &m, true, nil
}

func (s *AccountRepo) CreateAccount(ctx context.Context, m *AccountsModel) error {
	return s.Create(ctx, m)
}
