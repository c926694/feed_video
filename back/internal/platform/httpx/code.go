package httpx

import "net/http"

// 通用错误码，按 HTTP 语义分类，具体原因由 msg 表达
const (
	CodeOK               = 0
	CodeBadRequest       = 40000 // 请求参数错误
	CodeCredential       = 40001 // 用户名或密码错误
	CodeUnauthorized     = 40100 // 未认证或登录已失效
	CodeForbidden        = 40300 // 无权限操作该资源
	CodeNotFound         = 40400 // 资源不存在
	CodeMethodNotAllowed = 40500 // 请求方法不允许
	CodeConflict         = 40900 // 资源冲突
	CodeInternal         = 50000 // 服务器内部错误
)

// statusByCode 维护错误码到 HTTP 状态码的对应关系
var statusByCode = map[int]int{
	CodeBadRequest:       http.StatusBadRequest,
	CodeCredential:       http.StatusBadRequest,
	CodeUnauthorized:     http.StatusUnauthorized,
	CodeForbidden:        http.StatusForbidden,
	CodeNotFound:         http.StatusNotFound,
	CodeMethodNotAllowed: http.StatusMethodNotAllowed,
	CodeConflict:         http.StatusConflict,
	CodeInternal:         http.StatusInternalServerError,
}

// StatusOf 返回错误码对应的 HTTP 状态码，未登记的错误码按服务器内部错误处理
func StatusOf(code int) int {
	if status, ok := statusByCode[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}
