package follow

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
	currentUserID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeUnauthorized, "登录状态无效，请重新登录"))
		return
	}
	result, err := h.service.Follow(targetUserID, currentUserID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}
