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
	NewRAGDBFromAllDB,
)

type DB struct {
	cfg  *conf.Config
	Base *base.DB
	Rag  *rag.DB
}

func NewAllDB(
	c *conf.Config,
) *DB {
	return &DB{
		cfg:  c,
		Base: base.NewDB(c),
		Rag:  rag.NewDB(c),
	}
}

func NewRAGDBFromAllDB(allDb *DB) *rag.DB {
	if allDb == nil {
		return nil
	}
	return allDb.Rag
}
