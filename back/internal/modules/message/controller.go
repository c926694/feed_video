package message

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

// ListMine 通知列表，双字段游标分页
func (h *Controller) ListMine(c *gin.Context) {
	lastAt, err := strconv.ParseInt(c.DefaultQuery("last_created_at", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_created_at 参数不合法"))
		return
	}
	lastID, err := strconv.ParseUint(c.DefaultQuery("last_id", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_id 参数不合法"))
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	res, err := h.Logic.ListMine(c.Request.Context(), auth.UserID(c), lastAt, lastID, limit)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, res)
}

// Unread 未读会话数
func (h *Controller) Unread(c *gin.Context) {
	count, err := h.Logic.Unread(c.Request.Context(), auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, UnreadRes{UnreadCount: count})
}

// MarkRead 标记已读，message_ids 为空表示全部
func (h *Controller) MarkRead(c *gin.Context) {
	var req ReadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		// 请求体为空按全部已读处理
		req.MessageIDs = nil
	}
	for _, id := range req.MessageIDs {
		if id == 0 {
			httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "message_ids 不合法"))
			return
		}
	}
	updated, err := h.Logic.MarkRead(c.Request.Context(), auth.UserID(c), req.MessageIDs)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, ReadRes{Updated: updated})
}
