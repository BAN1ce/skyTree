package response

type Base struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Success bool        `json:"success"`
}

func WithData(data interface{}) *Base {
	return &Base{
		Code:    0,
		Msg:     "",
		Data:    data,
		Success: true,
	}
}

func WithSuccess() *Base {
	return &Base{
		Code:    0,
		Msg:     "",
		Data:    nil,
		Success: true,
	}
}

func WithCode(code int) *Base {
	return &Base{
		Code:    code,
		Msg:     "",
		Data:    nil,
		Success: false,
	}
}
