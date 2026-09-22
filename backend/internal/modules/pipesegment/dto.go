package pipesegment

import (
	"github.com/gofiber/fiber/v2"

	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/shared/refx"
)

// SaveRequest 新增或修改管段的请求体。
type SaveRequest struct {
	Code         string  `json:"code" label:"管段编号" validate:"required,min=2,max=64"`
	Name         string  `json:"name" label:"管段名称" validate:"required,max=128"`
	RoadID       uint    `json:"roadId" label:"所属道路" validate:"required"`
	PipeType     string  `json:"pipeType" label:"管段类型" validate:"required"`
	Material     string  `json:"material" label:"管材" validate:"max=32"`
	DiameterMm   int     `json:"diameterMm" label:"管径(mm)" validate:"gt=0,lte=5000"`
	LengthM      float64 `json:"lengthM" label:"管段长度(m)" validate:"gt=0,lte=100000"`
	DepthM       float64 `json:"depthM" label:"埋深(m)" validate:"gte=0,lte=50"`
	StartManhole string  `json:"startManhole" label:"起始检查井" validate:"max=64"`
	EndManhole   string  `json:"endManhole" label:"终点检查井" validate:"max=64"`
	BuildYear    int     `json:"buildYear" label:"建设年份" validate:"omitempty,gte=1900,lte=2100"`
	OwnerUnit    string  `json:"ownerUnit" label:"权属单位" validate:"max=128"`
	Status       string  `json:"status" label:"运行状态"`
	Remark       string  `json:"remark" label:"备注" validate:"max=1000"`
}

// ListQuery 管段列表查询条件。DistrictIDs / RoadIDs 为层级多选，
// 选了片区时按其下道路过滤，两个条件取交集。
type ListQuery struct {
	Keyword     string
	DistrictIDs []uint
	RoadIDs     []uint
	PipeType    string
	Status      string
	Page        httpx.PageQuery
}

// ParseListQuery 从请求 query 中解析列表查询条件。
func ParseListQuery(c *fiber.Ctx) ListQuery {
	return ListQuery{
		Keyword:     httpx.TrimmedQuery(c, "keyword"),
		DistrictIDs: httpx.UintIDsQuery(c, "districtIds"),
		RoadIDs:     httpx.UintIDsQuery(c, "roadIds"),
		PipeType:    httpx.TrimmedQuery(c, "pipeType"),
		Status:      httpx.TrimmedQuery(c, "status"),
		Page:        httpx.ParsePage(c),
	}
}

// DetailResponse 管段详情：基础档案 + 任务统计 + 最近任务。
type DetailResponse struct {
	Segment     *PipeSegment    `json:"segment"`
	Hierarchy   *hierarchy.Info `json:"hierarchy"`
	TaskStats   refx.TaskStats  `json:"taskStats"`
	RecentTasks []refx.TaskRef  `json:"recentTasks"`
}

// OptionsResponse 下拉选项：管段列表（带层级），层级树改由 /hierarchy/tree 获取。
type OptionsResponse struct {
	Items []Brief `json:"items"`
}
