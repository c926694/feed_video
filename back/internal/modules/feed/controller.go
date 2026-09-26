package feed

import (
	"math"
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
	lastScore, err := strconv.ParseFloat(c.DefaultQuery("last_score", strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64)), 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_score 参数不合法"))
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	list, nextScore, err := h.Logic.GetFeedVideos(c.Request.Context(), limit, lastScore, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, FeedRes{FeedVideoList: list, LastScore: nextScore})
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
	lastScore, err := strconv.ParseFloat(c.DefaultQuery("last_score", strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64)), 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_score 参数不合法"))
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	list, nextScore, err := h.Logic.GetFollowFeedVideos(c.Request.Context(), limit, lastScore, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, FeedRes{FeedVideoList: list, LastScore: nextScore})
}
