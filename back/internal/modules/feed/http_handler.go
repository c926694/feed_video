package feed

import (
	"math"
	"net/http"
	"simple_tiktok/internal/dto/res"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/constants"
	"simple_tiktok/internal/pkg/response"
	"simple_tiktok/internal/pkg/type_convert"
	"simple_tiktok/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
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
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	userId, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	videoInfoResList, nextScore, err := h.service.GetFeedVideos(limit, lastScore, constants.FeedVideoKey, userId)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, &res.FeedVideoRes{
		FeedVideoList: videoInfoResList,
		LastScore:     nextScore,
	})
}

func (h *HTTPHandler) GetFeedHotVideos(c *gin.Context) {
	interval, err := strconv.Atoi(c.DefaultQuery("interval", "60"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid interval")
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid limit")
		return
	}
	offset, err := strconv.ParseUint(c.DefaultQuery("offset", "0"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid offset")
		return
	}
	userId, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	videoInfoResList, nextOffset, hasMore, err := h.service.GetFeedHotVideos(limit, offset, interval, userId)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, &res.HotFeedVideoRes{
		FeedVideoList: videoInfoResList,
		NextOffset:    nextOffset,
		HasMore:       hasMore,
		Interval:      interval,
	})
}

func (h *HTTPHandler) GetFollowFeedVideos(c *gin.Context) {
	lastScore, err := strconv.ParseFloat(c.DefaultQuery("last_score", strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64)), 64)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "3"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	userId, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	videoInfoResList, nextScore, err := h.service.GetFollowFeedVideos(limit, lastScore, userId)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, &res.FeedVideoRes{
		FeedVideoList: videoInfoResList,
		LastScore:     nextScore,
	})
}
