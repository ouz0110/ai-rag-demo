package base

import (
	"context"
	"errors"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/pkg/database"

	"gorm.io/gorm"
)

// NocliSessionModel nocli会话表
type NocliSessionModel struct {
	// 主键
	ID int64
	// 会话唯一标识
	SessionID string
	// 用户openid
	Openid string
	// 会话名称
	Name string
	// 会话状态
	Status pb.SessionStatus
	// 创建时间
	CreatedAt int64
	// 更新时间
	UpdatedAt int64
}

func (NocliSessionModel) TableName() string {
	return "nocli_sessions"
}

func (m *NocliSessionModel) DTO() *NocliSession {
	return &NocliSession{
		ID:        m.ID,
		SessionID: m.SessionID,
		Openid:    m.Openid,
		Name:      m.Name,
		Status:    m.Status,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// NocliSession 会话DTO
type NocliSession struct {
	ID        int64            `json:"id"`
	SessionID string           `json:"session_id"`
	Openid    string           `json:"openid"`
	Name      string           `json:"name"`
	Status    pb.SessionStatus `json:"status"`
	CreatedAt int64            `json:"created_at"`
	UpdatedAt int64            `json:"updated_at"`
}

// NocliSessionRepo 会话数据仓库
type NocliSessionRepo struct {
	database.TableRepo[*NocliSessionModel]
}

func (s *NocliSessionRepo) GetBySessionID(ctx context.Context, sessionID string) (*NocliSessionModel, bool, error) {
	var m NocliSessionModel
	if err := s.GormDB(ctx).Model(&NocliSessionModel{}).Where("session_id=?", sessionID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &m, true, nil
}

func (s *NocliSessionRepo) GetByOpenid(ctx context.Context, openid string) ([]NocliSessionModel, error) {
	var list []NocliSessionModel
	if err := s.GormDB(ctx).Model(&NocliSessionModel{}).Where("openid=?", openid).Order("updated_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *NocliSessionRepo) Create(ctx context.Context, m *NocliSessionModel) error {
	return s.TableRepo.Create(ctx, m)
}

func (s *NocliSessionRepo) UpdateName(ctx context.Context, sessionID, name string) error {
	return s.GormDB(ctx).Model(&NocliSessionModel{}).Where("session_id=?", sessionID).Update("name", name).Error
}

func (s *NocliSessionRepo) UpdateStatus(ctx context.Context, sessionID string, status pb.SessionStatus) error {
	return s.GormDB(ctx).Model(&NocliSessionModel{}).Where("session_id=?", sessionID).Update("status", status).Error
}

func (s *NocliSessionRepo) UpdateUpdatedAt(ctx context.Context, sessionID string, updatedAt int64) error {
	return s.GormDB(ctx).Model(&NocliSessionModel{}).Where("session_id=?", sessionID).Update("updated_at", updatedAt).Error
}

func (s *NocliSessionRepo) ListByOpenid(ctx context.Context, openid string, page, pageSize int32) ([]NocliSessionModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := int((page - 1) * pageSize)

	var total int64
	db := s.GormDB(ctx).Model(&NocliSessionModel{}).Where("openid=?", openid)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []NocliSessionModel
	if err := db.Order("updated_at DESC").Offset(offset).Limit(int(pageSize)).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *NocliSessionRepo) DeleteBySessionID(ctx context.Context, sessionID string) error {
	return s.GormDB(ctx).Model(&NocliSessionModel{}).Where("session_id=?", sessionID).Delete(&NocliSessionModel{}).Error
}
