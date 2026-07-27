package data

import (
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data/base"
	"ai-rag-demo/internal/data/rag"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAllDB,
	NewRAGRepoFromAllDB,
)

type DB struct {
	cfg  *conf.Config
	Base *base.DB
}

func NewAllDB(
	c *conf.Config,
) *DB {
	return &DB{
		cfg:  c,
		Base: base.NewDB(c),
	}
}

func NewRAGRepoFromAllDB(allDb *DB) *rag.RAGRepo {
	if allDb == nil || allDb.Base == nil {
		return nil
	}
	return rag.NewRAGRepo(allDb.Base.DB)
}
