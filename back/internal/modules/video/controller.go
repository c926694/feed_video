package video

import (
	"errors"
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

// UploadCredential 签发直传凭证并生成存储路径
func (h *Controller) UploadCredential(c *gin.Context) {
	var credentialReq CredentialReq
	if err := c.ShouldBind(&credentialReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	result, err := h.Logic.UploadCredential(c.Request.Context(), auth.UserID(c), credentialReq)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

// CreateVideo 创建发布记录，文件由前端直传 OSS。重复提交返回冲突并带上已创建的记录。
func (h *Controller) CreateVideo(c *gin.Context) {
	var createReq CreateReq
	if err := c.ShouldBind(&createReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	result, err := h.Logic.CreateVideo(c.Request.Context(), createReq, auth.UserID(c))
	if err != nil {
		var appErr *httpx.AppError
		if errors.As(err, &appErr) && appErr.Code == httpx.CodeConflict {
			httpx.FailWithData(c, err, result)
			return
		}
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

// UpdateStatus 更新发布状态：published 发布完成、failed 标记失败、created 重试
func (h *Controller) UpdateStatus(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	var updateReq UpdateStatusReq
	if err = c.ShouldBind(&updateReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	result, err := h.Logic.UpdateStatus(c.Request.Context(), videoID, auth.UserID(c), updateReq.Status)
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
