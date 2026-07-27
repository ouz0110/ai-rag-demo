package store

import (
	"fmt"
	"strings"

	"ai-rag-demo/internal/conf"
)

// NewVectorStore 根据 Config 中的 Driver 配置匹配创建对应的向量存储适配器
func NewVectorStore(cfg *conf.Config) (VectorStore, error) {
	if cfg == nil || cfg.Source.VectorDB == nil {
		return nil, fmt.Errorf("vector_db configuration is missing")
	}

	driver := strings.ToLower(cfg.Source.VectorDB.Driver)
	if driver == "" {
		driver = "milvus"
	}

	switch driver {
	case "milvus":
		return newMilvusAdapter(cfg.Source.VectorDB.Milvus)
	default:
		return nil, fmt.Errorf("unsupported vector db driver: %s", driver)
	}
}
