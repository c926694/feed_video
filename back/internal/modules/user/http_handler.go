package user

import (
	"errors"
	"mime/multipart"
	"net/http"
	"simple_tiktok/internal/dto/req"
	"simple_tiktok/internal/middleware"
	"simple_tiktok/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	userID, err := h.service.Register(c, registerReq)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			response.Fail(c, http.StatusConflict, errors.New("user already exists").Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, "注册失败")
		return
	}
	response.OK(c, userID)
}

func (h *HTTPHandler) Login(c *gin.Context) {
	var loginReq req.LoginReq
	if err := c.ShouldBind(&loginReq); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	token, err := h.service.Login(loginReq.Username, loginReq.Password)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, token)
}

func (h *HTTPHandler) GetUserInfo(c *gin.Context) {
	userID := c.MustGet(middleware.UserCtx).(uint64)
	userInfoRes, err := h.service.GetUserInfo(userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取个人信息失败")
		return
	}
	response.OK(c, userInfoRes)
}

func (h *HTTPHandler) Logout(c *gin.Context) {
	userID := c.MustGet(middleware.UserCtx).(uint64)
	err := h.service.Logout(userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "退出失败")
		return
	}
	response.OK(c, nil)
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
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, userInfo)
}
