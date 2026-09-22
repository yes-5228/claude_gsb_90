package hierarchy

import (
	"github.com/gofiber/fiber/v2"

	"github.com/drainage/desilting/internal/httpx"
)

// Handler 层级 HTTP 接口。
type Handler struct {
	svc *Service
}

// NewHandler 构造处理器。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Tree 层级树。
func (h *Handler) Tree(c *fiber.Ctx) error {
	tree, err := h.svc.Tree(c.UserContext())
	if err != nil {
		return err
	}
	return httpx.OK(c, tree)
}

// CreateDistrict 新增片区。
func (h *Handler) CreateDistrict(c *fiber.Ctx) error {
	var req SaveDistrictRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	district, err := h.svc.CreateDistrict(c.UserContext(), req)
	if err != nil {
		return err
	}
	return httpx.Created(c, district)
}

// RenameDistrict 片区改名。
func (h *Handler) RenameDistrict(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "片区")
	if err != nil {
		return err
	}
	var req RenameDistrictRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	district, err := h.svc.RenameDistrict(c.UserContext(), id, req)
	if err != nil {
		return err
	}
	return httpx.Message(c, "片区已改名", district)
}

// DeleteDistrict 删除片区。
func (h *Handler) DeleteDistrict(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "片区")
	if err != nil {
		return err
	}
	var req OperatorRequest
	if len(c.Body()) > 0 {
		if err := httpx.BindAndValidate(c, &req); err != nil {
			return err
		}
	}
	if err := h.svc.DeleteDistrict(c.UserContext(), id, req); err != nil {
		return err
	}
	return httpx.Message(c, "片区已删除", fiber.Map{"id": id})
}

// DistrictRefs 片区引用计数。
func (h *Handler) DistrictRefs(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "片区")
	if err != nil {
		return err
	}
	refs, err := h.svc.DistrictRefCounts(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, refs)
}

// CreateRoad 新增道路。
func (h *Handler) CreateRoad(c *fiber.Ctx) error {
	var req SaveRoadRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	road, err := h.svc.CreateRoad(c.UserContext(), req)
	if err != nil {
		return err
	}
	return httpx.Created(c, road)
}

// RenameRoad 道路改名。
func (h *Handler) RenameRoad(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "道路")
	if err != nil {
		return err
	}
	var req RenameRoadRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	road, err := h.svc.RenameRoad(c.UserContext(), id, req)
	if err != nil {
		return err
	}
	return httpx.Message(c, "道路已改名", road)
}

// MoveRoad 道路归属调整。
func (h *Handler) MoveRoad(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "道路")
	if err != nil {
		return err
	}
	var req MoveRoadRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	road, err := h.svc.MoveRoad(c.UserContext(), id, req)
	if err != nil {
		return err
	}
	return httpx.Message(c, "道路归属已调整", road)
}

// DeleteRoad 删除道路。
func (h *Handler) DeleteRoad(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "道路")
	if err != nil {
		return err
	}
	var req OperatorRequest
	if len(c.Body()) > 0 {
		if err := httpx.BindAndValidate(c, &req); err != nil {
			return err
		}
	}
	if err := h.svc.DeleteRoad(c.UserContext(), id, req); err != nil {
		return err
	}
	return httpx.Message(c, "道路已删除", fiber.Map{"id": id})
}

// RoadRefs 道路引用计数。
func (h *Handler) RoadRefs(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "道路")
	if err != nil {
		return err
	}
	refs, err := h.svc.RoadRefCounts(c.UserContext(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, refs)
}

// Batch 批量层级调整。
func (h *Handler) Batch(c *fiber.Ctx) error {
	var req BatchRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	result, err := h.svc.Batch(c.UserContext(), req)
	if err != nil {
		return err
	}
	return httpx.Message(c, "层级调整已全部生效", result)
}

// Logs 变更日志。
func (h *Handler) Logs(c *fiber.Ctx) error {
	query := ParseLogListQuery(c)
	logs, total, err := h.svc.Logs(c.UserContext(), query)
	if err != nil {
		return err
	}
	return httpx.OKPage(c, logs, total, query.Page.Page, query.Page.PageSize)
}
