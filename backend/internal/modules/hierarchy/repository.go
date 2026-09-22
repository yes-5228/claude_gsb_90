package hierarchy

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound 层级节点不存在。
var ErrNotFound = errors.New("层级节点不存在")

// Repository 层级数据访问。
type Repository struct {
	db *gorm.DB
}

// NewRepository 构造仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// DB 暴露底层连接，供 service 做跨表引用检查。
func (r *Repository) DB() *gorm.DB {
	return r.db
}

// Transaction 在单个事务内执行批量调整。
func (r *Repository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

// CreateDistrict 新增片区。
func (r *Repository) CreateDistrict(ctx context.Context, district *District) error {
	return r.db.WithContext(ctx).Create(district).Error
}

// CreateDistrictTx 在给定事务内新增片区。
func (r *Repository) CreateDistrictTx(ctx context.Context, tx *gorm.DB, district *District) error {
	return tx.WithContext(ctx).Create(district).Error
}

// CreateRoad 新增道路。
func (r *Repository) CreateRoad(ctx context.Context, road *Road) error {
	return r.db.WithContext(ctx).Create(road).Error
}

// CreateRoadTx 在给定事务内新增道路。
func (r *Repository) CreateRoadTx(ctx context.Context, tx *gorm.DB, road *Road) error {
	return tx.WithContext(ctx).Create(road).Error
}

// FindDistrict 按主键查询片区。
func (r *Repository) FindDistrict(ctx context.Context, id uint) (*District, error) {
	var district District
	err := r.db.WithContext(ctx).First(&district, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &district, nil
}

// FindDistrictForUpdate 在事务内锁定片区行（PostgreSQL 行锁；SQLite 自动忽略）。
func (r *Repository) FindDistrictForUpdate(ctx context.Context, tx *gorm.DB, id uint, lock bool) (*District, error) {
	var district District
	query := tx.WithContext(ctx)
	if lock {
		query = withForUpdate(query)
	}
	err := query.First(&district, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &district, nil
}

// FindRoad 按主键查询道路。
func (r *Repository) FindRoad(ctx context.Context, id uint) (*Road, error) {
	var road Road
	err := r.db.WithContext(ctx).First(&road, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &road, nil
}

// FindRoadForUpdate 在事务内锁定道路行（PostgreSQL 行锁；SQLite 自动忽略）。
func (r *Repository) FindRoadForUpdate(ctx context.Context, tx *gorm.DB, id uint, lock bool) (*Road, error) {
	var road Road
	query := tx.WithContext(ctx)
	if lock {
		query = withForUpdate(query)
	}
	err := query.First(&road, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &road, nil
}

// SaveDistrictTx 事务内保存片区。
func (r *Repository) SaveDistrictTx(ctx context.Context, tx *gorm.DB, district *District) error {
	return tx.WithContext(ctx).Save(district).Error
}

// SaveRoadTx 事务内保存道路。
func (r *Repository) SaveRoadTx(ctx context.Context, tx *gorm.DB, road *Road) error {
	return tx.WithContext(ctx).Save(road).Error
}

// DeleteDistrictTx 事务内删除片区。
func (r *Repository) DeleteDistrictTx(ctx context.Context, tx *gorm.DB, id uint) error {
	result := tx.WithContext(ctx).Delete(&District{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRoadTx 事务内删除道路。
func (r *Repository) DeleteRoadTx(ctx context.Context, tx *gorm.DB, id uint) error {
	result := tx.WithContext(ctx).Delete(&Road{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// RoadsByDistrict 查询片区下道路。
func (r *Repository) RoadsByDistrict(ctx context.Context, districtID uint) ([]Road, error) {
	roads := make([]Road, 0)
	err := r.db.WithContext(ctx).
		Where("district_id = ?", districtID).
		Order("sort_order ASC, id ASC").
		Find(&roads).Error
	return roads, err
}

// AllDistricts 全部片区（按排序号、ID）。
func (r *Repository) AllDistricts(ctx context.Context) ([]District, error) {
	districts := make([]District, 0)
	err := r.db.WithContext(ctx).
		Order("sort_order ASC, id ASC").
		Find(&districts).Error
	return districts, err
}

// AllRoads 全部道路。
func (r *Repository) AllRoads(ctx context.Context) ([]Road, error) {
	roads := make([]Road, 0)
	err := r.db.WithContext(ctx).
		Order("district_id ASC, sort_order ASC, id ASC").
		Find(&roads).Error
	return roads, err
}

// DistrictExistsByName 片区名是否已占用（excludeID 用于改名时排除自身）。
func (r *Repository) DistrictExistsByName(ctx context.Context, tx *gorm.DB, name string, excludeID uint) (bool, error) {
	query := tx.WithContext(ctx).Model(&District{}).Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// RoadExistsByName 同一片区下道路名是否已占用（excludeID 用于改名时排除自身）。
func (r *Repository) RoadExistsByName(ctx context.Context, tx *gorm.DB, districtID uint, name string, excludeID uint) (bool, error) {
	query := tx.WithContext(ctx).Model(&Road{}).
		Where("district_id = ? AND name = ?", districtID, name)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// WriteLogsTx 事务内批量写入变更日志。
func (r *Repository) WriteLogsTx(ctx context.Context, tx *gorm.DB, logs []ChangeLog) error {
	if len(logs) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&logs).Error
}

// Logs 分页查询变更日志。
func (r *Repository) Logs(ctx context.Context, query LogListQuery) ([]ChangeLog, int64, error) {
	tx := r.db.WithContext(ctx).Model(&ChangeLog{})
	if query.NodeType != "" {
		tx = tx.Where("node_type = ?", query.NodeType)
	}
	if query.Action != "" {
		tx = tx.Where("action = ?", query.Action)
	}
	if query.BatchID != "" {
		tx = tx.Where("batch_id = ?", query.BatchID)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]ChangeLog, 0)
	err := tx.Order("id DESC").
		Offset((query.Page.Page - 1) * query.Page.PageSize).
		Limit(query.Page.PageSize).
		Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
