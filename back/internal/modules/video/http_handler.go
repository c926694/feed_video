package video

import (
	"net/http"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/response"
	"simple_tiktok/internal/pkg/type_convert"
	"strconv"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(videoService *Service) *HTTPHandler {
	return &HTTPHandler{
		service: videoService,
	}
}

func (h *HTTPHandler) CreateVideo(c *gin.Context) {
	var createVideoReq req.UploadVideoReq
	play, _ := c.FormFile("play")
	cover, _ := c.FormFile("cover")
	title := c.PostForm("title")
	description := c.PostForm("description")
	createVideoReq = req.UploadVideoReq{
		Title:       title,
		Description: description,
		Play:        play,
		Cover:       cover,
	}
	videoRes, err := h.service.CreateVideo(
		createVideoReq, c.MustGet(middleware.UserCtx).(uint64), c.MustGet(middleware.UserNickName).(string))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, videoRes)
}

func (h *HTTPHandler) GetVideoInfo(c *gin.Context) {
	rawID := c.Param("id")
	if rawID == "me" {
		limit, err := strconv.ParseUint(c.DefaultQuery("limit", "60"), 10, 64)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "invalid limit")
			return
		}
		userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "invalid user")
			return
		}
		videoInfoResList, err := h.service.GetMyVideos(userID, limit)
		if err != nil {
			response.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, videoInfoResList)
		return
	}

	videoID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid video id")
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "invalid user")
		return
	}
	videoInfoRes, err := h.service.GetVideoInfo(videoID, userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, videoInfoRes)
}

func (h *HTTPHandler) GetMyVideos(c *gin.Context) {
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "60"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	videoInfoResList, err := h.service.GetMyVideos(userID, limit)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, videoInfoResList)
}

func (h *HTTPHandler) DeleteVideos(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	err = h.service.DeleteVideo(videoID, userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, nil)
}
