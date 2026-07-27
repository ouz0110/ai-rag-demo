package rag

import (
	"fmt"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/database"
)

type DB struct {
	*database.DB
	KBRepo    KnowledgeBaseRepo
	DocRepo   DocumentRepo
	ChunkRepo ChunkRepo
}

const dbName = "base"

func NewDB(c *conf.Config) *DB {
	db, err := conf.NewDB(dbName, c)
	if err != nil {
		fmt.Println("db " + dbName + " init failed")
		return nil
	}

	return &DB{
		DB:        db,
		KBRepo:    KnowledgeBaseRepo{TableRepo: database.NewTableRepo[*KnowledgeBaseModel](db)},
		DocRepo:   DocumentRepo{TableRepo: database.NewTableRepo[*KnowledgeDocumentModel](db)},
		ChunkRepo: ChunkRepo{TableRepo: database.NewTableRepo[*KnowledgeChunkModel](db)},
	}
}
