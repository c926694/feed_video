package video

import (
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/type_convert"
	"simple_tiktok/internal/platform/httpx"
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
	play, err := c.FormFile("play")
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请选择要上传的视频文件"))
		return
	}
	cover, err := c.FormFile("cover")
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请选择视频封面"))
		return
	}
	createVideoReq := req.UploadVideoReq{
		Title:       c.PostForm("title"),
		Description: c.PostForm("description"),
		Play:        play,
		Cover:       cover,
	}
	videoRes, err := h.service.CreateVideo(
		createVideoReq, c.MustGet(middleware.UserCtx).(uint64), c.MustGet(middleware.UserNickName).(string))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, videoRes)
}

func (h *HTTPHandler) GetVideoInfo(c *gin.Context) {
	rawID := c.Param("id")
	if rawID == "me" {
		limit, err := strconv.ParseUint(c.DefaultQuery("limit", "60"), 10, 64)
		if err != nil {
			httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
			return
		}
		userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
		if err != nil {
			httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
			return
		}
		videoInfoResList, err := h.service.GetMyVideos(userID, limit)
		if err != nil {
			httpx.Fail(c, err)
			return
		}
		httpx.OK(c, videoInfoResList)
		return
	}

	videoID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
		return
	}
	videoInfoRes, err := h.service.GetVideoInfo(videoID, userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, videoInfoRes)
}

func (h *HTTPHandler) GetMyVideos(c *gin.Context) {
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "60"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
		return
	}
	videoInfoResList, err := h.service.GetMyVideos(userID, limit)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, videoInfoResList)
}

func (h *HTTPHandler) DeleteVideos(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
		return
	}
	err = h.service.DeleteVideo(videoID, userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, nil)
}
