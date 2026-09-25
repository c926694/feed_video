package comment

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
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
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	userID := auth.UserID(c)
	commentRes, err := h.service.CreateComment(userID, commentReq)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, commentRes)
}

func (h *HTTPHandler) Delete(c *gin.Context) {
	userID := auth.UserID(c)
	commentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "评论 ID 不合法"))
		return
	}
	err = h.service.DeleteComment(userID, commentID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, nil)
}

func (h *HTTPHandler) List(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("videoId"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	userID := auth.UserID(c)
	commentList, err := h.service.ListByVideoId(videoID, userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, commentList)
}
