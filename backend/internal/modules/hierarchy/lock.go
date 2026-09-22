package hierarchy

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// usingSQLite 标记当前连接是否为 SQLite：SQLite 不支持 FOR UPDATE，
// 其写事务天然串行执行，无需行锁。
var usingSQLite bool

// SetDialector 由 service 初始化时根据连接类型设定锁行为。
func SetDialector(db *gorm.DB) {
	_, usingSQLite = db.Dialector.(*sqlite.Dialector)
}

// withForUpdate 给查询追加 SELECT ... FOR UPDATE 行锁（SQLite 下原样返回）。
//
// 生产环境使用 PostgreSQL，调整事务通过行锁与并发登记任务 / 记录时读取层级快照的
// 事务串行化，保证调整期间正在录入的数据只会按"调整前"或"调整后"其中一个确定口径
// 归属，不会读到新旧混杂的名称。
func withForUpdate(tx *gorm.DB) *gorm.DB {
	if usingSQLite {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}
