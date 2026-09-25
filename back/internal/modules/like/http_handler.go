package like

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
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
	userID := auth.UserID(c)
	result, err := h.service.LikeVideo(c.Request.Context(), targetID, userID)
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
	userID := auth.UserID(c)
	result, err := h.service.LikeComment(c.Request.Context(), commentID, userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}
