package like

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

func NewHTTPHandler(likeService *Service) *HTTPHandler {
	return &HTTPHandler{
		service: likeService,
	}
}

func (h *HTTPHandler) LikeVideo(c *gin.Context) {
	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	result, err := h.service.LikeVideo(targetID, userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *HTTPHandler) LikeComment(c *gin.Context) {
	commentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	result, err := h.service.LikeComment(commentID, userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}
