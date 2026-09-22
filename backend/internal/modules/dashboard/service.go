// Package dashboard 汇总看板模块：跨模块只读统计，不写入任何业务数据。
package dashboard

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/shared/date"
	"github.com/drainage/desilting/internal/shared/num"
	"github.com/drainage/desilting/internal/shared/refx"
)

// Service 看板统计。
type Service struct {
	db *gorm.DB
}

// NewService 构造服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Overview 总览指标。
type Overview struct {
	SegmentTotal          int64            `json:"segmentTotal"`
	SegmentTotalLengthM   float64          `json:"segmentTotalLengthM"`
	SegmentByStatus       map[string]int64 `json:"segmentByStatus"`
	UncleanedSegmentCount int64            `json:"uncleanedSegmentCount"`

	TaskTotal    int64            `json:"taskTotal"`
	TaskByStatus map[string]int64 `json:"taskByStatus"`
	TaskOverdue  int64            `json:"taskOverdue"`

	RecordTotal       int64   `json:"recordTotal"`
	SludgeTotalM3     float64 `json:"sludgeTotalM3"`
	SludgeThisMonthM3 float64 `json:"sludgeThisMonthM3"`
	CleanedLengthM    float64 `json:"cleanedLengthM"`

	AcceptanceTotal        int64   `json:"acceptanceTotal"`
	AcceptancePassCount    int64   `json:"acceptancePassCount"`
	AcceptancePassRate     float64 `json:"acceptancePassRate"`
	PendingAcceptanceCount int64   `json:"pendingAcceptanceCount"`
	PendingRectifyCount    int64   `json:"pendingRectifyCount"`
}

// Overview 汇总各模块关键指标。
func (s *Service) Overview(ctx context.Context) (*Overview, error) {
	result := &Overview{
		SegmentByStatus: make(map[string]int64),
		TaskByStatus:    make(map[string]int64),
	}

	// ---------- 管段台账 ----------
	type segmentAgg struct {
		Total     int64
		LengthM   float64
		Uncleaned int64
	}
	var segmentStats segmentAgg
	err := s.db.WithContext(ctx).Table(refx.TablePipeSegments).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(length_m), 0) AS length_m,
			COALESCE(SUM(CASE WHEN last_cleaned_at IS NULL THEN 1 ELSE 0 END), 0) AS uncleaned`).
		Scan(&segmentStats).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计管段台账失败", err)
	}
	result.SegmentTotal = segmentStats.Total
	result.SegmentTotalLengthM = num.Round2(segmentStats.LengthM)
	result.UncleanedSegmentCount = segmentStats.Uncleaned

	segmentStatus, err := s.countBy(ctx, refx.TablePipeSegments, "status")
	if err != nil {
		return nil, httpx.WrapInternal("统计管段状态失败", err)
	}
	result.SegmentByStatus = segmentStatus

	// ---------- 清淤任务 ----------
	var taskTotal int64
	if err := s.db.WithContext(ctx).Table(refx.TableCleaningTasks).Count(&taskTotal).Error; err != nil {
		return nil, httpx.WrapInternal("统计任务总数失败", err)
	}
	result.TaskTotal = taskTotal

	taskStatus, err := s.countBy(ctx, refx.TableCleaningTasks, "status")
	if err != nil {
		return nil, httpx.WrapInternal("统计任务状态失败", err)
	}
	result.TaskByStatus = taskStatus

	// 超期任务：计划完成日期已过，但仍未进入验收环节
	var overdue int64
	today := date.Today()
	err = s.db.WithContext(ctx).Table(refx.TableCleaningTasks).
		Where("plan_end_date < ?", today.Time).
		Where("status IN ?", []string{cleaningtask.StatusPending, cleaningtask.StatusInProgress}).
		Count(&overdue).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计超期任务失败", err)
	}
	result.TaskOverdue = overdue

	// ---------- 清淤记录 ----------
	type recordAgg struct {
		Total   int64
		Sludge  float64
		LengthM float64
	}
	var recordStats recordAgg
	err = s.db.WithContext(ctx).Table(refx.TableCleaningRecords).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(sludge_volume_m3), 0) AS sludge,
			COALESCE(SUM(length_m), 0) AS length_m`).
		Scan(&recordStats).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计清淤记录失败", err)
	}
	result.RecordTotal = recordStats.Total
	result.SludgeTotalM3 = num.Round2(recordStats.Sludge)
	result.CleanedLengthM = num.Round2(recordStats.LengthM)

	monthStart := date.New(time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC))
	var monthSludge float64
	err = s.db.WithContext(ctx).Table(refx.TableCleaningRecords).
		Select("COALESCE(SUM(sludge_volume_m3), 0)").
		Where("cleaned_at >= ?", monthStart.Time).
		Scan(&monthSludge).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计本月清淤量失败", err)
	}
	result.SludgeThisMonthM3 = num.Round2(monthSludge)

	// ---------- 验收记录 ----------
	var acceptanceTotal int64
	if err := s.db.WithContext(ctx).Table(refx.TableAcceptanceRecords).Count(&acceptanceTotal).Error; err != nil {
		return nil, httpx.WrapInternal("统计验收总数失败", err)
	}
	result.AcceptanceTotal = acceptanceTotal

	acceptanceByResult, err := s.countBy(ctx, refx.TableAcceptanceRecords, "result")
	if err != nil {
		return nil, httpx.WrapInternal("统计验收结论失败", err)
	}
	result.AcceptancePassCount = acceptanceByResult["pass"]
	if acceptanceTotal > 0 {
		result.AcceptancePassRate = num.Round2(float64(result.AcceptancePassCount) / float64(acceptanceTotal) * 100)
	}

	var pendingRectify int64
	err = s.db.WithContext(ctx).Table(refx.TableAcceptanceRecords).
		Where("result = ? AND rectified_at IS NULL", "rework").
		Count(&pendingRectify).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计待整改数量失败", err)
	}
	result.PendingRectifyCount = pendingRectify

	var pendingAcceptance int64
	err = s.db.WithContext(ctx).Table(refx.TableCleaningTasks).
		Where("status = ?", cleaningtask.StatusCompleted).
		Count(&pendingAcceptance).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计待验收任务失败", err)
	}
	result.PendingAcceptanceCount = pendingAcceptance

	return result, nil
}

// DistrictStat 片区维度的统计（按当前层级口径）。
type DistrictStat struct {
	DistrictID            uint       `json:"districtId"`
	District              string     `json:"district"`
	SegmentCount          int64      `json:"segmentCount"`
	SegmentLengthM        float64    `json:"segmentLengthM"`
	UncleanedSegmentCount int64      `json:"uncleanedSegmentCount"`
	LastCleanedAt         *date.Date `json:"lastCleanedAt"`
	TaskCount             int64      `json:"taskCount"`
	AcceptedTaskCount     int64      `json:"acceptedTaskCount"`
	SludgeVolumeM3        float64    `json:"sludgeVolumeM3"`
}

// DistrictStats 按当前片区层级统计管段规模与清淤成果。
//
// 看板反映"当前口径"：管段按当前所属片区汇总；任务 / 清淤量通过管段关联回片区，
// 与台账、任务列表、记录列表共用同一套层级。
func (s *Service) DistrictStats(ctx context.Context) ([]DistrictStat, error) {
	type segmentRow struct {
		DistrictID            uint
		District              string
		SegmentCount          int64
		SegmentLengthM        float64
		UncleanedSegmentCount int64
		LastCleanedAt         *date.Date
	}
	segmentRows := make([]segmentRow, 0)
	// 层级行缺失时用 || 拼兜底名称，SQLite 与 PostgreSQL 均支持该字符串拼接操作符。
	err := s.db.WithContext(ctx).Table(refx.TablePipeSegments + " AS p").
		Select(`p.district_id AS district_id,
			COALESCE(d.name, '片区#' || p.district_id) AS district,
			COUNT(*) AS segment_count,
			COALESCE(SUM(p.length_m), 0) AS segment_length_m,
			COALESCE(SUM(CASE WHEN p.last_cleaned_at IS NULL THEN 1 ELSE 0 END), 0) AS uncleaned_segment_count,
			MAX(p.last_cleaned_at) AS last_cleaned_at`).
		Joins("LEFT JOIN " + refx.TableDistricts + " AS d ON d.id = p.district_id").
		Group("p.district_id, d.name").
		Order("d.name ASC, p.district_id ASC").
		Scan(&segmentRows).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计片区管段失败", err)
	}

	type taskRow struct {
		DistrictID        uint
		TaskCount         int64
		AcceptedTaskCount int64
		SludgeVolumeM3    float64
	}
	taskRows := make([]taskRow, 0)
	err = s.db.WithContext(ctx).Table(refx.TableCleaningTasks+" AS t").
		Select(`s.district_id AS district_id,
			COUNT(DISTINCT t.id) AS task_count,
			COUNT(DISTINCT CASE WHEN t.status = ? THEN t.id END) AS accepted_task_count,
			COALESCE(SUM(r.sludge_volume_m3), 0) AS sludge_volume_m3`, cleaningtask.StatusAccepted).
		Joins("INNER JOIN " + refx.TablePipeSegments + " AS s ON s.id = t.pipe_segment_id").
		Joins("LEFT JOIN " + refx.TableCleaningRecords + " AS r ON r.task_id = t.id").
		Group("s.district_id").
		Scan(&taskRows).Error
	if err != nil {
		return nil, httpx.WrapInternal("统计片区清淤量失败", err)
	}

	taskByDistrict := make(map[uint]taskRow, len(taskRows))
	for _, row := range taskRows {
		taskByDistrict[row.DistrictID] = row
	}

	stats := make([]DistrictStat, 0, len(segmentRows))
	for _, row := range segmentRows {
		item := DistrictStat{
			DistrictID:            row.DistrictID,
			District:              row.District,
			SegmentCount:          row.SegmentCount,
			SegmentLengthM:        num.Round2(row.SegmentLengthM),
			UncleanedSegmentCount: row.UncleanedSegmentCount,
			LastCleanedAt:         row.LastCleanedAt,
		}
		if task, ok := taskByDistrict[row.DistrictID]; ok {
			item.TaskCount = task.TaskCount
			item.AcceptedTaskCount = task.AcceptedTaskCount
			item.SludgeVolumeM3 = num.Round2(task.SludgeVolumeM3)
		}
		stats = append(stats, item)
	}
	return stats, nil
}

// PendingAcceptanceItem 待验收任务。
type PendingAcceptanceItem struct {
	TaskID          uint       `json:"taskId"`
	Code            string     `json:"code"`
	Title           string     `json:"title"`
	SegmentCode     string     `json:"segmentCode"`
	SegmentName     string     `json:"segmentName"`
	SegmentDistrict string     `json:"segmentDistrict"`
	SegmentRoad     string     `json:"segmentRoad"`
	DistrictID      uint       `json:"districtId"`
	TeamName        string     `json:"teamName"`
	PlanEndDate     date.Date  `json:"planEndDate"`
	FinishedAt      *time.Time `json:"finishedAt"`
	RecordCount     int64      `json:"recordCount"`
	SludgeVolumeM3  float64    `json:"sludgeVolumeM3"`
	OverdueDays     int        `json:"overdueDays"`
}

// PendingAcceptance 待验收任务清单，按完工时间升序（先完工先验收）。
//
// 看板使用当前层级名称（通过管段关联层级表），反映实时归属。
func (s *Service) PendingAcceptance(ctx context.Context, limit int) ([]PendingAcceptanceItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	items := make([]PendingAcceptanceItem, 0, limit)
	err := s.db.WithContext(ctx).Table(refx.TableCleaningTasks+" AS t").
		Select(`t.id AS task_id, t.code, t.title, t.team_name, t.plan_end_date, t.finished_at,
			COALESCE(s.code, '') AS segment_code,
			COALESCE(s.name, '') AS segment_name,
			COALESCE(d.name, '') AS segment_district,
			COALESCE(rd.name, '') AS segment_road,
			COALESCE(s.district_id, 0) AS district_id,
			COALESCE(r.record_count, 0) AS record_count,
			COALESCE(r.sludge_volume, 0) AS sludge_volume_m3`).
		Joins("LEFT JOIN "+refx.TablePipeSegments+" AS s ON s.id = t.pipe_segment_id").
		Joins("LEFT JOIN "+refx.TableDistricts+" AS d ON d.id = s.district_id").
		Joins("LEFT JOIN "+refx.TableRoads+" AS rd ON rd.id = s.road_id").
		Joins(`LEFT JOIN (
			SELECT task_id, COUNT(*) AS record_count, SUM(sludge_volume_m3) AS sludge_volume
			FROM `+refx.TableCleaningRecords+` GROUP BY task_id
		) AS r ON r.task_id = t.id`).
		Where("t.status = ?", cleaningtask.StatusCompleted).
		Order("t.finished_at ASC, t.id ASC").
		Limit(limit).
		Scan(&items).Error
	if err != nil {
		return nil, httpx.WrapInternal("查询待验收任务失败", err)
	}

	today := date.Today()
	for i := range items {
		items[i].SludgeVolumeM3 = num.Round2(items[i].SludgeVolumeM3)
		if items[i].PlanEndDate.IsZero() {
			continue
		}
		if today.After(items[i].PlanEndDate) {
			items[i].OverdueDays = int(today.Time.Sub(items[i].PlanEndDate.Time).Hours() / 24)
		}
	}
	return items, nil
}

// RecentRecordItem 最近清淤记录。
type RecentRecordItem struct {
	RecordID        uint      `json:"recordId"`
	Code            string    `json:"code"`
	CleanedAt       date.Date `json:"cleanedAt"`
	TaskID          uint      `json:"taskId"`
	TaskCode        string    `json:"taskCode"`
	TaskTitle       string    `json:"taskTitle"`
	SegmentCode     string    `json:"segmentCode"`
	SegmentName     string    `json:"segmentName"`
	SegmentDistrict string    `json:"segmentDistrict"`
	SegmentRoad     string    `json:"segmentRoad"`
	TeamName        string    `json:"teamName"`
	RecorderName    string    `json:"recorderName"`
	LengthM         float64   `json:"lengthM"`
	SludgeVolumeM3  float64   `json:"sludgeVolumeM3"`
}

// RecentRecords 最近录入的清淤记录。
//
// 名称口径与记录列表一致：优先取记录登记当时固化的层级快照，
// 旧数据快照为空时回退到管段当前层级。
func (s *Service) RecentRecords(ctx context.Context, limit int) ([]RecentRecordItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	items := make([]RecentRecordItem, 0, limit)
	err := s.db.WithContext(ctx).Table(refx.TableCleaningRecords + " AS r").
		Select(`r.id AS record_id, r.code, r.cleaned_at, r.length_m, r.sludge_volume_m3, r.recorder_name,
			t.id AS task_id, t.code AS task_code, t.title AS task_title, t.team_name,
			COALESCE(s.code, '') AS segment_code,
			COALESCE(s.name, '') AS segment_name,
			COALESCE(NULLIF(r.district_snapshot, ''), d.name, '') AS segment_district,
			COALESCE(NULLIF(r.road_snapshot, ''), rd.name, '') AS segment_road`).
		Joins("INNER JOIN " + refx.TableCleaningTasks + " AS t ON t.id = r.task_id").
		Joins("LEFT JOIN " + refx.TablePipeSegments + " AS s ON s.id = t.pipe_segment_id").
		Joins("LEFT JOIN " + refx.TableDistricts + " AS d ON d.id = COALESCE(r.district_snapshot_id, s.district_id)").
		Joins("LEFT JOIN " + refx.TableRoads + " AS rd ON rd.id = COALESCE(r.road_snapshot_id, s.road_id)").
		Order("r.cleaned_at DESC, r.id DESC").
		Limit(limit).
		Scan(&items).Error
	if err != nil {
		return nil, httpx.WrapInternal("查询最近清淤记录失败", err)
	}
	return items, nil
}

// countBy 按指定列做分组计数。
func (s *Service) countBy(ctx context.Context, table, column string) (map[string]int64, error) {
	type row struct {
		Key   string
		Total int64
	}
	rows := make([]row, 0)
	err := s.db.WithContext(ctx).Table(table).
		Select(column + " AS key, COUNT(*) AS total").
		Group(column).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.Key] = item.Total
	}
	return result, nil
}
