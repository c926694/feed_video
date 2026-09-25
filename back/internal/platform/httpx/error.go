package httpx

// AppError 业务错误，Code 表明错误类别，Msg 直接返回给前端展示
type AppError struct {
	Code int
	Msg  string
}

func (e *AppError) Error() string {
	return e.Msg
}

// New 构造业务错误
func New(code int, msg string) *AppError {
	return &AppError{Code: code, Msg: msg}
}
