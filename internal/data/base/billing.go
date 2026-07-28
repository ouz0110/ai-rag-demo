package base

import (
	"context"
	"errors"
	"time"

	"ai-rag-demo/internal/pkg/database"

	"gorm.io/gorm"
)

type ServiceType string

const (
	ServiceTypeOpenAI    ServiceType = "openai"
	ServiceTypeEmbedding ServiceType = "embedding"
	ServiceTypeRerank    ServiceType = "rerank"
)

// BillingRuleModel 计费单价规则模型
type BillingRuleModel struct {
	ID              uint64      `gorm:"primaryKey;column:id;comment:主键ID"`
	Provider        string      `gorm:"column:provider;type:varchar(32);not null;comment:供应商"`
	ServiceType     ServiceType `gorm:"column:service_type;type:varchar(32);not null;comment:服务类型"`
	ModelName       string      `gorm:"column:model_name;type:varchar(64);not null;comment:模型名称"`
	InputUnitPrice  float64     `gorm:"column:input_unit_price;type:decimal(18,6);not null;default:0.000000;comment:输入单价"`
	OutputUnitPrice float64     `gorm:"column:output_unit_price;type:decimal(18,6);not null;default:0.000000;comment:输出单价"`
	UnitSize        int         `gorm:"column:unit_size;type:int;not null;default:1000;comment:计费单位基数"`
	Status          int32       `gorm:"column:status;type:tinyint;not null;default:1;comment:状态: 1-生效 2-失效"`
	CreatedAt       time.Time   `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time   `gorm:"column:updated_at;autoUpdateTime"`
}

func (BillingRuleModel) TableName() string {
	return "billing_rules"
}

// UserBalanceModel 用户AI计费余额模型
type UserBalanceModel struct {
	ID            uint64    `gorm:"primaryKey;column:id;comment:主键ID"`
	UserID        string    `gorm:"column:user_id;type:varchar(64);not null;uniqueIndex;comment:用户ID"`
	Balance       float64   `gorm:"column:balance;type:decimal(18,6);not null;default:100.000000;comment:当前可用余额"`
	GiftBalance   float64   `gorm:"column:gift_balance;type:decimal(18,6);not null;default:0.000000;comment:赠送余额"`
	TotalConsumed float64   `gorm:"column:total_consumed;type:decimal(18,6);not null;default:0.000000;comment:历史累计消耗"`
	Version       uint64    `gorm:"column:version;type:bigint;not null;default:0;comment:乐观锁版本"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserBalanceModel) TableName() string {
	return "user_balances"
}

// BillingUsageLogModel AI消费消耗流水明细模型
type BillingUsageLogModel struct {
	ID               uint64      `gorm:"primaryKey;column:id;comment:主键ID"`
	RequestID        string      `gorm:"column:request_id;type:varchar(64);not null;uniqueIndex;comment:请求ID"`
	UserID           string      `gorm:"column:user_id;type:varchar(64);not null;index:idx_user_created;comment:用户ID"`
	ServiceType      ServiceType `gorm:"column:service_type;type:varchar(32);not null;comment:服务类型"`
	Provider         string      `gorm:"column:provider;type:varchar(32);not null;comment:供应商"`
	ModelName        string      `gorm:"column:model_name;type:varchar(64);not null;comment:使用模型"`
	PromptTokens     int32       `gorm:"column:prompt_tokens;type:int;not null;default:0;comment:输入Tokens数"`
	CompletionTokens int32       `gorm:"column:completion_tokens;type:int;not null;default:0;comment:输出Tokens数"`
	TotalTokens      int32       `gorm:"column:total_tokens;type:int;not null;default:0;comment:总Tokens数"`
	DocCount         int32       `gorm:"column:doc_count;type:int;not null;default:0;comment:Rerank文档数"`
	TotalCost        float64     `gorm:"column:total_cost;type:decimal(18,6);not null;default:0.000000;comment:实际扣除费用"`
	Status           int32       `gorm:"column:status;type:tinyint;not null;default:1;comment:状态: 1-成功 2-部分退款 3-失败"`
	RawUsageJSON     string      `gorm:"column:raw_usage_json;type:text;comment:原始Usage数据"`
	CreatedAt        time.Time   `gorm:"column:created_at;autoCreateTime;index:idx_user_created"`
}

func (BillingUsageLogModel) TableName() string {
	return "billing_usage_logs"
}

type BillingRepo struct {
	database.TableRepo[*BillingUsageLogModel]
}

func NewBillingRepo(db database.TableRepo[*BillingUsageLogModel]) BillingRepo {
	return BillingRepo{TableRepo: db}
}

// GetUserBalance 获取用户余额记录 (如果不存在则自动创建并充值 100 试用额度)
func (r *BillingRepo) GetUserBalance(ctx context.Context, userID string) (*UserBalanceModel, error) {
	var balance UserBalanceModel
	err := r.GormDB(ctx).Model(&UserBalanceModel{}).Where("user_id = ?", userID).First(&balance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			balance = UserBalanceModel{
				UserID:        userID,
				Balance:       100.0, // 初始化赠送 100 额度
				GiftBalance:   100.0,
				TotalConsumed: 0.0,
				Version:       1,
			}
			if createErr := r.GormDB(ctx).Model(&UserBalanceModel{}).Create(&balance).Error; createErr != nil {
				return nil, createErr
			}
			return &balance, nil
		}
		return nil, err
	}
	return &balance, nil
}

// RechargeUserBalance 给指定用户追加额度
func (r *BillingRepo) RechargeUserBalance(ctx context.Context, userID string, amount float64) (*UserBalanceModel, error) {
	if amount <= 0 {
		return nil, errors.New("recharge amount must be positive")
	}
	err := r.GormDB(ctx).Model(&UserBalanceModel{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"balance":      gorm.Expr("balance + ?", amount),
			"gift_balance": gorm.Expr("gift_balance + ?", amount),
		}).Error
	if err != nil {
		return nil, err
	}
	return r.GetUserBalance(ctx, userID)
}

// DeductUserBalance 扣除用户余额
func (r *BillingRepo) DeductUserBalance(ctx context.Context, userID string, cost float64) error {
	if cost <= 0 {
		return nil
	}
	result := r.GormDB(ctx).Model(&UserBalanceModel{}).
		Where("user_id = ? AND balance >= ?", userID, cost).
		Updates(map[string]interface{}{
			"balance":        gorm.Expr("balance - ?", cost),
			"total_consumed": gorm.Expr("total_consumed + ?", cost),
			"version":        gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("insufficient balance")
	}
	return nil
}

// IsRequestProcessed 检查 request_id 是否已存在 (幂等校验)
func (r *BillingRepo) IsRequestProcessed(ctx context.Context, requestID string) (bool, error) {
	var count int64
	err := r.GormDB(ctx).Model(&BillingUsageLogModel{}).Where("request_id = ?", requestID).Count(&count).Error
	return count > 0, err
}

// CreateUsageLog 写入消费明细记录
func (r *BillingRepo) CreateUsageLog(ctx context.Context, log *BillingUsageLogModel) error {
	return r.GormDB(ctx).Model(&BillingUsageLogModel{}).Create(log).Error
}

// ListUsageLogs 分页获取用户消费流水
func (r *BillingRepo) ListUsageLogs(ctx context.Context, userID string, limit, offset int) ([]*BillingUsageLogModel, int64, error) {
	var logs []*BillingUsageLogModel
	var total int64

	db := r.GormDB(ctx).Model(&BillingUsageLogModel{}).Where("user_id = ?", userID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := db.Order("id desc").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// ListBillingRules 获取所有有效的计费规则
func (r *BillingRepo) ListBillingRules(ctx context.Context) ([]*BillingRuleModel, error) {
	var rules []*BillingRuleModel
	err := r.GormDB(ctx).Model(&BillingRuleModel{}).Where("status = 1").Find(&rules).Error
	return rules, err
}
