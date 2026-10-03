package master

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

type ListFilter struct {
	Offset int
	Limit  int
	Q      string
}

type MasterRepository interface {
	List(context.Context, Catalog, ListFilter) ([]CatalogItem, int64, error)
}

type gormMasterRepository struct {
	db *gorm.DB
}

func NewMasterRepository(db *gorm.DB) MasterRepository {
	return &gormMasterRepository{db: db}
}

func (r *gormMasterRepository) List(ctx context.Context, catalog Catalog, filter ListFilter) ([]CatalogItem, int64, error) {
	table, ok := catalogTable(catalog)
	if !ok {
		return nil, 0, fmt.Errorf("unknown master catalog")
	}
	items := make([]CatalogItem, 0, filter.Limit)
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table(table)
		if filter.Q != "" {
			query = query.Where("strpos(lower(name), lower(?)) > 0 OR strpos(lower(slug), lower(?)) > 0", filter.Q, filter.Q)
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		return query.Order("lower(name)").Order("id").Offset(filter.Offset).Limit(filter.Limit).Find(&items).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
