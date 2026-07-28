package base

import (
	"fmt"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/database"
)

type DB struct {
	*database.DB
	AccountRepo        AccountRepo
	NocliSessionRepo   NocliSessionRepo
	NocliMessageRepo   NocliMessageRepo
	NocliInterruptRepo NocliInterruptRepo
	BillingRepo        BillingRepo
}

const dbName = "base"

func NewDB(c *conf.Config) *DB {
	db, err := conf.NewDB(dbName, c)
	if err != nil {
		fmt.Println("db " + dbName + " init failed")
		return nil
	}

	return &DB{
		DB:                 db,
		AccountRepo:        AccountRepo{TableRepo: database.NewTableRepo[*AccountsModel](db)},
		NocliSessionRepo:   NocliSessionRepo{TableRepo: database.NewTableRepo[*NocliSessionModel](db)},
		NocliMessageRepo:   NocliMessageRepo{TableRepo: database.NewTableRepo[*NocliMessageModel](db)},
		NocliInterruptRepo: NocliInterruptRepo{TableRepo: database.NewTableRepo[*NocliInterruptModel](db)},
		BillingRepo:        BillingRepo{TableRepo: database.NewTableRepo[*BillingUsageLogModel](db)},
	}
}
