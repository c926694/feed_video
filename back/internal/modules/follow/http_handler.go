package follow

import (
	"net/http"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/response"
	"simple_tiktok/internal/pkg/type_convert"
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
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	currentUserID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.service.Follow(targetUserID, currentUserID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, result)
}
