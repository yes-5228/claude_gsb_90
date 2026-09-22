// Package router 负责把各业务模块的路由装配到 Fiber 应用上。
package router

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/drainage/desilting/internal/config"
	"github.com/drainage/desilting/internal/httpx"
	"github.com/drainage/desilting/internal/modules/acceptance"
	"github.com/drainage/desilting/internal/modules/cleaningrecord"
	"github.com/drainage/desilting/internal/modules/cleaningtask"
	"github.com/drainage/desilting/internal/modules/dashboard"
	"github.com/drainage/desilting/internal/modules/hierarchy"
	"github.com/drainage/desilting/internal/modules/meta"
	"github.com/drainage/desilting/internal/modules/pipesegment"
)

// startedAt 记录进程启动时间，用于健康检查展示运行时长。
var startedAt = time.Now()

// Setup 注册健康检查与全部业务模块路由。
//
// 模块之间的依赖在这里显式装配：清淤记录依赖清淤任务，验收依赖任务、
// 清淤记录与管段台账，看板只读依赖全部模块。
func Setup(app *fiber.App, db *gorm.DB, cfg *config.Config) {
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return httpx.OK(c, fiber.Map{
			"status": "ok",
			"env":    cfg.AppEnv,
			"db":     cfg.Describe(),
			"uptime": time.Since(startedAt).Round(time.Second).String(),
		})
	})

	api := app.Group("/api/v1")
	meta.Register(api)

	// 层级先于台账注册：管段、任务、记录、验收共用同一套片区 / 道路层级。
	hierarchyService := hierarchy.Register(api, db)
	segmentService := pipesegment.Register(api, db, pipesegmentHierarchyGateway{svc: hierarchyService})
	taskService := cleaningtask.Register(api, db, segmentService, taskSnapshotGateway{svc: hierarchyService})
	recordService := cleaningrecord.Register(api, db, taskService, recordSnapshotGateway{svc: hierarchyService})
	acceptance.Register(api, db, taskService, segmentService, recordService, acceptanceSnapshotGateway{svc: hierarchyService})
	dashboard.Register(api, db)
}
