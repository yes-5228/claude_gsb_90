package refx

import (
	"context"

	"gorm.io/gorm"
)

// TaskBrief 任务精简信息（含所属管段），供清淤记录、验收记录列表拼接展示。
//
// 片区 / 道路名称取任务登记当时固化的层级快照，层级调整后历史记录仍按当时口径展示。
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
	DistrictID      uint   `json:"districtId"`
	RoadID          uint   `json:"roadId"`
	SegmentDistrict string `json:"segmentDistrict"`
	SegmentRoad     string `json:"segmentRoad"`
}

// TaskBriefsByIDs 批量查询任务精简信息，顺带带出管段编号、名称与登记当时层级快照。
func TaskBriefsByIDs(ctx context.Context, db *gorm.DB, taskIDs []uint) (map[uint]TaskBrief, error) {
	result := make(map[uint]TaskBrief, len(taskIDs))
	if len(taskIDs) == 0 {
		return result, nil
	}
	briefs := make([]TaskBrief, 0, len(taskIDs))
	err := db.WithContext(ctx).Table(TableCleaningTasks+" AS t").
		Select(`t.id, t.code, t.title, t.status, t.priority, t.pipe_segment_id, t.team_name,
			t.district_id, t.road_id,
			COALESCE(t.district_name, '') AS segment_district,
			COALESCE(t.road_name, '') AS segment_road,
			COALESCE(s.code, '') AS segment_code,
			COALESCE(s.name, '') AS segment_name`).
		Joins("LEFT JOIN "+TablePipeSegments+" AS s ON s.id = t.pipe_segment_id").
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
