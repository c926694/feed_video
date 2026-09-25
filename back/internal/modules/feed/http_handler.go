package feed

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
	"simple_tiktok/internal/service"
)

type HTTPHandler struct {
	service *service.FeedService
}

func NewHTTPHandler(feedService *service.FeedService) *HTTPHandler {
	return &HTTPHandler{
		service: feedService,
	}
}

func (h *HTTPHandler) GetFeedVideos(c *gin.Context) {
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
	videoInfoResList, nextScore, err := h.service.GetFeedVideos(limit, lastScore, constants.FeedVideoKey, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, &res.FeedVideoRes{
		FeedVideoList: videoInfoResList,
		LastScore:     nextScore,
	})
}

func (h *HTTPHandler) GetFeedHotVideos(c *gin.Context) {
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
	videoInfoResList, nextOffset, hasMore, err := h.service.GetFeedHotVideos(limit, offset, interval, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, &res.HotFeedVideoRes{
		FeedVideoList: videoInfoResList,
		NextOffset:    nextOffset,
		HasMore:       hasMore,
		Interval:      interval,
	})
}

func (h *HTTPHandler) GetFollowFeedVideos(c *gin.Context) {
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
	videoInfoResList, nextScore, err := h.service.GetFollowFeedVideos(limit, lastScore, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, &res.FeedVideoRes{
		FeedVideoList: videoInfoResList,
		LastScore:     nextScore,
	})
}
