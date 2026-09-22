package refx

import (
	"context"

	"gorm.io/gorm"
)

// TaskBrief 任务精简信息（含所属管段与层级快照），供清淤记录、验收记录列表拼接展示。
//
// 片区 / 道路名称优先取任务登记当时固化的快照；层级后续调整时，历史记录仍按
// 登记当时的层级展示。数据迁移产生的旧任务若快照为空，则回退到管段当前层级。
type TaskBrief struct {
	ID              uint   `json:"id"`
	Code            string `json:"code"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	Priority        string `json:"priority"`
	PipeSegmentID   uint   `json:"pipeSegmentId"`
	TeamName        string `json:"teamName"`
	SegmentCode     string `json:"segmentCode"`
	SegmentName     string `json:"segmentName"`
	SegmentDistrict string `json:"segmentDistrict"`
	SegmentRoad     string `json:"segmentRoad"`
	DistrictID      uint   `json:"districtId"`
	RoadID          *uint  `json:"roadId"`
}

// TaskBriefsByIDs 批量查询任务精简信息，顺带带出管段编号、名称与层级快照。
func TaskBriefsByIDs(ctx context.Context, db *gorm.DB, taskIDs []uint) (map[uint]TaskBrief, error) {
	result := make(map[uint]TaskBrief, len(taskIDs))
	if len(taskIDs) == 0 {
		return result, nil
	}
	briefs := make([]TaskBrief, 0, len(taskIDs))
	err := db.WithContext(ctx).Table(TableCleaningTasks+" AS t").
		Select(`t.id, t.code, t.title, t.status, t.priority, t.pipe_segment_id, t.team_name,
			COALESCE(s.code, '') AS segment_code,
			COALESCE(s.name, '') AS segment_name,
			COALESCE(NULLIF(t.district_snapshot, ''), d.name, '') AS segment_district,
			COALESCE(NULLIF(t.road_snapshot, ''), rd.name, '') AS segment_road,
			COALESCE(t.district_snapshot_id, s.district_id, 0) AS district_id,
			COALESCE(t.road_snapshot_id, s.road_id) AS road_id`).
		Joins("LEFT JOIN "+TablePipeSegments+" AS s ON s.id = t.pipe_segment_id").
		Joins("LEFT JOIN "+TableDistricts+" AS d ON d.id = COALESCE(t.district_snapshot_id, s.district_id)").
		Joins("LEFT JOIN "+TableRoads+" AS rd ON rd.id = COALESCE(t.road_snapshot_id, s.road_id)").
		Where("t.id IN ?", taskIDs).
		Scan(&briefs).Error
	if err != nil {
		return nil, err
	}
	for _, brief := range briefs {
		result[brief.ID] = brief
	}
	return result, nil
}
