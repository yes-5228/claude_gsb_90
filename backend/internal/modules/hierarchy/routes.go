package hierarchy

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Register 注册片区-道路层级路由，并返回 service 供其他模块装配依赖。
func Register(router fiber.Router, db *gorm.DB) *Service {
	svc := NewService(NewRepository(db))
	handler := NewHandler(svc)

	group := router.Group("/hierarchy")
	group.Get("/tree", handler.Tree)
	group.Get("/change-logs", handler.ChangeLogs)

	group.Post("/districts", handler.CreateDistrict)
	group.Put("/districts/:id/rename", handler.RenameDistrict)
	group.Delete("/districts/:id", handler.DeleteDistrict)

	group.Post("/roads", handler.CreateRoad)
	group.Put("/roads/:id/rename", handler.RenameRoad)
	group.Put("/roads/:id/move", handler.MoveRoad)
	group.Post("/roads/batch-move", handler.BatchMoveRoads)
	group.Delete("/roads/:id", handler.DeleteRoad)

	return svc
}
