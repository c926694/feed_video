package user

import (
	"mime/multipart"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(userService *Service) *HTTPHandler {
	return &HTTPHandler{
		service: userService,
	}
}

func (h *HTTPHandler) Register(c *gin.Context) {
	var registerReq req.RegisterReq
	if err := c.ShouldBind(&registerReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	userID, err := h.service.Register(c, registerReq)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, userID)
}

func (h *HTTPHandler) Login(c *gin.Context) {
	var loginReq req.LoginReq
	if err := c.ShouldBind(&loginReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	token, err := h.service.Login(loginReq.Username, loginReq.Password)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, token)
}

func (h *HTTPHandler) GetUserInfo(c *gin.Context) {
	userID := c.MustGet(middleware.UserCtx).(uint64)
	userInfoRes, err := h.service.GetUserInfo(userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, userInfoRes)
}

func (h *HTTPHandler) Logout(c *gin.Context) {
	userID := c.MustGet(middleware.UserCtx).(uint64)
	err := h.service.Logout(userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, nil)
}

func (h *HTTPHandler) UpdateProfile(c *gin.Context) {
	var profileReq req.UpdateUserProfileReq
	_ = c.ShouldBind(&profileReq)
	var avatar *multipart.FileHeader
	file, err := c.FormFile("avatar")
	if err == nil {
		avatar = file
	}
	userID := c.MustGet(middleware.UserCtx).(uint64)
	userInfo, err := h.service.UpdateProfile(userID, profileReq.Nickname, avatar)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, userInfo)
}
