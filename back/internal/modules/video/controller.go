package video

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

func (h *Controller) CreateVideo(c *gin.Context) {
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
	createReq := CreateReq{
		Title:       c.PostForm("title"),
		Description: c.PostForm("description"),
		Play:        play,
		Cover:       cover,
	}
	result, err := h.Logic.CreateVideo(c.Request.Context(), createReq, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Controller) GetVideoInfo(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	info, err := h.Logic.GetVideoInfo(c.Request.Context(), videoID, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, info)
}

func (h *Controller) GetMyVideos(c *gin.Context) {
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "60"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	list, err := h.Logic.GetMyVideos(c.Request.Context(), auth.UserID(c), limit)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, list)
}

func (h *Controller) DeleteVideos(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	if err = h.Logic.DeleteVideo(c.Request.Context(), videoID, auth.UserID(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, nil)
}
