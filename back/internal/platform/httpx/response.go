package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 所有 HTTP 接口的统一返回结构
type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// OK 成功响应
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: CodeOK, Msg: "ok", Data: data})
}

// Fail 失败响应，业务错误按自身的错误码输出，其余错误按服务器内部错误输出并记录日志
func Fail(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		c.JSON(StatusOf(appErr.Code), Response{Code: appErr.Code, Msg: appErr.Msg, Data: nil})
		return
	}
	slog.Error("未识别的错误", "path", c.FullPath(), "error", err)
	c.JSON(http.StatusInternalServerError, Response{Code: CodeInternal, Msg: "服务器内部错误", Data: nil})
}
