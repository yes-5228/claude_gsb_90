package database

import (
	"errors"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/modules/hierarchy"
)

// legacySegment 旧版本管段上以字符串存放的片区 / 道路。
type legacySegment struct {
	ID       uint
	District string
	RoadName string
}

// BackfillHierarchy 把旧版本字符串形式的片区 / 道路回填为层级表与外键，
// 并为历史任务、清淤记录补齐登记当时的层级快照。
//
// 幂等：只处理 road_id / 快照为空的行；层级表已存在同名片区、道路时直接复用，
// 因此重复执行（或全新数据库）都不会产生重复数据。
func BackfillHierarchy(db *gorm.DB) error {
	// 全新数据库没有遗留的 district 字符串列，直接跳过。
	if !columnExists(db, "pipe_segments", "district") {
		return nil
	}

	var pending []legacySegment
	if err := db.Table("pipe_segments").
		Select("id, district, road_name").
		Where("road_id IS NULL OR road_id = 0").
		Scan(&pending).Error; err != nil {
		// 列已不存在（例如后续彻底下线遗留列），视为无需回填。
		return nil
	}
	if len(pending) == 0 {
		return backfillSnapshots(db)
	}

	// (片区名, 道路名) -> 层级 ID 的映射。
	districtIDs := make(map[string]uint)
	roadIDs := make(map[roadKey]uint)

	ensureDistrict := func(name string) (uint, error) {
		if id, ok := districtIDs[name]; ok {
			return id, nil
		}
		var d hierarchy.District
		err := db.Where("name = ?", name).First(&d).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			d = hierarchy.District{Name: name}
			if err := db.Create(&d).Error; err != nil {
				return 0, err
			}
		} else if err != nil {
			return 0, err
		}
		districtIDs[name] = d.ID
		return d.ID, nil
	}
	ensureRoad := func(districtID uint, districtName, roadName string) (uint, error) {
		key := roadKey{district: districtName, road: roadName}
		if id, ok := roadIDs[key]; ok {
			return id, nil
		}
		var road hierarchy.Road
		err := db.Where("district_id = ? AND name = ?", districtID, roadName).First(&road).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			road = hierarchy.Road{DistrictID: districtID, Name: roadName}
			if err := db.Create(&road).Error; err != nil {
				return 0, err
			}
		} else if err != nil {
			return 0, err
		}
		roadIDs[key] = road.ID
		return road.ID, nil
	}

	for _, item := range pending {
		districtName := item.District
		if districtName == "" {
			districtName = "未分片区"
		}
		roadName := item.RoadName
		if roadName == "" {
			roadName = "未命名道路"
		}
		districtID, err := ensureDistrict(districtName)
		if err != nil {
			return err
		}
		roadID, err := ensureRoad(districtID, districtName, roadName)
		if err != nil {
			return err
		}
		if err := db.Table("pipe_segments").
			Where("id = ?", item.ID).
			Update("road_id", roadID).Error; err != nil {
			return err
		}
	}

	return backfillSnapshots(db)
}

type roadKey struct {
	district string
	road     string
}

// snapshotRow 用于把当前层级回填到历史任务 / 记录的快照列。
type snapshotRow struct {
	ID           uint
	DistrictID   uint
	RoadID       uint
	DistrictName string
	RoadName     string
}

// backfillSnapshots 为升级前创建、尚无层级快照的任务 / 记录补齐快照。
//
// 升级瞬间层级未做过任何调整，当前层级即登记当时层级，因此直接按当前管段归属补齐。
func backfillSnapshots(db *gorm.DB) error {
	const join = "FROM pipe_segments AS s " +
		"JOIN roads AS r ON r.id = s.road_id " +
		"JOIN districts AS d ON d.id = r.district_id "

	var tasks []snapshotRow
	if err := db.Table("cleaning_tasks AS t").
		Select("t.id AS id, r.district_id AS district_id, s.road_id AS road_id, d.name AS district_name, r.name AS road_name").
		Joins("JOIN pipe_segments AS s ON s.id = t.pipe_segment_id").
		Joins("JOIN roads AS r ON r.id = s.road_id").
		Joins("JOIN districts AS d ON d.id = r.district_id").
		Where("t.district_id IS NULL OR t.district_id = 0").
		Scan(&tasks).Error; err != nil {
		return err
	}
	for _, row := range tasks {
		if err := db.Table("cleaning_tasks").Where("id = ?", row.ID).
			Updates(map[string]any{
				"district_id":   row.DistrictID,
				"road_id":       row.RoadID,
				"district_name": row.DistrictName,
				"road_name":     row.RoadName,
			}).Error; err != nil {
			return err
		}
	}

	var records []snapshotRow
	if err := db.Table("cleaning_records AS cr").
		Select("cr.id AS id, r.district_id AS district_id, s.road_id AS road_id, d.name AS district_name, r.name AS road_name").
		Joins("JOIN cleaning_tasks AS t ON t.id = cr.task_id").
		Joins("JOIN pipe_segments AS s ON s.id = t.pipe_segment_id").
		Joins("JOIN roads AS r ON r.id = s.road_id").
		Joins("JOIN districts AS d ON d.id = r.district_id").
		Where("cr.district_id IS NULL OR cr.district_id = 0").
		Scan(&records).Error; err != nil {
		return err
	}
	for _, row := range records {
		if err := db.Table("cleaning_records").Where("id = ?", row.ID).
			Updates(map[string]any{
				"district_id":   row.DistrictID,
				"road_id":       row.RoadID,
				"district_name": row.DistrictName,
				"road_name":     row.RoadName,
			}).Error; err != nil {
			return err
		}
	}

	_ = join
	return nil
}

// columnExists 判断某张表是否存在指定列，兼容 PostgreSQL 与 SQLite。
func columnExists(db *gorm.DB, table, column string) bool {
	var name string
	var err error
	switch db.Dialector.Name() {
	case "postgres":
		err = db.Raw(
			"SELECT column_name FROM information_schema.columns WHERE table_name = ? AND column_name = ? LIMIT 1",
			table, column,
		).Scan(&name).Error
	default:
		err = db.Raw("SELECT name FROM pragma_table_info(?) WHERE name = ? LIMIT 1", table, column).Scan(&name).Error
	}
	return err == nil && name != ""
}
