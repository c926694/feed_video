package feed

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

func (h *Controller) GetFeedVideos(c *gin.Context) {
	lastCreatedAt, err := strconv.ParseInt(c.DefaultQuery("last_created_at", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_created_at 参数不合法"))
		return
	}
	lastID, err := strconv.ParseUint(c.DefaultQuery("last_id", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_id 参数不合法"))
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	list, nextCreatedAt, nextID, err := h.Logic.GetFeedVideos(c.Request.Context(), limit, lastCreatedAt, lastID, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, FeedRes{FeedVideoList: list, LastCreatedAt: nextCreatedAt, LastId: nextID})
}

func (h *Controller) GetFeedHotVideos(c *gin.Context) {
	interval, err := strconv.Atoi(c.DefaultQuery("interval", "60"))
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "interval 参数不合法"))
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	offset, err := strconv.ParseUint(c.DefaultQuery("offset", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "offset 参数不合法"))
		return
	}
	list, nextOffset, hasMore, err := h.Logic.GetHotVideos(c.Request.Context(), limit, offset, interval, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, HotFeedRes{
		FeedVideoList: list,
		NextOffset:    nextOffset,
		HasMore:       hasMore,
		Interval:      interval,
	})
}

func (h *Controller) GetFollowFeedVideos(c *gin.Context) {
	lastCreatedAt, err := strconv.ParseInt(c.DefaultQuery("last_created_at", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_created_at 参数不合法"))
		return
	}
	lastID, err := strconv.ParseUint(c.DefaultQuery("last_id", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_id 参数不合法"))
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	list, nextCreatedAt, nextID, err := h.Logic.GetFollowFeedVideos(c.Request.Context(), limit, lastCreatedAt, lastID, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, FeedRes{FeedVideoList: list, LastCreatedAt: nextCreatedAt, LastId: nextID})
}
