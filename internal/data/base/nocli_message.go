package base

import (
	"ai-rag-demo/internal/pkg/database"
	"context"
)

// NocliMessageModel nocli消息表
type NocliMessageModel struct {
	// 主键
	ID int64
	// 会话ID
	SessionID string
	// 消息内容（JSON字符串）
	Msg string
	// 创建时间
	CreatedAt int64
}

func (NocliMessageModel) TableName() string {
	return "nocli_messages"
}

func (m *NocliMessageModel) DTO() *NocliMessage {
	return &NocliMessage{
		ID:        m.ID,
		SessionID: m.SessionID,
		Msg:       m.Msg,
		CreatedAt: m.CreatedAt,
	}
}

// NocliMessage 消息DTO
type NocliMessage struct {
	ID        int64  `json:"id"`
	SessionID string `json:"session_id"`
	Msg       string `json:"msg"`
	CreatedAt int64  `json:"created_at"`
}

// NocliMessageRepo 消息数据仓库
type NocliMessageRepo struct {
	database.TableRepo[*NocliMessageModel]
}

func (s *NocliMessageRepo) GetBySessionID(ctx context.Context, sessionID string) ([]NocliMessageModel, error) {
	var list []NocliMessageModel
	if err := s.GormDB(ctx).Where("session_id=?", sessionID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *NocliMessageRepo) GetRecentBySessionID(ctx context.Context, sessionID string, limit int) ([]NocliMessageModel, error) {
	var list []NocliMessageModel
	if err := s.GormDB(ctx).Where("session_id=?", sessionID).Order("id DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *NocliMessageRepo) Create(ctx context.Context, m *NocliMessageModel) error {
	return s.TableRepo.Create(ctx, m)
}

func (s *NocliMessageRepo) CreateBatch(ctx context.Context, ms []*NocliMessageModel) error {
	if len(ms) == 0 {
		return nil
	}
	return s.GormDB(ctx).Create(ms).Error
}

func (s *NocliMessageRepo) DeleteBySessionID(ctx context.Context, sessionID string) error {
	return s.GormDB(ctx).Model(&NocliMessageModel{}).Where("session_id=?", sessionID).Delete(&NocliMessageModel{}).Error
}
