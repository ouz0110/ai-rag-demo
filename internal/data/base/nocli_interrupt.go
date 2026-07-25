package base

import (
	"context"
	"errors"

	pb "ai-rag-demo/api/nocli/v1"
	"ai-rag-demo/internal/pkg/database"

	"gorm.io/gorm"
)

// NocliInterruptModel nocli中断审批表
type NocliInterruptModel struct {
	// 主键
	ID int64
	// 中断唯一标识
	InterruptID string
	// 会话ID
	SessionID string
	// 中断状态
	Status pb.InterruptStatus
	// OpenAI ToolCallID
	ToolCallID string
	// 待执行的工具名称
	ToolName string
	// 待执行的工具参数(JSON)
	Arguments string
	// 拒绝原因/用户意见
	RejectReason string
	// 处理人openid
	HandlerOpenid string
	// 创建时间
	CreatedAt int64
	// 处理时间
	HandledAt int64
}

func (NocliInterruptModel) TableName() string {
	return "nocli_interrupts"
}

func (m *NocliInterruptModel) DTO() *NocliInterrupt {
	return &NocliInterrupt{
		ID:            m.ID,
		InterruptID:   m.InterruptID,
		SessionID:     m.SessionID,
		Status:        m.Status,
		ToolCallID:    m.ToolCallID,
		ToolName:      m.ToolName,
		Arguments:     m.Arguments,
		RejectReason:  m.RejectReason,
		HandlerOpenid: m.HandlerOpenid,
		CreatedAt:     m.CreatedAt,
		HandledAt:     m.HandledAt,
	}
}

// NocliInterrupt DTO
type NocliInterrupt struct {
	ID            int64              `json:"id"`
	InterruptID   string             `json:"interrupt_id"`
	SessionID     string             `json:"session_id"`
	Status        pb.InterruptStatus `json:"status"`
	ToolCallID    string             `json:"tool_call_id"`
	ToolName      string             `json:"tool_name"`
	Arguments     string             `json:"arguments"`
	RejectReason  string             `json:"reject_reason"`
	HandlerOpenid string             `json:"handler_openid"`
	CreatedAt     int64              `json:"created_at"`
	HandledAt     int64              `json:"handled_at"`
}

// NocliInterruptRepo 中断数据仓库
type NocliInterruptRepo struct {
	database.TableRepo[*NocliInterruptModel]
}

func (s *NocliInterruptRepo) GetByInterruptID(ctx context.Context, interruptID string) (*NocliInterruptModel, bool, error) {
	var m NocliInterruptModel
	if err := s.GormDB(ctx).Model(&NocliInterruptModel{}).Where("interrupt_id=?", interruptID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &m, true, nil
}

func (s *NocliInterruptRepo) GetPendingBySessionID(ctx context.Context, sessionID string) ([]NocliInterruptModel, error) {
	var list []NocliInterruptModel
	if err := s.GormDB(ctx).Model(&NocliInterruptModel{}).Where("session_id=? AND status=?", sessionID, pb.InterruptStatus_INTERRUPT_STATUS_PENDING).Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *NocliInterruptRepo) CreateBatch(ctx context.Context, models []*NocliInterruptModel) error {
	if len(models) == 0 {
		return nil
	}
	return s.GormDB(ctx).Model(&NocliInterruptModel{}).Create(&models).Error
}

func (s *NocliInterruptRepo) UpdateStatus(ctx context.Context, interruptID string, status pb.InterruptStatus, handledAt int64, handlerOpenid, reason string) error {
	updates := map[string]interface{}{
		"status":         status,
		"handled_at":     handledAt,
		"handler_openid": handlerOpenid,
		"reject_reason":  reason,
	}
	return s.GormDB(ctx).Model(&NocliInterruptModel{}).Where("interrupt_id=?", interruptID).Updates(updates).Error
}
