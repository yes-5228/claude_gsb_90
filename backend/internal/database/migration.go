package database

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/modules/acceptance"
	"github.com/drainage/desilting/internal/modules/cleaningrecord"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/modules/pipesegment"
)

// BackfillHierarchy 把升级前以自由文本存放在管段上的片区 / 道路迁移到统一层级表，
// 并回填任务、清淤记录、验收记录登记当时的层级快照。
//
// 迁移是幂等的：
//   - 已经有层级引用（district_id 非空）的管段不会再处理；
//   - 同名片区 / 同片区下同名道路只创建一次并复用；
//   - 快照列非空的业务行保持不变。
//
// 历史业务数据按"迁移当时"管段上的名称固化快照，保证升级后历史数据仍展示
// 升级前登记的片区 / 道路名称，不会因后续改名而变化。
func BackfillHierarchy(db *gorm.DB) error {
	// 全新数据库没有旧的文本列，直接跳过。
	if !columnExists(db, &pipesegment.PipeSegment{}, "district") {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		type legacySegment struct {
			ID       uint
			District string
			RoadName string
		}
		rows := make([]legacySegment, 0)
		if err := tx.Table("pipe_segments").
			Select("id, district, road_name").
			Where("district_id = 0 OR district_id IS NULL").
			Scan(&rows).Error; err != nil {
			return fmt.Errorf("读取旧管段层级失败: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}

		districtIDs := make(map[string]uint)
		roadIDs := make(map[string]uint) // key: districtID \x00 roadName

		for _, row := range rows {
			districtName := firstNonEmpty(row.District, "未分组片区")
			districtID, err := ensureDistrict(tx, districtName, districtIDs)
			if err != nil {
				return err
			}
			var roadID *uint
			if roadName := row.RoadName; roadName != "" {
				key := hierarchyKey(districtID, roadName)
				id, err := ensureRoad(tx, districtID, roadName, roadIDs, key)
				if err != nil {
					return err
				}
				roadID = &id
			}
			updates := map[string]any{"district_id": districtID, "road_id": roadID}
			if err := tx.Table("pipe_segments").Where("id = ?", row.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("回填管段层级引用失败: %w", err)
			}
		}

		// 任务 / 记录 / 验收按其关联管段的层级回填快照。
		if err := backfillTaskSnapshots(tx); err != nil {
			return err
		}
		if err := backfillRecordSnapshots(tx); err != nil {
			return err
		}
		if err := backfillAcceptanceSnapshots(tx); err != nil {
			return err
		}
		return nil
	})
}

func ensureDistrict(tx *gorm.DB, name string, cache map[string]uint) (uint, error) {
	if id, ok := cache[name]; ok {
		return id, nil
	}
	var district hierarchy.District
	err := tx.Where("name = ?", name).First(&district).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		district = hierarchy.District{Name: name}
		if err := tx.Create(&district).Error; err != nil {
			return 0, fmt.Errorf("创建片区 %s 失败: %w", name, err)
		}
		cache[name] = district.ID
		return district.ID, nil
	}
	if err != nil {
		return 0, err
	}
	cache[name] = district.ID
	return district.ID, nil
}

func ensureRoad(tx *gorm.DB, districtID uint, name string, cache map[string]uint, key string) (uint, error) {
	if id, ok := cache[key]; ok {
		return id, nil
	}
	var road hierarchy.Road
	err := tx.Where("district_id = ? AND name = ?", districtID, name).First(&road).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		road = hierarchy.Road{DistrictID: districtID, Name: name}
		if err := tx.Create(&road).Error; err != nil {
			return 0, fmt.Errorf("创建道路 %s 失败: %w", name, err)
		}
		cache[key] = road.ID
		return road.ID, nil
	}
	if err != nil {
		return 0, err
	}
	cache[key] = road.ID
	return road.ID, nil
}

func hierarchyKey(districtID uint, roadName string) string {
	return fmt.Sprintf("%d\x00%s", districtID, roadName)
}

func backfillTaskSnapshots(tx *gorm.DB) error {
	if !columnExists(tx, &cleaningtask.CleaningTask{}, "district_snapshot_id") {
		return nil
	}
	return tx.Exec(`UPDATE cleaning_tasks
		SET district_snapshot_id = (SELECT district_id FROM pipe_segments WHERE pipe_segments.id = cleaning_tasks.pipe_segment_id),
			district_snapshot = COALESCE((SELECT name FROM districts WHERE id = (SELECT district_id FROM pipe_segments WHERE pipe_segments.id = cleaning_tasks.pipe_segment_id)), ''),
			road_snapshot_id = (SELECT road_id FROM pipe_segments WHERE pipe_segments.id = cleaning_tasks.pipe_segment_id),
			road_snapshot = COALESCE((SELECT name FROM roads WHERE id = (SELECT road_id FROM pipe_segments WHERE pipe_segments.id = cleaning_tasks.pipe_segment_id)), '')
		WHERE district_snapshot_id = 0 OR district_snapshot_id IS NULL`).Error
}

func backfillRecordSnapshots(tx *gorm.DB) error {
	if !columnExists(tx, &cleaningrecord.CleaningRecord{}, "district_snapshot_id") {
		return nil
	}
	return tx.Exec(`UPDATE cleaning_records
		SET district_snapshot_id = (
				SELECT s.district_id FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				WHERE t.id = cleaning_records.task_id
			),
			district_snapshot = COALESCE((
				SELECT d.name FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				JOIN districts d ON d.id = s.district_id
				WHERE t.id = cleaning_records.task_id
			), ''),
			road_snapshot_id = (
				SELECT s.road_id FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				WHERE t.id = cleaning_records.task_id
			),
			road_snapshot = COALESCE((
				SELECT rd.name FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				JOIN roads rd ON rd.id = s.road_id
				WHERE t.id = cleaning_records.task_id
			), '')
		WHERE district_snapshot_id = 0 OR district_snapshot_id IS NULL`).Error
}

func backfillAcceptanceSnapshots(tx *gorm.DB) error {
	if !columnExists(tx, &acceptance.AcceptanceRecord{}, "district_snapshot_id") {
		return nil
	}
	return tx.Exec(`UPDATE acceptance_records
		SET district_snapshot_id = (
				SELECT s.district_id FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				WHERE t.id = acceptance_records.task_id
			),
			district_snapshot = COALESCE((
				SELECT d.name FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				JOIN districts d ON d.id = s.district_id
				WHERE t.id = acceptance_records.task_id
			), ''),
			road_snapshot_id = (
				SELECT s.road_id FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				WHERE t.id = acceptance_records.task_id
			),
			road_snapshot = COALESCE((
				SELECT rd.name FROM cleaning_tasks t
				JOIN pipe_segments s ON s.id = t.pipe_segment_id
				JOIN roads rd ON rd.id = s.road_id
				WHERE t.id = acceptance_records.task_id
			), '')
		WHERE district_snapshot_id = 0 OR district_snapshot_id IS NULL`).Error
}

func columnExists(db *gorm.DB, model any, column string) bool {
	if !db.Migrator().HasTable(model) {
		return false
	}
	return db.Migrator().HasColumn(model, column)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
