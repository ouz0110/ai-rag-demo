package openaierr

// InterruptErr 高危操作拦截/中断专用错误结构体
type InterruptErr struct {
	Message string
}

func NewInterruptErr(msg string) InterruptErr {
	return InterruptErr{Message: msg}
}

func (e InterruptErr) Error() string {
	return e.Message
}
