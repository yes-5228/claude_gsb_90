package cleaningtask_test

import (
	"context"
	"testing"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/testsupport"
)

func cleaningtaskListQuery() cleaningtask.ListQuery {
	return cleaningtask.ListQuery{Page: httpx.PageQuery{Page: 1, PageSize: 50}}
}

// TestTaskSnapshotKeepsHierarchyAtRegistrationTime 验证层级调整后历史任务仍按
// 登记当时的层级展示，而新登记的任务采用调整后的层级名称。
func TestTaskSnapshotKeepsHierarchyAtRegistrationTime(t *testing.T) {
	fixture := testsupport.NewFixture(t)

	oldName := fixture.District.Name
	task := fixture.CreateTask(t, fixture.Segment.ID, "调整前登记的任务")
	if task.DistrictSnapshot != oldName || task.RoadSnapshot != fixture.Road.Name {
		t.Fatalf("任务登记时层级快照不符合预期: district=%q road=%q",
			task.DistrictSnapshot, task.RoadSnapshot)
	}

	// 片区改名：历史任务名称不变，当前层级已变为新名称。
	newName := "城东新区"
	_, err := fixture.Hierarchy.RenameDistrict(context.Background(), fixture.District.ID,
		hierarchy.RenameDistrictRequest{Name: newName, Reason: "区划调整"})
	testsupport.RequireNoError(t, err)

	historical := fixture.Reload(t, task.ID)
	if historical.DistrictSnapshot != oldName {
		t.Fatalf("历史任务应保留旧片区名 %q，实际 %q", oldName, historical.DistrictSnapshot)
	}

	// 改名后新登记的任务按新口径归属。
	later := fixture.CreateTask(t, fixture.Segment.ID, "调整后登记的任务")
	if later.DistrictSnapshot != newName {
		t.Fatalf("新任务应使用新片区名 %q，实际 %q", newName, later.DistrictSnapshot)
	}
	if later.DistrictSnapshotID != fixture.District.ID {
		t.Fatalf("片区改名后快照 ID 应保持指向同一节点，实际 %d", later.DistrictSnapshotID)
	}

	// 任务列表中的管段信息取当前层级名称，历史任务行自身快照仍展示旧名称。
	items, total, err := fixture.Tasks.List(context.Background(), cleaningtaskListQuery())
	testsupport.RequireNoError(t, err)
	if total < 2 || len(items) < 2 {
		t.Fatalf("期望列表至少 2 条任务，实际 total=%d", total)
	}
	for _, item := range items {
		switch item.ID {
		case task.ID:
			if item.DistrictSnapshot != oldName {
				t.Fatalf("列表中历史任务片区应保持 %q，实际 %q", oldName, item.DistrictSnapshot)
			}
		case later.ID:
			if item.DistrictSnapshot != newName {
				t.Fatalf("列表中新任务片区应为 %q，实际 %q", newName, item.DistrictSnapshot)
			}
		}
	}
}

// TestTaskSnapshotFollowsRoadMove 验证道路归属调整后，调整前的任务保留旧归属名称，
// 管段跟随道路调整到新片区，新任务按新片区登记。
func TestTaskSnapshotFollowsRoadMove(t *testing.T) {
	fixture := testsupport.NewFixture(t)
	target, err := fixture.Hierarchy.CreateDistrict(context.Background(), hierarchy.SaveDistrictRequest{Name: "城北片区"})
	testsupport.RequireNoError(t, err)

	task := fixture.CreateTask(t, fixture.Segment.ID, "调整前登记的任务")
	if task.DistrictSnapshotID != fixture.District.ID {
		t.Fatalf("初始任务应归属城东片区，实际 %d", task.DistrictSnapshotID)
	}

	_, err = fixture.Hierarchy.MoveRoad(context.Background(), fixture.Road.ID,
		hierarchy.MoveRoadRequest{DistrictID: target.ID, Reason: "路网调整"})
	testsupport.RequireNoError(t, err)

	// 历史任务保留调整前口径。
	historical := fixture.Reload(t, task.ID)
	if historical.DistrictSnapshotID != fixture.District.ID {
		t.Fatalf("历史任务归属片区不应被道路调整改变，实际 %d", historical.DistrictSnapshotID)
	}

	// 管段跟随道路到新片区。
	segment, err := fixture.Segments.FindByID(context.Background(), fixture.Segment.ID)
	testsupport.RequireNoError(t, err)
	if segment.DistrictID != target.ID {
		t.Fatalf("道路移动后管段应归到目标片区 %d，实际 %d", target.ID, segment.DistrictID)
	}

	// 新任务按调整后的口径登记到新片区。
	later := fixture.CreateTask(t, fixture.Segment.ID, "调整后登记的任务")
	if later.DistrictSnapshotID != target.ID {
		t.Fatalf("新任务应按新口径归属目标片区 %d，实际 %d", target.ID, later.DistrictSnapshotID)
	}
}
