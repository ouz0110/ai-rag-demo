package rag

import "time"

// KnowledgeDocumentModel 知识库主文档表结构模型
type KnowledgeDocumentModel struct {
	ID           int64     `gorm:"primaryKey;autoIncrement;comment:主键ID"`                                      // 主键ID
	TenantID     string    `gorm:"type:varchar(64);not null;index:idx_tenant_doc;comment:租户ID"`                // 租户ID
	KBID         string    `gorm:"type:varchar(64);not null;default:'kb_default_system';index:idx_kb_doc;comment:关联知识库UUID"` // 关联知识库UUID
	CollectionID int64     `gorm:"not null;index;comment:所属知识库Collection ID"`                                 // 所属知识库Collection ID
	DocID        string    `gorm:"type:varchar(64);not null;uniqueIndex;comment:业务文档UUID"`                     // 业务文档UUID
	Title        string    `gorm:"type:varchar(255);not null;comment:文档标题"`                                    // 文档标题
	SourceType   string    `gorm:"type:varchar(32);not null;comment:文档类型(pdf,docx,md,web)"`                   // 文档类型
	SourceURL    string    `gorm:"type:varchar(1024);comment:原始文件存储地址(OSS)"`                                  // 原始文件存储地址
	FilePath     string    `gorm:"type:varchar(512);index;comment:文件磁盘绝对路径"`                                  // 文件磁盘绝对路径
	FileHash     string    `gorm:"type:varchar(64);index;comment:文件内容SHA256哈希"`                                // 文件内容SHA256哈希
	Status       int32     `gorm:"type:tinyint;default:0;comment:处理状态(0:待处理,1:解析中,2:已向量化,3:失败)"`              // 处理状态
	TotalChunks  int32     `gorm:"type:int;default:0;comment:总切片数"`                                            // 总切片数
	ErrMsg       string    `gorm:"type:text;comment:失败异常信息"`                                                   // 失败异常信息
	CreatedAt    time.Time `gorm:"autoCreateTime;comment:创建时间"`                                                // 创建时间
	UpdatedAt    time.Time `gorm:"autoUpdateTime;comment:更新时间"`                                                // 更新时间
}

func (m *KnowledgeDocumentModel) TableName() string {
	return "knowledge_documents"
}
