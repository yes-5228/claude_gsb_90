package router

import (
	"context"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/modules/acceptance"
	"github.com/drainage/desilting/internal/modules/cleaningrecord"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/hierarchy"
)

// taskSnapshotGateway 把 hierarchy.Service 适配成任务模块需要的快照网关，
// 避免 hierarchy 反向依赖业务模块。
type taskSnapshotGateway struct{ svc *hierarchy.Service }

func (g taskSnapshotGateway) SnapshotTx(
	ctx context.Context,
	segmentID uint,
	fn func(tx *gorm.DB, snap cleaningtask.Snapshot) error,
) error {
	return g.svc.SnapshotTx(ctx, segmentID, func(tx *gorm.DB, snap hierarchy.Snapshot) error {
		return fn(tx, cleaningtask.Snapshot{
			DistrictID:   snap.DistrictID,
			DistrictName: snap.DistrictName,
			RoadID:       snap.RoadID,
			RoadName:     snap.RoadName,
		})
	})
}

// recordSnapshotGateway 适配清淤记录模块的快照网关。
type recordSnapshotGateway struct{ svc *hierarchy.Service }

func (g recordSnapshotGateway) SnapshotTx(
	ctx context.Context,
	segmentID uint,
	fn func(tx *gorm.DB, snap cleaningrecord.Snapshot) error,
) error {
	return g.svc.SnapshotTx(ctx, segmentID, func(tx *gorm.DB, snap hierarchy.Snapshot) error {
		return fn(tx, cleaningrecord.Snapshot{
			DistrictID:   snap.DistrictID,
			DistrictName: snap.DistrictName,
			RoadID:       snap.RoadID,
			RoadName:     snap.RoadName,
		})
	})
}

// acceptanceSnapshotGateway 适配验收模块的事务内快照网关。
type acceptanceSnapshotGateway struct{ svc *hierarchy.Service }

func (g acceptanceSnapshotGateway) SnapshotInTx(
	ctx context.Context,
	tx *gorm.DB,
	segmentID uint,
) (acceptance.Snapshot, error) {
	snap, err := g.svc.SnapshotInTx(ctx, tx, segmentID)
	if err != nil {
		return acceptance.Snapshot{}, err
	}
	return acceptance.Snapshot{
		DistrictID:   snap.DistrictID,
		DistrictName: snap.DistrictName,
		RoadID:       snap.RoadID,
		RoadName:     snap.RoadName,
	}, nil
}

// pipesegmentHierarchyGateway 适配管段台账模块的层级校验网关。
type pipesegmentHierarchyGateway struct{ svc *hierarchy.Service }

func (g pipesegmentHierarchyGateway) EnsureNode(ctx context.Context, districtID uint, roadID uint) error {
	return g.svc.EnsureNode(ctx, districtID, roadID)
}
