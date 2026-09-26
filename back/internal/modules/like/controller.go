package like

import (
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

func (h *Controller) LikeVideo(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	liked, err := h.Logic.SwitchVideoLike(c.Request.Context(), videoID, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, VideoLikeRes{VideoId: videoID, IsLiked: liked})
}

func (h *Controller) LikeComment(c *gin.Context) {
	commentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "评论 ID 不合法"))
		return
	}
	liked, err := h.Logic.SwitchCommentLike(c.Request.Context(), commentID, auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, CommentLikeRes{CommentId: commentID, IsLiked: liked})
}
