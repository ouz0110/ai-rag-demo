package database

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type contextKey struct{}

type DB struct {
	cfg    *Config
	gormDB *gorm.DB
}

type Config struct {
	Source       string
	Ca           string
	MaxIdleConns int
	MaxOpenConns int
	MaxLifetime  time.Duration
	Logger       logger.Interface
}

func New(c *Config, defaultCa string) (*DB, error) {
	ca := c.Ca
	if ca == "" {
		ca = defaultCa
	}
	dsnStr := c.Source
	if ca != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(ca)) {
			return nil, errors.New("failed to parse CA certificate")
		}
		dsn, err := mysql.ParseDSN(dsnStr)
		if err != nil {
			return nil, errors.Wrap(err, "mysql ParseDSN error")
		}
		host, _, err := net.SplitHostPort(dsn.Addr)
		if err != nil {
			return nil, errors.Wrap(err, "mysql SplitHostPort error")
		}
		tlsConfig := &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: true, // 强烈建议 false，验证服务器身份
			ServerName:         host, // ip连接不需要配置，域名连接需要配置
		}

		// 注册 TLS 配置
		err = mysql.RegisterTLSConfig("custom", tlsConfig)
		if err != nil {
			return nil, errors.Wrap(err, "mysql RegisterTLSConfig")
		}
		dsnStr = dsnStr + "&tls=custom"
	}

	cf := &gorm.Config{
		PrepareStmt: true,
		Logger:      c.Logger,
	}
	gormDb, err := gorm.Open(gormmysql.Open(dsnStr), cf)
	if err != nil {
		return nil, err
	}

	// 配置
	sqlDB, err := gormDb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(c.MaxLifetime)

	return &DB{cfg: c, gormDB: gormDb}, nil
}

func (d *DB) AutoMigrate(dst ...interface{}) error {
	return d.gormDB.AutoMigrate(dst...)
}

type transactionTxs map[string]*gorm.DB

// InTransaction 在事务中执行，示例：
//
//	InTransaction(ctx, func(ctx context.Context) error {
//		err := UpdateData(ctx,data)
//		if err != nil {
//			return err // 注意错误要直接返回，不能赋值给闭包外的变量，事务通过判定该函数返回决定是否需要回滚。
//		}
//
//		return nil
//	})
func (d *DB) InTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if txs, ok := ctx.Value(contextKey{}).(transactionTxs); ok {
			txs[d.cfg.Source] = tx
		} else {
			ctx = context.WithValue(ctx, contextKey{}, transactionTxs{d.cfg.Source: tx})
		}
		return fn(ctx)
	})
}

func (d *DB) GormDB(ctx context.Context) *gorm.DB {
	return d.fromContext(ctx)
}

func (d *DB) fromContext(ctx context.Context) *gorm.DB {
	txs, ok := ctx.Value(contextKey{}).(transactionTxs)
	if !ok {
		return d.gormDB
	}
	tx, ok := txs[d.cfg.Source]
	if !ok {
		return d.gormDB
	}

	return tx
}

// DTOer 将本地数据转换为可在外部使用的数据。
type DTOer[D any] interface {
	DTO() (D, error)
}

// OTDer 将外部数据转换本地数据。
type OTDer[D any] interface {
	DTO(D) error
}

func IsRecordNotFoundError(err error) bool {
	return err != nil && errors.Is(err, gorm.ErrRecordNotFound)
}

func sqlCommonCount(sql string) string {
	return fmt.Sprintf(`SELECT COUNT(*) FROM (%s) t`, sql)
}
