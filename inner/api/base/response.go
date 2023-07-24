package base

const (
	CodeClientNotExists = 1000 + iota
)

type Response struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Success bool        `json:"success"`
}
type Page struct {
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
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

func WithError(err error) *Response {
	// Fixme: use specific error code
	return &Response{
		Code:    500,
		Msg:     err.Error(),
		Data:    nil,
		Success: false,
	}
}
