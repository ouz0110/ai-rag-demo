package database

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type TableRepo[M schema.Tabler] struct {
	db *DB
}

func NewTableRepo[M schema.Tabler](db *DB) TableRepo[M] {
	return TableRepo[M]{db: db}
}

func (r *TableRepo[M]) Unscoped() (tx *TableRepo[M]) {
	return &TableRepo[M]{db: &DB{gormDB: r.db.gormDB.Unscoped()}}
}

func (r *TableRepo[M]) GormDB(ctx context.Context) *gorm.DB {
	return r.db.fromContext(ctx)
}

func (r *TableRepo[M]) DB() *DB {
	return r.db
}

func (r *TableRepo[M]) Save(ctx context.Context, m M) (M, error) {
	err := r.db.fromContext(ctx).Save(m).Error
	if err != nil {
		return m, err
	}

	return m, nil
}

func (r *TableRepo[M]) Update(ctx context.Context, m M) (M, error) {
	err := r.db.fromContext(ctx).Updates(m).Error
	if err != nil {
		return m, err
	}

	return m, nil
}

func (r *TableRepo[M]) SaveMulti(ctx context.Context, ms ...M) error {
	if len(ms) == 0 {
		return nil
	}
	err := r.db.fromContext(ctx).Save(ms).Error
	if err != nil {
		return err
	}

	return nil
}

func (r *TableRepo[M]) Create(ctx context.Context, ms ...M) error {
	if len(ms) == 0 {
		return nil
	}
	if err := r.db.fromContext(ctx).Create(ms).Error; err != nil {
		return errors.Wrap(err, "create record error")
	}
	return nil
}

func (r *TableRepo[M]) Delete(ctx context.Context, m M) error {
	err := r.db.fromContext(ctx).Delete(m).Error
	if err != nil {
		return err
	}
	log.Printf("TableRepo.Delete record from %s: %+v", m.TableName(), m)

	return nil
}

func (r *TableRepo[M]) Page(_ context.Context, query *gorm.DB, page, size int) {
	if page > 0 && size > 0 {
		offset := (page - 1) * size
		query = query.Offset(offset).Limit(size)
	}
	if size > 0 {
		query = query.Limit(size)
	}
}

func (r *TableRepo[M]) Sort(_ context.Context, query *gorm.DB, sort, defaultSort []string) {
	if len(sort) == 0 {
		sort = defaultSort
	}
	if len(sort) > 0 {
		for _, s := range sort {
			query = query.Order(s)
		}
	}
}
