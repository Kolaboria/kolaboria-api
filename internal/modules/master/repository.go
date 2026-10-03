package master

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"gorm.io/gorm"
)

type ListFilter struct {
	Offset int
	Limit  int
	Q      string
}

type MasterRepository interface {
	List(context.Context, Catalog, ListFilter) ([]CatalogItem, int64, error)
	SearchLocations(context.Context, string) ([]SearchLocationResponse, error)
	GetLocationDetail(context.Context, int64) (*LocationDetailResponse, error)
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

func (r *gormMasterRepository) SearchLocations(ctx context.Context, query string) ([]SearchLocationResponse, error) {
	results := make([]SearchLocationResponse, 0)
	searchTerm := "%" + query + "%"
	err := r.db.WithContext(ctx).Raw(`
		SELECT 
			v.id AS village_id, 
			v.name AS village, 
			d.name AS district, 
			r.name AS regency, 
			p.name AS province, 
			v.postal_code AS postal_code
		FROM villages v
		JOIN districts d ON v.district_id = d.id
		JOIN regencies r ON d.regency_id = r.id
		JOIN provinces p ON r.province_id = p.id
		WHERE v.name ILIKE ? OR d.name ILIKE ? OR r.name ILIKE ? OR p.name ILIKE ?
		ORDER BY 
			(v.name ILIKE ?) DESC, 
			v.name ASC
		LIMIT 20
	`, searchTerm, searchTerm, searchTerm, searchTerm, query).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *gormMasterRepository) GetLocationDetail(ctx context.Context, villageID int64) (*LocationDetailResponse, error) {
	type locationDetailRow struct {
		ProvinceID   int64  `gorm:"column:province_id"`
		ProvinceName string `gorm:"column:province_name"`
		RegencyID    int64  `gorm:"column:regency_id"`
		RegencyName  string `gorm:"column:regency_name"`
		DistrictID   int64  `gorm:"column:district_id"`
		DistrictName string `gorm:"column:district_name"`
		VillageID    int64  `gorm:"column:village_id"`
		VillageName  string `gorm:"column:village_name"`
		PostalCode   string `gorm:"column:postal_code"`
	}

	var row locationDetailRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT 
			p.id AS province_id, 
			p.name AS province_name, 
			r.id AS regency_id, 
			r.name AS regency_name, 
			d.id AS district_id, 
			d.name AS district_name, 
			v.id AS village_id, 
			v.name AS village_name, 
			v.postal_code AS postal_code
		FROM villages v
		JOIN districts d ON v.district_id = d.id
		JOIN regencies r ON d.regency_id = r.id
		JOIN provinces p ON r.province_id = p.id
		WHERE v.id = ?
		LIMIT 1
	`, villageID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrNotFound
		}
		return nil, err
	}

	return &LocationDetailResponse{
		Province:   LocationInfo{ID: row.ProvinceID, Name: row.ProvinceName},
		Regency:    LocationInfo{ID: row.RegencyID, Name: row.RegencyName},
		District:   LocationInfo{ID: row.DistrictID, Name: row.DistrictName},
		Village:    LocationInfo{ID: row.VillageID, Name: row.VillageName},
		PostalCode: row.PostalCode,
	}, nil
}

