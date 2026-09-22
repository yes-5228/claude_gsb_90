package hierarchy_test

import (
	"context"
	"testing"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/testsupport"
)

func createDistrict(t *testing.T, fixture *testsupport.Fixture, name string) *hierarchy.District {
	t.Helper()
	district, err := fixture.Hierarchy.CreateDistrict(context.Background(), hierarchy.SaveDistrictRequest{Name: name})
	if err != nil {
		t.Fatalf("创建片区失败: %v", err)
	}
	return district
}

func TestCreateDistrictRejectsDuplicateName(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	createDistrict(t, fixture, "城北片区")

	_, err := fixture.Hierarchy.CreateDistrict(context.Background(), hierarchy.SaveDistrictRequest{Name: "城北片区"})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)
}

func TestCreateRoadRejectsDuplicateNameInSameDistrict(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	other := createDistrict(t, fixture, "城北片区")

	// 同一片区重名拒绝。
	_, err := fixture.Hierarchy.CreateRoad(context.Background(), hierarchy.SaveRoadRequest{
		DistrictID: fixture.District.ID, Name: fixture.Road.Name,
	})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)

	// 不同片区允许同名道路。
	road, err := fixture.Hierarchy.CreateRoad(context.Background(), hierarchy.SaveRoadRequest{
		DistrictID: other.ID, Name: fixture.Road.Name,
	})
	testsupport.RequireNoError(t, err)
	if road.DistrictID != other.ID {
		t.Fatalf("期望道路归属新城北片区，实际 %d", road.DistrictID)
	}
}

func TestRenameDistrictWritesLog(t *testing.T) {
	fixture := testsupport.NewFixture(t)

	_, err := fixture.Hierarchy.RenameDistrict(context.Background(), fixture.District.ID,
		hierarchy.RenameDistrictRequest{Name: "城东新区", Operator: "张工", Reason: "区划调整"})
	testsupport.RequireNoError(t, err)

	tree, err := fixture.Hierarchy.Tree(context.Background())
	testsupport.RequireNoError(t, err)
	found := false
	for _, node := range tree.Districts {
		if node.ID == fixture.District.ID {
			found = node.Name == "城东新区"
		}
	}
	if !found {
		t.Fatalf("改名后层级树未出现新名称: %+v", tree.Districts)
	}

	logs, total, err := fixture.Hierarchy.Logs(context.Background(), hierarchy.LogListQuery{
		NodeType: hierarchy.NodeTypeDistrict,
	})
	if err != nil {
		t.Fatalf("查询变更日志失败: %v", err)
	}
	if total == 0 || len(logs) == 0 {
		t.Fatalf("期望至少一条片区变更日志")
	}
	latest := logs[0]
	if latest.Action != hierarchy.ActionRenameDistrict || latest.Operator != "张工" {
		t.Fatalf("变更日志内容不符合预期: %+v", latest)
	}
}

func TestDeleteDistrictBlockedWhenReferencedAndWhenHasRoads(t *testing.T) {
	fixture := testsupport.NewFixture(t)

	// fixture 默认片区下既有道路又有管段，删除必须被拒绝。
	err := fixture.Hierarchy.DeleteDistrict(context.Background(), fixture.District.ID, hierarchy.OperatorRequest{})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)

	// 空片区可以删除。
	empty := createDistrict(t, fixture, "无人片区")
	testsupport.RequireNoError(t, fixture.Hierarchy.DeleteDistrict(context.Background(), empty.ID,
		hierarchy.OperatorRequest{Operator: "张工"}))
	_, err = fixture.Hierarchy.CreateRoad(context.Background(), hierarchy.SaveRoadRequest{
		DistrictID: empty.ID, Name: "不存在的路",
	})
	testsupport.RequireAppError(t, err, httpx.CodeNotFound)
}

func TestDeleteRoadBlockedWhenReferencedBySegment(t *testing.T) {
	fixture := testsupport.NewFixture(t)

	err := fixture.Hierarchy.DeleteRoad(context.Background(), fixture.Road.ID, hierarchy.OperatorRequest{})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)
}

func TestBatchAdjustIsAtomicAndMovesSegments(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	target := createDistrict(t, fixture, "城北片区")

	resp, err := fixture.Hierarchy.Batch(context.Background(), hierarchy.BatchRequest{
		Reason:    "片区整合",
		Operator:  "李工",
		Districts: []hierarchy.BatchDistrict{{ID: fixture.District.ID, Name: "城东整合区"}},
		Roads:     []hierarchy.BatchRoad{{ID: fixture.Road.ID, DistrictID: target.ID}},
	})
	testsupport.RequireNoError(t, err)
	if resp.DistrictChanges != 1 || resp.RoadChanges != 1 {
		t.Fatalf("期望各 1 项改动，实际 %+v", resp)
	}

	// 道路归到新城北片区，引用它的管段也跟着调整。
	segment, err := fixture.Segments.FindByID(context.Background(), fixture.Segment.ID)
	testsupport.RequireNoError(t, err)
	if segment.DistrictID != target.ID {
		t.Fatalf("期望管段随道路调整到片区 %d，实际 %d", target.ID, segment.DistrictID)
	}
	if segment.RoadID == nil || *segment.RoadID != fixture.Road.ID {
		t.Fatalf("期望管段仍挂在调整后的道路上，实际 %+v", segment.RoadID)
	}
}

func TestBatchAdjustRollsBackOnConflict(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	other := createDistrict(t, fixture, "城北片区")

	// 批次里第二项是不存在的片区，整批必须回滚，第一项改名也不能生效。
	_, err := fixture.Hierarchy.Batch(context.Background(), hierarchy.BatchRequest{
		Districts: []hierarchy.BatchDistrict{
			{ID: fixture.District.ID, Name: "城东整合区"},
			{ID: 99999, Name: "幽灵片区"},
		},
		Roads: []hierarchy.BatchRoad{{ID: fixture.Road.ID, DistrictID: other.ID}},
	})
	testsupport.RequireAppError(t, err, httpx.CodeNotFound)

	// 片区名称应保持不变。
	tree, err := fixture.Hierarchy.Tree(context.Background())
	testsupport.RequireNoError(t, err)
	for _, node := range tree.Districts {
		if node.ID == fixture.District.ID && node.Name != fixture.District.Name {
			t.Fatalf("批次失败后改名不应生效，期望 %s，实际 %s", fixture.District.Name, node.Name)
		}
	}
}

func TestMoveRoadMovesSegmentAndRejectsTargetDuplicate(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	target := createDistrict(t, fixture, "城北片区")
	_, err := fixture.Hierarchy.CreateRoad(context.Background(), hierarchy.SaveRoadRequest{
		DistrictID: target.ID, Name: fixture.Road.Name,
	})
	testsupport.RequireNoError(t, err)

	// 目标片区已有同名道路，移动被拒绝。
	_, err = fixture.Hierarchy.MoveRoad(context.Background(), fixture.Road.ID,
		hierarchy.MoveRoadRequest{DistrictID: target.ID})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)

	// 改名后再移动成功，管段跟随调整。
	_, err = fixture.Hierarchy.RenameRoad(context.Background(), fixture.Road.ID,
		hierarchy.RenameRoadRequest{Name: "测试道路改"})
	testsupport.RequireNoError(t, err)
	_, err = fixture.Hierarchy.MoveRoad(context.Background(), fixture.Road.ID,
		hierarchy.MoveRoadRequest{DistrictID: target.ID, Reason: "路网调整"})
	testsupport.RequireNoError(t, err)

	segment, err := fixture.Segments.FindByID(context.Background(), fixture.Segment.ID)
	testsupport.RequireNoError(t, err)
	if segment.DistrictID != target.ID {
		t.Fatalf("期望管段跟随道路调整到目标片区，实际 %d", segment.DistrictID)
	}
}
