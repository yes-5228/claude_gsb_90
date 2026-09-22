package httpx

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// trimSpace 是 strings.TrimSpace 的简写，便于在参数解析里连续调用。
func trimSpace(value string) string {
	return strings.TrimSpace(value)
}

// UintIDsQuery 解析逗号分隔或重复出现的多值 ID 查询参数（用于层级多选筛选）。
//
// 支持 districtIds=1,2,3 与 districtIds=1&districtIds=2 两种形式，
// 自动去重、去 0、去非法值。
func UintIDsQuery(c *fiber.Ctx, key string) []uint {
	values := c.Request().URI().QueryArgs().PeekMulti(key)
	seen := make(map[uint]struct{})
	out := make([]uint, 0)
	for _, raw := range values {
		for _, item := range strings.Split(string(raw), ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			id, err := strconv.ParseUint(item, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			if _, ok := seen[uint(id)]; ok {
				continue
			}
			seen[uint(id)] = struct{}{}
			out = append(out, uint(id))
		}
	}
	return out
}
