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
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "用户 ID 不合法"))
		return
	}
	followed, err := h.Logic.SetFollow(c.Request.Context(), targetUserID, auth.UserID(c), true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, FollowRes{Following: targetUserID, IsFollow: followed})
}

func (h *Controller) Unfollow(c *gin.Context) {
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "用户 ID 不合法"))
		return
	}
	followed, err := h.Logic.SetFollow(c.Request.Context(), targetUserID, auth.UserID(c), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, FollowRes{Following: targetUserID, IsFollow: followed})
}
