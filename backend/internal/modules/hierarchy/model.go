// Package hierarchy 片区-道路层级模块：统一维护片区与道路两级层级，
// 管段台账、清淤任务、清淤记录与看板共用同一套层级与名称。
//
// 设计要点：
//   - 层级是片区 -> 道路的两级结构，管段挂在道路上（pipe_segments.road_id）。
//   - 层级的新增、改名、归属调整、删除都会写入变更记录（HierarchyChangeLog）。
//   - 被管段引用的道路、其下还有道路的片区不允许删除。
//   - 任务与记录在登记当时固化层级快照（名称与 ID），层级调整后历史数据仍按
//     登记当时的层级展示；看板与台账始终展示当前层级。
package hierarchy

import "time"

// 层级节点类型。
const (
	NodeTypeDistrict = "district" // 片区
	NodeTypeRoad     = "road"     // 道路
)

// 变更动作。
const (
	ActionCreate = "create" // 新增
	ActionRename = "rename" // 改名
	ActionMove   = "move"   // 归属调整
	ActionDelete = "delete" // 删除
)

// District 片区。
type District struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	SortOrder int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 指定表名。
func (District) TableName() string {
	return "districts"
}

// Road 道路，归属于某个片区。
type Road struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:128;not null;uniqueIndex:idx_road_district_name" json:"name"`
	DistrictID uint      `gorm:"not null;index;uniqueIndex:idx_road_district_name" json:"districtId"`
	SortOrder  int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TableName 指定表名。同一片区下道路名称唯一，不同片区允许重名。
func (Road) TableName() string {
	return "roads"
}

// HierarchyChangeLog 层级变更记录：新增、改名、归属调整、删除都在此留痕。
//
// 同一次批量调整共用一个 BatchID，便于把“一次性生效”的整组操作关联起来。
type HierarchyChangeLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BatchID   string    `gorm:"size:40;index;not null" json:"batchId"`
	NodeType  string    `gorm:"size:16;index;not null" json:"nodeType"`
	NodeID    uint      `gorm:"index;not null" json:"nodeId"`
	NodeName  string    `gorm:"size:128;not null" json:"nodeName"`
	Action    string    `gorm:"size:16;index;not null" json:"action"`
	FromValue string    `gorm:"size:255" json:"fromValue"`
	ToValue   string    `gorm:"size:255" json:"toValue"`
	Operator  string    `gorm:"size:64" json:"operator"`
	Remark    string    `gorm:"size:255" json:"remark"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

// TableName 指定表名。
func (HierarchyChangeLog) TableName() string {
	return "hierarchy_change_logs"
}

// RoadNode 树形结构中的道路节点。
type RoadNode struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	DistrictID   uint   `json:"districtId"`
	SortOrder    int    `json:"sortOrder"`
	SegmentCount int64  `json:"segmentCount"`
}

// DistrictNode 树形结构中的片区节点，携带其下道路。
type DistrictNode struct {
	ID           uint       `json:"id"`
	Name         string     `json:"name"`
	SortOrder    int        `json:"sortOrder"`
	RoadCount    int64      `json:"roadCount"`
	SegmentCount int64      `json:"segmentCount"`
	Roads        []RoadNode `json:"roads"`
}

// Info 道路的完整层级信息（道路 + 所属片区），供其他模块解析 roadId 使用。
type Info struct {
	RoadID       uint   `json:"roadId"`
	RoadName     string `json:"roadName"`
	DistrictID   uint   `json:"districtId"`
	DistrictName string `json:"districtName"`
}
