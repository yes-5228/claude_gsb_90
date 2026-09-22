package testsupport

import (
	"context"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/modules/acceptance"
	"github.com/drainage/desilting/internal/modules/cleaningrecord"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/modules/pipesegment"
)

// 测试用的层级网关适配，签名与 router 中的生产适配保持一致。

type hierarchySegmentGateway struct{ h *hierarchy.Service }

func (g hierarchySegmentGateway) EnsureNode(ctx context.Context, districtID uint, roadID uint) error {
	return g.h.EnsureNode(ctx, districtID, roadID)
}

type hierarchyTaskGateway struct{ h *hierarchy.Service }

func (g hierarchyTaskGateway) SnapshotTx(
	ctx context.Context,
	segmentID uint,
	fn func(tx *gorm.DB, snap cleaningtask.Snapshot) error,
) error {
	return g.h.SnapshotTx(ctx, segmentID, func(tx *gorm.DB, snap hierarchy.Snapshot) error {
		return fn(tx, cleaningtask.Snapshot{
			DistrictID:   snap.DistrictID,
			DistrictName: snap.DistrictName,
			RoadID:       snap.RoadID,
			RoadName:     snap.RoadName,
		})
	})
}

type hierarchyRecordGateway struct{ h *hierarchy.Service }

func (g hierarchyRecordGateway) SnapshotTx(
	ctx context.Context,
	segmentID uint,
	fn func(tx *gorm.DB, snap cleaningrecord.Snapshot) error,
) error {
	return g.h.SnapshotTx(ctx, segmentID, func(tx *gorm.DB, snap hierarchy.Snapshot) error {
		return fn(tx, cleaningrecord.Snapshot{
			DistrictID:   snap.DistrictID,
			DistrictName: snap.DistrictName,
			RoadID:       snap.RoadID,
			RoadName:     snap.RoadName,
		})
	})
}

type hierarchyAcceptanceGateway struct{ h *hierarchy.Service }

func (g hierarchyAcceptanceGateway) SnapshotInTx(
	ctx context.Context,
	tx *gorm.DB,
	segmentID uint,
) (acceptance.Snapshot, error) {
	snap, err := g.h.SnapshotInTx(ctx, tx, segmentID)
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

// 编译期保证适配满足各模块接口。
var (
	_ pipesegment.HierarchyGateway   = hierarchySegmentGateway{}
	_ cleaningtask.SnapshotGateway   = hierarchyTaskGateway{}
	_ cleaningrecord.SnapshotGateway = hierarchyRecordGateway{}
	_ acceptance.SnapshotGateway     = hierarchyAcceptanceGateway{}
)
