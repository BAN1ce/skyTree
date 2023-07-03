package api

type Response struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Success bool        `json:"success"`
}

func WithData(data interface{}) *Response {
	return &Response{
		Code:    0,
		Msg:     "",
		Data:    data,
		Success: true,
	}
}

func WithSuccess() *Response {
	return &Response{
		Code:    0,
		Msg:     "",
		Data:    nil,
		Success: true,
	}
}

func WithCode(code int) *Response {
	return &Response{
		Code:    code,
		Msg:     "",
		Data:    nil,
		Success: false,
	}
}
