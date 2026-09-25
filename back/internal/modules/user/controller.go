package user

import (
	"mime/multipart"

	"github.com/gin-gonic/gin"

	"simple_tiktok/internal/platform/auth"
	"simple_tiktok/internal/platform/httpx"
)

type Controller struct {
	model *Model
}

func NewController(model *Model) *Controller {
	return &Controller{model: model}
}

func (h *Controller) Register(c *gin.Context) {
	var registerReq RegisterReq
	if err := c.ShouldBind(&registerReq); err != nil {
		httpx.Fail(c, httpx.New(httpx.CodeBadRequest, "请求参数格式错误"))
		return
	}
	userID, err := h.model.Register(c.Request.Context(), registerReq)
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
	token, err := h.model.Login(c.Request.Context(), loginReq.Username, loginReq.Password)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, token)
}

func (h *Controller) GetUserInfo(c *gin.Context) {
	info, err := h.model.GetInfo(c.Request.Context(), auth.UserID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, info)
}

func (h *Controller) Logout(c *gin.Context) {
	if err := h.model.Logout(c.Request.Context(), auth.UserID(c)); err != nil {
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
	info, err := h.model.UpdateProfile(c.Request.Context(), auth.UserID(c), profileReq.Nickname, avatar)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, info)
}
