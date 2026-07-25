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
	if err := s.GormDB(ctx).Model(&NocliMessageModel{}).Where("session_id=?", sessionID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *NocliMessageRepo) ListBySessionIDPage(ctx context.Context, sessionID string, page, pageSize int32) ([]NocliMessageModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := int((page - 1) * pageSize)

	var total int64
	db := s.GormDB(ctx).Model(&NocliMessageModel{}).Where("session_id=?", sessionID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []NocliMessageModel
	if err := db.Order("id DESC").Offset(offset).Limit(int(pageSize)).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	// 翻转切片还原按时间/ID正序 (使单页内的消息保持时间先后顺序)
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}

	return list, total, nil
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
