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
	liked, err := h.Logic.SetVideoLike(c.Request.Context(), videoID, auth.UserID(c), true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, VideoLikeRes{VideoId: videoID, IsLiked: liked})
}

func (h *Controller) UnlikeVideo(c *gin.Context) {
	videoID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "视频 ID 不合法"))
		return
	}
	liked, err := h.Logic.SetVideoLike(c.Request.Context(), videoID, auth.UserID(c), false)
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
	liked, err := h.Logic.SetCommentLike(c.Request.Context(), commentID, auth.UserID(c), true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, CommentLikeRes{CommentId: commentID, IsLiked: liked})
}

func (h *Controller) UnlikeComment(c *gin.Context) {
	commentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "评论 ID 不合法"))
		return
	}
	liked, err := h.Logic.SetCommentLike(c.Request.Context(), commentID, auth.UserID(c), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, CommentLikeRes{CommentId: commentID, IsLiked: liked})
}

func (h *Controller) ListMine(c *gin.Context) {
	lastCreatedAt, err := strconv.ParseInt(c.DefaultQuery("last_created_at", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_created_at 参数不合法"))
		return
	}
	lastID, err := strconv.ParseUint(c.DefaultQuery("last_id", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "last_id 参数不合法"))
		return
	}
	limit, err := strconv.ParseUint(c.DefaultQuery("limit", "0"), 10, 64)
	if err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "limit 参数不合法"))
		return
	}
	list, err := h.Logic.ListMyLikedVideos(c.Request.Context(), auth.UserID(c), lastCreatedAt, lastID, limit)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, list)
}
