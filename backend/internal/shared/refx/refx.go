// Package refx 集中处理跨模块的反向引用检查与跨表统计。
//
// 模块之间的依赖方向是单向的：清淤记录 -> 清淤任务 -> 管段台账。
// 如果让上游模块反过来 import 下游模块（例如管段模块要检查自己是否已被任务引用），
// 就会形成循环依赖。因此这类反向查询统一收敛到这里，改用表名直接查询。
//
// 注意：这里的表名必须与各模块 model 中的 TableName() 保持一致。
package refx

import (
	"context"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/shared/num"
)

// 各模块的表名。
const (
	TablePipeSegments      = "pipe_segments"
	TableCleaningTasks     = "cleaning_tasks"
	TableCleaningRecords   = "cleaning_records"
	TableAcceptanceRecords = "acceptance_records"
	TableDistricts         = "districts"
	TableRoads             = "roads"
)

// HierarchyRefs 层级节点（片区 / 道路）被业务数据引用的计数。
//
// 用于层级删除保护：只要管段、任务、清淤记录或验收记录中还残留该层级的快照引用，
// 就不允许直接删除。
type HierarchyRefs struct {
	SegmentCount int64
	TaskCount    int64
	RecordCount  int64
	AcceptCount  int64
}

// Referenced 是否存在任意引用。
func (r HierarchyRefs) Referenced() bool {
	return r.SegmentCount > 0 || r.TaskCount > 0 || r.RecordCount > 0 || r.AcceptCount > 0
}

func countRefs(ctx context.Context, db *gorm.DB, column string, id uint, tables ...string) (int64, error) {
	var total int64
	for _, table := range tables {
		var count int64
		if err := db.WithContext(ctx).Table(table).Where(column+" = ?", id).Count(&count).Error; err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// DistrictRefs 统计片区在各业务表（含层级快照列）中的引用数量。
//
// 管段直接引用 district_id；任务 / 记录 / 验收登记时固化的快照为 district_snapshot_id。
func DistrictRefs(ctx context.Context, db *gorm.DB, districtID uint) (HierarchyRefs, error) {
	refs := HierarchyRefs{}
	var err error
	if refs.SegmentCount, err = countRefs(ctx, db, "district_id", districtID, TablePipeSegments); err != nil {
		return refs, err
	}
	if refs.TaskCount, err = countRefs(ctx, db, "district_snapshot_id", districtID, TableCleaningTasks); err != nil {
		return refs, err
	}
	if refs.RecordCount, err = countRefs(ctx, db, "district_snapshot_id", districtID, TableCleaningRecords); err != nil {
		return refs, err
	}
	if refs.AcceptCount, err = countRefs(ctx, db, "district_snapshot_id", districtID, TableAcceptanceRecords); err != nil {
		return refs, err
	}
	return refs, nil
}

// RoadRefs 统计道路在各业务表（含层级快照列）中的引用数量。
//
// 道路快照可空，未挂道路的数据 road_snapshot_id 为 NULL，不会被计入。
func RoadRefs(ctx context.Context, db *gorm.DB, roadID uint) (HierarchyRefs, error) {
	refs := HierarchyRefs{}
	var err error
	if refs.SegmentCount, err = countRefs(ctx, db, "road_id", roadID, TablePipeSegments); err != nil {
		return refs, err
	}
	if refs.TaskCount, err = countRefs(ctx, db, "road_snapshot_id", roadID, TableCleaningTasks); err != nil {
		return refs, err
	}
	if refs.RecordCount, err = countRefs(ctx, db, "road_snapshot_id", roadID, TableCleaningRecords); err != nil {
		return refs, err
	}
	if refs.AcceptCount, err = countRefs(ctx, db, "road_snapshot_id", roadID, TableAcceptanceRecords); err != nil {
		return refs, err
	}
	return refs, nil
}

// SegmentsInDistrict 返回片区下（直接挂载）的管段 ID。
func SegmentsInDistrict(ctx context.Context, db *gorm.DB, districtID uint) ([]uint, error) {
	ids := make([]uint, 0)
	err := db.WithContext(ctx).Table(TablePipeSegments).
		Where("district_id = ?", districtID).
		Pluck("id", &ids).Error
	return ids, err
}

// TaskStats 某个管段下的清淤任务数量汇总。
type TaskStats struct {
	Total      int64 `json:"total"`
	Pending    int64 `json:"pending"`
	InProgress int64 `json:"inProgress"`
	Completed  int64 `json:"completed"`
	Accepted   int64 `json:"accepted"`
	Cancelled  int64 `json:"cancelled"`
}

// TaskRef 管段详情中展示的精简任务信息。
type TaskRef struct {
	ID             uint    `json:"id"`
	Code           string  `json:"code"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	Priority       string  `json:"priority"`
	TeamName       string  `json:"teamName"`
	PlanStartDate  string  `json:"planStartDate"`
	PlanEndDate    string  `json:"planEndDate"`
	RecordCount    int64   `json:"recordCount"`
	SludgeVolumeM3 float64 `json:"sludgeVolumeM3"`
}

// HasTasksForSegment 管段是否已经被清淤任务引用。
func HasTasksForSegment(ctx context.Context, db *gorm.DB, segmentID uint) (bool, error) {
	return exists(ctx, db, TableCleaningTasks, "pipe_segment_id = ?", segmentID)
}

// HasRecordsForTask 任务下是否已经录入清淤记录。
func HasRecordsForTask(ctx context.Context, db *gorm.DB, taskID uint) (bool, error) {
	return exists(ctx, db, TableCleaningRecords, "task_id = ?", taskID)
}

// HasAcceptanceForTask 任务下是否已经有验收记录。
func HasAcceptanceForTask(ctx context.Context, db *gorm.DB, taskID uint) (bool, error) {
	return exists(ctx, db, TableAcceptanceRecords, "task_id = ?", taskID)
}

// HasAcceptanceForRecord 清淤记录是否已经被验收记录引用。
func HasAcceptanceForRecord(ctx context.Context, db *gorm.DB, recordID uint) (bool, error) {
	return exists(ctx, db, TableAcceptanceRecords, "cleaning_record_id = ?", recordID)
}

// TaskStatsForSegment 汇总某管段下各状态的任务数量。
func TaskStatsForSegment(ctx context.Context, db *gorm.DB, segmentID uint) (TaskStats, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rows []row
	err := db.WithContext(ctx).Table(TableCleaningTasks).
		Select("status, COUNT(*) AS total").
		Where("pipe_segment_id = ?", segmentID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return TaskStats{}, err
	}
	var stats TaskStats
	for _, item := range rows {
		stats.Total += item.Total
		switch item.Status {
		case "pending":
			stats.Pending = item.Total
		case "in_progress":
			stats.InProgress = item.Total
		case "completed":
			stats.Completed = item.Total
		case "accepted":
			stats.Accepted = item.Total
		case "cancelled":
			stats.Cancelled = item.Total
		}
	}
	return stats, nil
}

// RecentTasksForSegment 查询某管段最近的任务，并带上已录入的清淤量与记录条数。
func RecentTasksForSegment(ctx context.Context, db *gorm.DB, segmentID uint, limit int) ([]TaskRef, error) {
	if limit <= 0 {
		limit = 5
	}
	refs := make([]TaskRef, 0, limit)
	err := db.WithContext(ctx).Table(TableCleaningTasks+" AS t").
		Select(`t.id, t.code, t.title, t.status, t.priority, t.team_name,
			t.plan_start_date, t.plan_end_date,
			COALESCE(r.record_count, 0) AS record_count,
			COALESCE(r.sludge_volume, 0) AS sludge_volume_m3`).
		Joins(`LEFT JOIN (
			SELECT task_id, COUNT(*) AS record_count, SUM(sludge_volume_m3) AS sludge_volume
			FROM `+TableCleaningRecords+` GROUP BY task_id
		) AS r ON r.task_id = t.id`).
		Where("t.pipe_segment_id = ?", segmentID).
		Order("t.plan_start_date DESC, t.id DESC").
		Limit(limit).
		Scan(&refs).Error
	for i := range refs {
		refs[i].SludgeVolumeM3 = num.Round2(refs[i].SludgeVolumeM3)
	}
	return refs, err
}

func exists(ctx context.Context, db *gorm.DB, table, where string, args ...any) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Table(table).Where(where, args...).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
