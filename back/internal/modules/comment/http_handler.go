package comment

import (
	"net/http"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/response"
	"simple_tiktok/internal/pkg/type_convert"
	"strconv"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(commentService *Service) *HTTPHandler {
	return &HTTPHandler{
		service: commentService,
	}
}

func (h *HTTPHandler) Create(c *gin.Context) {
	var commentReq req.CommentReq
	if err := c.ShouldBind(&commentReq); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	commentRes, err := h.service.CreateComment(userID, commentReq)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, commentRes)
}

func (h *HTTPHandler) Delete(c *gin.Context) {
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	commentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	err = h.service.DeleteComment(userID, commentID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, nil)
}

func (h *HTTPHandler) List(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("videoId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	userID, err := type_convert.AnyToUint64(c.MustGet(middleware.UserCtx))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	commentList, err := h.service.ListByVideoId(videoID, userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, commentList)
}
