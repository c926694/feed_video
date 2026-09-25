package like

import (
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/type_convert"
	"simple_tiktok/internal/platform/httpx"
	"strconv"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(likeService *Service) *HTTPHandler {
	return &HTTPHandler{
		service: likeService,
	}
}

func (h *HTTPHandler) LikeVideo(c *gin.Context) {
	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
		return
	}
	result, err := h.service.LikeVideo(targetID, userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *HTTPHandler) LikeComment(c *gin.Context) {
	commentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "评论 ID 不合法"))
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
		return
	}
	result, err := h.service.LikeComment(commentID, userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}
