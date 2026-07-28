package conf

import (
	"fmt"

	"ai-rag-demo/internal/pkg/database"
	"ai-rag-demo/internal/pkg/log"

	gormlogger "gorm.io/gorm/logger"
)

func NewDB(name string, c *Config, autoMigrateTables ...interface{}) (*database.DB, error) {
	dbCfg, ok := c.Source.Database[name]
	if !ok {
		return nil, fmt.Errorf("database '%s' config missing", name)
	}

	var logger gormlogger.Interface = log.NewGormLogger(fmt.Sprintf("gorm-%s", name))
	// if IsTestEnv() {
	// 	logger = gormlogger.Default.LogMode(gormlogger.Info)
	// }
	databaseCli, err := database.New(&database.Config{
		Source:       dbCfg.Source,
		Ca:           dbCfg.Ca,
		MaxIdleConns: dbCfg.MaxIdleConns,
		MaxOpenConns: dbCfg.MaxOpenConns,
		MaxLifetime:  dbCfg.MaxLifetime.Duration,
		Logger:       logger,
	}, c.Source.MysqlDefaultCa)
	if err != nil {
		return nil, err
	}

	// 只在测试环境自动迁移
	// if len(autoMigrateTables) > 0 && dbCfg.AutoMigrate == "on" && IsTestEnv() {
	// 	if err = databaseCli.AutoMigrate(autoMigrateTables...); err != nil {
	// 		return nil, fmt.Errorf("auto migrate tables failed: %w", err)
	// 	}
	// }
	fmt.Printf("database '%s' init success\n", name)

	return databaseCli, nil
}
