package kafka

import "errors"

// permanentError 标记重试也不会成功的错误
type permanentError struct {
	err error
}

func (e *permanentError) Error() string {
	return e.err.Error()
}

func (e *permanentError) Unwrap() error {
	return e.err
}

// Permanent 标记一个重试也不会成功的错误。处理函数遇到这类错误会直接跳过，
// 例如消息体无法解析、事件字段非法、目标资源已经不存在。
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent 判断错误是否被标记为不可重试
func IsPermanent(err error) bool {
	var target *permanentError
	return errors.As(err, &target)
}
