package hierarchy

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound 层级节点不存在。
var ErrNotFound = errors.New("层级节点不存在")

// 各业务表名（与对应模块 TableName 保持一致）。
const (
	tablePipeSegments    = "pipe_segments"
	tableCleaningTasks   = "cleaning_tasks"
	tableCleaningRecords = "cleaning_records"
)

// Repository 层级数据访问。
type Repository struct {
	db        *gorm.DB
	dialector string
}

// NewRepository 构造仓储。
func NewRepository(db *gorm.DB) *Repository {
	dialector := ""
	if db.Dialector != nil {
		dialector = db.Dialector.Name()
	}
	return &Repository{db: db, dialector: dialector}
}

// forUpdateClause 返回 SELECT ... FOR UPDATE 行级锁子句。
func forUpdateClause() clause.Expression {
	return clause.Locking{Strength: "UPDATE"}
}

// DB 暴露底层连接。
func (r *Repository) DB() *gorm.DB {
	return r.db
}

// ---------- 片区 ----------

// CreateDistrict 新增片区。
func (r *Repository) CreateDistrict(ctx context.Context, tx *gorm.DB, d *District) error {
	return r.conn(tx).WithContext(ctx).Create(d).Error
}

// SaveDistrict 保存片区全部字段。
func (r *Repository) SaveDistrict(ctx context.Context, tx *gorm.DB, d *District) error {
	return r.conn(tx).WithContext(ctx).Save(d).Error
}

// FindDistrict 按主键查询片区（tx 为 nil 时使用独立连接）。
func (r *Repository) FindDistrict(ctx context.Context, tx *gorm.DB, id uint) (*District, error) {
	var d District
	err := r.conn(tx).WithContext(ctx).First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// FindDistrictByName 按名称查询片区（用于唯一校验）。
func (r *Repository) FindDistrictByName(ctx context.Context, tx *gorm.DB, name string, excludeID uint) (*District, bool, error) {
	var d District
	q := r.conn(tx).WithContext(ctx).Model(&District{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &d, true, nil
}

// ListDistricts 查询全部片区（按排序、名称）。
func (r *Repository) ListDistricts(ctx context.Context) ([]District, error) {
	out := make([]District, 0)
	err := r.db.WithContext(ctx).
		Order("sort_order ASC, id ASC").
		Find(&out).Error
	return out, err
}

// DistrictsByIDs 批量查询片区。
func (r *Repository) DistrictsByIDs(ctx context.Context, tx *gorm.DB, ids []uint) (map[uint]District, error) {
	result := make(map[uint]District, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var list []District
	if err := r.conn(tx).WithContext(ctx).Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, err
	}
	for _, d := range list {
		result[d.ID] = d
	}
	return result, nil
}

// DeleteDistrict 删除片区。
func (r *Repository) DeleteDistrict(ctx context.Context, tx *gorm.DB, id uint) error {
	res := r.conn(tx).WithContext(ctx).Delete(&District{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CountRoadsIn 统计片区下的道路数量。
func (r *Repository) CountRoadsIn(ctx context.Context, tx *gorm.DB, districtID uint) (int64, error) {
	var count int64
	err := r.conn(tx).WithContext(ctx).Model(&Road{}).
		Where("district_id = ?", districtID).
		Count(&count).Error
	return count, err
}

// ---------- 道路 ----------

// CreateRoad 新增道路。
func (r *Repository) CreateRoad(ctx context.Context, tx *gorm.DB, road *Road) error {
	return r.conn(tx).WithContext(ctx).Create(road).Error
}

// SaveRoad 保存道路全部字段。
func (r *Repository) SaveRoad(ctx context.Context, tx *gorm.DB, road *Road) error {
	return r.conn(tx).WithContext(ctx).Save(road).Error
}

// FindRoad 按主键查询道路（tx 为 nil 时使用独立连接）。
func (r *Repository) FindRoad(ctx context.Context, tx *gorm.DB, id uint) (*Road, error) {
	var road Road
	err := r.conn(tx).WithContext(ctx).First(&road, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &road, nil
}

// FindRoadForUpdate 在事务内加行级锁查询道路（仅 PostgreSQL 生效），
// 用于归属调整与业务录入并发时锁定读取口径。
func (r *Repository) FindRoadForUpdate(ctx context.Context, tx *gorm.DB, id uint) (*Road, error) {
	var road Road
	q := r.conn(tx).WithContext(ctx)
	if r.dialector == "postgres" {
		q = q.Clauses(forUpdateClause())
	}
	err := q.First(&road, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &road, nil
}

// RoadsByIDs 批量查询道路。
func (r *Repository) RoadsByIDs(ctx context.Context, ids []uint) (map[uint]Road, error) {
	result := make(map[uint]Road, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var list []Road
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, err
	}
	for _, road := range list {
		result[road.ID] = road
	}
	return result, nil
}

// DeleteRoad 删除道路。
func (r *Repository) DeleteRoad(ctx context.Context, tx *gorm.DB, id uint) error {
	res := r.conn(tx).WithContext(ctx).Delete(&Road{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// RoadExistsWithinDistrict 判断同片区下道路名称是否已占用。
func (r *Repository) RoadExistsWithinDistrict(ctx context.Context, tx *gorm.DB, districtID uint, name string, excludeRoadID uint) (bool, error) {
	q := r.conn(tx).WithContext(ctx).Model(&Road{}).
		Where("district_id = ? AND name = ?", districtID, name)
	if excludeRoadID > 0 {
		q = q.Where("id <> ?", excludeRoadID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListRoads 查询全部道路（按片区、排序、名称）。
func (r *Repository) ListRoads(ctx context.Context) ([]Road, error) {
	out := make([]Road, 0)
	err := r.db.WithContext(ctx).
		Order("district_id ASC, sort_order ASC, id ASC").
		Find(&out).Error
	return out, err
}

// SegmentCountsByRoad 统计每条道路下的当前管段数量。
func (r *Repository) SegmentCountsByRoad(ctx context.Context) (map[uint]int64, error) {
	type row struct {
		RoadID uint
		Total  int64
	}
	rows := make([]row, 0)
	err := r.db.WithContext(ctx).Table(tablePipeSegments).
		Select("road_id AS road_id, COUNT(*) AS total").
		Where("road_id IS NOT NULL AND road_id > 0").
		Group("road_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]int64, len(rows))
	for _, item := range rows {
		result[item.RoadID] = item.Total
	}
	return result, nil
}

// CountSegmentsOnRoad 统计某条道路下的当前管段数量（tx 为 nil 时使用独立连接）。
func (r *Repository) CountSegmentsOnRoad(ctx context.Context, tx *gorm.DB, roadID uint) (int64, error) {
	var count int64
	err := r.conn(tx).WithContext(ctx).Table(tablePipeSegments).
		Where("road_id = ?", roadID).
		Count(&count).Error
	return count, err
}

// ---------- 变更记录 ----------

// AddLogs 写入一批变更记录（同一事务内）。
func (r *Repository) AddLogs(ctx context.Context, tx *gorm.DB, logs []*HierarchyChangeLog) error {
	if len(logs) == 0 {
		return nil
	}
	now := time.Now()
	for _, log := range logs {
		if log.CreatedAt.IsZero() {
			log.CreatedAt = now
		}
	}
	return r.conn(tx).WithContext(ctx).Create(&logs).Error
}

// ListLogs 分页查询变更记录（按时间倒序）。
func (r *Repository) ListLogs(ctx context.Context, query ChangeLogQuery) ([]HierarchyChangeLog, int64, error) {
	query.Page.Normalize()
	base := r.db.WithContext(ctx).Model(&HierarchyChangeLog{})
	if query.NodeType != "" {
		base = base.Where("node_type = ?", query.NodeType)
	}
	if query.NodeID > 0 {
		base = base.Where("node_id = ?", query.NodeID)
	}
	if query.Action != "" {
		base = base.Where("action = ?", query.Action)
	}
	if query.BatchID != "" {
		base = base.Where("batch_id = ?", query.BatchID)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	out := make([]HierarchyChangeLog, 0)
	err := base.Order("created_at DESC, id DESC").
		Offset(query.Page.Offset()).
		Limit(query.Page.PageSize).
		Find(&out).Error
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Transaction 在事务中执行层级操作，保证批量调整原子生效。
func (r *Repository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func (r *Repository) conn(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}
