package pipesegment_test

import (
	"context"
	"testing"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/modules/pipesegment"
	"github.com/drainage/desilting/internal/testsupport"
)

func segmentRequest(code string, roadID uint) pipesegment.SaveRequest {
	return pipesegment.SaveRequest{
		Code:         code,
		Name:         "管段 " + code,
		RoadID:       roadID,
		PipeType:     pipesegment.TypeRainwater,
		Material:     "concrete",
		DiameterMm:   800,
		LengthM:      156.5,
		DepthM:       3.2,
		StartManhole: "Y1-08",
		EndManhole:   "Y1-12",
		BuildYear:    2012,
		OwnerUnit:    "市政排水管理处",
	}
}

// createRoad 在指定片区下准备一条道路。
func createRoad(t *testing.T, fixture *testsupport.Fixture, district, road string) uint {
	t.Helper()
	d, err := fixture.Hierarchy.CreateDistrict(context.Background(), hierarchy.SaveDistrictRequest{Name: district})
	if err != nil {
		t.Fatalf("创建片区失败: %v", err)
	}
	r, err := fixture.Hierarchy.CreateRoad(context.Background(), hierarchy.SaveRoadRequest{
		Name: road, DistrictID: d.ID,
	})
	if err != nil {
		t.Fatalf("创建道路失败: %v", err)
	}
	return r.ID
}

func TestCreateSegmentRejectsDuplicatedCode(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	roadID := createRoad(t, fixture, "城西片区", "中山北路")

	_, err := fixture.Segments.Create(context.Background(), segmentRequest("PS-TEST-001", roadID))
	appErr := testsupport.RequireAppError(t, err, httpx.CodeConflict)
	if appErr.Status != 409 {
		t.Fatalf("期望 HTTP 状态码 409，实际 %d", appErr.Status)
	}
}

func TestCreateSegmentRejectsUnknownPipeType(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	request := segmentRequest("PS-TEST-002", fixture.Segment.RoadID)
	request.PipeType = "stormwater"

	_, err := fixture.Segments.Create(context.Background(), request)
	testsupport.RequireAppError(t, err, httpx.CodeValidation)
}

func TestCreateSegmentRejectsUnknownRoad(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	request := segmentRequest("PS-TEST-099", 999999)

	_, err := fixture.Segments.Create(context.Background(), request)
	testsupport.RequireAppError(t, err, httpx.CodeValidation)
}

func TestCreateSegmentDefaultsStatusToNormal(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	segment := fixture.CreateSegment(t, "PS-TEST-003", "城东片区")

	if segment.Status != pipesegment.StatusNormal {
		t.Fatalf("期望默认状态为 %s，实际 %s", pipesegment.StatusNormal, segment.Status)
	}
}

func TestDeleteSegmentBlockedWhenReferencedByTask(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	fixture.CreateTask(t, fixture.Segment.ID, "引用管段的任务")

	err := fixture.Segments.Delete(context.Background(), fixture.Segment.ID)
	testsupport.RequireAppError(t, err, httpx.CodeConflict)

	if _, err := fixture.Segments.FindByID(context.Background(), fixture.Segment.ID); err != nil {
		t.Fatalf("管段不应被删除: %v", err)
	}
}

func TestDeleteSegmentSucceedsWhenNotReferenced(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	segment := fixture.CreateSegment(t, "PS-TEST-004", "城西片区")

	testsupport.RequireNoError(t, fixture.Segments.Delete(context.Background(), segment.ID))

	err := fixture.Segments.Delete(context.Background(), segment.ID)
	testsupport.RequireAppError(t, err, httpx.CodeNotFound)
}

func TestListSegmentsFiltersByDistrictAndKeyword(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	westRoad := createRoad(t, fixture, "城西片区", "解放路")
	southRoad := createRoad(t, fixture, "城南片区", "长江路")
	segWest := fixture.CreateSegmentOnRoad(t, "PS-TEST-005", westRoad)
	_ = segWest
	fixture.CreateSegmentOnRoad(t, "PS-TEST-006", southRoad)

	// 找到“城西片区”的 ID。
	tree, err := fixture.Hierarchy.Tree(context.Background())
	testsupport.RequireNoError(t, err)
	var westDistrictID uint
	for _, d := range tree {
		if d.Name == "城西片区" {
			westDistrictID = d.ID
		}
	}

	byDistrict, total, err := fixture.Segments.List(context.Background(), pipesegment.ListQuery{
		DistrictIDs: []uint{westDistrictID},
		Page:        httpx.PageQuery{Page: 1, PageSize: 10},
	})
	testsupport.RequireNoError(t, err)
	if total != 1 || len(byDistrict) != 1 || byDistrict[0].Code != "PS-TEST-005" {
		t.Fatalf("期望按片区筛出 PS-TEST-005，实际 total=%d %+v", total, byDistrict)
	}

	byKeyword, _, err := fixture.Segments.List(context.Background(), pipesegment.ListQuery{
		Keyword: "ps-TEST-006",
		Page:    httpx.PageQuery{Page: 1, PageSize: 10},
	})
	testsupport.RequireNoError(t, err)
	if len(byKeyword) != 1 || byKeyword[0].Code != "PS-TEST-006" {
		t.Fatalf("期望按编号关键字忽略大小写命中 PS-TEST-006，实际 %+v", byKeyword)
	}
}

func TestDetailReturnsTaskStats(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	fixture.CreateTask(t, fixture.Segment.ID, "任务一")
	fixture.CreateTask(t, fixture.Segment.ID, "任务二")

	detail, err := fixture.Segments.Detail(context.Background(), fixture.Segment.ID)
	testsupport.RequireNoError(t, err)

	if detail.TaskStats.Total != 2 || detail.TaskStats.Pending != 2 {
		t.Fatalf("期望 2 条待开工任务，实际 %+v", detail.TaskStats)
	}
	if len(detail.RecentTasks) != 2 {
		t.Fatalf("期望最近任务列表包含 2 条记录，实际 %d", len(detail.RecentTasks))
	}
}

func TestHistoryIncludesAcceptanceResult(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	task := fixture.TaskReadyForAcceptance(t, fixture.Segment.ID, "清淤任务")
	_, err := fixture.Acceptances.Create(context.Background(), testsupport.PassRequest(task.ID, 90))
	testsupport.RequireNoError(t, err)

	items, err := fixture.Segments.History(context.Background(), fixture.Segment.ID)
	testsupport.RequireNoError(t, err)
	if len(items) != 1 {
		t.Fatalf("期望 1 条清淤履历，实际 %d", len(items))
	}
	if items[0].AcceptanceResult != "pass" {
		t.Fatalf("期望履历中带出验收结论 pass，实际 %q", items[0].AcceptanceResult)
	}
	if items[0].RecordCount != 1 {
		t.Fatalf("期望履历中记录条数为 1，实际 %d", items[0].RecordCount)
	}
}
