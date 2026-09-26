package user

import (
	"mime/multipart"

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

func (h *Controller) Register(c *gin.Context) {
	var registerReq RegisterReq
	if err := c.ShouldBind(&registerReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	userID, err := h.Logic.Register(c.Request.Context(), registerReq)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, userID)
}

func (h *Controller) Login(c *gin.Context) {
	var loginReq LoginReq
	if err := c.ShouldBind(&loginReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	pair, err := h.Logic.Login(c.Request.Context(), loginReq.Username, loginReq.Password)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, pair)
}

// Refresh 用 refresh token 换一对新令牌，前端在 access 过期时调用
func (h *Controller) Refresh(c *gin.Context) {
	var refreshReq RefreshReq
	if err := c.ShouldBind(&refreshReq); err != nil || refreshReq.RefreshToken == "" {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "缺少 refresh_token"))
		return
	}
	pair, err := h.Logic.Refresh(c.Request.Context(), refreshReq.RefreshToken)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, pair)
}

func (h *Controller) GetUserInfo(c *gin.Context) {
	info, err := h.Logic.GetInfo(c.Request.Context(), auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, info)
}

// Logout 结束当前这条会话，其他设备不受影响
func (h *Controller) Logout(c *gin.Context) {
	var logoutReq RefreshReq
	if err := c.ShouldBind(&logoutReq); err != nil || logoutReq.RefreshToken == "" {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "缺少 refresh_token"))
		return
	}
	if err := h.Logic.Logout(c.Request.Context(), logoutReq.RefreshToken); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, nil)
}

func (h *Controller) UpdateProfile(c *gin.Context) {
	var profileReq UpdateProfileReq
	_ = c.ShouldBind(&profileReq)
	var avatar *multipart.FileHeader
	file, err := c.FormFile("avatar")
	if err == nil {
		avatar = file
	}
	info, err := h.Logic.UpdateProfile(c.Request.Context(), auth.UserID(c), profileReq.Nickname, avatar)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, info)
}
