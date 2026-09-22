package hierarchy

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/drainage/desilting/internal/httpx"
)

// SaveDistrictRequest 新增片区请求体。
type SaveDistrictRequest struct {
	Name     string `json:"name" label:"片区名称" validate:"required,max=64"`
	Operator string `json:"operator" validate:"max=64"`
	Remark   string `json:"remark" validate:"max=255"`
}

// RenameDistrictRequest 片区改名请求体。
type RenameDistrictRequest struct {
	Name     string `json:"name" label:"片区名称" validate:"required,max=64"`
	Operator string `json:"operator" validate:"max=64"`
	Remark   string `json:"remark" validate:"max=255"`
}

// SaveRoadRequest 新增道路请求体。
type SaveRoadRequest struct {
	Name       string `json:"name" label:"道路名称" validate:"required,max=128"`
	DistrictID uint   `json:"districtId" label:"所属片区" validate:"required"`
	Operator   string `json:"operator" validate:"max=64"`
	Remark     string `json:"remark" validate:"max:255"`
}

// RenameRoadRequest 道路改名请求体。
type RenameRoadRequest struct {
	Name     string `json:"name" label:"道路名称" validate:"required,max=128"`
	Operator string `json:"operator" validate:"max=64"`
	Remark   string `json:"remark" validate:"max=255"`
}

// MoveRoadRequest 道路归属调整请求体。
type MoveRoadRequest struct {
	DistrictID uint   `json:"districtId" label:"目标片区" validate:"required"`
	Operator   string `json:"operator" validate:"max=64"`
	Remark     string `json:"remark" validate:"max=255"`
}

// RoadMoveItem 批量归属调整中的单条道路。
type RoadMoveItem struct {
	RoadID     uint `json:"roadId" validate:"required"`
	DistrictID uint `json:"districtId" validate:"required"`
}

// BatchMoveRoadsRequest 批量调整道路归属。
//
// 整组操作在一个事务内完成，任一道路或目标片区不合法则整批回滚，不会部分成功。
type BatchMoveRoadsRequest struct {
	Items    []RoadMoveItem `json:"items" validate:"required,min=1,max=500,dive"`
	Operator string         `json:"operator" validate:"max=64"`
	Remark   string         `json:"remark" validate:"max=255"`
}

// DeleteRequest 删除层级节点请求体（删除人/备注，便于留痕）。
type DeleteRequest struct {
	Operator string `json:"operator" validate:"max=64"`
	Remark   string `json:"remark" validate:"max=255"`
}

// ChangeLogQuery 变更记录查询条件。
type ChangeLogQuery struct {
	NodeType string
	NodeID   uint
	Action   string
	BatchID  string
	Page     httpx.PageQuery
}

// ParseChangeLogQuery 解析变更记录查询条件。
func ParseChangeLogQuery(c *fiber.Ctx) ChangeLogQuery {
	return ChangeLogQuery{
		NodeType: httpx.TrimmedQuery(c, "nodeType"),
		NodeID:   uint(c.QueryInt("nodeId", 0)),
		Action:   httpx.TrimmedQuery(c, "action"),
		BatchID:  httpx.TrimmedQuery(c, "batchId"),
		Page:     httpx.ParsePage(c),
	}
}

func clean(v string) string {
	return strings.TrimSpace(v)
}
