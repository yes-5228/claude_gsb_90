package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/httpx"
)

// Service 片区-道路层级业务逻辑。
type Service struct {
	repo *Repository
}

// NewService 构造服务。
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Tree 返回片区 -> 道路的层级树（含当前管段数量）。
func (s *Service) Tree(ctx context.Context) ([]DistrictNode, error) {
	districts, err := s.repo.ListDistricts(ctx)
	if err != nil {
		return nil, httpx.WrapInternal("查询片区失败", err)
	}
	roads, err := s.repo.ListRoads(ctx)
	if err != nil {
		return nil, httpx.WrapInternal("查询道路失败", err)
	}
	counts, err := s.repo.SegmentCountsByRoad(ctx)
	if err != nil {
		return nil, httpx.WrapInternal("统计道路管段数量失败", err)
	}

	roadsByDistrict := make(map[uint][]Road, len(districts))
	for _, road := range roads {
		roadsByDistrict[road.DistrictID] = append(roadsByDistrict[road.DistrictID], road)
	}

	nodes := make([]DistrictNode, 0, len(districts))
	for _, d := range districts {
		node := DistrictNode{ID: d.ID, Name: d.Name, SortOrder: d.SortOrder, Roads: make([]RoadNode, 0)}
		for _, road := range roadsByDistrict[d.ID] {
			node.Roads = append(node.Roads, RoadNode{
				ID:           road.ID,
				Name:         road.Name,
				DistrictID:   road.DistrictID,
				SortOrder:    road.SortOrder,
				SegmentCount: counts[road.ID],
			})
			node.RoadCount++
			node.SegmentCount += counts[road.ID]
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// CreateDistrict 新增片区并留痕。
func (s *Service) CreateDistrict(ctx context.Context, req SaveDistrictRequest) (*District, error) {
	name := clean(req.Name)
	if name == "" {
		return nil, httpx.Validation("片区名称不能为空")
	}

	var created *District
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		if dup, ok, err := s.repo.FindDistrictByName(ctx, tx, name, 0); err != nil {
			return httpx.WrapInternal("校验片区名称失败", err)
		} else if ok {
			return httpx.Conflict(fmt.Sprintf("片区「%s」已存在", dup.Name))
		}
		d := &District{Name: name}
		if err := s.repo.CreateDistrict(ctx, tx, d); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.Conflict(fmt.Sprintf("片区「%s」已存在", name))
			}
			return httpx.WrapInternal("新增片区失败", err)
		}
		if err := s.repo.AddLogs(ctx, tx, []*HierarchyChangeLog{
			s.log(NodeTypeDistrict, d.ID, d.Name, ActionCreate, "", "新增片区", clean(req.Operator), clean(req.Remark)),
		}); err != nil {
			return httpx.WrapInternal("写入层级变更记录失败", err)
		}
		created = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// RenameDistrict 片区改名并留痕。
func (s *Service) RenameDistrict(ctx context.Context, id uint, req RenameDistrictRequest) (*District, error) {
	name := clean(req.Name)
	if name == "" {
		return nil, httpx.Validation("片区名称不能为空")
	}

	var updated *District
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		d, err := s.repo.FindDistrict(ctx, tx, id)
		if err != nil {
			return notFound(err, "片区不存在")
		}
		if dup, ok, err := s.repo.FindDistrictByName(ctx, tx, name, id); err != nil {
			return httpx.WrapInternal("校验片区名称失败", err)
		} else if ok {
			return httpx.Conflict(fmt.Sprintf("片区「%s」已存在", dup.Name))
		}
		from := d.Name
		d.Name = name
		if err := s.repo.SaveDistrict(ctx, tx, d); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.Conflict(fmt.Sprintf("片区「%s」已存在", name))
			}
			return httpx.WrapInternal("片区改名失败", err)
		}
		if err := s.repo.AddLogs(ctx, tx, []*HierarchyChangeLog{
			s.log(NodeTypeDistrict, d.ID, name, ActionRename, from, name, clean(req.Operator), clean(req.Remark)),
		}); err != nil {
			return httpx.WrapInternal("写入层级变更记录失败", err)
		}
		updated = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteDistrict 删除片区。其下还有道路的片区不允许删除。
func (s *Service) DeleteDistrict(ctx context.Context, id uint, req DeleteRequest) error {
	return s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		d, err := s.repo.FindDistrict(ctx, tx, id)
		if err != nil {
			return notFound(err, "片区不存在")
		}
		roadCount, err := s.repo.CountRoadsIn(ctx, tx, id)
		if err != nil {
			return httpx.WrapInternal("检查片区下道路失败", err)
		}
		if roadCount > 0 {
			return httpx.Conflict(fmt.Sprintf("片区「%s」下仍有 %d 条道路，请先调整或删除道路后再删除片区", d.Name, roadCount))
		}
		if err := s.repo.DeleteDistrict(ctx, tx, id); err != nil {
			return notFound(err, "片区不存在")
		}
		if err := s.repo.AddLogs(ctx, tx, []*HierarchyChangeLog{
			s.log(NodeTypeDistrict, id, d.Name, ActionDelete, d.Name, "", clean(req.Operator), clean(req.Remark)),
		}); err != nil {
			return httpx.WrapInternal("写入层级变更记录失败", err)
		}
		return nil
	})
}

// CreateRoad 在指定片区下新增道路并留痕。
func (s *Service) CreateRoad(ctx context.Context, req SaveRoadRequest) (*Road, error) {
	name := clean(req.Name)
	if name == "" {
		return nil, httpx.Validation("道路名称不能为空")
	}
	if req.DistrictID == 0 {
		return nil, httpx.Validation("请选择所属片区")
	}

	var created *Road
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		d, err := s.repo.FindDistrict(ctx, tx, req.DistrictID)
		if err != nil {
			return notFound(err, "所属片区不存在")
		}
		if exists, err := s.repo.RoadExistsWithinDistrict(ctx, tx, d.ID, name, 0); err != nil {
			return httpx.WrapInternal("校验道路名称失败", err)
		} else if exists {
			return httpx.Conflict(fmt.Sprintf("片区「%s」下已存在道路「%s」", d.Name, name))
		}
		road := &Road{Name: name, DistrictID: d.ID}
		if err := s.repo.CreateRoad(ctx, tx, road); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.Conflict(fmt.Sprintf("片区「%s」下已存在道路「%s」", d.Name, name))
			}
			return httpx.WrapInternal("新增道路失败", err)
		}
		if err := s.repo.AddLogs(ctx, tx, []*HierarchyChangeLog{
			s.log(NodeTypeRoad, road.ID, road.Name, ActionCreate, "", fmt.Sprintf("归属片区：%s", d.Name), clean(req.Operator), clean(req.Remark)),
		}); err != nil {
			return httpx.WrapInternal("写入层级变更记录失败", err)
		}
		created = road
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// RenameRoad 道路改名并留痕。
func (s *Service) RenameRoad(ctx context.Context, id uint, req RenameRoadRequest) (*Road, error) {
	name := clean(req.Name)
	if name == "" {
		return nil, httpx.Validation("道路名称不能为空")
	}

	var updated *Road
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		road, err := s.repo.FindRoad(ctx, tx, id)
		if err != nil {
			return notFound(err, "道路不存在")
		}
		if exists, err := s.repo.RoadExistsWithinDistrict(ctx, tx, road.DistrictID, name, id); err != nil {
			return httpx.WrapInternal("校验道路名称失败", err)
		} else if exists {
			return httpx.Conflict(fmt.Sprintf("该片区下已存在道路「%s」", name))
		}
		from := road.Name
		road.Name = name
		if err := s.repo.SaveRoad(ctx, tx, road); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return httpx.Conflict(fmt.Sprintf("该片区下已存在道路「%s」", name))
			}
			return httpx.WrapInternal("道路改名失败", err)
		}
		if err := s.repo.AddLogs(ctx, tx, []*HierarchyChangeLog{
			s.log(NodeTypeRoad, road.ID, name, ActionRename, from, name, clean(req.Operator), clean(req.Remark)),
		}); err != nil {
			return httpx.WrapInternal("写入层级变更记录失败", err)
		}
		updated = road
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// MoveRoad 把单条道路调整到目标片区。
func (s *Service) MoveRoad(ctx context.Context, roadID, targetDistrictID uint, req MoveRoadRequest) error {
	return s.BatchMoveRoads(ctx, BatchMoveRoadsRequest{
		Items:    []RoadMoveItem{{RoadID: roadID, DistrictID: targetDistrictID}},
		Operator: req.Operator,
		Remark:   req.Remark,
	})
}

// BatchMoveRoads 批量调整道路归属。
//
// 整组操作在同一个事务内执行：先在事务内完成全部前置校验（道路存在、目标片区存在、
// 非同片区、目标片区内不重名），全部通过后才落库并写变更记录；任一条不合法则整批
// 回滚，绝不会出现部分成功。
func (s *Service) BatchMoveRoads(ctx context.Context, req BatchMoveRoadsRequest) error {
	if len(req.Items) == 0 {
		return httpx.Validation("请选择需要调整归属的道路")
	}

	// 同一次请求内不允许把同一条道路调整到两个不同片区。
	seen := make(map[uint]uint, len(req.Items))
	for _, item := range req.Items {
		if item.RoadID == 0 {
			return httpx.Validation("道路 ID 不合法")
		}
		if item.DistrictID == 0 {
			return httpx.Validation("目标片区不能为空")
		}
		if prev, dup := seen[item.RoadID]; dup && prev != item.DistrictID {
			return httpx.Validation(fmt.Sprintf("道路 #%d 在同一次调整中出现了不同的目标片区", item.RoadID))
		}
		seen[item.RoadID] = item.DistrictID
	}
	operator := clean(req.Operator)
	remark := clean(req.Remark)
	batchID := newBatchID()

	return s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		// 先锁定并读取全部待调整道路，收集来源与目标片区。
		roads := make([]*Road, len(req.Items))
		districtIDSet := make(map[uint]struct{}, len(req.Items)+1)
		for i, item := range req.Items {
			road, err := s.repo.FindRoadForUpdate(ctx, tx, item.RoadID)
			if err != nil {
				return notFound(err, fmt.Sprintf("道路 #%d 不存在", item.RoadID))
			}
			roads[i] = road
			districtIDSet[road.DistrictID] = struct{}{}
			districtIDSet[item.DistrictID] = struct{}{}
		}
		districtIDs := make([]uint, 0, len(districtIDSet))
		for id := range districtIDSet {
			districtIDs = append(districtIDs, id)
		}
		districts, err := s.repo.DistrictsByIDs(ctx, tx, districtIDs)
		if err != nil {
			return httpx.WrapInternal("查询片区失败", err)
		}
		for id := range districtIDSet {
			if _, ok := districts[id]; !ok {
				return httpx.NotFound(fmt.Sprintf("片区 #%d 不存在", id))
			}
		}

		logs := make([]*HierarchyChangeLog, 0, len(req.Items))
		for i, item := range req.Items {
			road := roads[i]
			if road.DistrictID == item.DistrictID {
				// 已在目标片区下，无需调整，跳过（不阻断整批）。
				continue
			}
			target := districts[item.DistrictID]
			if exists, err := s.repo.RoadExistsWithinDistrict(ctx, tx, target.ID, road.Name, road.ID); err != nil {
				return httpx.WrapInternal("校验道路名称失败", err)
			} else if exists {
				return httpx.Conflict(fmt.Sprintf(
					"道路「%s」调整到片区「%s」后会与已有道路重名，请先改名再调整归属", road.Name, target.Name))
			}
			fromDistrict := districts[road.DistrictID]
			entry := s.log(NodeTypeRoad, road.ID, road.Name, ActionMove,
				fromDistrict.Name, target.Name, operator, remark)
			entry.BatchID = batchID
			logs = append(logs, entry)
			road.DistrictID = target.ID
			if err := s.repo.SaveRoad(ctx, tx, road); err != nil {
				return httpx.WrapInternal("调整道路归属失败", err)
			}
		}
		if err := s.repo.AddLogs(ctx, tx, logs); err != nil {
			return httpx.WrapInternal("写入层级变更记录失败", err)
		}
		return nil
	})
}

// DeleteRoad 删除道路。被当前管段台账引用的道路不允许删除。
//
// 历史任务 / 记录按登记当时的层级快照展示，不作为删除阻断条件；一旦道路被删除，
// 后续新管段不能再选择该道路，从而保证“被引用的层级不允许直接删除”。
func (s *Service) DeleteRoad(ctx context.Context, id uint, req DeleteRequest) error {
	return s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		road, err := s.repo.FindRoad(ctx, tx, id)
		if err != nil {
			return notFound(err, "道路不存在")
		}
		districts, err := s.repo.DistrictsByIDs(ctx, tx, []uint{road.DistrictID})
		if err != nil {
			return httpx.WrapInternal("查询所属片区失败", err)
		}
		count, err := s.repo.CountSegmentsOnRoad(ctx, tx, id)
		if err != nil {
			return httpx.WrapInternal("检查道路引用失败", err)
		}
		if count > 0 {
			return httpx.Conflict(fmt.Sprintf("道路「%s」下仍有 %d 个管段，不允许删除；如需调整请先变更管段归属道路", road.Name, count))
		}
		if err := s.repo.DeleteRoad(ctx, tx, id); err != nil {
			return notFound(err, "道路不存在")
		}
		districtName := districts[road.DistrictID].Name
		if err := s.repo.AddLogs(ctx, tx, []*HierarchyChangeLog{
			s.log(NodeTypeRoad, id, road.Name, ActionDelete,
				fmt.Sprintf("%s / %s", districtName, road.Name), "", clean(req.Operator), clean(req.Remark)),
		}); err != nil {
			return httpx.WrapInternal("写入层级变更记录失败", err)
		}
		return nil
	})
}

// RoadInfo 在事务（或独立连接）中按当前层级解析道路的完整归属，供任务 / 记录
// 登记时固化层级快照使用。
//
// 传入 tx 且底层为 PostgreSQL 时会对道路加行级锁：层级归属调整与业务录入并发时，
// 二者在道路行上串行化，事务先提交者确定归属口径，录入中的记录必然归属到
// “调整前”或“调整后”其中一个确定的层级，不会出现中间态。
func (s *Service) RoadInfo(ctx context.Context, tx *gorm.DB, roadID uint) (Info, error) {
	var (
		road *Road
		err  error
	)
	if tx != nil {
		road, err = s.repo.FindRoadForUpdate(ctx, tx, roadID)
	} else {
		road, err = s.repo.FindRoad(ctx, nil, roadID)
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Info{}, httpx.Validation("所选道路不存在，请刷新层级后重试")
		}
		return Info{}, httpx.WrapInternal("查询道路失败", err)
	}
	districts, err := s.repo.DistrictsByIDs(ctx, tx, []uint{road.DistrictID})
	if err != nil {
		return Info{}, httpx.WrapInternal("查询所属片区失败", err)
	}
	d := districts[road.DistrictID]
	return Info{
		RoadID:       road.ID,
		RoadName:     road.Name,
		DistrictID:   d.ID,
		DistrictName: d.Name,
	}, nil
}

// EnsureRoad 校验道路存在并属于（可选的）指定片区，供管段保存时使用。
func (s *Service) EnsureRoad(ctx context.Context, roadID, districtID uint) (*Info, error) {
	if roadID == 0 {
		return nil, httpx.Validation("请选择所属道路")
	}
	info, err := s.RoadInfo(ctx, nil, roadID)
	if err != nil {
		return nil, err
	}
	if districtID > 0 && info.DistrictID != districtID {
		return nil, httpx.Validation("所选道路不属于该片区，请重新选择")
	}
	return &info, nil
}

// ChangeLogs 分页查询层级变更记录。
func (s *Service) ChangeLogs(ctx context.Context, query ChangeLogQuery) ([]HierarchyChangeLog, int64, error) {
	items, total, err := s.repo.ListLogs(ctx, query)
	if err != nil {
		return nil, 0, httpx.WrapInternal("查询层级变更记录失败", err)
	}
	return items, total, nil
}

func (s *Service) log(nodeType string, nodeID uint, nodeName, action, from, to, operator, remark string) *HierarchyChangeLog {
	return &HierarchyChangeLog{
		BatchID:   newBatchID(),
		NodeType:  nodeType,
		NodeID:    nodeID,
		NodeName:  nodeName,
		Action:    action,
		FromValue: from,
		ToValue:   to,
		Operator:  operator,
		Remark:    remark,
		CreatedAt: time.Now(),
	}
}

func newBatchID() string {
	return "BC" + time.Now().Format("20060102150405.000000")
}

func notFound(err error, msg string) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound(msg)
	}
	return httpx.WrapInternal(msg, err)
}
