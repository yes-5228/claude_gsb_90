// Package hierarchy 片区 / 道路层级模块。
//
// 片区（District）与道路（Road）组成两级层级，是管段台账、任务列表、记录列表、
// 验收列表与运行看板共用的唯一层级来源：
//
//   - 管段通过 district_id / road_id 引用层级，不再各自填写自由文本；
//   - 任务、清淤记录、验收记录在登记时固化一份层级名称快照，
//     层级后续调整时，历史数据仍按登记当时的名称展示；
//   - 新增、改名、归属调整、删除全部收敛在本模块，并逐条写入变更日志；
//   - 一次调整请求内的所有改动在同一个事务内批量生效，要么全部成功要么全部回滚。
package hierarchy

import "time"

// 层级节点类型。
const (
	NodeTypeDistrict = "district" // 片区
	NodeTypeRoad     = "road"     // 道路（归属某个片区）
)

// 变更动作。
const (
	ActionCreateDistrict = "create_district" // 新增片区
	ActionCreateRoad     = "create_road"     // 新增道路
	ActionRenameDistrict = "rename_district" // 片区改名
	ActionRenameRoad     = "rename_road"     // 道路改名
	ActionMoveRoad       = "move_road"       // 道路归属调整
	ActionDeleteDistrict = "delete_district" // 删除片区
	ActionDeleteRoad     = "delete_road"     // 删除道路
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

// Road 道路，归属于一个片区。
type Road struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DistrictID uint      `gorm:"index;uniqueIndex:idx_road_district_name;not null" json:"districtId"`
	Name       string    `gorm:"size:128;uniqueIndex:idx_road_district_name;not null" json:"name"`
	SortOrder  int       `gorm:"not null;default:0" json:"sortOrder"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TableName 指定表名。
func (Road) TableName() string {
	return "roads"
}

// ChangeLog 层级变更日志：新增、改名、归属调整、删除都会逐条记录。
//
// 一个批量调整请求共用同一个 BatchID，便于回溯"这次操作一共改了哪些节点"。
// Before/After 存节点当时的 JSON 快照，即使节点后续被删除，日志仍可独立阅读。
type ChangeLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BatchID   string    `gorm:"size:40;index;not null" json:"batchId"`
	NodeType  string    `gorm:"size:16;index;not null" json:"nodeType"`
	NodeID    uint      `gorm:"index;not null" json:"nodeId"`
	Action    string    `gorm:"size:32;index;not null" json:"action"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Before    string    `gorm:"type:text" json:"before"`
	After     string    `gorm:"type:text" json:"after"`
	Reason    string    `gorm:"size:255" json:"reason"`
	Operator  string    `gorm:"size:64" json:"operator"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

// TableName 指定表名。
func (ChangeLog) TableName() string {
	return "hierarchy_change_logs"
}
