package comment

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
)

type Controller struct {
	model *Model
}

func NewController(model *Model) *Controller {
	return &Controller{model: model}
}

func (h *Controller) Create(c *gin.Context) {
	var createReq CreateReq
	if err := c.ShouldBind(&createReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	result, err := h.model.Create(c.Request.Context(), auth.UserID(c), createReq)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Controller) Delete(c *gin.Context) {
	commentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "评论 ID 不合法"))
		return
	}
	if err = h.model.Delete(c.Request.Context(), auth.UserID(c), commentID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, nil)
}

func (h *Controller) List(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("videoId"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	list, err := h.model.ListByVideo(c.Request.Context(), videoID, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, list)
}
