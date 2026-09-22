package hierarchy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/shared/refx"
)

// Service 片区 / 道路层级业务逻辑。
type Service struct {
	repo *Repository
}

// NewService 构造服务。
func NewService(repo *Repository) *Service {
	SetDialector(repo.DB())
	return &Service{repo: repo}
}

// Tree 返回完整层级树（片区 + 下属道路）。
func (s *Service) Tree(ctx context.Context) (*TreeResponse, error) {
	districts, err := s.repo.AllDistricts(ctx)
	if err != nil {
		return nil, httpx.WrapInternal("查询片区失败", err)
	}
	roads, err := s.repo.AllRoads(ctx)
	if err != nil {
		return nil, httpx.WrapInternal("查询道路失败", err)
	}
	roadsByDistrict := make(map[uint][]Road, len(districts))
	for _, road := range roads {
		roadsByDistrict[road.DistrictID] = append(roadsByDistrict[road.DistrictID], road)
	}
	nodes := make([]DistrictNode, 0, len(districts))
	for _, district := range districts {
		nodes = append(nodes, DistrictNode{
			District: district,
			Roads:    roadsByDistrict[district.ID],
		})
	}
	return &TreeResponse{Districts: nodes}, nil
}

// DistrictRefCounts 片区被业务数据引用的计数。
func (s *Service) DistrictRefCounts(ctx context.Context, id uint) (RefCounts, error) {
	refs, err := refx.DistrictRefs(ctx, s.repo.DB(), id)
	if err != nil {
		return RefCounts{}, httpx.WrapInternal("统计片区引用失败", err)
	}
	return RefCounts{
		SegmentCount: refs.SegmentCount,
		TaskCount:    refs.TaskCount,
		RecordCount:  refs.RecordCount,
		AcceptCount:  refs.AcceptCount,
	}, nil
}

// RoadRefCounts 道路被业务数据引用的计数。
func (s *Service) RoadRefCounts(ctx context.Context, id uint) (RefCounts, error) {
	refs, err := refx.RoadRefs(ctx, s.repo.DB(), id)
	if err != nil {
		return RefCounts{}, httpx.WrapInternal("统计道路引用失败", err)
	}
	return RefCounts{
		SegmentCount: refs.SegmentCount,
		TaskCount:    refs.TaskCount,
		RecordCount:  refs.RecordCount,
		AcceptCount:  refs.AcceptCount,
	}, nil
}

// CreateDistrict 新增片区。
func (s *Service) CreateDistrict(ctx context.Context, req SaveDistrictRequest) (*District, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, httpx.Validation("片区名称不能为空")
	}
	district := &District{}
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		exists, err := s.repo.DistrictExistsByName(ctx, tx, name, 0)
		if err != nil {
			return httpx.WrapInternal("校验片区名称失败", err)
		}
		if exists {
			return httpx.Conflict(fmt.Sprintf("片区 %s 已存在", name))
		}
		district.Name = name
		if err := s.repo.CreateDistrictTx(ctx, tx, district); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.Conflict(fmt.Sprintf("片区 %s 已存在", name))
			}
			return httpx.WrapInternal("新增片区失败", err)
		}
		return s.writeLog(ctx, tx, []ChangeLog{newLog(NodeTypeDistrict, district.ID, ActionCreateDistrict,
			name, nil, district, req.Operator, req.Reason)})
	})
	if err != nil {
		return nil, err
	}
	return district, nil
}

// CreateRoad 新增道路。
func (s *Service) CreateRoad(ctx context.Context, req SaveRoadRequest) (*Road, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, httpx.Validation("道路名称不能为空")
	}
	road := &Road{}
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		district, err := s.repo.FindDistrictForUpdate(ctx, tx, req.DistrictID, true)
		if err != nil {
			return notFound(err, "所属片区不存在")
		}
		exists, err := s.repo.RoadExistsByName(ctx, tx, district.ID, name, 0)
		if err != nil {
			return httpx.WrapInternal("校验道路名称失败", err)
		}
		if exists {
			return httpx.Conflict(fmt.Sprintf("片区 %s 下已存在道路 %s", district.Name, name))
		}
		road.DistrictID = district.ID
		road.Name = name
		if err := s.repo.CreateRoadTx(ctx, tx, road); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.Conflict(fmt.Sprintf("片区 %s 下已存在道路 %s", district.Name, name))
			}
			return httpx.WrapInternal("新增道路失败", err)
		}
		return s.writeLog(ctx, tx, []ChangeLog{newLog(NodeTypeRoad, road.ID, ActionCreateRoad,
			name, nil, road, req.Operator, req.Reason)})
	})
	if err != nil {
		return nil, err
	}
	return road, nil
}

// RenameDistrict 片区改名。
func (s *Service) RenameDistrict(ctx context.Context, id uint, req RenameDistrictRequest) (*District, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, httpx.Validation("片区名称不能为空")
	}
	district := &District{}
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		current, err := s.repo.FindDistrictForUpdate(ctx, tx, id, true)
		if err != nil {
			return notFound(err, "片区不存在")
		}
		before := *current
		if current.Name == name {
			district = current
			return nil
		}
		exists, err := s.repo.DistrictExistsByName(ctx, tx, name, id)
		if err != nil {
			return httpx.WrapInternal("校验片区名称失败", err)
		}
		if exists {
			return httpx.Conflict(fmt.Sprintf("片区 %s 已存在", name))
		}
		current.Name = name
		if err := s.repo.SaveDistrictTx(ctx, tx, current); err != nil {
			return httpx.WrapInternal("片区改名失败", err)
		}
		district = current
		return s.writeLog(ctx, tx, []ChangeLog{newLog(NodeTypeDistrict, id, ActionRenameDistrict,
			name, before, current, req.Operator, req.Reason)})
	})
	if err != nil {
		return nil, err
	}
	return district, nil
}

// RenameRoad 道路改名。
func (s *Service) RenameRoad(ctx context.Context, id uint, req RenameRoadRequest) (*Road, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, httpx.Validation("道路名称不能为空")
	}
	road := &Road{}
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		current, err := s.repo.FindRoadForUpdate(ctx, tx, id, true)
		if err != nil {
			return notFound(err, "道路不存在")
		}
		before := *current
		if current.Name == name {
			road = current
			return nil
		}
		exists, err := s.repo.RoadExistsByName(ctx, tx, current.DistrictID, name, id)
		if err != nil {
			return httpx.WrapInternal("校验道路名称失败", err)
		}
		if exists {
			return httpx.Conflict(fmt.Sprintf("该片区下已存在道路 %s", name))
		}
		current.Name = name
		if err := s.repo.SaveRoadTx(ctx, tx, current); err != nil {
			return httpx.WrapInternal("道路改名失败", err)
		}
		road = current
		return s.writeLog(ctx, tx, []ChangeLog{newLog(NodeTypeRoad, id, ActionRenameRoad,
			name, before, current, req.Operator, req.Reason)})
	})
	if err != nil {
		return nil, err
	}
	return road, nil
}

// MoveRoad 把道路调整到另一个片区，同时把引用该道路的管段归属到目标片区。
func (s *Service) MoveRoad(ctx context.Context, id uint, req MoveRoadRequest) (*Road, error) {
	road := &Road{}
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		current, err := s.lockRoad(ctx, tx, id)
		if err != nil {
			return notFound(err, "道路不存在")
		}
		target, err := s.lockDistrict(ctx, tx, req.DistrictID)
		if err != nil {
			return notFound(err, "目标片区不存在")
		}
		before := *current
		if current.DistrictID == target.ID {
			road = current
			return nil
		}
		// 同名冲突在调整期间也要拦住，避免移动后目标片区出现重名道路。
		exists, err := s.repo.RoadExistsByName(ctx, tx, target.ID, current.Name, id)
		if err != nil {
			return httpx.WrapInternal("校验道路名称失败", err)
		}
		if exists {
			return httpx.Conflict(fmt.Sprintf("目标片区 %s 下已存在同名道路 %s", target.Name, current.Name))
		}
		current.DistrictID = target.ID
		if err := s.repo.SaveRoadTx(ctx, tx, current); err != nil {
			return httpx.WrapInternal("调整道路归属失败", err)
		}
		// 管段归属随道路一起调整，保证台账、看板与层级保持同一套归属。
		if err := tx.WithContext(ctx).Table(refx.TablePipeSegments).
			Where("road_id = ?", id).
			Update("district_id", target.ID).Error; err != nil {
			return httpx.WrapInternal("调整管段归属失败", err)
		}
		road = current
		return s.writeLog(ctx, tx, []ChangeLog{newLog(NodeTypeRoad, id, ActionMoveRoad,
			current.Name, before, current, req.Operator, req.Reason)})
	})
	if err != nil {
		return nil, err
	}
	return road, nil
}

// DeleteDistrict 删除片区。被管段或历史业务快照引用的片区不允许直接删除。
//
// 片区下仍挂有道路时也拒绝删除，需先移走或删除道路。
func (s *Service) DeleteDistrict(ctx context.Context, id uint, req OperatorRequest) error {
	return s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		district, err := s.lockDistrict(ctx, tx, id)
		if err != nil {
			return notFound(err, "片区不存在")
		}
		roadCount := int64(0)
		if err := tx.WithContext(ctx).Model(&Road{}).Where("district_id = ?", id).Count(&roadCount).Error; err != nil {
			return httpx.WrapInternal("检查片区道路失败", err)
		}
		if roadCount > 0 {
			return httpx.Conflict(fmt.Sprintf("片区 %s 下还有 %d 条道路，请先移走或删除道路", district.Name, roadCount))
		}
		refs, err := refx.DistrictRefs(ctx, tx, id)
		if err != nil {
			return httpx.WrapInternal("检查片区引用失败", err)
		}
		if refs.Referenced() {
			return httpx.Conflict(referenceMessage("片区", district.Name, refs))
		}
		if err := s.repo.DeleteDistrictTx(ctx, tx, id); err != nil {
			return notFound(err, "片区不存在")
		}
		return s.writeLog(ctx, tx, []ChangeLog{newLog(NodeTypeDistrict, id, ActionDeleteDistrict,
			district.Name, district, nil, req.Operator, req.Reason)})
	})
}

// DeleteRoad 删除道路。被管段或历史业务快照引用的道路不允许直接删除。
func (s *Service) DeleteRoad(ctx context.Context, id uint, req OperatorRequest) error {
	return s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		road, err := s.lockRoad(ctx, tx, id)
		if err != nil {
			return notFound(err, "道路不存在")
		}
		refs, err := refx.RoadRefs(ctx, tx, id)
		if err != nil {
			return httpx.WrapInternal("检查道路引用失败", err)
		}
		if refs.Referenced() {
			return httpx.Conflict(referenceMessage("道路", road.Name, refs))
		}
		if err := s.repo.DeleteRoadTx(ctx, tx, id); err != nil {
			return notFound(err, "道路不存在")
		}
		return s.writeLog(ctx, tx, []ChangeLog{newLog(NodeTypeRoad, id, ActionDeleteRoad,
			road.Name, road, nil, req.Operator, req.Reason)})
	})
}

// Batch 批量调整层级（改名 / 归属调整）。
//
// 所有改动要么全部生效、要么整体回滚，不存在部分成功。
func (s *Service) Batch(ctx context.Context, req BatchRequest) (*BatchResponse, error) {
	if len(req.Districts) == 0 && len(req.Roads) == 0 {
		return nil, httpx.Validation("没有需要调整的层级项")
	}
	districtChanges := 0
	roadChanges := 0
	batchID := newBatchID()

	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		logs := make([]ChangeLog, 0, len(req.Districts)+len(req.Roads))

		// 按主键排序加锁，保证并发批次之间锁顺序一致，避免死锁。
		districtIDs := make([]uint, 0, len(req.Districts))
		for _, item := range req.Districts {
			districtIDs = append(districtIDs, item.ID)
		}
		lockedDistricts, err := s.lockDistricts(ctx, tx, districtIDs)
		if err != nil {
			return err
		}

		roadIDs := make([]uint, 0, len(req.Roads))
		for _, item := range req.Roads {
			roadIDs = append(roadIDs, item.ID)
			if item.DistrictID > 0 {
				districtIDs = append(districtIDs, item.DistrictID)
			}
		}
		// 归属调整的目标片区也要锁定。
		lockedDistricts, err = s.lockDistricts(ctx, tx, districtIDs)
		if err != nil {
			return err
		}
		lockedRoads, err := s.lockRoads(ctx, tx, roadIDs)
		if err != nil {
			return err
		}

		// 片区改名：批次内与存量名称都不能冲突。
		districtNames := make(map[string]uint)
		for _, district := range lockedDistricts {
			districtNames[district.Name] = district.ID
		}
		for _, item := range req.Districts {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue // 未给新名称则跳过，不算改动。
			}
			current := lockedDistricts[item.ID]
			if current.Name == name {
				continue
			}
			if owner, dup := districtNames[name]; dup && owner != item.ID {
				return httpx.Conflict(fmt.Sprintf("片区 %s 与批次内另一处改名冲突", name))
			}
			exists, err := s.repo.DistrictExistsByName(ctx, tx, name, item.ID)
			if err != nil {
				return httpx.WrapInternal("校验片区名称失败", err)
			}
			if exists {
				return httpx.Conflict(fmt.Sprintf("片区 %s 已存在", name))
			}
			before := *current
			current.Name = name
			if err := s.repo.SaveDistrictTx(ctx, tx, current); err != nil {
				return httpx.WrapInternal("片区改名失败", err)
			}
			districtNames[name] = item.ID
			delete(districtNames, before.Name)
			districtChanges++
			logs = append(logs, newLog(NodeTypeDistrict, item.ID, ActionRenameDistrict,
				name, before, current, req.Operator, req.Reason))
		}

		// 道路改名 / 归属调整。同名冲突由数据库唯一索引 + RoadExistsByName 双重保证。
		movedSegmentRoads := make(map[uint]uint) // roadID -> 目标片区，用于批量更新管段归属
		for _, item := range req.Roads {
			current := lockedRoads[item.ID]
			before := *current
			changed := false

			name := strings.TrimSpace(item.Name)
			if name != "" && current.Name != name {
				duplicate, err := s.repo.RoadExistsByName(ctx, tx, current.DistrictID, name, item.ID)
				if err != nil {
					return httpx.WrapInternal("校验道路名称失败", err)
				}
				if duplicate {
					return httpx.Conflict(fmt.Sprintf("片区下已存在道路 %s", name))
				}
				current.Name = name
				changed = true
			}
			if item.DistrictID > 0 && current.DistrictID != item.DistrictID {
				target := lockedDistricts[item.DistrictID]
				duplicate, err := s.repo.RoadExistsByName(ctx, tx, target.ID, current.Name, item.ID)
				if err != nil {
					return httpx.WrapInternal("校验道路名称失败", err)
				}
				if duplicate {
					return httpx.Conflict(fmt.Sprintf("目标片区 %s 下已存在同名道路 %s", target.Name, current.Name))
				}
				current.DistrictID = target.ID
				changed = true
				movedSegmentRoads[item.ID] = target.ID
			}
			if !changed {
				continue
			}
			if err := s.repo.SaveRoadTx(ctx, tx, current); err != nil {
				return httpx.WrapInternal("保存道路调整失败", err)
			}
			action := ActionRenameRoad
			if before.DistrictID != current.DistrictID {
				action = ActionMoveRoad
			}
			roadChanges++
			logs = append(logs, newLog(NodeTypeRoad, item.ID, action, current.Name, before, current,
				req.Operator, req.Reason))
		}

		// 道路归属调整后，引用这些道路的管段一并归到目标片区。
		for roadID, districtID := range movedSegmentRoads {
			if err := tx.WithContext(ctx).Table(refx.TablePipeSegments).
				Where("road_id = ?", roadID).
				Update("district_id", districtID).Error; err != nil {
				return httpx.WrapInternal("调整管段归属失败", err)
			}
		}

		for i := range logs {
			logs[i].BatchID = batchID
		}
		if err := s.repo.WriteLogsTx(ctx, tx, logs); err != nil {
			return httpx.WrapInternal("写入层级变更日志失败", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &BatchResponse{BatchID: batchID, DistrictChanges: districtChanges, RoadChanges: roadChanges}, nil
}

// Logs 分页查询层级变更日志。
func (s *Service) Logs(ctx context.Context, query LogListQuery) ([]ChangeLog, int64, error) {
	if query.Page.Page < 1 {
		query.Page.Page = 1
	}
	if query.Page.PageSize < 1 {
		query.Page.PageSize = 20
	}
	logs, total, err := s.repo.Logs(ctx, query)
	if err != nil {
		return nil, 0, httpx.WrapInternal("查询层级变更日志失败", err)
	}
	return logs, total, nil
}

// EnsureNode 校验片区 / 道路组合是否合法，供管段台账保存时调用。
//
// roadID 为 0 表示管段不挂道路；否则道路必须存在且归属于该片区。
func (s *Service) EnsureNode(ctx context.Context, districtID uint, roadID uint) error {
	if districtID == 0 {
		return httpx.Validation("请选择所属片区")
	}
	district, err := s.repo.FindDistrict(ctx, districtID)
	if err != nil {
		return notFound(err, "所属片区不存在，可能已被调整，请刷新后重试")
	}
	if roadID == 0 {
		return nil
	}
	road, err := s.repo.FindRoad(ctx, roadID)
	if err != nil {
		return notFound(err, "所在道路不存在，可能已被调整，请刷新后重试")
	}
	if road.DistrictID != district.ID {
		return httpx.Validation(fmt.Sprintf("道路 %s 不属于片区 %s，请重新选择", road.Name, district.Name))
	}
	return nil
}

// SnapshotInTx 在给定事务内读取管段当前层级，并锁定对应层级行与管段行，
// 供任务 / 记录 / 验收登记时固化"登记当时"的名称快照。
func (s *Service) SnapshotInTx(ctx context.Context, tx *gorm.DB, segmentID uint) (Snapshot, error) {
	snap := Snapshot{}

	// 先锁管段行：道路归属调整会更新管段 district_id，行锁让两类事务严格串行，
	// 正在录入的记录只会落在调整前或调整后其中一个确定口径上。
	var row struct {
		DistrictID uint
		RoadID     *uint
	}
	err := withForUpdate(tx.WithContext(ctx).Table(refx.TablePipeSegments)).
		Select("district_id, road_id").
		Where("id = ?", segmentID).
		Scan(&row).Error
	if err != nil {
		return snap, httpx.WrapInternal("读取管段层级失败", err)
	}
	if row.DistrictID == 0 {
		return snap, httpx.NotFound("管段不存在")
	}

	district, err := s.repo.FindDistrictForUpdate(ctx, tx, row.DistrictID, true)
	if err != nil {
		return snap, notFound(err, "片区不存在")
	}
	snap.DistrictID = district.ID
	snap.DistrictName = district.Name
	if row.RoadID != nil && *row.RoadID > 0 {
		road, err := s.repo.FindRoadForUpdate(ctx, tx, *row.RoadID, true)
		if err != nil {
			return snap, notFound(err, "道路不存在")
		}
		snap.RoadID = &road.ID
		snap.RoadName = road.Name
	}
	return snap, nil
}

// SnapshotTx 在单个事务内锁定层级快照并执行业务写入，供任务 / 记录登记使用。
//
// 回调收到的是标准库闭包参数，业务模块通过各自网关接口把 Snapshot 映射成本包类型，
// 从而在不依赖本模块的前提下复用同一套加锁口径。
func (s *Service) SnapshotTx(
	ctx context.Context,
	segmentID uint,
	fn func(tx *gorm.DB, snap Snapshot) error,
) error {
	return s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		snap, err := s.SnapshotInTx(ctx, tx, segmentID)
		if err != nil {
			return err
		}
		return fn(tx, snap)
	})
}

func (s *Service) lockDistrict(ctx context.Context, tx *gorm.DB, id uint) (*District, error) {
	return s.repo.FindDistrictForUpdate(ctx, tx, id, true)
}

func (s *Service) lockRoad(ctx context.Context, tx *gorm.DB, id uint) (*Road, error) {
	return s.repo.FindRoadForUpdate(ctx, tx, id, true)
}

// lockDistricts 一次性按 ID 排序锁定片区，去重后以 map 返回。
func (s *Service) lockDistricts(ctx context.Context, tx *gorm.DB, ids []uint) (map[uint]*District, error) {
	unique := uniqueSorted(ids)
	result := make(map[uint]*District, len(unique))
	for _, id := range unique {
		district, err := s.lockDistrict(ctx, tx, id)
		if err != nil {
			return nil, notFound(err, "片区不存在")
		}
		result[id] = district
	}
	return result, nil
}

// lockRoads 一次性按 ID 排序锁定道路，去重后以 map 返回。
func (s *Service) lockRoads(ctx context.Context, tx *gorm.DB, ids []uint) (map[uint]*Road, error) {
	unique := uniqueSorted(ids)
	result := make(map[uint]*Road, len(unique))
	for _, id := range unique {
		road, err := s.lockRoad(ctx, tx, id)
		if err != nil {
			return nil, notFound(err, "道路不存在")
		}
		result[id] = road
	}
	return result, nil
}

func (s *Service) writeLog(ctx context.Context, tx *gorm.DB, logs []ChangeLog) error {
	batchID := newBatchID()
	for i := range logs {
		if logs[i].BatchID == "" {
			logs[i].BatchID = batchID
		}
	}
	if err := s.repo.WriteLogsTx(ctx, tx, logs); err != nil {
		return httpx.WrapInternal("写入层级变更日志失败", err)
	}
	return nil
}

// OperatorRequest 删除等操作的操作人与原因。
type OperatorRequest struct {
	Reason   string `json:"reason" label:"调整原因" validate:"max=255"`
	Operator string `json:"operator" label:"操作人" validate:"max=64"`
}

func newLog(nodeType string, nodeID uint, action, name string, before, after any, operator, reason string) ChangeLog {
	return ChangeLog{
		NodeType: nodeType,
		NodeID:   nodeID,
		Action:   action,
		Name:     name,
		Before:   toJSON(before),
		After:    toJSON(after),
		Reason:   strings.TrimSpace(reason),
		Operator: strings.TrimSpace(operator),
	}
}

func toJSON(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func newBatchID() string {
	return "B" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func uniqueSorted(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	// 简单插入排序，数量通常很小。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func referenceMessage(kind, name string, refs refx.HierarchyRefs) string {
	parts := make([]string, 0, 4)
	if refs.SegmentCount > 0 {
		parts = append(parts, fmt.Sprintf("管段 %d 条", refs.SegmentCount))
	}
	if refs.TaskCount > 0 {
		parts = append(parts, fmt.Sprintf("清淤任务 %d 条", refs.TaskCount))
	}
	if refs.RecordCount > 0 {
		parts = append(parts, fmt.Sprintf("清淤记录 %d 条", refs.RecordCount))
	}
	if refs.AcceptCount > 0 {
		parts = append(parts, fmt.Sprintf("验收记录 %d 条", refs.AcceptCount))
	}
	return fmt.Sprintf("%s %s 已被%s引用，不允许直接删除；请先调整归属或处理相关业务数据",
		kind, name, strings.Join(parts, "、"))
}

func notFound(err error, message string) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound(message)
	}
	return httpx.WrapInternal(message, err)
}
