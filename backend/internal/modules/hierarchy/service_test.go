package hierarchy_test

import (
	"context"
	"testing"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/testsupport"
)

// setup 建立一个带两片区、两道路的独立内存库。
func setup(t *testing.T) (*testsupport.Fixture, *hierarchy.District, *hierarchy.District, *hierarchy.Road, *hierarchy.Road) {
	t.Helper()
	fixture := &testsupport.Fixture{Services: testsupport.NewServices(testsupport.NewDB(t))}
	ctx := context.Background()
	var err error
	east, err := fixture.Hierarchy.CreateDistrict(ctx, hierarchy.SaveDistrictRequest{Name: "城东片区"})
	if err != nil {
		t.Fatalf("创建城东片区失败: %v", err)
	}
	west, err := fixture.Hierarchy.CreateDistrict(ctx, hierarchy.SaveDistrictRequest{Name: "城西片区"})
	if err != nil {
		t.Fatalf("创建城西片区失败: %v", err)
	}
	r1, err := fixture.Hierarchy.CreateRoad(ctx, hierarchy.SaveRoadRequest{Name: "中山北路", DistrictID: east.ID})
	if err != nil {
		t.Fatalf("创建道路失败: %v", err)
	}
	r2, err := fixture.Hierarchy.CreateRoad(ctx, hierarchy.SaveRoadRequest{Name: "滨江大道", DistrictID: east.ID})
	if err != nil {
		t.Fatalf("创建道路失败: %v", err)
	}
	return fixture, east, west, r1, r2
}

func TestCreateAndRenameDistrictLogsChange(t *testing.T) {
	fixture := &testsupport.Fixture{Services: testsupport.NewServices(testsupport.NewDB(t))}
	svc := fixture.Services
	ctx := context.Background()

	d, err := svc.Hierarchy.CreateDistrict(ctx, hierarchy.SaveDistrictRequest{Name: "城南片区", Operator: "张三"})
	testsupport.RequireNoError(t, err)

	_, err = svc.Hierarchy.RenameDistrict(ctx, d.ID, hierarchy.RenameDistrictRequest{Name: "城南新区", Operator: "张三"})
	testsupport.RequireNoError(t, err)

	items, total, err := svc.Hierarchy.ChangeLogs(ctx, hierarchy.ChangeLogQuery{NodeType: hierarchy.NodeTypeDistrict})
	testsupport.RequireNoError(t, err)
	if total < 2 {
		t.Fatalf("期望至少 2 条片区变更记录（新增+改名），实际 %d", total)
	}
	foundRename := false
	for _, item := range items {
		if item.Action == hierarchy.ActionRename && item.FromValue == "城南片区" && item.ToValue == "城南新区" {
			foundRename = true
		}
	}
	if !foundRename {
		t.Fatalf("未找到片区改名的变更记录: %+v", items)
	}
}

func TestDuplicateDistrictRejected(t *testing.T) {
	fixture := &testsupport.Fixture{Services: testsupport.NewServices(testsupport.NewDB(t))}
	svc := fixture.Services
	ctx := context.Background()
	if _, err := svc.Hierarchy.CreateDistrict(ctx, hierarchy.SaveDistrictRequest{Name: "城东片区"}); err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}
	_, err := svc.Hierarchy.CreateDistrict(ctx, hierarchy.SaveDistrictRequest{Name: "城东片区"})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)
}

func TestDeleteDistrictBlockedWhenRoadsRemain(t *testing.T) {
	fixture, east, _, _, _ := setup(t)
	ctx := context.Background()

	err := fixture.Hierarchy.DeleteDistrict(ctx, east.ID, hierarchy.DeleteRequest{})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)
}

func TestDeleteRoadBlockedWhenReferencedBySegment(t *testing.T) {
	fixture, east, _, r1, _ := setup(t)
	ctx := context.Background()

	fixture.CreateSegmentOnRoad(t, "PS-001", r1.ID)
	err := fixture.Hierarchy.DeleteRoad(ctx, r1.ID, hierarchy.DeleteRequest{})
	testsupport.RequireAppError(t, err, httpx.CodeConflict)

	// 没有管段引用的道路可以删除，删空后片区道路数下降。
	r3, err := fixture.Hierarchy.CreateRoad(ctx, hierarchy.SaveRoadRequest{Name: "临时道路", DistrictID: east.ID})
	testsupport.RequireNoError(t, err)
	testsupport.RequireNoError(t, fixture.Hierarchy.DeleteRoad(ctx, r3.ID, hierarchy.DeleteRequest{}))
}

func TestBatchMoveRoadsAtomicNoPartialSuccess(t *testing.T) {
	fixture, east, west, r1, r2 := setup(t)
	ctx := context.Background()

	// r1 合法调整到城西；r2 的目标片区故意给一个不存在的 ID，整批应回滚。
	err := fixture.Hierarchy.BatchMoveRoads(ctx, hierarchy.BatchMoveRoadsRequest{
		Items: []hierarchy.RoadMoveItem{
			{RoadID: r1.ID, DistrictID: west.ID},
			{RoadID: r2.ID, DistrictID: 999999},
		},
	})
	testsupport.RequireAppError(t, err, httpx.CodeNotFound)

	// 两条道路都必须仍在城东片区（r1 没有部分成功）。
	roads, err := fixture.Hierarchy.Tree(ctx)
	testsupport.RequireNoError(t, err)
	for _, d := range roads {
		if d.ID == east.ID && len(d.Roads) != 2 {
			t.Fatalf("整批回滚后城东应仍有 2 条道路，实际 %d", len(d.Roads))
		}
		if d.ID == west.ID && len(d.Roads) != 0 {
			t.Fatalf("整批回滚后城西不应有道路，实际 %d", len(d.Roads))
		}
	}
}

func TestBatchMoveRoadsSucceedsAndSharesBatchID(t *testing.T) {
	fixture, _, west, r1, r2 := setup(t)
	ctx := context.Background()

	err := fixture.Hierarchy.BatchMoveRoads(ctx, hierarchy.BatchMoveRoadsRequest{
		Items: []hierarchy.RoadMoveItem{
			{RoadID: r1.ID, DistrictID: west.ID},
			{RoadID: r2.ID, DistrictID: west.ID},
		},
		Operator: "李四",
	})
	testsupport.RequireNoError(t, err)

	tree, err := fixture.Hierarchy.Tree(ctx)
	testsupport.RequireNoError(t, err)
	for _, d := range tree {
		if d.ID == west.ID && len(d.Roads) != 2 {
			t.Fatalf("期望城西片区下有 2 条道路，实际 %d", len(d.Roads))
		}
	}

	logs, total, err := fixture.Hierarchy.ChangeLogs(ctx, hierarchy.ChangeLogQuery{Action: hierarchy.ActionMove})
	testsupport.RequireNoError(t, err)
	if total != 2 {
		t.Fatalf("期望 2 条归属调整记录，实际 %d", total)
	}
	if logs[0].BatchID != logs[1].BatchID {
		t.Fatalf("批量调整应共享同一批次号，实际 %s / %s", logs[0].BatchID, logs[1].BatchID)
	}
}

func TestTaskSnapshotKeptAfterRoadMoved(t *testing.T) {
	fixture, east, west, r1, _ := setup(t)
	ctx := context.Background()

	segment := fixture.CreateSegmentOnRoad(t, "PS-001", r1.ID)
	task := fixture.CreateTask(t, segment.ID, "层级快照任务")

	// 任务登记当时归属城东片区 / 中山北路。
	taskAfterCreate, err := fixture.Tasks.FindByID(ctx, task.ID)
	testsupport.RequireNoError(t, err)
	if taskAfterCreate.DistrictName != "城东片区" || taskAfterCreate.RoadName != "中山北路" {
		t.Fatalf("任务初始层级快照错误: %s / %s", taskAfterCreate.DistrictName, taskAfterCreate.RoadName)
	}

	// 之后把道路整体调整到城西片区。
	testsupport.RequireNoError(t, fixture.Hierarchy.MoveRoad(ctx, r1.ID, west.ID, hierarchy.MoveRoadRequest{}))

	// 历史任务仍按登记当时的层级展示，不随层级调整而改变。
	taskAfterMove, err := fixture.Tasks.FindByID(ctx, task.ID)
	testsupport.RequireNoError(t, err)
	if taskAfterMove.DistrictName != "城东片区" {
		t.Fatalf("历史任务片区应保持登记当时的城东片区，实际 %s", taskAfterMove.DistrictName)
	}
	if taskAfterMove.DistrictID != east.ID {
		t.Fatalf("历史任务片区 ID 快照应保持不变，实际 %d", taskAfterMove.DistrictID)
	}

	// 管段台账（当前层级）应跟随到城西片区。
	detail, err := fixture.Segments.Detail(ctx, segment.ID)
	testsupport.RequireNoError(t, err)
	if detail.Hierarchy == nil || detail.Hierarchy.DistrictName != "城西片区" {
		t.Fatalf("管段当前层级应跟随为城西片区，实际 %+v", detail.Hierarchy)
	}
}
