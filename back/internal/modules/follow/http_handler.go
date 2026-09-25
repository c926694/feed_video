package follow

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(followService *Service) *HTTPHandler {
	return &HTTPHandler{
		service: followService,
	}
}

func (h *HTTPHandler) Follow(c *gin.Context) {
	targetUserID, err := strconv.ParseUint(c.Param("follower"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "用户 ID 不合法"))
		return
	}
	currentUserID := auth.UserID(c)
	result, err := h.service.Follow(c.Request.Context(), targetUserID, currentUserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}
