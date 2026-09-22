package hierarchy

import (
	"github.com/gofiber/fiber/v2"

	"github.com/drainage/desilting/internal/httpx"
)

// Handler 片区-道路层级 HTTP 接口。
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
	d, err := h.svc.CreateDistrict(c.UserContext(), req)
	if err != nil {
		return err
	}
	return httpx.Created(c, d)
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
	d, err := h.svc.RenameDistrict(c.UserContext(), id, req)
	if err != nil {
		return err
	}
	return httpx.Message(c, "片区已改名", d)
}

// DeleteDistrict 删除片区。
func (h *Handler) DeleteDistrict(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "片区")
	if err != nil {
		return err
	}
	var req DeleteRequest
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

// MoveRoad 单条道路归属调整。
func (h *Handler) MoveRoad(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "道路")
	if err != nil {
		return err
	}
	var req MoveRoadRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.svc.MoveRoad(c.UserContext(), id, req.DistrictID, req); err != nil {
		return err
	}
	return httpx.Message(c, "道路归属已调整", fiber.Map{"id": id, "districtId": req.DistrictID})
}

// BatchMoveRoads 批量调整道路归属。
func (h *Handler) BatchMoveRoads(c *fiber.Ctx) error {
	var req BatchMoveRoadsRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.svc.BatchMoveRoads(c.UserContext(), req); err != nil {
		return err
	}
	return httpx.Message(c, "道路归属已批量调整", fiber.Map{"count": len(req.Items)})
}

// DeleteRoad 删除道路。
func (h *Handler) DeleteRoad(c *fiber.Ctx) error {
	id, err := httpx.PathID(c, "id", "道路")
	if err != nil {
		return err
	}
	var req DeleteRequest
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

// ChangeLogs 层级变更记录。
func (h *Handler) ChangeLogs(c *fiber.Ctx) error {
	query := ParseChangeLogQuery(c)
	items, total, err := h.svc.ChangeLogs(c.UserContext(), query)
	if err != nil {
		return err
	}
	return httpx.OKPage(c, items, total, query.Page.Page, query.Page.PageSize)
}
