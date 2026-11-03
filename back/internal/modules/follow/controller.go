package follow

import (
	"context"
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

// ListFollowing 某个人关注了谁
func (h *Controller) ListFollowing(c *gin.Context) {
	res, err := h.listUsers(c, h.Logic.ListFollowing)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// ListFollowers 谁关注了这个人
func (h *Controller) ListFollowers(c *gin.Context) {
	res, err := h.listUsers(c, h.Logic.ListFollowers)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// listUsers 两个列表接口共用的参数解析
func (h *Controller) listUsers(c *gin.Context, load func(ctx context.Context, userID uint64, viewerID uint64, lastID uint64, limit uint64) (*FollowListRes, error)) (*FollowListRes, error) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return nil, httpx.New(httpx.CodeBadRequest, "用户 ID 不合法")
	}
	lastID, err := strconv.ParseUint(c.DefaultQuery("last_id", "0"), 10, 64)
	if err != nil {
		return nil, httpx.New(httpx.CodeBadRequest, "last_id 参数不合法")
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "0"), 10, 64)
	if err != nil {
		return nil, httpx.New(httpx.CodeBadRequest, "limit 参数不合法")
	}
	return load(c.Request.Context(), userID, auth.UserID(c), lastID, limit)
}
