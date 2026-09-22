package hierarchy

import "github.com/gofiber/fiber/v2"

// DistrictNode 层级树中的片区节点（含下属道路）。
type DistrictNode struct {
	District
	Roads []Road `json:"roads"`
}

// TreeResponse 层级树。
type TreeResponse struct {
	Districts []DistrictNode `json:"districts"`
}

// RefCounts 节点被业务数据引用的计数，用于删除保护与前端提示。
type RefCounts struct {
	SegmentCount int64 `json:"segmentCount"`
	TaskCount    int64 `json:"taskCount"`
	RecordCount  int64 `json:"recordCount"`
	AcceptCount  int64 `json:"acceptanceCount"`
}

// Referenced 任一业务引用计数大于 0 即视为被引用，不允许直接删除。
func (r RefCounts) Referenced() bool {
	return r.SegmentCount > 0 || r.TaskCount > 0 || r.RecordCount > 0 || r.AcceptCount > 0
}

// Snapshot 层级名称快照。任务 / 清淤记录 / 验收记录在登记当时固化，
// 之后片区改名或道路归属调整都不影响历史数据的展示。
//
// RoadID 为 nil 表示登记当时该管段只挂了片区、未挂道路。
type Snapshot struct {
	DistrictID   uint   `json:"-"`
	DistrictName string `json:"districtName"`
	RoadID       *uint  `json:"-"`
	RoadName     string `json:"roadName"`
}

// SaveDistrictRequest 新增片区。
type SaveDistrictRequest struct {
	Name     string `json:"name" label:"片区名称" validate:"required,max=64"`
	Reason   string `json:"reason" label:"调整原因" validate:"max=255"`
	Operator string `json:"operator" label:"操作人" validate:"max=64"`
}

// SaveRoadRequest 新增道路。
type SaveRoadRequest struct {
	DistrictID uint   `json:"districtId" label:"所属片区" validate:"required"`
	Name       string `json:"name" label:"道路名称" validate:"required,max=128"`
	Reason     string `json:"reason" label:"调整原因" validate:"max=255"`
	Operator   string `json:"operator" label:"操作人" validate:"max=64"`
}

// RenameRoadRequest 道路改名。
type RenameRoadRequest struct {
	Name     string `json:"name" label:"道路名称" validate:"required,max=128"`
	Reason   string `json:"reason" label:"调整原因" validate:"max=255"`
	Operator string `json:"operator" label:"操作人" validate:"max=64"`
}

// RenameDistrictRequest 片区改名。
type RenameDistrictRequest struct {
	Name     string `json:"name" label:"片区名称" validate:"required,max=64"`
	Reason   string `json:"reason" label:"调整原因" validate:"max=255"`
	Operator string `json:"operator" label:"操作人" validate:"max=64"`
}

// MoveRoadRequest 道路归属调整（挂到另一个片区）。
type MoveRoadRequest struct {
	DistrictID uint   `json:"districtId" label:"目标片区" validate:"required"`
	Reason     string `json:"reason" label:"调整原因" validate:"max=255"`
	Operator   string `json:"operator" label:"操作人" validate:"max=64"`
}

// BatchRequest 批量层级调整。
//
// 所有改动在同一个事务内提交：先做完全部前置校验（含同名冲突、引用保护），
// 再逐条落库并写变更日志。任何一条不合法都整体回滚，不会出现部分成功。
//
// 同一节点在一次批次中应只出现一次；name 非空时改名，districtId 非 0 时调整归属，
// 可同时给出以实现"改名 + 归属调整"。
type BatchRequest struct {
	Reason    string          `json:"reason" label:"调整原因" validate:"max=255"`
	Operator  string          `json:"operator" label:"操作人" validate:"max=64"`
	Districts []BatchDistrict `json:"districts" validate:"dive"`
	Roads     []BatchRoad     `json:"roads" validate:"dive"`
}

// BatchDistrict 批量调整中的片区项（目前支持改名）。
type BatchDistrict struct {
	ID   uint   `json:"id" validate:"required"`
	Name string `json:"name" label:"片区名称" validate:"max=64"`
}

// BatchRoad 批量调整中的道路项（改名 / 归属调整）。
type BatchRoad struct {
	ID         uint   `json:"id" validate:"required"`
	Name       string `json:"name" label:"道路名称" validate:"max=128"`
	DistrictID uint   `json:"districtId"`
}

// BatchResponse 批量调整结果。
type BatchResponse struct {
	BatchID         string `json:"batchId"`
	DistrictChanges int    `json:"districtChanges"`
	RoadChanges     int    `json:"roadChanges"`
}

// LogListQuery 变更日志查询条件。
type LogListQuery struct {
	NodeType string
	Action   string
	BatchID  string
	Page     struct {
		Page     int
		PageSize int
	}
}

// ParseLogListQuery 解析变更日志查询条件。
func ParseLogListQuery(c *fiber.Ctx) LogListQuery {
	query := LogListQuery{
		NodeType: c.Query("nodeType"),
		Action:   c.Query("action"),
		BatchID:  c.Query("batchId"),
	}
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := c.QueryInt("pageSize", 20)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query.Page.Page = page
	query.Page.PageSize = pageSize
	return query
}
