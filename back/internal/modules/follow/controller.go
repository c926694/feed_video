package follow

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
)

type Controller struct {
	Logic *Logic
}

func NewController(Logic *Logic) *Controller {
	return &Controller{Logic: Logic}
}

func (h *Controller) Follow(c *gin.Context) {
	targetUserID, err := strconv.ParseUint(c.Param("follower"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "用户 ID 不合法"))
		return
	}
	followed, err := h.Logic.SwitchFollow(c.Request.Context(), targetUserID, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, SwitchRes{Following: targetUserID, IsFollow: followed})
}
